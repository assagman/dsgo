package module

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/logging"
)

// ScoringFunction evaluates the quality of a prediction
type ScoringFunction func(inputs map[string]any, prediction *core.Prediction) (float64, error)

// BestOfN executes a module N times and returns the best result.
//
// Thread Safety: BestOfN is now completely thread-safe. When using WithParallel(true),
// the module automatically creates independent instances for parallel execution,
// eliminating data race concerns even with stateful modules like Predict, ChainOfThought,
// and ReAct that maintain internal History.
type BestOfN struct {
	Module      core.Module
	N           int
	Scorer      ScoringFunction
	Parallel    bool
	ReturnAll   bool
	MaxFailures int     // Maximum number of failures before giving up
	Threshold   float64 // Early-stop if score meets or exceeds this threshold
}

// BestOfNResult contains the results of BestOfN execution (deprecated - use Prediction.Completions)
type BestOfNResult struct {
	BestOutput   map[string]any
	BestScore    float64
	AllOutputs   []map[string]any
	AllScores    []float64
	FailureCount int
}

// NewBestOfN creates a new BestOfN module
func NewBestOfN(module core.Module, n int) *BestOfN {
	return &BestOfN{
		Module:      module,
		N:           n,
		Scorer:      nil, // Must be set by user
		Parallel:    false,
		ReturnAll:   false,
		MaxFailures: n / 2, // Allow up to half the attempts to fail
		Threshold:   0,     // No threshold by default
	}
}

// WithScorer sets the scoring function
func (b *BestOfN) WithScorer(scorer ScoringFunction) *BestOfN {
	b.Scorer = scorer
	return b
}

// WithParallel enables parallel execution.
func (b *BestOfN) WithParallel(parallel bool) *BestOfN {
	b.Parallel = parallel
	return b
}

// WithReturnAll enables returning all results, not just the best
func (b *BestOfN) WithReturnAll(returnAll bool) *BestOfN {
	b.ReturnAll = returnAll
	return b
}

// WithMaxFailures sets the maximum number of failures before giving up
func (b *BestOfN) WithMaxFailures(max int) *BestOfN {
	b.MaxFailures = max
	return b
}

// WithThreshold sets the early-stop threshold
func (b *BestOfN) WithThreshold(threshold float64) *BestOfN {
	b.Threshold = threshold
	return b
}

// GetSignature returns the module's signature
func (b *BestOfN) GetSignature() *core.Signature {
	return b.Module.GetSignature()
}

// Forward executes the module N times and returns the best result
func (b *BestOfN) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleBestOfN, b.Module.GetSignature().Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleBestOfN, time.Since(startTime), predErr)
	}()

	if b.Scorer == nil {
		predErr = fmt.Errorf("scorer function must be set")
		return nil, predErr
	}

	if b.N <= 0 {
		predErr = fmt.Errorf("n must be positive")
		return nil, predErr
	}

	if b.Parallel {
		var res *core.Prediction
		res, predErr = b.forwardParallel(ctx, inputs)
		return res, predErr
	}
	var res *core.Prediction
	res, predErr = b.forwardSequential(ctx, inputs)
	return res, predErr
}

func (b *BestOfN) forwardSequential(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	var allPredictions []*core.Prediction
	var bestPrediction *core.Prediction
	bestScore := -1.0
	failureCount := 0

	for i := 0; i < b.N; i++ {
		prediction, err := b.Module.Forward(ctx, inputs)
		if err != nil {
			failureCount++
			if failureCount > b.MaxFailures {
				return nil, fmt.Errorf("exceeded maximum failures (%d/%d): %w", failureCount, b.N, err)
			}
			continue
		}

		score, err := b.Scorer(inputs, prediction)
		if err != nil {
			failureCount++
			if failureCount > b.MaxFailures {
				return nil, fmt.Errorf("scoring failed (%d/%d): %w", failureCount, b.N, err)
			}
			continue
		}

		logging.GetLogger().Debug(ctx, "BestOfN attempt", map[string]any{
			"attempt": i + 1,
			"score":   score,
		})

		allPredictions = append(allPredictions, prediction)

		if bestPrediction == nil || score > bestScore {
			bestPrediction = prediction
			bestScore = score
		}

		// Early stop if threshold is met
		if b.Threshold > 0 && score >= b.Threshold {
			break
		}
	}

	if bestPrediction == nil {
		return nil, fmt.Errorf("all %d attempts failed", b.N)
	}

	// Set score on best prediction
	bestPrediction.Score = bestScore

	// If ReturnAll is enabled, add all completions
	if b.ReturnAll {
		var completions []map[string]any
		for _, pred := range allPredictions {
			completions = append(completions, pred.Outputs)
		}
		bestPrediction.Completions = completions
	}

	return bestPrediction, nil
}

func (b *BestOfN) forwardParallel(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	type result struct {
		prediction *core.Prediction
		score      float64
		err        error
	}

	results := make(chan result, b.N)
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(instanceIndex int) {
			defer wg.Done()

			// Create independent module instance for safety
			module := b.createIndependentModule()

			prediction, err := module.Forward(ctx, inputs)
			if err != nil {
				results <- result{err: err}
				return
			}

			score, err := b.Scorer(inputs, prediction)
			if err != nil {
				results <- result{err: err}
				return
			}

			results <- result{prediction: prediction, score: score}
		}(i)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allPredictions []*core.Prediction
	var bestPrediction *core.Prediction
	bestScore := -1.0
	failureCount := 0

	for res := range results {
		if res.err != nil {
			failureCount++
			continue
		}

		allPredictions = append(allPredictions, res.prediction)

		logging.GetLogger().Debug(ctx, "BestOfN attempt", map[string]any{
			"score": res.score,
		})

		if bestPrediction == nil || res.score > bestScore {
			bestPrediction = res.prediction
			bestScore = res.score
		}
	}

	if failureCount > b.MaxFailures {
		return nil, fmt.Errorf("exceeded maximum failures (%d/%d)", failureCount, b.N)
	}

	if bestPrediction == nil {
		return nil, fmt.Errorf("all %d attempts failed", b.N)
	}

	// Set score on best prediction
	bestPrediction.Score = bestScore

	// If ReturnAll is enabled, add all completions
	if b.ReturnAll {
		var completions []map[string]any
		for _, pred := range allPredictions {
			completions = append(completions, pred.Outputs)
		}
		bestPrediction.Completions = completions
	}

	return bestPrediction, nil
}

// DefaultScorer returns a simple length-based scorer
// This is a basic scorer that prefers longer outputs
func DefaultScorer() ScoringFunction {
	return func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		totalLength := 0
		for _, v := range prediction.Outputs {
			totalLength += len(fmt.Sprintf("%v", v))
		}
		return float64(totalLength), nil
	}
}

// ConfidenceScorer returns a scorer based on a confidence field
func ConfidenceScorer(field string) ScoringFunction {
	return func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		confidence, exists := prediction.Outputs[field]
		if !exists {
			return 0, fmt.Errorf("confidence field '%s' not found in outputs", field)
		}

		switch v := confidence.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			// Try to parse as float
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
				return 0, fmt.Errorf("cannot parse confidence as float: %v", v)
			}
			return f, nil
		default:
			return 0, fmt.Errorf("confidence field has unexpected type: %T", confidence)
		}
	}
}

// Clone creates an independent copy of BestOfN module
func (b *BestOfN) Clone() core.Module {
	cloned := &BestOfN{
		Module:      b.Module.Clone(),
		N:           b.N,
		Scorer:      b.Scorer,
		Parallel:    b.Parallel,
		ReturnAll:   b.ReturnAll,
		MaxFailures: b.MaxFailures,
		Threshold:   b.Threshold,
	}
	return cloned
}

// createIndependentModule creates an independent instance of wrapped module
func (b *BestOfN) createIndependentModule() core.Module {
	// All modules must implement Clone() by definition
	return b.Module.Clone()
}
