package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestChainOfThought_Forward_Success(t *testing.T) {
	sig := dsgo.NewSignature("Solve problem").
		AddInput("problem", dsgo.FieldTypeString, "The problem").
		AddOutput("answer", dsgo.FieldTypeString, "The answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"reasoning": "Step 1... Step 2...", "answer": "42"}`,
			}, nil
		},
	}

	cot := NewChainOfThought(sig, lm)
	outputs, err := cot.Forward(context.Background(), map[string]interface{}{
		"problem": "What is 6*7?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	if reasoning, ok := outputs["reasoning"]; !ok || reasoning == "" {
		t.Error("ChainOfThought should include reasoning")
	}
}

func TestChainOfThought_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	cot := NewChainOfThought(sig, lm)

	_, err := cot.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestChainOfThought_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("problem", dsgo.FieldTypeString, "Problem")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	cot := NewChainOfThought(sig, lm)
	_, err := cot.Forward(context.Background(), map[string]interface{}{
		"problem": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestChainOfThought_ParseOutput_CodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	cot := NewChainOfThought(sig, nil)

	content := "```json\n{\"reasoning\": \"thinking\", \"answer\": \"result\"}\n```"
	outputs, err := cot.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "result" {
		t.Errorf("Expected answer='result', got %v", outputs["answer"])
	}
}

func TestChainOfThought_ParseOutput_EmbeddedJSON(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	cot := NewChainOfThought(sig, nil)

	content := "Let me think... {\"answer\": \"embedded\"} that's the answer"
	outputs, err := cot.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "embedded" {
		t.Errorf("Expected answer='embedded', got %v", outputs["answer"])
	}
}

func TestChainOfThought_WithOptions(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	cot := NewChainOfThought(sig, lm)

	customOpts := &dsgo.GenerateOptions{Temperature: 0.9}
	cot.WithOptions(customOpts)

	if cot.Options.Temperature != 0.9 {
		t.Error("WithOptions should set custom options")
	}
}

func TestChainOfThought_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	cot := NewChainOfThought(sig, lm)

	if cot.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestChainOfThought_BuildPrompt(t *testing.T) {
	sig := dsgo.NewSignature("Solve the problem").
		AddInput("problem", dsgo.FieldTypeString, "Problem to solve").
		AddOutput("answer", dsgo.FieldTypeString, "The answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			content := messages[0].Content
			if !contains(content, "step-by-step") {
				t.Error("Prompt should include step-by-step instruction")
			}
			if !contains(content, "reasoning") {
				t.Error("Prompt should request reasoning field")
			}
			return &dsgo.GenerateResult{Content: `{"reasoning": "test", "answer": "ok"}`}, nil
		},
	}

	cot := NewChainOfThought(sig, lm)
	_, err := cot.Forward(context.Background(), map[string]interface{}{
		"problem": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestChainOfThought_ParseOutput_GenericCodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	cot := NewChainOfThought(sig, nil)

	content := "```\n{\"reasoning\": \"test\", \"answer\": \"result\"}\n```"
	outputs, err := cot.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "result" {
		t.Error("Should parse generic code blocks")
	}
}

func TestChainOfThought_BuildPrompt_NoDescription(t *testing.T) {
	sig := dsgo.NewSignature("").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{Content: `{"reasoning": "test", "answer": "ok"}`}, nil
		},
	}

	cot := NewChainOfThought(sig, lm)
	_, err := cot.Forward(context.Background(), map[string]interface{}{
		"problem": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should work without description: %v", err)
	}
}
