package module

import (
	"context"
	"fmt"
	"sync"

	"github.com/assagman/dsgo"
)

// ScoringFunction evaluates the quality of a prediction
type ScoringFunction func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error)

// BestOfN executes a module N times and returns the best result.
//
// IMPORTANT: When using WithParallel(true), ensure the module is stateless
// or provide N independent module instances. Modules that maintain internal
// state (e.g., History in Predict or ChainOfThought) will cause data races
// when shared across goroutines.
//
// Safe parallel usage patterns:
//   - Use stateless modules (no internal state mutation)
//   - Create N independent instances of stateful modules
//   - Use separate History instances for each parallel execution
//
// Example with independent instances:
//
//	modules := make([]dsgo.Module, n)
//	for i := 0; i < n; i++ {
//	    modules[i] = module.NewPredict(sig, lm) // Each has its own History
//	}
//	// Execute with BestOfN wrapping each independently
type BestOfN struct {
	Module      dsgo.Module
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
func NewBestOfN(module dsgo.Module, n int) *BestOfN {
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
// WARNING: Only use with stateless modules or independent instances.
// See BestOfN type documentation for safe usage patterns.
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
func (b *BestOfN) GetSignature() *dsgo.Signature {
	return b.Module.GetSignature()
}

// Forward executes the module N times and returns the best result
func (b *BestOfN) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	if b.Scorer == nil {
		return nil, fmt.Errorf("scorer function must be set")
	}

	if b.N <= 0 {
		return nil, fmt.Errorf("n must be positive")
	}

	if b.Parallel {
		return b.forwardParallel(ctx, inputs)
	}
	return b.forwardSequential(ctx, inputs)
}

func (b *BestOfN) forwardSequential(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	var allPredictions []*dsgo.Prediction
	var bestPrediction *dsgo.Prediction
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

func (b *BestOfN) forwardParallel(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	type result struct {
		prediction *dsgo.Prediction
		score      float64
		err        error
	}

	results := make(chan result, b.N)
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			prediction, err := b.Module.Forward(ctx, inputs)
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
		}()
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allPredictions []*dsgo.Prediction
	var bestPrediction *dsgo.Prediction
	bestScore := -1.0
	failureCount := 0

	for res := range results {
		if res.err != nil {
			failureCount++
			continue
		}

		allPredictions = append(allPredictions, res.prediction)

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
	return func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error) {
		totalLength := 0
		for _, v := range prediction.Outputs {
			totalLength += len(fmt.Sprintf("%v", v))
		}
		return float64(totalLength), nil
	}
}

// ConfidenceScorer returns a scorer based on a confidence field
func ConfidenceScorer(field string) ScoringFunction {
	return func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error) {
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
