package module

import (
	"context"
	"fmt"
	"sync"

	"github.com/assagman/dsgo"
)

// ScoringFunction evaluates the quality of an output
type ScoringFunction func(inputs map[string]any, outputs map[string]any) (float64, error)

// BestOfN executes a module N times and returns the best result
type BestOfN struct {
	Module      dsgo.Module
	N           int
	Scorer      ScoringFunction
	Parallel    bool
	ReturnAll   bool
	MaxFailures int // Maximum number of failures before giving up
}

// BestOfNResult contains the results of BestOfN execution
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
	}
}

// WithScorer sets the scoring function
func (b *BestOfN) WithScorer(scorer ScoringFunction) *BestOfN {
	b.Scorer = scorer
	return b
}

// WithParallel enables parallel execution
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

// GetSignature returns the module's signature
func (b *BestOfN) GetSignature() *dsgo.Signature {
	return b.Module.GetSignature()
}

// Forward executes the module N times and returns the best result
func (b *BestOfN) Forward(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if b.Scorer == nil {
		return nil, fmt.Errorf("scorer function must be set")
	}

	if b.N <= 0 {
		return nil, fmt.Errorf("N must be positive")
	}

	if b.Parallel {
		return b.forwardParallel(ctx, inputs)
	}
	return b.forwardSequential(ctx, inputs)
}

func (b *BestOfN) forwardSequential(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	var allOutputs []map[string]any
	var allScores []float64
	var bestOutput map[string]any
	bestScore := -1.0
	failureCount := 0

	for i := 0; i < b.N; i++ {
		outputs, err := b.Module.Forward(ctx, inputs)
		if err != nil {
			failureCount++
			if failureCount > b.MaxFailures {
				return nil, fmt.Errorf("exceeded maximum failures (%d/%d): %w", failureCount, b.N, err)
			}
			continue
		}

		score, err := b.Scorer(inputs, outputs)
		if err != nil {
			failureCount++
			if failureCount > b.MaxFailures {
				return nil, fmt.Errorf("scoring failed (%d/%d): %w", failureCount, b.N, err)
			}
			continue
		}

		allOutputs = append(allOutputs, outputs)
		allScores = append(allScores, score)

		if bestOutput == nil || score > bestScore {
			bestOutput = outputs
			bestScore = score
		}
	}

	if bestOutput == nil {
		return nil, fmt.Errorf("all %d attempts failed", b.N)
	}

	// If ReturnAll is enabled, add metadata to the result
	if b.ReturnAll {
		result := make(map[string]any)
		for k, v := range bestOutput {
			result[k] = v
		}
		result["_best_of_n_score"] = bestScore
		result["_best_of_n_all_scores"] = allScores
		return result, nil
	}

	return bestOutput, nil
}

func (b *BestOfN) forwardParallel(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	type result struct {
		outputs map[string]any
		score   float64
		err     error
	}

	results := make(chan result, b.N)
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			outputs, err := b.Module.Forward(ctx, inputs)
			if err != nil {
				results <- result{err: err}
				return
			}

			score, err := b.Scorer(inputs, outputs)
			if err != nil {
				results <- result{err: err}
				return
			}

			results <- result{outputs: outputs, score: score}
		}()
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allOutputs []map[string]any
	var allScores []float64
	var bestOutput map[string]any
	bestScore := -1.0
	failureCount := 0

	for res := range results {
		if res.err != nil {
			failureCount++
			continue
		}

		allOutputs = append(allOutputs, res.outputs)
		allScores = append(allScores, res.score)

		if bestOutput == nil || res.score > bestScore {
			bestOutput = res.outputs
			bestScore = res.score
		}
	}

	if failureCount > b.MaxFailures {
		return nil, fmt.Errorf("exceeded maximum failures (%d/%d)", failureCount, b.N)
	}

	if bestOutput == nil {
		return nil, fmt.Errorf("all %d attempts failed", b.N)
	}

	// If ReturnAll is enabled, add metadata to the result
	if b.ReturnAll {
		result := make(map[string]any)
		for k, v := range bestOutput {
			result[k] = v
		}
		result["_best_of_n_score"] = bestScore
		result["_best_of_n_all_scores"] = allScores
		return result, nil
	}

	return bestOutput, nil
}

// DefaultScorer returns a simple length-based scorer
// This is a basic scorer that prefers longer outputs
func DefaultScorer() ScoringFunction {
	return func(inputs map[string]any, outputs map[string]any) (float64, error) {
		totalLength := 0
		for _, v := range outputs {
			totalLength += len(fmt.Sprintf("%v", v))
		}
		return float64(totalLength), nil
	}
}

// ConfidenceScorer returns a scorer based on a confidence field
func ConfidenceScorer(field string) ScoringFunction {
	return func(inputs map[string]any, outputs map[string]any) (float64, error) {
		confidence, exists := outputs[field]
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
