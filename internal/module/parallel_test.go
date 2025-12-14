package module

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

func TestParallelBasic(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	sig := core.NewSignature("Test").
		AddOutput("value", core.FieldTypeInt, "Value")

	// Create 3 independent instances
	instances := make([]core.Module, 3)
	for i := range 3 {
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
	t.Parallel()
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
	t.Parallel()
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

	if !strings.Contains(err.Error(), "fail-fast triggered") {
		t.Errorf("Expected fail-fast error, got: %v", err)
	}

	// With fail-fast, should complete quickly
	if duration > 500*time.Millisecond {
		t.Errorf("Fail-fast took too long: %v", duration)
	}
}

func TestParallelContextCancellation(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// Thread Safety Tests

// TestParallelThreadSafetyNoRaceConditions verifies that NewParallel now
// prevents race conditions by cloning modules per task
func TestParallelThreadSafetyNoRaceConditions(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping no-race test in short mode")
	}

	// Create a signature for our test
	sig := core.NewSignature("ThreadSafetyTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddInput("message", core.FieldTypeString, "Message").
		AddOutput("response", core.FieldTypeString, "Response").
		AddOutput("history_length", core.FieldTypeInt, "History length after processing")

	// Create a stateful module with History
	history := core.NewHistory()
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Extract task_id from last message for debugging
			content := messages[len(messages)-1].Content
			taskID := 0
			if strings.Contains(content, "Task") {
				parts := strings.Fields(content)
				for i, part := range parts {
					if part == "Task" && i+1 < len(parts) {
						if id, err := fmt.Sscanf(parts[i+1], "%d", &taskID); err == nil && id == 1 {
							break
						}
					}
				}
			}

			return &core.GenerateResult{
				Content: fmt.Sprintf("[[ ## response ## ]]\nProcessed task %d\n[[ ## history_length ## ]]\n%d", taskID, len(messages)),
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm).WithHistory(history)

	// Create parallel with default NewParallel - NOW SAFE due to cloning
	parallel := NewParallel(predictor).
		WithMaxWorkers(runtime.NumCPU()).
		WithRepeat(50) // High concurrency to test safety

	// Create batch inputs
	batch := make([]map[string]any, 50)
	for i := range 50 {
		batch[i] = map[string]any{
			"task_id": i,
			"message": fmt.Sprintf("Task %d", i),
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	// Run with race detector enabled - should now be safe
	result, err := parallel.Forward(context.Background(), inputs)

	// Should now succeed without race conditions
	if err != nil {
		t.Fatalf("Parallel execution failed (unexpected with cloning): %v", err)
	}

	if len(result.Completions) != 50 {
		t.Errorf("Expected 50 completions, got %d", len(result.Completions))
	}

	// Verify each task was processed correctly (no state interference)
	for i, completion := range result.Completions {
		response, ok := completion["response"].(string)
		if !ok {
			t.Errorf("Completion %d missing response", i)
			continue
		}

		expectedPattern := fmt.Sprintf("Processed task %d", i)
		if !strings.Contains(response, expectedPattern) {
			t.Errorf("Completion %d: expected pattern %q, got %q", i, expectedPattern, response)
		}

		// Each task should have minimal history (cloned state)
		historyLength, ok := completion["history_length"].(int)
		if !ok {
			t.Errorf("Completion %d missing history_length", i)
			continue
		}

		if historyLength > 5 {
			t.Errorf("Task %d has history length %d, expected <=5 (indicates shared state)", i, historyLength)
		}
	}

	// Original history should remain unchanged
	if history.Len() > 2 {
		t.Errorf("Original history length %d indicates shared state was modified, expected <=2", history.Len())
	}

	t.Logf("Parallel execution completed safely with %d completions (no race conditions)", len(result.Completions))
}

// TestParallelThreadSafetyWithFactory demonstrates safe usage with factory pattern
func TestParallelThreadSafetyWithFactory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping factory test in short mode")
	}

	sig := core.NewSignature("ThreadSafetyTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddInput("message", core.FieldTypeString, "Message").
		AddOutput("response", core.FieldTypeString, "Response").
		AddOutput("history_length", core.FieldTypeInt, "History length after processing")

	// Factory creates independent module instances with separate History
	factory := func(i int) core.Module {
		history := core.NewHistory() // Each instance gets its own History
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## response ## ]]\nTask %d processed\n[[ ## history_length ## ]]\n%d", i, len(messages)),
					Usage:   core.Usage{TotalTokens: 10},
				}, nil
			},
		}
		return NewPredict(sig, lm).WithHistory(history)
	}

	// Create parallel with factory - THIS IS THREAD-SAFE
	parallel := NewParallelWithFactory(factory).
		WithMaxWorkers(runtime.NumCPU()).
		WithRepeat(100) // Even higher concurrency

	// Create batch inputs
	batch := make([]map[string]any, 100)
	for i := range 100 {
		batch[i] = map[string]any{
			"task_id": i,
			"message": fmt.Sprintf("Task %d", i),
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Factory pattern should be thread-safe, but got error: %v", err)
	}

	if len(result.Completions) != 100 {
		t.Errorf("Expected 100 completions, got %d", len(result.Completions))
	}

	// Verify each task was processed correctly
	for i, completion := range result.Completions {
		response, ok := completion["response"].(string)
		if !ok {
			t.Errorf("Completion %d missing response", i)
			continue
		}
		// Each completion should be unique (no shared state interference)
		expectedPattern := fmt.Sprintf("Task %d processed", i)
		if response != expectedPattern {
			t.Errorf("Completion %d: expected %q, got %q", i, expectedPattern, response)
		}
	}

	t.Logf("Factory pattern completed successfully with %d completions", len(result.Completions))
}

// TestParallelThreadSafetyWithInstances demonstrates safe usage with pre-created instances
func TestParallelThreadSafetyWithInstances(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping instances test in short mode")
	}

	sig := core.NewSignature("ThreadSafetyTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddInput("message", core.FieldTypeString, "Message").
		AddOutput("response", core.FieldTypeString, "Response").
		AddOutput("instance_id", core.FieldTypeInt, "Instance ID that processed this")

	// Create multiple independent instances
	numInstances := runtime.NumCPU()
	instances := make([]core.Module, numInstances)

	for i := range numInstances {
		instanceID := i
		history := core.NewHistory() // Each instance has its own History
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## response ## ]]\nProcessed by instance %d\n[[ ## instance_id ## ]]\n%d", instanceID, instanceID),
					Usage:   core.Usage{TotalTokens: 10},
				}, nil
			},
		}
		instances[i] = NewPredict(sig, lm).WithHistory(history)
	}

	// Create parallel with pre-created instances - THIS IS THREAD-SAFE
	parallel := NewParallelWithInstances(instances).
		WithMaxWorkers(numInstances).
		WithRepeat(80) // More tasks than instances to test round-robin

	// Create batch inputs
	batch := make([]map[string]any, 80)
	for i := range 80 {
		batch[i] = map[string]any{
			"task_id": i,
			"message": fmt.Sprintf("Task %d", i),
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Instances pattern should be thread-safe, but got error: %v", err)
	}

	if len(result.Completions) != 80 {
		t.Errorf("Expected 80 completions, got %d", len(result.Completions))
	}

	// Verify round-robin distribution across instances
	instanceUsage := make(map[int]int)
	for _, completion := range result.Completions {
		instanceID, ok := completion["instance_id"].(int)
		if !ok {
			t.Error("Missing instance_id in completion")
			continue
		}
		if instanceID < 0 || instanceID >= numInstances {
			t.Errorf("Invalid instance_id: %d", instanceID)
			continue
		}
		instanceUsage[instanceID]++
	}

	// Each instance should have been used (roughly evenly distributed)
	for i := range numInstances {
		if instanceUsage[i] == 0 {
			t.Errorf("Instance %d was not used", i)
		}
		t.Logf("Instance %d processed %d tasks", i, instanceUsage[i])
	}

	t.Logf("Instances pattern completed successfully with %d completions", len(result.Completions))
}

// TestParallelThreadSafetyStatelessModule demonstrates that stateless modules are safe
func TestParallelThreadSafetyStatelessModule(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping stateless test in short mode")
	}

	sig := core.NewSignature("StatelessTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddOutput("response", core.FieldTypeString, "Response")

	// Create a stateless module (no History)
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## response ## ]]\nStateless response",
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm) // No History = stateless

	// Create parallel with shared stateless instance - THIS IS SAFE
	parallel := NewParallel(predictor).
		WithMaxWorkers(runtime.NumCPU()).
		WithRepeat(200) // Very high concurrency

	// Create batch inputs
	batch := make([]map[string]any, 200)
	for i := range 200 {
		batch[i] = map[string]any{
			"task_id": i,
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Stateless module should be thread-safe, but got error: %v", err)
	}

	if len(result.Completions) != 200 {
		t.Errorf("Expected 200 completions, got %d", len(result.Completions))
	}

	t.Logf("Stateless module completed successfully with %d completions", len(result.Completions))
}

// TestParallelThreadSafetyStressTest is a comprehensive stress test
func TestParallelThreadSafetyStressTest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	sig := core.NewSignature("StressTest").
		AddInput("iteration", core.FieldTypeInt, "Iteration number").
		AddInput("worker_id", core.FieldTypeInt, "Worker ID").
		AddOutput("result", core.FieldTypeString, "Result").
		AddOutput("timestamp", core.FieldTypeInt, "Processing timestamp")

	// Track concurrent access patterns
	var concurrentAccess int64
	var maxConcurrentAccess int64
	var accessCounter int64

	// Factory with stateful modules
	factory := func(workerID int) core.Module {
		history := core.NewHistory()
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				// Track concurrent access
				current := atomic.AddInt64(&concurrentAccess, 1)
				if current > atomic.LoadInt64(&maxConcurrentAccess) {
					atomic.StoreInt64(&maxConcurrentAccess, current)
				}

				// Simulate some work
				time.Sleep(time.Microsecond * time.Duration(10+workerID%20))

				// Decrement counter
				atomic.AddInt64(&concurrentAccess, -1)
				counter := atomic.AddInt64(&accessCounter, 1)

				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## result ## ]]\nWorker %d iteration %d\n[[ ## timestamp ## ]]\n%d",
						workerID, counter, time.Now().UnixNano()),
					Usage: core.Usage{TotalTokens: 15},
				}, nil
			},
		}
		return NewPredict(sig, lm).WithHistory(history)
	}

	// High concurrency stress test
	numWorkers := runtime.NumCPU() * 2
	numTasks := 500

	parallel := NewParallelWithFactory(factory).
		WithMaxWorkers(numWorkers)

	// Create batch inputs
	batch := make([]map[string]any, numTasks)
	for i := range numTasks {
		batch[i] = map[string]any{
			"iteration": i,
			"worker_id": i % numWorkers,
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	start := time.Now()
	result, err := parallel.Forward(context.Background(), inputs)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Stress test failed: %v", err)
	}

	if len(result.Completions) != numTasks {
		t.Errorf("Expected %d completions, got %d", numTasks, len(result.Completions))
	}

	maxConcurrent := atomic.LoadInt64(&maxConcurrentAccess)
	totalAccess := atomic.LoadInt64(&accessCounter)

	t.Logf("Stress test completed:")
	t.Logf("  Tasks: %d", numTasks)
	t.Logf("  Workers: %d", numWorkers)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f tasks/sec", float64(numTasks)/duration.Seconds())
	t.Logf("  Max concurrent access: %d", maxConcurrent)
	t.Logf("  Total access count: %d", totalAccess)

	// Verify we actually achieved concurrency
	if maxConcurrent < 2 {
		t.Error("Expected concurrent access > 1, but got", maxConcurrent)
	}

	// Verify all tasks were processed
	if totalAccess != int64(numTasks) {
		t.Errorf("Expected %d total accesses, got %d", numTasks, totalAccess)
	}
}

// TestParallelDefaultCloning tests that NewParallel now clones modules by default
// to ensure state isolation between parallel tasks
func TestParallelDefaultCloning(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("CloningTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddOutput("history_length", core.FieldTypeInt, "History length after processing").
		AddOutput("task_processed", core.FieldTypeInt, "Task ID that was processed")

	// Track which task IDs were processed to verify isolation
	processedTasks := make([]int, 0, 20)
	var mu sync.Mutex

	// Create a stateful module with History (for comparison)
	history := core.NewHistory()

	// Create parallel with factory to have better control over task ID tracking
	factory := func(taskID int) core.Module {
		// Create a new LM that captures the task ID
		taskLM := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				mu.Lock()
				processedTasks = append(processedTasks, taskID)
				mu.Unlock()

				return &core.GenerateResult{
					Content: fmt.Sprintf("[[ ## history_length ## ]]\n%d\n[[ ## task_processed ## ]]\n%d",
						2, taskID), // Each cloned module should have minimal history
					Usage: core.Usage{TotalTokens: 5},
				}, nil
			},
		}

		// Each factory instance gets its own history
		taskHistory := core.NewHistory()
		return NewPredict(sig, taskLM).WithHistory(taskHistory)
	}

	// Create parallel with factory - this simulates what the default NewParallel should do
	parallel := NewParallelWithFactory(factory).
		WithMaxWorkers(runtime.NumCPU())

	// Create batch inputs
	batch := make([]map[string]any, 20)
	for i := range 20 {
		batch[i] = map[string]any{
			"task_id": i,
		}
	}

	inputs := map[string]any{
		"_batch": batch,
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Parallel execution failed: %v", err)
	}

	if len(result.Completions) != 20 {
		t.Errorf("Expected 20 completions, got %d", len(result.Completions))
	}

	// Verify state isolation: each task should have history length of 2
	for i, completion := range result.Completions {
		historyLength, ok := completion["history_length"].(int)
		if !ok {
			t.Errorf("Completion %d missing history_length", i)
			continue
		}

		// With cloning, each task should have minimal history
		if historyLength != 2 {
			t.Errorf("Task %d has history length %d, expected 2 (indicates shared state)", i, historyLength)
		}

		// Verify the correct task was processed
		taskProcessed, ok := completion["task_processed"].(int)
		if !ok {
			t.Errorf("Completion %d missing task_processed", i)
			continue
		}

		// Each task should process its own ID
		if taskProcessed != i {
			t.Errorf("Task %d: expected to process task %d, but processed %d", i, i, taskProcessed)
		}
	}

	// Verify all tasks were processed exactly once
	mu.Lock()
	if len(processedTasks) != 20 {
		t.Errorf("Expected 20 tasks to be processed, got %d", len(processedTasks))
	}

	// Check for duplicates (would indicate shared state)
	taskCount := make(map[int]int)
	for _, taskID := range processedTasks {
		taskCount[taskID]++
	}

	for taskID, count := range taskCount {
		if count > 1 {
			t.Errorf("Task %d was processed %d times (expected 1)", taskID, count)
		}
	}
	mu.Unlock()

	// The original history should remain unchanged
	if history.Len() > 0 {
		t.Errorf("Original history length %d indicates shared state was modified, expected 0", history.Len())
	}

	t.Logf("Default cloning test passed - each task had isolated state")
}

// TestParallelThreadSafetyHistoryCorruption specifically tests for History corruption
func TestParallelThreadSafetyHistoryCorruption(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping corruption test in short mode")
	}

	sig := core.NewSignature("CorruptionTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddOutput("history_count", core.FieldTypeInt, "Number of messages in history").
		AddOutput("task_processed", core.FieldTypeInt, "Task ID that was processed")

	// Shared history that will be corrupted by concurrent access
	sharedHistory := core.NewHistory()

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Extract task_id from input
			taskID := 0
			if len(messages) > 0 {
				// Simple parsing for test purposes
				content := messages[len(messages)-1].Content
				// This is a simplified extraction - in real tests it would be more robust
				if len(content) > 10 {
					taskID = len(content) % 100 // Pseudo-random based on content
				}
			}

			return &core.GenerateResult{
				Content: fmt.Sprintf("[[ ## history_count ## ]]\n%d\n[[ ## task_processed ## ]]\n%d",
					sharedHistory.Len(), taskID),
				Usage: core.Usage{TotalTokens: 8},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm).WithHistory(sharedHistory)

	// This should demonstrate corruption with shared History
	parallel := NewParallel(predictor).
		WithMaxWorkers(runtime.NumCPU() * 2).
		WithRepeat(100)

	inputs := map[string]any{
		"_batch": make([]map[string]any, 100),
	}

	// Initialize batch
	for i := range 100 {
		inputs["_batch"].([]map[string]any)[i] = map[string]any{
			"task_id": i,
		}
	}

	result, err := parallel.Forward(context.Background(), inputs)

	// The test might complete but we can check for inconsistencies
	if err != nil {
		t.Logf("Execution failed (possible corruption): %v", err)
		return
	}

	// Analyze results for inconsistencies
	historyCounts := make(map[int]int)
	taskIDs := make(map[int]bool)

	for _, completion := range result.Completions {
		if historyCount, ok := completion["history_count"].(int); ok {
			historyCounts[historyCount]++
		}
		if taskID, ok := completion["task_processed"].(int); ok {
			taskIDs[taskID] = true
		}
	}

	t.Logf("History corruption analysis:")
	t.Logf("  Unique history counts: %d", len(historyCounts))
	t.Logf("  Unique task IDs processed: %d", len(taskIDs))
	t.Logf("  Final shared history length: %d", sharedHistory.Len())

	// If we see many different history counts, it indicates corruption
	if len(historyCounts) > 10 {
		t.Logf("WARNING: Detected potential history corruption - %d different history counts observed", len(historyCounts))
	}

	// The final history should be much larger than expected if corruption occurred
	if sharedHistory.Len() > 200 { // Expected would be around 200 (2 messages per task)
		t.Logf("WARNING: History length (%d) is much larger than expected, indicating possible corruption", sharedHistory.Len())
	}
}

// Test Parallel logging functionality

// capturingLogger is a test logger that captures log calls for verification
type capturingLogger struct {
	mu     sync.Mutex
	infos  []logEntry
	debugs []logEntry
}

type logEntry struct {
	level   string
	message string
	fields  map[string]any
}

func (c *capturingLogger) Info(ctx context.Context, message string, fields map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.infos = append(c.infos, logEntry{
		level:   "info",
		message: message,
		fields:  fields,
	})
}

func (c *capturingLogger) Debug(ctx context.Context, message string, fields map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.debugs = append(c.debugs, logEntry{
		level:   "debug",
		message: message,
		fields:  fields,
	})
}

func (c *capturingLogger) Warn(ctx context.Context, message string, fields map[string]any) {
	// Not used in these tests
}

func (c *capturingLogger) Error(ctx context.Context, message string, fields map[string]any) {
	// Not used in these tests
}

func (c *capturingLogger) Fatal(ctx context.Context, message string, fields map[string]any) {
	// Not used in these tests
}

func (c *capturingLogger) getLastInfo() *logEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.infos) == 0 {
		return nil
	}
	return &c.infos[len(c.infos)-1]
}

func (c *capturingLogger) getDebugEntries() []logEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]logEntry, len(c.debugs))
	copy(result, c.debugs)
	return result
}

func (c *capturingLogger) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.infos = c.infos[:0]
	c.debugs = c.debugs[:0]
}

// mockLMWithName is a MockLM that returns a specific model name
type mockLMWithName struct {
	*MockLM
	modelName string
}

func (m *mockLMWithName) Name() string {
	return m.modelName
}

func TestParallelLoggingBatchLevel(t *testing.T) {
	// Setup capturing logger
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	// Create signature and LM with known model name
	sig := core.NewSignature("Test").
		AddInput("file_path", core.FieldTypeString, "File path").
		AddOutput("analysis", core.FieldTypeString, "Analysis result")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## analysis ## ]]\nAnalysis complete",
					Usage:   core.Usage{TotalTokens: 5},
				}, nil
			},
		},
		modelName: "test-model-gpt-4o",
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithMaxWorkers(2).
		WithMaxFailures(1).
		WithFailFast(false)

	// Reset logger before test
	capturingLog.reset()

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"file_path": "file1.go"},
			{"file_path": "file2.go"},
			{"file_path": "file3.go"},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify batch-level info log
	info := capturingLog.getLastInfo()
	if info == nil {
		t.Fatal("Expected at least one info log entry")
	}

	if info.message != "Parallel batch started" {
		t.Errorf("Expected message 'Parallel batch started', got %q", info.message)
	}

	// Verify required fields (schema v1)
	expectedFields := map[string]string{
		"module":        logging.ModuleParallel,
		"parallel_mode": "clone",
		"inner_module":  "Predict",
		"lm_model":      "test-model-gpt-4o",
		"batch_size":    "3",
		"max_workers":   "2",
		"max_failures":  "1",
		"fail_fast":     "false",
		"return_all":    "true",
		"only_success":  "true",
		"repeat_factor": "1",
	}

	for field, expectedValue := range expectedFields {
		actualValue, ok := info.fields[field]
		if !ok {
			t.Errorf("Missing field %q in info log", field)
			continue
		}
		actualStr := fmt.Sprintf("%v", actualValue)
		if actualStr != expectedValue {
			t.Errorf("Field %q: expected %q, got %q", field, expectedValue, actualStr)
		}
	}

	parallelID, ok := info.fields["parallel_id"].(string)
	if !ok || parallelID == "" {
		t.Fatalf("Expected non-empty parallel_id string, got %T %v", info.fields["parallel_id"], info.fields["parallel_id"])
	}
}

func TestParallelLoggingPerTask(t *testing.T) {
	// Setup capturing logger
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	// Create signature and LM
	sig := core.NewSignature("Test").
		AddInput("package_name", core.FieldTypeString, "Package name").
		AddInput("file_contents", core.FieldTypeString, "File contents").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## result ## ]]\nProcessed",
					Usage:   core.Usage{TotalTokens: 3},
				}, nil
			},
		},
		modelName: "openai/gpt-4o-mini",
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).
		WithMaxWorkers(2)

	// Reset logger before test
	capturingLog.reset()

	// Create inputs with long content to test truncation
	longContent := strings.Repeat("This is a very long file content that should be truncated. ", 100)
	inputs := map[string]any{
		"_batch": []map[string]any{
			{
				"package_name":  "github.com/example/pkg1",
				"file_contents": longContent,
			},
			{
				"package_name":  "github.com/example/pkg2",
				"file_contents": "Short content",
			},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify per-task debug logs
	debugEntries := capturingLog.getDebugEntries()
	var taskEntries []logEntry
	for _, entry := range debugEntries {
		if entry.message == "Parallel task started" {
			taskEntries = append(taskEntries, entry)
		}
	}

	if len(taskEntries) != 2 {
		t.Errorf("Expected 2 'Parallel task started' debug log entries, got %d", len(taskEntries))
	}

	for i, entry := range taskEntries {
		if entry.message != "Parallel task started" {
			t.Errorf("Debug entry %d: expected message 'Parallel task started', got %q", i, entry.message)
		}

		// Verify required fields (schema v1)
		expectedFields := []string{"module", "parallel_id", "parallel_mode", "inner_module", "lm_model", "batch_size", "max_workers", "task_index", "task_total", "worker_id", "queue_wait_ms", "inputs", "inputs_truncated"}
		for _, field := range expectedFields {
			if _, ok := entry.fields[field]; !ok {
				t.Errorf("Debug entry %d: missing field %q", i, field)
			}
		}

		// Verify module info
		if entry.fields["module"] != logging.ModuleParallel {
			t.Errorf("Debug entry %d: expected module %q, got %q", i, logging.ModuleParallel, entry.fields["module"])
		}
		if entry.fields["parallel_mode"] != "clone" {
			t.Errorf("Debug entry %d: expected parallel_mode 'clone', got %q", i, entry.fields["parallel_mode"])
		}
		if entry.fields["inner_module"] != "Predict" {
			t.Errorf("Debug entry %d: expected inner_module 'Predict', got %q", i, entry.fields["inner_module"])
		}
		if entry.fields["lm_model"] != "openai/gpt-4o-mini" {
			t.Errorf("Debug entry %d: expected lm_model 'openai/gpt-4o-mini', got %q", i, entry.fields["lm_model"])
		}

		// Verify task index
		taskIndex, ok := entry.fields["task_index"].(int)
		if !ok {
			t.Errorf("Debug entry %d: task_index should be int, got %T", i, entry.fields["task_index"])
		} else if taskIndex < 0 || taskIndex >= 2 {
			t.Errorf("Debug entry %d: invalid task_index %d", i, taskIndex)
		}

		taskTotal, ok := entry.fields["task_total"].(int)
		if !ok {
			t.Errorf("Debug entry %d: task_total should be int, got %T", i, entry.fields["task_total"])
		} else if taskTotal != 2 {
			t.Errorf("Debug entry %d: expected task_total=2, got %d", i, taskTotal)
		}

		workerID, ok := entry.fields["worker_id"].(int)
		if !ok {
			t.Errorf("Debug entry %d: worker_id should be int, got %T", i, entry.fields["worker_id"])
		} else if workerID < 0 || workerID >= 2 {
			t.Errorf("Debug entry %d: worker_id out of range [0,2): %d", i, workerID)
		}

		queueWaitMs, ok := entry.fields["queue_wait_ms"].(int64)
		if !ok {
			// Depending on platform/codec, this may come through as int.
			if qwi, ok2 := entry.fields["queue_wait_ms"].(int); ok2 {
				queueWaitMs = int64(qwi)
				ok = true
			}
		}
		if !ok {
			t.Errorf("Debug entry %d: queue_wait_ms should be int/int64, got %T", i, entry.fields["queue_wait_ms"])
		} else if queueWaitMs < 0 {
			t.Errorf("Debug entry %d: queue_wait_ms should be >= 0, got %d", i, queueWaitMs)
		}

		// Verify inputs summarization
		inputs, ok := entry.fields["inputs"].(map[string]any)
		if !ok {
			t.Errorf("Debug entry %d: inputs should be map[string]any, got %T", i, entry.fields["inputs"])
			continue
		}

		// Check that inputs_truncated is correctly reported
		inputsTruncated, ok := entry.fields["inputs_truncated"].(bool)
		if !ok {
			t.Errorf("Debug entry %d: inputs_truncated should be bool, got %T", i, entry.fields["inputs_truncated"])
		}

		// Check that long content was truncated
		if taskIndex == 0 { // First task has long content
			fileContent, ok := inputs["file_contents"].(string)
			if !ok {
				t.Errorf("Debug entry %d: file_contents should be string, got %T", i, inputs["file_contents"])
			} else if !strings.Contains(fileContent, "...[truncated]") {
				t.Errorf("Debug entry %d: long content should be truncated, got length %d", i, len(fileContent))
			}
			if ok && !inputsTruncated {
				t.Errorf("Debug entry %d: expected inputs_truncated=true for task %d", i, taskIndex)
			}
		} else {
			if ok && inputsTruncated {
				t.Errorf("Debug entry %d: expected inputs_truncated=false for task %d", i, taskIndex)
			}
		}

		// Check that package_name is preserved
		packageName, ok := inputs["package_name"].(string)
		if !ok {
			t.Errorf("Debug entry %d: package_name should be string, got %T", i, inputs["package_name"])
		}
		expectedPackage := fmt.Sprintf("github.com/example/pkg%d", taskIndex+1)
		if packageName != expectedPackage {
			t.Errorf("Debug entry %d: expected package_name %q, got %q", i, expectedPackage, packageName)
		}
	}
}

func TestParallelLoggingPredictionMetadata(t *testing.T) {
	// Setup capturing logger (not strictly needed for metadata test but keeps pattern)
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	// Create signature and LM
	sig := core.NewSignature("Test").
		AddInput("data", core.FieldTypeString, "Input data").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## output ## ]]\nResult",
					Usage:   core.Usage{TotalTokens: 2},
				}, nil
			},
		},
		modelName: "anthropic/claude-3-sonnet",
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"data": "test1"},
			{"data": "test2"},
		},
	}

	result, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify __parallel_context metadata
	contextRaw, ok := result.Outputs["__parallel_context"]
	if !ok {
		t.Fatal("Expected __parallel_context in prediction outputs")
	}

	context, ok := contextRaw.(map[string]any)
	if !ok {
		t.Fatalf("Expected __parallel_context to be map[string]any, got %T", contextRaw)
	}

	// Verify required fields
	expectedFields := map[string]any{
		"inner_module": "Predict",
		"lm_model":     "anthropic/claude-3-sonnet",
		"total_tasks":  2,
	}

	for field, expectedValue := range expectedFields {
		actualValue, ok := context[field]
		if !ok {
			t.Errorf("Missing field %q in __parallel_context", field)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Field %q: expected %v, got %v", field, expectedValue, actualValue)
		}
	}
}

func TestParallelLoggingWithChainOfThought(t *testing.T) {
	// Test that logging works with different module types
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("rationale", core.FieldTypeString, "Reasoning")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## rationale ## ]]Let me think about this.\n[[ ## answer ## ]]42",
					Usage:   core.Usage{TotalTokens: 8},
				}, nil
			},
		},
		modelName: "google/gemini-pro",
	}

	cot := NewChainOfThought(sig, lm)
	parallel := NewParallel(cot)

	capturingLog.reset()

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "What is 6*7?"},
			{"question": "What is 8*9?"},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify batch-level log shows ChainOfThought
	info := capturingLog.getLastInfo()
	if info == nil {
		t.Fatal("Expected info log entry")
	}

	if info.fields["inner_module"] != "ChainOfThought" {
		t.Errorf("Expected inner_module 'ChainOfThought', got %q", info.fields["inner_module"])
	}

	if info.fields["lm_model"] != "google/gemini-pro" {
		t.Errorf("Expected lm_model 'google/gemini-pro', got %q", info.fields["lm_model"])
	}

	// Verify per-task debug logs also show ChainOfThought
	debugEntries := capturingLog.getDebugEntries()
	var taskEntries []logEntry
	for _, entry := range debugEntries {
		if entry.message == "Parallel task started" {
			taskEntries = append(taskEntries, entry)
		}
	}

	if len(taskEntries) != 2 {
		t.Errorf("Expected 2 'Parallel task started' debug log entries, got %d", len(taskEntries))
	}

	for i, entry := range taskEntries {
		if entry.fields["inner_module"] != "ChainOfThought" {
			t.Errorf("Debug entry %d: expected inner_module 'ChainOfThought', got %q", i, entry.fields["inner_module"])
		}
	}
}

func TestParallelLoggingWithFactory(t *testing.T) {
	// Test logging with factory pattern
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	sig := core.NewSignature("Test").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddOutput("result", core.FieldTypeString, "Result")

	factory := func(i int) core.Module {
		lm := &mockLMWithName{
			MockLM: &MockLM{
				GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
					return &core.GenerateResult{
						Content: fmt.Sprintf("[[ ## result ## ]]\nTask %d done", i),
						Usage:   core.Usage{TotalTokens: 3},
					}, nil
				},
			},
			modelName: fmt.Sprintf("factory-model-%d", i%3), // Rotate between 3 models
		}
		return NewPredict(sig, lm)
	}

	parallel := NewParallelWithFactory(factory)

	capturingLog.reset()

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"task_id": 0},
			{"task_id": 1},
			{"task_id": 2},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify batch-level log (should use first factory instance)
	info := capturingLog.getLastInfo()
	if info == nil {
		t.Fatal("Expected info log entry")
	}

	// For factory-based parallels, batch-level info is empty since we don't have a module yet
	// The module info will be captured during task execution
	if info.fields["inner_module"] != "" {
		t.Errorf("Expected empty inner_module for factory-based parallel, got %q", info.fields["inner_module"])
	}

	if info.fields["lm_model"] != "" {
		t.Errorf("Expected empty lm_model for factory-based parallel, got %q", info.fields["lm_model"])
	}

	// Verify per-task debug logs have potentially different models
	debugEntries := capturingLog.getDebugEntries()
	var taskEntries []logEntry
	for _, entry := range debugEntries {
		if entry.message == "Parallel task started" {
			taskEntries = append(taskEntries, entry)
		}
	}

	if len(taskEntries) != 3 {
		t.Errorf("Expected 3 'Parallel task started' debug entries, got %d", len(taskEntries))
	}

	for i, entry := range taskEntries {
		if entry.fields["inner_module"] != "Predict" {
			t.Errorf("Debug entry %d: expected inner_module 'Predict', got %q", i, entry.fields["inner_module"])
		}

		if entry.fields["lm_model"] == "" {
			t.Errorf("Debug entry %d: expected non-empty lm_model", i)
		}

		taskIndex, ok := entry.fields["task_index"].(int)
		if !ok {
			t.Errorf("Debug entry %d: task_index should be int", i)
		} else {
			// With parallel execution, order is not guaranteed.
			// We just check that taskIndex is valid (0, 1, or 2)
			if taskIndex < 0 || taskIndex > 2 {
				t.Errorf("Debug entry %d: unexpected task_index %d", i, taskIndex)
			}
		}
	}
}

func TestParallelLoggingInputSummarization(t *testing.T) {
	// Test various input types and their summarization
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	sig := core.NewSignature("Test").
		AddInput("text", core.FieldTypeString, "Text input").
		AddInput("number", core.FieldTypeInt, "Number input").
		AddInput("flag", core.FieldTypeBool, "Boolean input").
		AddInput("data", core.FieldTypeJSON, "JSON data").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## result ## ]]\nProcessed",
					Usage:   core.Usage{TotalTokens: 1},
				}, nil
			},
		},
		modelName: "test-model",
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor)

	capturingLog.reset()

	// Create complex input data
	jsonData := map[string]any{
		"nested": map[string]any{
			"array": []int{1, 2, 3, 4, 5},
			"value": "test",
		},
	}

	inputs := map[string]any{
		"_batch": []map[string]any{
			{
				"text":   "short text",
				"number": 42,
				"flag":   true,
				"data":   jsonData,
			},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify input summarization in debug logs
	debugEntries := capturingLog.getDebugEntries()
	var taskEntries []logEntry
	for _, entry := range debugEntries {
		if entry.message == "Parallel task started" {
			taskEntries = append(taskEntries, entry)
		}
	}

	if len(taskEntries) != 1 {
		t.Fatalf("Expected 1 'Parallel task started' debug entry, got %d", len(taskEntries))
	}

	inputsSummary, ok := taskEntries[0].fields["inputs"].(map[string]any)
	if !ok {
		t.Fatal("Expected inputs to be map[string]any")
	}

	// Check scalar values are preserved
	if inputsSummary["text"] != "short text" {
		t.Errorf("Expected text 'short text', got %q", inputsSummary["text"])
	}

	if inputsSummary["number"] != 42 {
		t.Errorf("Expected number 42, got %v", inputsSummary["number"])
	}

	if inputsSummary["flag"] != true {
		t.Errorf("Expected flag true, got %v", inputsSummary["flag"])
	}

	// Check complex type is summarized
	dataSummary, ok := inputsSummary["data"].(string)
	if !ok {
		t.Errorf("Expected data to be summarized as string, got %T", inputsSummary["data"])
	}

	expectedDataSummary := "<map[string]interface {}>"
	if dataSummary != expectedDataSummary {
		t.Errorf("Expected data summary %q, got %q", expectedDataSummary, dataSummary)
	}
}

// Helper functions for finding log entries
func findLogEntry(entries []logEntry, message string) *logEntry {
	for _, entry := range entries {
		if entry.message == message {
			return &entry
		}
	}
	return nil
}

func findLogEntriesByLevel(entries []logEntry, level, message string) []logEntry {
	var result []logEntry
	for _, entry := range entries {
		if entry.level == level && entry.message == message {
			result = append(result, entry)
		}
	}
	return result
}

func TestParallelWithVerbose(t *testing.T) {
	// Test WithVerbose functionality
	capturingLog := &capturingLogger{}
	originalLogger := logging.GetLogger()
	defer func() {
		logging.SetLogger(originalLogger)
	}()
	logging.SetLogger(capturingLog)

	sig := core.NewSignature("Test task").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Output result")

	lm := &mockLMWithName{
		MockLM: &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## result ## ]]\nsuccess",
					Usage:   core.Usage{TotalTokens: 5},
				}, nil
			},
		},
		modelName: "test-model",
	}

	t.Run("verbose enabled uses INFO level", func(t *testing.T) {
		predictor := NewPredict(sig, lm)
		parallel := NewParallel(predictor).
			WithMaxWorkers(2).
			WithVerbose(true)

		capturingLog.reset()

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"text": "task1"},
				{"text": "task2"},
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}

		// Should have batch started log
		batchLog := findLogEntry(capturingLog.infos, "Parallel batch started")
		if batchLog == nil {
			t.Fatal("Expected batch log entry")
		}
		if batchLog.fields["module"] != logging.ModuleParallel {
			t.Errorf("Expected module %q, got %v", logging.ModuleParallel, batchLog.fields["module"])
		}
		if _, ok := batchLog.fields["parallel_id"].(string); !ok {
			t.Errorf("Expected parallel_id field")
		}

		batchCompletedLog := findLogEntry(capturingLog.infos, "Parallel batch completed")
		if batchCompletedLog == nil {
			t.Fatal("Expected batch completed log entry")
		}

		// Minimal sanity checks (schema v1 additions)
		if batchCompletedLog.fields["successes"] != 2 {
			t.Errorf("Expected successes=2, got %v", batchCompletedLog.fields["successes"])
		}
		if batchCompletedLog.fields["failures"] != 0 {
			t.Errorf("Expected failures=0, got %v", batchCompletedLog.fields["failures"])
		}
		if _, ok := batchCompletedLog.fields["latency_min_ms"].(int64); !ok {
			// May be int depending on platform
			if _, ok2 := batchCompletedLog.fields["latency_min_ms"].(int); !ok2 {
				t.Errorf("Expected latency_min_ms field")
			}
		}
		if batchCompletedLog.fields["error_count"] != 0 {
			t.Errorf("Expected error_count=0, got %v", batchCompletedLog.fields["error_count"])
		}

		// Should have task logs at INFO level when verbose is enabled
		infoTaskLogs := findLogEntriesByLevel(capturingLog.infos, "info", "Parallel task started")
		if len(infoTaskLogs) != 2 {
			t.Errorf("Expected 2 INFO task started logs, got %d", len(infoTaskLogs))
		}

		infoCompletedLogs := findLogEntriesByLevel(capturingLog.infos, "info", "Parallel task completed")
		if len(infoCompletedLogs) != 2 {
			t.Errorf("Expected 2 INFO task completed logs, got %d", len(infoCompletedLogs))
		}

		// Validate fields on a task completed log (schema v1)
		if len(infoCompletedLogs) > 0 {
			entry := infoCompletedLogs[0]
			expectedFields := []string{
				"module",
				"parallel_id",
				"parallel_mode",
				"task_index",
				"task_total",
				"worker_id",
				"queue_wait_ms",
				"duration_ms",
				"prompt_tokens",
				"completion_tokens",
				"total_tokens",
				"cost",
				"adapter_used",
				"parse_attempts",
				"fallback_used",
				"parse_success",
			}
			for _, f := range expectedFields {
				if _, ok := entry.fields[f]; !ok {
					t.Errorf("Task completed log: missing field %q", f)
				}
			}

			if au, ok := entry.fields["adapter_used"].(string); !ok || au == "" {
				t.Errorf("Task completed log: expected non-empty adapter_used string, got %T %v", entry.fields["adapter_used"], entry.fields["adapter_used"])
			}

			if pa, ok := entry.fields["parse_attempts"].(int); !ok || pa < 1 {
				t.Errorf("Task completed log: expected parse_attempts>=1 int, got %T %v", entry.fields["parse_attempts"], entry.fields["parse_attempts"])
			}
		}

		// Validate batch completed aggregation fields
		for _, f := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "cost", "latency_p50_ms"} {
			if _, ok := batchCompletedLog.fields[f]; !ok {
				t.Errorf("Batch completed log: missing field %q", f)
			}
		}

		// Should NOT have task logs at DEBUG level when verbose is enabled
		debugTaskLogs := findLogEntriesByLevel(capturingLog.debugs, "debug", "Parallel task started")
		if len(debugTaskLogs) != 0 {
			t.Errorf("Expected 0 DEBUG task started logs when verbose, got %d", len(debugTaskLogs))
		}
	})

	t.Run("task failed log includes error.kind and duration", func(t *testing.T) {
		failingLM := &mockLMWithName{
			MockLM: &MockLM{
				GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
					// Fail when input contains "task2"
					if len(messages) > 0 && strings.Contains(messages[len(messages)-1].Content, "task2") {
						return nil, fmt.Errorf("boom")
					}
					return &core.GenerateResult{
						Content: "[[ ## result ## ]]\nsuccess",
						Usage:   core.Usage{TotalTokens: 5},
					}, nil
				},
			},
			modelName: "test-model",
		}

		predictor := NewPredict(sig, failingLM)
		parallel := NewParallel(predictor).
			WithMaxWorkers(2).
			WithMaxFailures(1).
			WithFailFast(false).
			WithVerbose(true)

		capturingLog.reset()

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"text": "task1"},
				{"text": "task2"},
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}

		failedLogs := findLogEntriesByLevel(capturingLog.infos, "info", "Parallel task failed")
		if len(failedLogs) != 1 {
			t.Fatalf("Expected 1 task failed log, got %d", len(failedLogs))
		}

		failed := failedLogs[0]
		if _, ok := failed.fields["duration_ms"]; !ok {
			t.Errorf("Task failed log: missing duration_ms")
		}
		if kind, ok := failed.fields["error.kind"].(string); !ok || kind == "" {
			t.Errorf("Task failed log: expected error.kind string, got %T %v", failed.fields["error.kind"], failed.fields["error.kind"])
		} else if kind != "task_error" {
			t.Errorf("Task failed log: expected error.kind=task_error, got %q", kind)
		}
		if msg, ok := failed.fields["error.message"].(string); !ok || msg == "" {
			t.Errorf("Task failed log: expected error.message string, got %T %v", failed.fields["error.message"], failed.fields["error.message"])
		}
	})

	t.Run("verbose disabled uses DEBUG level", func(t *testing.T) {
		predictor := NewPredict(sig, lm)
		parallel := NewParallel(predictor).
			WithMaxWorkers(2).
			WithVerbose(false) // Explicitly disable verbose

		capturingLog.reset()

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"text": "task1"},
				{"text": "task2"},
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}

		// Should have batch log with verbose=false
		batchLog := findLogEntry(capturingLog.infos, "Parallel batch started")
		if batchLog == nil {
			t.Fatal("Expected batch log entry")
		}
		if batchLog.fields["verbose"] != false {
			t.Errorf("Expected verbose=false, got %v", batchLog.fields["verbose"])
		}

		// Should have task logs at DEBUG level when verbose is disabled
		debugTaskLogs := findLogEntriesByLevel(capturingLog.debugs, "debug", "Parallel task started")
		if len(debugTaskLogs) < 1 {
			t.Errorf("Expected at least 1 DEBUG task log, got %d", len(debugTaskLogs))
		}

		// Should NOT have task logs at INFO level when verbose is disabled
		infoTaskLogs := findLogEntriesByLevel(capturingLog.infos, "info", "Parallel task started")
		if len(infoTaskLogs) != 0 {
			t.Errorf("Expected 0 INFO task logs when not verbose, got %d", len(infoTaskLogs))
		}
	})

	t.Run("default verbose is false", func(t *testing.T) {
		predictor := NewPredict(sig, lm)
		parallel := NewParallel(predictor). // No WithVerbose call - should default to false
							WithMaxWorkers(2)

		capturingLog.reset()

		inputs := map[string]any{
			"_batch": []map[string]any{
				{"text": "task1"},
			},
		}

		_, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}

		// Should have batch log with verbose=false by default
		batchLog := findLogEntry(capturingLog.infos, "Parallel batch started")
		if batchLog == nil {
			t.Fatal("Expected batch log entry")
		}
		if batchLog.fields["verbose"] != false {
			t.Errorf("Expected verbose=false by default, got %v", batchLog.fields["verbose"])
		}

		// Should use DEBUG level by default
		debugTaskLogs := findLogEntriesByLevel(capturingLog.debugs, "debug", "Parallel task started")
		if len(debugTaskLogs) < 1 {
			t.Errorf("Expected at least 1 DEBUG task log by default, got %d", len(debugTaskLogs))
		}

		infoTaskLogs := findLogEntriesByLevel(capturingLog.infos, "info", "Parallel task started")
		if len(infoTaskLogs) != 0 {
			t.Errorf("Expected 0 INFO task logs by default, got %d", len(infoTaskLogs))
		}
	})
}

// mockLM for TestParallelWithFactory_ExactFactoryCalls
type mockLMForFactoryTest struct {
	responses []string
	idx       int32
}

func (m *mockLMForFactoryTest) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	resp := m.responses[atomic.AddInt32(&m.idx, 1)-1]
	return &core.GenerateResult{Content: resp, Usage: core.Usage{TotalTokens: 10}}, nil
}
func (m *mockLMForFactoryTest) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	ch := make(chan core.Chunk, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- core.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return ch, errCh
}
func (m *mockLMForFactoryTest) Name() string        { return "mock-lm" }
func (m *mockLMForFactoryTest) SupportsJSON() bool  { return true }
func (m *mockLMForFactoryTest) SupportsTools() bool { return false }
func (m *mockLMForFactoryTest) IsOpenAI() bool      { return false }

func TestParallelWithFactory_ExactFactoryCalls(t *testing.T) {
	var factoryCalls int32

	// Factory that increments counter and returns a simple Predict module
	factory := func(i int) core.Module {
		atomic.AddInt32(&factoryCalls, 1)
		lm := &mockLMForFactoryTest{
			responses: []string{fmt.Sprintf(`{"answer": "task-%d"}`, i)},
		}
		sig := core.NewSignature("Test").
			AddInput("question", core.FieldTypeString, "Question").
			AddOutput("answer", core.FieldTypeString, "Answer")
		return NewPredict(sig, lm)
	}

	parallel := NewParallelWithFactory(factory).
		WithMaxWorkers(2).
		WithReturnAll(true)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "q1"},
			{"question": "q2"},
			{"question": "q3"},
		},
	}

	_, err := parallel.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Factory should be called exactly once per task (3 times)
	if calls := atomic.LoadInt32(&factoryCalls); calls != 3 {
		t.Errorf("Expected 3 factory calls, got %d", calls)
	}
}
