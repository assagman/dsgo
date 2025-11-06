package module

import (
	"context"
	"errors"
	"strings"
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

	if outputs.Outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs.Outputs["answer"])
	}

	if !outputs.HasRationale() {
		t.Error("ChainOfThought should include reasoning/rationale")
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

// TestChainOfThought_ReasoningInRationale verifies that reasoning is stored in Rationale field,
// not in Outputs["reasoning"]. This prevents the bug found in examples/sentiment and examples/interview.
func TestChainOfThought_ReasoningInRationale(t *testing.T) {
	sig := dsgo.NewSignature("Test signature").
		AddInput("question", dsgo.FieldTypeString, "The question").
		AddOutput("answer", dsgo.FieldTypeString, "The answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{
					"reasoning": "This is my step-by-step reasoning",
					"answer": "42"
				}`,
			}, nil
		},
	}

	cot := NewChainOfThought(sig, lm)
	ctx := context.Background()
	inputs := map[string]any{"question": "What is the answer?"}

	result, err := cot.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Forward() failed: %v", err)
	}

	// Verify reasoning is in Rationale field
	if result.Rationale == "" {
		t.Error("Expected reasoning in Rationale field, got empty string")
	}
	if result.Rationale != "This is my step-by-step reasoning" {
		t.Errorf("Expected reasoning in Rationale, got: %s", result.Rationale)
	}

	// Verify reasoning is NOT in Outputs map (unless explicitly in signature)
	if _, exists := result.Outputs["reasoning"]; exists {
		t.Error("Reasoning should not be in Outputs map when not defined in signature")
	}

	// Verify answer is in Outputs
	if answer, ok := result.Outputs["answer"].(string); !ok || answer != "42" {
		t.Errorf("Expected answer='42' in Outputs, got: %v", result.Outputs["answer"])
	}
}

// TestChainOfThought_WithAdapter tests adapter configuration
func TestChainOfThought_WithAdapter(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	lm := &MockLM{}
	adapter := dsgo.NewChatAdapter()

	cot := NewChainOfThought(sig, lm).WithAdapter(adapter)
	if cot.Adapter != adapter {
		t.Error("WithAdapter should set custom adapter")
	}
}

// TestChainOfThought_FinishReasonHandling tests all finish_reason scenarios
func TestChainOfThought_FinishReasonHandling(t *testing.T) {
	sig := dsgo.NewSignature("Test signature").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	tests := []struct {
		name          string
		finishReason  string
		content       string
		expectError   bool
		errorContains string
	}{
		{
			name:         "finish_reason=stop with valid content",
			finishReason: "stop",
			content:      "[[ ## reasoning ## ]]\nThinking...\n[[ ## answer ## ]]\nTest answer",
			expectError:  false,
		},
		{
			name:          "finish_reason=length (error)",
			finishReason:  "length",
			content:       "[[ ## reasoning ## ]]\nThinking...\n[[ ## answer ## ]]\nPartial",
			expectError:   true,
			errorContains: "model hit max_tokens limit (finish_reason=length)",
		},
		{
			name:          "finish_reason=tool_calls (error)",
			finishReason:  "tool_calls",
			content:       "",
			expectError:   true,
			errorContains: "finish_reason=tool_calls) but ChainOfThought module doesn't support tool loops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &MockLM{
				GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
					return &dsgo.GenerateResult{
						Content:      tt.content,
						FinishReason: tt.finishReason,
						Usage:        dsgo.Usage{TotalTokens: 10},
					}, nil
				},
			}

			cot := NewChainOfThought(sig, mockLM)
			_, err := cot.Forward(context.Background(), map[string]any{"input": "test"})

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errorContains)
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}
