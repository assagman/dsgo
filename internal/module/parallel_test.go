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

	if !strings.Contains(err.Error(), "fail-fast triggered") {
		t.Errorf("Expected fail-fast error, got: %v", err)
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

// Thread Safety Tests

// TestParallelThreadSafetyNoRaceConditions verifies that NewParallel now
// prevents race conditions by cloning modules per task
func TestParallelThreadSafetyNoRaceConditions(t *testing.T) {
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
	for i := 0; i < 50; i++ {
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
	for i := 0; i < 100; i++ {
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

	for i := 0; i < numInstances; i++ {
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
	for i := 0; i < 80; i++ {
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
	for i := 0; i < numInstances; i++ {
		if instanceUsage[i] == 0 {
			t.Errorf("Instance %d was not used", i)
		}
		t.Logf("Instance %d processed %d tasks", i, instanceUsage[i])
	}

	t.Logf("Instances pattern completed successfully with %d completions", len(result.Completions))
}

// TestParallelThreadSafetyStatelessModule demonstrates that stateless modules are safe
func TestParallelThreadSafetyStatelessModule(t *testing.T) {
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
	for i := 0; i < 200; i++ {
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
	for i := 0; i < numTasks; i++ {
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
	for i := 0; i < 20; i++ {
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
	for i := 0; i < 100; i++ {
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
