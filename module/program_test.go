package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestProgram_Forward_Success(t *testing.T) {
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

	if outputs.Outputs["step1"] != "done" {
		t.Error("Should include output from first module")
	}

	if outputs.Outputs["step2"] != "complete" {
		t.Error("Should include output from second module")
	}
}

func TestProgram_Forward_NoModules(t *testing.T) {
	program := NewProgram("empty")

	_, err := program.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error when program has no modules")
	}
}

func TestProgram_Forward_ModuleError(t *testing.T) {
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
	sig := core.NewSignature("LastModule")
	module := &MockModule{SignatureValue: sig}

	program := NewProgram("test").AddModule(module)

	if program.GetSignature() != sig {
		t.Error("GetSignature should return last module's signature")
	}
}

func TestProgram_GetSignature_NoModules(t *testing.T) {
	program := NewProgram("empty")

	if program.GetSignature() != nil {
		t.Error("GetSignature should return nil for empty program")
	}
}

func TestProgram_Name(t *testing.T) {
	program := NewProgram("my-program")

	if program.Name() != "my-program" {
		t.Errorf("Expected name 'my-program', got '%s'", program.Name())
	}
}

func TestProgram_ModuleCount(t *testing.T) {
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
