package module

import (
	"context"
	"fmt"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestProgram_VerboseLogging(t *testing.T) {
	sig := core.NewSignature("Test module").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": "success"},
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
		SignatureValue: sig,
	}

	// Test verbose=true (should not crash)
	verboseProgram := NewProgram("Verbose Test Program").
		WithVerbose(true).
		AddModule(mockModule)

	ctx := context.Background()
	_, err := verboseProgram.Forward(ctx, map[string]any{"text": "test"})
	if err != nil {
		t.Errorf("Unexpected error with verbose logging: %v", err)
	}

	// Test verbose=false (default)
	quietProgram := NewProgram("Quiet Test Program").
		WithVerbose(false).
		AddModule(mockModule)

	_, err = quietProgram.Forward(ctx, map[string]any{"text": "test"})
	if err != nil {
		t.Errorf("Unexpected error with quiet logging: %v", err)
	}
}

func TestProgram_ExecutionIDAndRetention(t *testing.T) {
	sig := core.NewSignature("Test module").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			text := fmt.Sprintf("%v", inputs["text"])
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": text},
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
		SignatureValue: sig,
	}

	program := NewProgram("Retention Test Program").
		WithExecutionRetention(2). // Keep only 2 executions
		AddModule(mockModule)

	ctx := context.Background()

	// Run multiple executions
	var executionIDs []ExecutionID
	for i := 0; i < 3; i++ {
		_, err := program.Forward(ctx, map[string]any{"text": i})
		if err != nil {
			t.Errorf("Execution %d failed: %v", i, err)
			continue
		}

		id := program.GetLastExecutionID()
		if id == "" {
			t.Errorf("Expected execution ID for run %d", i)
		}
		executionIDs = append(executionIDs, id)
	}

	// Check that only last 2 executions are retained
	allIDs := program.GetAllExecutionIDs()
	if len(allIDs) != 2 {
		t.Errorf("Expected 2 retained executions, got %d: %v", len(allIDs), allIDs)
	}

	// Check that last two are ones retained (order may vary due to map iteration)
	if len(allIDs) >= 2 {
		// Just check that we have exactly 2 executions and they're from the last 2 runs
		retainedSet := make(map[string]bool)
		for _, id := range allIDs {
			retainedSet[string(id)] = true
		}

		expectedIDs := []string{string(executionIDs[1]), string(executionIDs[2])}
		for _, expectedID := range expectedIDs {
			if !retainedSet[expectedID] {
				t.Errorf("Expected to retain execution %s, got %v", expectedID, allIDs)
			}
		}
	}

	// Test accessing by ID
	for _, id := range allIDs {
		execution := program.GetExecutionByID(id)
		if execution == nil {
			t.Errorf("Expected to find execution %s", id)
			continue
		}
		if execution.ID != id {
			t.Errorf("Execution ID mismatch, expected %s, got %s", id, execution.ID)
		}
	}

	// Test accessing non-existent execution
	nonExistent := program.GetExecutionByID(ExecutionID("non-existent"))
	if nonExistent != nil {
		t.Errorf("Expected nil for non-existent execution")
	}
}

func TestProgram_ForwardWithTrace(t *testing.T) {
	sig := core.NewSignature("Test module").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": "success"},
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
		SignatureValue: sig,
	}

	program := NewProgram("Trace Test Program").AddModule(mockModule)
	ctx := context.Background()
	result, err := program.ForwardWithTrace(ctx, map[string]any{"text": "test"})
	if err != nil {
		t.Fatalf("ForwardWithTrace failed: %v", err)
	}

	if result.Prediction == nil {
		t.Error("Expected prediction in result")
	}
	if result.Execution == nil {
		t.Error("Expected execution in result")
	}
	if result.ExecutionID == "" {
		t.Error("Expected execution ID in result")
	}
	if result.ExecutionID != result.Execution.ID {
		t.Errorf("Execution ID mismatch: %v != %v", result.ExecutionID, result.Execution.ID)
	}
}

func TestProgram_GetStepData(t *testing.T) {
	sig := core.NewSignature("Test module").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": "step-success"},
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
		SignatureValue: sig,
	}

	program := NewProgram("Step Data Test Program").AddModule(mockModule)
	ctx := context.Background()
	result, _ := program.ForwardWithTrace(ctx, map[string]any{"text": "test"})
	execID := result.ExecutionID

	// Test GetStepPrediction
	pred, err := program.GetStepPrediction(execID, 0)
	if err != nil {
		t.Errorf("GetStepPrediction failed: %v", err)
	}
	if pred.Outputs["result"] != "step-success" {
		t.Errorf("Expected step-success, got %v", pred.Outputs["result"])
	}

	// Test GetStepOutput
	val, err := program.GetStepOutput(execID, 0, "result")
	if err != nil {
		t.Errorf("GetStepOutput failed: %v", err)
	}
	if val != "step-success" {
		t.Errorf("Expected step-success, got %v", val)
	}

	// Test GetLastStepPrediction
	pred, err = program.GetLastStepPrediction(0)
	if err != nil {
		t.Errorf("GetLastStepPrediction failed: %v", err)
	}
	if pred.Outputs["result"] != "step-success" {
		t.Errorf("Expected step-success, got %v", pred.Outputs["result"])
	}

	// Test GetLastStepOutput
	val, err = program.GetLastStepOutput(0, "result")
	if err != nil {
		t.Errorf("GetLastStepOutput failed: %v", err)
	}
	if val != "step-success" {
		t.Errorf("Expected step-success, got %v", val)
	}

	// Edge case: out-of-bounds
	_, err = program.GetStepPrediction(execID, 1)
	if err == nil {
		t.Error("Expected error for out-of-bounds step index")
	}

	// Edge case: non-existent key
	_, err = program.GetStepOutput(execID, 0, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent output key")
	}

	// Edge case: non-existent execution
	_, err = program.GetStepPrediction(ExecutionID("invalid"), 0)
	if err == nil {
		t.Error("Expected error for invalid execution ID")
	}
}

func TestProgram_CloneFull(t *testing.T) {
	p := NewProgram("Clone Test").
		WithVerbose(true).
		WithInputs([]string{"input1"}).
		WithExecutionRetention(5)

	// Add a module
	mock := &MockModule{SignatureValue: core.NewSignature("Mock")}
	p.AddModule(mock)

	// Run once to populate state
	ctx := context.Background()
	_, _ = p.Forward(ctx, map[string]any{"input1": "val"})

	if len(p.executionOrder) == 0 {
		t.Fatal("Expected executions in original program")
	}

	clonedMod := p.Clone()
	cloned, ok := clonedMod.(*Program)
	if !ok {
		t.Fatal("Clone did not return *Program")
	}

	if cloned.name != p.name {
		t.Errorf("Name not cloned: %s != %s", cloned.name, p.name)
	}
	if cloned.verbose != p.verbose {
		t.Errorf("Verbose not cloned: %v != %v", cloned.verbose, p.verbose)
	}
	if cloned.retentionSize != p.retentionSize {
		t.Errorf("RetentionSize not cloned: %d != %d", cloned.retentionSize, p.retentionSize)
	}
	if len(cloned.baselineInputs) != len(p.baselineInputs) || cloned.baselineInputs[0] != p.baselineInputs[0] {
		t.Errorf("BaselineInputs not cloned correctly")
	}

	// Verify state reset
	if len(cloned.executionStore) != 0 {
		t.Error("executionStore should be empty in clone")
	}
	if len(cloned.executionOrder) != 0 {
		t.Error("executionOrder should be empty in clone")
	}
	if cloned.nextExecutionID != 0 {
		t.Errorf("nextExecutionID should be 0 in clone, got %d", cloned.nextExecutionID)
	}
}

func TestProgram_DeepCopyCorrectness(t *testing.T) {
	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": "original"},
			}, nil
		},
	}

	program := NewProgram("Deep Copy Test").AddModule(mockModule)
	ctx := context.Background()

	// 1. Check Forward return value
	pred, _ := program.Forward(ctx, nil)
	pred.Outputs["result"] = "mutated"

	exec := program.GetExecution()
	if exec.Steps[0].Prediction.Outputs["result"] != "original" {
		t.Error("Mutation of Forward() result affected stored execution trace")
	}

	// 2. Check GetStepPrediction return value
	execID := program.GetLastExecutionID()
	pred2, _ := program.GetStepPrediction(execID, 0)
	pred2.Outputs["result"] = "mutated-again"

	exec2 := program.GetExecution()
	if exec2.Steps[0].Prediction.Outputs["result"] != "original" {
		t.Error("Mutation of GetStepPrediction() result affected stored execution trace")
	}
}

func TestProgram_RetentionValidation(t *testing.T) {
	p := NewProgram("Retention Validation")

	p.WithExecutionRetention(-5)
	if p.retentionSize != 0 {
		t.Errorf("Expected retentionSize 0 for negative input, got %d", p.retentionSize)
	}

	p.WithExecutionRetention(10)
	if p.retentionSize != 10 {
		t.Errorf("Expected retentionSize 10, got %d", p.retentionSize)
	}
}

func TestProgram_ConcurrentAccess(t *testing.T) {
	sig := core.NewSignature("Concurrent test").
		AddInput("id", core.FieldTypeInt, "Request ID").
		AddOutput("result", core.FieldTypeString, "Result")

	mockModule := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]interface{}{"result": "done"},
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
		SignatureValue: sig,
	}

	program := NewProgram("Concurrent Test").
		WithExecutionRetention(100).
		AddModule(mockModule)

	ctx := context.Background()
	const numGoroutines = 50
	done := make(chan bool, numGoroutines)

	// Spawn goroutines that execute and read concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Execute with trace to get the execution ID for THIS execution
			result, err := program.ForwardWithTrace(ctx, map[string]any{"id": id})
			if err != nil {
				t.Errorf("Goroutine %d: ForwardWithTrace failed: %v", id, err)
				return
			}

			execID := result.ExecutionID

			// Read execution trace and mutate returned copy
			exec := program.GetExecution()
			if exec != nil && len(exec.Steps) > 0 && exec.Steps[0].Prediction != nil {
				// Mutate returned trace - should NOT affect internal state
				exec.Steps[0].Prediction.Outputs["result"] = "mutated"
			}

			// Read by ID
			_ = program.GetExecutionByID(execID)

			// List all IDs
			_ = program.GetAllExecutionIDs()

			// Get step data
			_, _ = program.GetStepPrediction(execID, 0)
			_, _ = program.GetStepOutput(execID, 0, "result")
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all stored execution IDs are unique
	allIDs := program.GetAllExecutionIDs()
	idSet := make(map[ExecutionID]bool)
	for _, id := range allIDs {
		if idSet[id] {
			t.Errorf("Duplicate execution ID in store: %s", id)
		}
		idSet[id] = true
	}

	// Verify no internal state corruption from mutations
	for _, id := range allIDs {
		exec := program.GetExecutionByID(id)
		if exec != nil && len(exec.Steps) > 0 && exec.Steps[0].Prediction != nil {
			if exec.Steps[0].Prediction.Outputs["result"] != "done" {
				t.Errorf("Internal state corrupted for execution %s: got %v", id, exec.Steps[0].Prediction.Outputs["result"])
			}
		}
	}

	t.Logf("Successfully ran %d concurrent executions with %d unique IDs retained", numGoroutines, len(allIDs))
}
