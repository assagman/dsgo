package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestProgram_Forward_Success(t *testing.T) {
	t.Parallel()
	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step1": "done"}), nil
		},
		SignatureValue: core.NewSignature("Module1"),
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			if inputs["step1"] != "done" {
				t.Error("Module2 should receive step1 output")
			}
			return core.NewPrediction(map[string]interface{}{"step2": "complete"}), nil
		},
		SignatureValue: core.NewSignature("Module2"),
	}

	program := NewProgram("test-program").
		AddModule(module1).
		AddModule(module2)

	outputs, err := program.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// With refactored Program, only last module's outputs are returned
	if outputs.Outputs["step1"] == "done" {
		t.Error("Should NOT include output from first module (no synthetic merge)")
	}

	if outputs.Outputs["step2"] != "complete" {
		t.Error("Should include output from last module")
	}
}

func TestProgram_Forward_NoModules(t *testing.T) {
	t.Parallel()
	program := NewProgram("empty")

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error when program has no modules")
	}
}

func TestProgram_Forward_ModuleError(t *testing.T) {
	t.Parallel()
	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"result": "ok"}), nil
		},
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return nil, errors.New("module2 error")
		},
	}

	program := NewProgram("test").AddModule(module1).AddModule(module2)

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should propagate module error")
	}
}

func TestProgram_GetSignature(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("LastModule")
	module := &MockModule{SignatureValue: sig}

	program := NewProgram("test").AddModule(module)

	if program.GetSignature() != sig {
		t.Error("GetSignature should return last module's signature")
	}
}

func TestProgram_GetSignature_NoModules(t *testing.T) {
	t.Parallel()
	program := NewProgram("empty")

	if program.GetSignature() != nil {
		t.Error("GetSignature should return nil for empty program")
	}
}

func TestProgram_Name(t *testing.T) {
	t.Parallel()
	program := NewProgram("my-program")

	if program.Name() != "my-program" {
		t.Errorf("Expected name 'my-program', got '%s'", program.Name())
	}
}

func TestProgram_ModuleCount(t *testing.T) {
	t.Parallel()
	program := NewProgram("test")

	if program.ModuleCount() != 0 {
		t.Error("New program should have 0 modules")
	}

	program.AddModule(&MockModule{})
	program.AddModule(&MockModule{})

	if program.ModuleCount() != 2 {
		t.Errorf("Expected 2 modules, got %d", program.ModuleCount())
	}
}

func TestProgram_InputMerging(t *testing.T) {
	t.Parallel()
	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			if inputs["original"] != "value" {
				t.Error("Module1 should receive original input")
			}
			return core.NewPrediction(map[string]interface{}{"intermediate": "result"}), nil
		},
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			if inputs["original"] != "value" {
				t.Error("Module2 should still have access to original input")
			}
			if inputs["intermediate"] != "result" {
				t.Error("Module2 should have module1's output")
			}
			return core.NewPrediction(map[string]interface{}{"final": "done"}), nil
		},
	}

	program := NewProgram("test").AddModule(module1).AddModule(module2)

	_, err := program.Forward(context.Background(), map[string]interface{}{
		"original": "value",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestProgram_Forward_ValidationSuccess(t *testing.T) {
	t.Parallel()
	// Module 1 produces valid JSON output
	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{
				"activities": []interface{}{"Visit temple", "Try ramen"}, // Valid JSON array
			}), nil
		},
		SignatureValue: core.NewSignature("Module1").
			AddOutput("activities", core.FieldTypeJSON, "List of activities"),
	}

	// Module 2 consumes the validated output
	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			activities, ok := inputs["activities"]
			if !ok {
				t.Error("Module2 should receive activities from Module1")
			}

			// Should be a slice (JSON array)
			if _, ok := activities.([]interface{}); !ok {
				t.Errorf("activities should be []interface{}, got %T", activities)
			}

			return core.NewPrediction(map[string]interface{}{"result": "done"}), nil
		},
		SignatureValue: core.NewSignature("Module2"),
	}

	program := NewProgram("test-validation-success").
		AddModule(module1).
		AddModule(module2)

	pred, err := program.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() should succeed with valid outputs, got error: %v", err)
	}

	if pred.Outputs["result"] != "done" {
		t.Error("Should complete full pipeline")
	}
}

func TestProgram_SignatureValidation_Success(t *testing.T) {
	t.Parallel()

	// Create compatible signatures
	sig1 := core.NewSignature("Module1").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("sentiment", core.FieldTypeString, "Sentiment analysis")

	sig2 := core.NewSignature("Module2").
		AddInput("sentiment", core.FieldTypeString, "Sentiment from module1").
		AddOutput("summary", core.FieldTypeString, "Summary")

	module1 := &MockModule{SignatureValue: sig1}
	module2 := &MockModule{SignatureValue: sig2}

	program := NewProgram("test")
	program.AddModule(module1)
	program.AddModule(module2)

	// Should validate successfully with proper inputs
	err := program.ValidateSignatures([]string{"text"})
	if err != nil {
		t.Fatalf("ValidateSignatures() should succeed, got error: %v", err)
	}
}

func TestProgram_SignatureValidation_MissingInput(t *testing.T) {
	t.Parallel()

	// Create incompatible signatures
	sig1 := core.NewSignature("Module1").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("analysis", core.FieldTypeString, "Analysis")

	sig2 := core.NewSignature("Module2").
		AddInput("sentiment", core.FieldTypeString, "Required sentiment"). // Not provided by module1
		AddOutput("summary", core.FieldTypeString, "Summary")

	module1 := &MockModule{SignatureValue: sig1}
	module2 := &MockModule{SignatureValue: sig2}

	program := NewProgram("test")
	program.AddModule(module1)
	program.AddModule(module2)

	// Should fail validation
	err := program.ValidateSignatures([]string{"text"})
	if err == nil {
		t.Fatal("ValidateSignatures() should fail with missing input")
	}

	// Check error type
	mismatchErr, ok := err.(*SignatureMismatch)
	if !ok {
		t.Fatalf("Expected SignatureMismatch error, got %T", err)
	}

	if mismatchErr.ModuleIndex != 1 {
		t.Errorf("Expected module index 1, got %d", mismatchErr.ModuleIndex)
	}

	if len(mismatchErr.MissingInputs) != 1 || mismatchErr.MissingInputs[0] != "sentiment" {
		t.Errorf("Expected missing input 'sentiment', got %v", mismatchErr.MissingInputs)
	}
}

func TestProgram_AddModuleValidated_Success(t *testing.T) {
	t.Parallel()

	sig1 := core.NewSignature("Module1").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	module1 := &MockModule{SignatureValue: sig1}

	program := NewProgram("test")

	// Should succeed with valid inputs
	err := program.AddModuleValidated(module1, []string{"text"})
	if err != nil {
		t.Fatalf("AddModuleValidated() should succeed, got error: %v", err)
	}

	if program.ModuleCount() != 1 {
		t.Error("Module should be added successfully")
	}
}

func TestProgram_AddModuleValidated_Failure(t *testing.T) {
	t.Parallel()

	sig1 := core.NewSignature("Module1").
		AddInput("missing", core.FieldTypeString, "Missing input").
		AddOutput("result", core.FieldTypeString, "Result")

	module1 := &MockModule{SignatureValue: sig1}

	program := NewProgram("test")

	// Should fail with missing inputs
	err := program.AddModuleValidated(module1, []string{"text"})
	if err == nil {
		t.Fatal("AddModuleValidated() should fail with missing input")
	}

	// Should not add the module (rollback)
	if program.ModuleCount() != 0 {
		t.Error("Module should not be added when validation fails")
	}
}

func TestProgram_ExecutionTrace_Complete(t *testing.T) {
	t.Parallel()

	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step1": "done"}).
				WithUsage(core.Usage{PromptTokens: 10, CompletionTokens: 5}), nil
		},
		SignatureValue: core.NewSignature("Module1"),
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step2": "complete"}).
				WithUsage(core.Usage{PromptTokens: 15, CompletionTokens: 10}), nil
		},
		SignatureValue: core.NewSignature("Module2"),
	}

	program := NewProgram("test-trace").AddModule(module1).AddModule(module2)

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	execution := program.GetExecution()
	if execution == nil {
		t.Fatal("GetExecution() should return execution trace")
	}

	if execution.Status != ExecutionStatusCompleted {
		t.Errorf("Expected status %s, got %s", ExecutionStatusCompleted, execution.Status)
	}

	if len(execution.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(execution.Steps))
	}

	// Check first step
	step1 := execution.Steps[0]
	if step1.Status != StepStatusCompleted {
		t.Errorf("Expected step1 status %s, got %s", StepStatusCompleted, step1.Status)
	}
	if step1.ModuleName != "Module1" {
		t.Errorf("Expected step1 module name 'Module1', got %s", step1.ModuleName)
	}
	if step1.Prediction == nil {
		t.Error("Step1 should have prediction")
	}

	// Check total usage
	if execution.TotalUsage.PromptTokens != 25 {
		t.Errorf("Expected total prompt tokens 25, got %d", execution.TotalUsage.PromptTokens)
	}
	if execution.TotalUsage.CompletionTokens != 15 {
		t.Errorf("Expected total completion tokens 15, got %d", execution.TotalUsage.CompletionTokens)
	}
}

func TestProgram_ExecutionTrace_PartialFailure(t *testing.T) {
	t.Parallel()

	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step1": "done"}), nil
		},
		SignatureValue: core.NewSignature("Module1"),
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return nil, errors.New("module2 failed")
		},
		SignatureValue: core.NewSignature("Module2"),
	}

	module3 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step3": "never reached"}), nil
		},
		SignatureValue: core.NewSignature("Module3"),
	}

	program := NewProgram("test-failure").AddModule(module1).AddModule(module2).AddModule(module3)

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("Forward() should fail")
	}

	execution := program.GetExecution()
	if execution == nil {
		t.Fatal("GetExecution() should return execution trace")
	}

	if execution.Status != ExecutionStatusFailed {
		t.Errorf("Expected status %s, got %s", ExecutionStatusFailed, execution.Status)
	}

	// Check step statuses
	if execution.Steps[0].Status != StepStatusCompleted {
		t.Error("Step1 should be completed")
	}
	if execution.Steps[1].Status != StepStatusFailed {
		t.Error("Step2 should be failed")
	}
	if execution.Steps[2].Status != StepStatusSkipped {
		t.Error("Step3 should be skipped")
	}
}

func TestProgram_GetMetrics(t *testing.T) {
	t.Parallel()

	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"step1": "done"}), nil
		},
		SignatureValue: core.NewSignature("Module1"),
	}

	program := NewProgram("test-metrics").AddModule(module1)

	// Should return nil before execution
	metrics := program.GetMetrics()
	if metrics != nil {
		t.Error("GetMetrics() should return nil before execution")
	}

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	metrics = program.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics() should return metrics after execution")
	}

	if metrics.TotalSteps != 1 {
		t.Errorf("Expected total steps 1, got %d", metrics.TotalSteps)
	}
	if metrics.CompletedSteps != 1 {
		t.Errorf("Expected completed steps 1, got %d", metrics.CompletedSteps)
	}
}

func TestProgram_LastPredictionPassthrough(t *testing.T) {
	t.Parallel()

	// Create modules with different outputs
	module1 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"intermediate": "value1"}), nil
		},
		SignatureValue: core.NewSignature("Module1"),
	}

	module2 := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			return core.NewPrediction(map[string]interface{}{"final": "value2"}).
				WithRationale("final rationale"), nil
		},
		SignatureValue: core.NewSignature("Module2"),
	}

	program := NewProgram("test-passthrough").AddModule(module1).AddModule(module2)

	result, err := program.Forward(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Should return LAST module's outputs, not merged
	if len(result.Outputs) != 1 {
		t.Errorf("Expected 1 output from last module, got %d", len(result.Outputs))
	}

	if result.Outputs["final"] != "value2" {
		t.Error("Should contain final module's output")
	}

	if result.Outputs["intermediate"] == "value1" {
		t.Error("Should NOT contain intermediate module's output (no synthetic merge)")
	}

	if result.Rationale != "final rationale" {
		t.Error("Should preserve rationale from last module")
	}

	if result.ModuleName != "test-passthrough" {
		t.Error("Should set module name to program name")
	}
}

func TestProgram_CompletionsPassthrough_Interface(t *testing.T) {
	t.Parallel()

	// Mock module that produces completions
	producer := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			completions := []map[string]interface{}{
				{"answer": "answer1", "rationale": "rationale1"},
				{"answer": "answer2", "rationale": "rationale2"},
			}
			return core.NewPrediction(map[string]interface{}{"output": "from producer"}).
				WithCompletions(completions), nil
		},
		SignatureValue: core.NewSignature("Producer"),
	}

	// Create MultiChainComparison to consume completions
	sig := core.NewSignature("Consumer").
		AddInput("text", core.FieldTypeString, "Text input").
		AddOutput("summary", core.FieldTypeString, "Summary")

	consumer := NewMultiChainComparison(sig, nil, 2) // LM can be nil for test

	program := NewProgram("test-completions").AddModule(producer).AddModule(consumer)

	// Verify consumer requires completions
	if !consumer.RequiresCompletions() {
		t.Error("MultiChainComparison should require completions")
	}

	// Test buildNextInputs logic
	producerResult := core.NewPrediction(map[string]interface{}{"output": "test"}).
		WithCompletions([]map[string]interface{}{{"answer": "test"}})

	nextInputs := program.buildNextInputs(map[string]interface{}{"text": "input"}, producerResult, 0)

	if nextInputs["completions"] == nil {
		t.Error("Should pass completions to next module that requires them")
	}
}

func TestProgram_NilInputs(t *testing.T) {
	t.Parallel()

	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			// Should receive non-nil map even if input was nil
			if inputs == nil {
				t.Error("Module should receive non-nil inputs map")
			}
			return core.NewPrediction(map[string]interface{}{"output": "ok"}), nil
		},
		SignatureValue: core.NewSignature("Module"),
	}

	program := NewProgram("test-nil").AddModule(module)

	_, err := program.Forward(context.Background(), nil)
	if err != nil {
		t.Fatalf("Forward() should handle nil inputs, got error: %v", err)
	}
}

func TestProgram_ConcurrentGetExecution(t *testing.T) {
	t.Parallel()

	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*core.Prediction, error) {
			// Simulate some work
			return core.NewPrediction(map[string]interface{}{"output": "done"}), nil
		},
		SignatureValue: core.NewSignature("Module"),
	}

	program := NewProgram("test-concurrent").AddModule(module)

	// Start execution in goroutine
	done := make(chan error, 1)
	go func() {
		_, err := program.Forward(context.Background(), map[string]interface{}{})
		done <- err
	}()

	// Concurrently access execution trace
	execution := program.GetExecution()
	// Should not race - execution may be nil or in-progress, but shouldn't crash
	_ = execution

	err := <-done
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// After completion, should be able to access execution trace
	execution = program.GetExecution()
	if execution == nil {
		t.Error("GetExecution() should return trace after completion")
	}
}
