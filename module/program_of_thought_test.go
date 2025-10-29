package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestProgramOfThought_Forward_Success(t *testing.T) {
	sig := dsgo.NewSignature("Solve math").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"code": "print(2+2)", "explanation": "Add 2+2", "answer": "4"}`,
			}, nil
		},
	}

	pot := NewProgramOfThought(sig, lm, "python")
	outputs, err := pot.Forward(context.Background(), map[string]interface{}{
		"problem": "What is 2+2?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "4" {
		t.Errorf("Expected answer='4', got %v", outputs.Outputs["answer"])
	}

	if _, exists := outputs.Outputs["code"]; !exists {
		t.Error("Should include code field")
	}
}

func TestProgramOfThought_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	pot := NewProgramOfThought(sig, lm, "python")

	_, err := pot.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestProgramOfThought_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("problem", dsgo.FieldTypeString, "Problem")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	pot := NewProgramOfThought(sig, lm, "python")
	_, err := pot.Forward(context.Background(), map[string]interface{}{
		"problem": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestProgramOfThought_WithOptions(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	pot := NewProgramOfThought(sig, lm, "python")

	customOpts := &dsgo.GenerateOptions{Temperature: 0.5}
	pot.WithOptions(customOpts)

	if pot.Options.Temperature != 0.5 {
		t.Error("WithOptions should set custom options")
	}
}

func TestProgramOfThought_WithAllowExecution(t *testing.T) {
	pot := NewProgramOfThought(dsgo.NewSignature("Test"), &MockLM{}, "python")

	if pot.AllowExecution {
		t.Error("Execution should be disabled by default")
	}

	pot.WithAllowExecution(true)

	if !pot.AllowExecution {
		t.Error("WithAllowExecution should enable execution")
	}
}

func TestProgramOfThought_WithExecutionTimeout(t *testing.T) {
	pot := NewProgramOfThought(dsgo.NewSignature("Test"), &MockLM{}, "python")
	pot.WithExecutionTimeout(60)

	if pot.ExecutionTimeout != 60 {
		t.Error("WithExecutionTimeout should set timeout")
	}
}

func TestProgramOfThought_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	pot := NewProgramOfThought(sig, &MockLM{}, "python")

	if pot.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestProgramOfThought_Language(t *testing.T) {
	tests := []string{"python", "javascript", "go"}

	for _, lang := range tests {
		pot := NewProgramOfThought(dsgo.NewSignature("Test"), &MockLM{}, lang)
		if pot.Language != lang {
			t.Errorf("Expected language '%s', got '%s'", lang, pot.Language)
		}
	}
}

func TestProgramOfThought_ExecuteCode_UnsupportedLanguage(t *testing.T) {
	pot := NewProgramOfThought(dsgo.NewSignature("Test"), &MockLM{}, "unsupported")

	_, err := pot.executeCode(context.Background(), "some code")
	if err == nil {
		t.Error("executeCode should error on unsupported language")
	}
}

func TestProgramOfThought_ExecuteCode_GoNotSupported(t *testing.T) {
	pot := NewProgramOfThought(dsgo.NewSignature("Test"), &MockLM{}, "go")

	_, err := pot.executeCode(context.Background(), "package main")
	if err == nil {
		t.Error("executeCode should error on Go (not yet supported)")
	}
}

func TestProgramOfThought_BuildPrompt_NoDescription(t *testing.T) {
	sig := dsgo.NewSignature("").
		AddInput("problem", dsgo.FieldTypeString, "Problem")

	pot := NewProgramOfThought(sig, &MockLM{}, "python")

	prompt, err := pot.buildPrompt(map[string]interface{}{
		"problem": "test",
	})

	if err != nil {
		t.Fatalf("buildPrompt() error = %v", err)
	}

	if !contains(prompt, "python") {
		t.Error("Prompt should mention language")
	}
}

func TestProgramOfThought_Forward_WithCodeExecution(t *testing.T) {
	sig := dsgo.NewSignature("Calculate").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"code": "print('2+2=4')", "answer": "4"}`,
			}, nil
		},
	}

	pot := NewProgramOfThought(sig, lm, "python").WithAllowExecution(true)
	outputs, err := pot.Forward(context.Background(), map[string]interface{}{
		"problem": "2+2",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if _, exists := outputs.Outputs["execution_result"]; !exists {
		t.Log("execution_result field expected when execution enabled")
	}
}

func TestProgramOfThought_BuildPrompt_NoOutputFields(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("problem", dsgo.FieldTypeString, "Problem")

	pot := NewProgramOfThought(sig, &MockLM{}, "python")

	prompt, err := pot.buildPrompt(map[string]interface{}{
		"problem": "test",
	})

	if err != nil {
		t.Fatalf("buildPrompt() error = %v", err)
	}

	if !contains(prompt, "code") {
		t.Error("Prompt should request code even without explicit output fields")
	}
}

func TestProgramOfThought_Forward_WithCodeExecutionError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"code": "syntax error!", "answer": "42"}`,
			}, nil
		},
	}

	pot := NewProgramOfThought(sig, lm, "python").WithAllowExecution(true)
	outputs, err := pot.Forward(context.Background(), map[string]interface{}{
		"problem": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should not fail on execution error: %v", err)
	}

	if _, exists := outputs.Outputs["execution_error"]; !exists {
		t.Error("Should include execution_error when code execution fails")
	}
}
