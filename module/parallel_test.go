package module

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
)

func TestParallelBasic(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("value", core.FieldTypeString, "Value")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## value ## ]]\nsuccess",
				Usage:   core.Usage{TotalTokens: 10, Cost: 0.001},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)

	// Test with batch input
	parallel := NewParallel(predictor).
		WithMaxWorkers(2)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{},
			{},
			{},
		},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Check primary output exists
	if _, ok := result.GetString("value"); !ok {
		t.Fatal("Expected value field")
	}

	// Check completions
	if !result.HasCompletions() {
		t.Fatal("Expected completions")
	}
	if len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}

	// Check aggregated usage
	if result.Usage.TotalTokens != 30 { // 3 tasks * 10 tokens
		t.Errorf("Expected 30 total tokens, got %d", result.Usage.TotalTokens)
	}
	if result.Usage.Cost != 0.003 { // 3 tasks * 0.001 cost
		t.Errorf("Expected 0.003 cost, got %f", result.Usage.Cost)
	}
}

func TestParallelMapOfSlices(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("id", core.FieldTypeString, "ID").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\nok",
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor)

	// Map-of-slices input
	inputs := map[string]any{
		"id": []any{"a", "b", "c"},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}
}

func TestParallelMismatchedSliceLengths(t *testing.T) {
	sig := core.NewSignature("Test")
	lm := &MockLM{}
	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor)

	inputs := map[string]any{
		"a": []any{1, 2, 3},
		"b": []any{4, 5}, // Different length
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err == nil {
		t.Fatal("Expected error for mismatched slice lengths")
	}
	if !strings.Contains(err.Error(), "equal length") {
		t.Errorf("Expected 'equal length' error, got: %v", err)
	}
}

func TestParallelWithRepeat(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("value", core.FieldTypeString, "Value").
		AddOutput("echo", core.FieldTypeString, "Echoed value")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## echo ## ]]\ntest",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithRepeat(3).
		WithMaxWorkers(3)

	inputs := map[string]any{
		"value": "test",
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}
}

func TestParallelWithFactory(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("id", core.FieldTypeInt, "Task ID").
		AddOutput("result", core.FieldTypeInt, "Result")

	// Factory creates independent module instances
	factory := func(i int) core.Module {
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## result ## ]]\n%d", i*10),
					Usage:   core.Usage{TotalTokens: 1},
				}, nil
			},
		}
		return NewPredict(sig, lm)
	}

	parallel := NewParallelWithFactory(factory).
		WithMaxWorkers(2)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"id": 1},
			{"id": 2},
			{"id": 3},
		},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}
}

func TestParallelWithInstances(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("value", core.FieldTypeInt, "Value")

	// Create 3 independent instances
	instances := make([]core.Module, 3)
	for i := 0; i < 3; i++ {
		val := i + 1
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## value ## ]]\n%d", val),
					Usage:   core.Usage{TotalTokens: 1},
				}, nil
			},
		}
		instances[i] = NewPredict(sig, lm)
	}

	parallel := NewParallelWithInstances(instances)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{},
			{},
			{},
		},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}
}

func TestParallelErrorHandling(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("fail", core.FieldTypeBool, "Whether to fail").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			content := messages[len(messages)-1].Content
			// Check if fail field is true
			if strings.Contains(content, "fail") && strings.Contains(content, "true") {
				return nil, errors.New("intentional failure")
			}
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\nsuccess",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)

	t.Run("MaxFailures", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithMaxFailures(1). // Allow 1 failure
			WithMaxWorkers(2)

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"fail": false},
				{"fail": true}, // This will fail
				{"fail": false},
			},
		}

		result, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Expected success with 1 failure allowed, got: %v", err)
		}

		// Should have 2 successful completions
		if len(result.Completions) != 2 {
			t.Errorf("Expected 2 completions, got %d", len(result.Completions))
		}
	})

	t.Run("ExceedMaxFailures", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithMaxFailures(0). // No failures allowed
			WithMaxWorkers(2)

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"fail": false},
				{"fail": true}, // This will fail
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err == nil {
			t.Fatal("Expected error when exceeding max failures")
		}
		if !strings.Contains(err.Error(), "exceeded max failures") {
			t.Errorf("Expected 'exceeded max failures' error, got: %v", err)
		}
	})

	t.Run("AllFail", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithMaxWorkers(2)

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"fail": true},
				{"fail": true},
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err == nil {
			t.Fatal("Expected error when all tasks fail")
		}
		if !strings.Contains(err.Error(), "all") {
			t.Errorf("Expected 'all tasks failed' error, got: %v", err)
		}
	})
}

func TestParallelFailFast(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("shouldFail", core.FieldTypeBool, "Should fail").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			content := messages[len(messages)-1].Content

			// Small delay to simulate work
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			// Check if should fail
			if strings.Contains(content, "shouldFail") && strings.Contains(content, "true") {
				return nil, errors.New("intentional failure")
			}

			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\nsuccess",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithFailFast(true).
		WithMaxWorkers(3)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"shouldFail": false},
			{"shouldFail": true}, // Will fail
			{"shouldFail": false},
		},
	}

	start := time.Now()
	_, err := parallel.Forward(context.Background(), inputs)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error with fail-fast")
	}

	if !strings.Contains(err.Error(), "fail-fast") {
		t.Errorf("Expected 'fail-fast' error, got: %v", err)
	}

	// With fail-fast, should complete quickly
	if duration > 500*time.Millisecond {
		t.Errorf("Fail-fast took too long: %v", duration)
	}
}

func TestParallelContextCancellation(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Simulate slow work
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\nsuccess",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithMaxWorkers(2)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	inputs := map[string]any{
		"_batch": []map[string]any{
			{},
			{},
			{},
		},
	}

	start := time.Now()
	_, err := parallel.Forward(ctx, inputs)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error from context cancellation")
	}

	// Should complete quickly due to cancellation
	if duration > 500*time.Millisecond {
		t.Errorf("Context cancellation took too long: %v", duration)
	}
}

func TestParallelMetrics(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("result", core.FieldTypeInt, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Add small delay to create varied latencies
			time.Sleep(10 * time.Millisecond)
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\n42",
				Usage:   core.Usage{TotalTokens: 10, Cost: 0.001},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithMaxWorkers(2)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{},
			{},
			{},
		},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Check for parallel metrics
	metricsRaw, ok := result.Outputs["__parallel_metrics"]
	if !ok {
		t.Fatal("Expected __parallel_metrics in outputs")
	}

	metrics, ok := metricsRaw.(ParallelMetrics)
	if !ok {
		t.Fatalf("Expected ParallelMetrics, got %T", metricsRaw)
	}

	if metrics.Total != 3 {
		t.Errorf("Expected total=3, got %d", metrics.Total)
	}
	if metrics.Successes != 3 {
		t.Errorf("Expected successes=3, got %d", metrics.Successes)
	}
	if metrics.Failures != 0 {
		t.Errorf("Expected failures=0, got %d", metrics.Failures)
	}

	// Check latency metrics
	if metrics.Latency.MinMs <= 0 {
		t.Error("Expected positive MinMs")
	}
	if metrics.Latency.MaxMs < metrics.Latency.MinMs {
		t.Error("MaxMs should be >= MinMs")
	}
	if metrics.Latency.AvgMs <= 0 {
		t.Error("Expected positive AvgMs")
	}
	if metrics.Latency.P50Ms <= 0 {
		t.Error("Expected positive P50Ms")
	}
}

func TestParallelGetSignature(t *testing.T) {
	sig := core.NewSignature("Test signature").
		AddInput("x", core.FieldTypeInt, "Input").
		AddOutput("y", core.FieldTypeInt, "Output")

	lm := &MockLM{}
	predictor := NewPredict(sig, lm)

	t.Run("WithModule", func(t *testing.T) {
		parallel := NewParallel(predictor)
		gotSig := parallel.GetSignature()
		if gotSig.Description != "Test signature" {
			t.Errorf("Expected signature description 'Test signature', got %q", gotSig.Description)
		}
	})

	t.Run("WithFactory", func(t *testing.T) {
		factory := func(i int) core.Module {
			return NewPredict(sig, lm)
		}
		parallel := NewParallelWithFactory(factory)
		gotSig := parallel.GetSignature()
		if gotSig.Description != "Test signature" {
			t.Errorf("Expected signature description 'Test signature', got %q", gotSig.Description)
		}
	})

	t.Run("WithInstances", func(t *testing.T) {
		instances := []core.Module{predictor}
		parallel := NewParallelWithInstances(instances)
		gotSig := parallel.GetSignature()
		if gotSig.Description != "Test signature" {
			t.Errorf("Expected signature description 'Test signature', got %q", gotSig.Description)
		}
	})
}

func TestParallelSingleInput(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("value", core.FieldTypeString, "Value").
		AddOutput("echo", core.FieldTypeString, "Echo")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## echo ## ]]\ntest",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor)

	// Single input (no batch, no slices, no repeat)
	inputs := map[string]any{
		"value": "test",
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Should process single input
	if len(result.Completions) != 1 {
		t.Errorf("Expected 1 completion, got %d", len(result.Completions))
	}
}

func TestParallelConfigOptions(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]\nok",
				Usage:   core.Usage{TotalTokens: 1},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)

	// Test WithReturnAll
	t.Run("WithReturnAll", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithReturnAll(false)

		result, err := parallel.Forward(context.Background(), map[string]any{"_batch": []map[string]any{{}, {}}})
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}
		if result.HasCompletions() {
			t.Error("Expected no completions when ReturnAll=false")
		}
	})

	// Test WithOnlySuccessful
	t.Run("WithOnlySuccessful", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithOnlySuccessful(false)

		result, err := parallel.Forward(context.Background(), map[string]any{"_batch": []map[string]any{{}}})
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}
		if !result.HasCompletions() {
			t.Error("Expected completions")
		}
	})

	// Test WithBatchKey
	t.Run("WithBatchKey", func(t *testing.T) {
		parallel := NewParallel(predictor).
			WithBatchKey("items")

		inputs := map[string]any{
			"items": []map[string]any{{}, {}},
		}
		result, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}
		if len(result.Completions) != 2 {
			t.Errorf("Expected 2 completions, got %d", len(result.Completions))
		}
	})
}
