package module

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestRefine_Forward_NoFeedback(t *testing.T) {
	sig := core.NewSignature("Generate answer").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "initial"}`,
			}, nil
		},
	}

	refine := NewRefine(sig, lm)
	outputs, err := refine.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "initial" {
		t.Errorf("Expected answer='initial', got %v", outputs.Outputs["answer"])
	}
}

func TestRefine_Forward_WithFeedback(t *testing.T) {
	sig := core.NewSignature("Generate answer").
		AddInput("question", core.FieldTypeString, "Question").
		AddInput("feedback", core.FieldTypeString, "Feedback").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{Content: `{"answer": "initial"}`}, nil
			}
			return &core.GenerateResult{Content: `{"answer": "refined"}`}, nil
		},
	}

	refine := NewRefine(sig, lm).WithMaxIterations(2)
	outputs, err := refine.Forward(context.Background(), map[string]interface{}{
		"question": "test",
		"feedback": "improve this",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "refined" {
		t.Errorf("Expected refined answer, got %v", outputs.Outputs["answer"])
	}

	if callCount != 2 {
		t.Errorf("Expected 2 LM calls, got %d", callCount)
	}
}

func TestRefine_Forward_InvalidInput(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("required", core.FieldTypeString, "Required")

	lm := &MockLM{}
	refine := NewRefine(sig, lm)

	_, err := refine.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestRefine_Forward_LMError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	refine := NewRefine(sig, lm)
	_, err := refine.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestRefine_WithOptions(t *testing.T) {
	sig := core.NewSignature("Test")
	lm := &MockLM{}
	refine := NewRefine(sig, lm)

	customOpts := &core.GenerateOptions{Temperature: 0.8}
	refine.WithOptions(customOpts)

	if refine.Options.Temperature != 0.8 {
		t.Error("WithOptions should set custom options")
	}
}

func TestRefine_WithMaxIterations(t *testing.T) {
	refine := NewRefine(core.NewSignature("Test"), &MockLM{})
	refine.WithMaxIterations(5)

	if refine.MaxIterations != 5 {
		t.Error("WithMaxIterations should set max iterations")
	}
}

func TestRefine_WithRefinementField(t *testing.T) {
	refine := NewRefine(core.NewSignature("Test"), &MockLM{})
	refine.WithRefinementField("custom_feedback")

	if refine.RefinementField != "custom_feedback" {
		t.Error("WithRefinementField should set refinement field")
	}
}

func TestRefine_GetSignature(t *testing.T) {
	sig := core.NewSignature("Test")
	refine := NewRefine(sig, &MockLM{})

	if refine.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestRefine_RefinementError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddInput("feedback", core.FieldTypeString, "Feedback").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{Content: `{"answer": "initial"}`}, nil
			}
			return nil, errors.New("refinement failed")
		},
	}

	refine := NewRefine(sig, lm).WithMaxIterations(2)
	outputs, err := refine.Forward(context.Background(), map[string]interface{}{
		"question": "test",
		"feedback": "improve",
	})

	if err != nil {
		t.Fatalf("Forward() should not error when refinement fails")
	}

	if outputs.Outputs["answer"] != "initial" {
		t.Error("Should return initial output when refinement fails")
	}
}

func TestRefine_Forward_MaxIterations1(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddInput("feedback", core.FieldTypeString, "Feedback").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			return &core.GenerateResult{Content: `{"answer": "initial"}`}, nil
		},
	}

	refine := NewRefine(sig, lm).WithMaxIterations(1)
	outputs, err := refine.Forward(context.Background(), map[string]interface{}{
		"question": "test",
		"feedback": "improve",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call with max_iterations=1, got %d", callCount)
	}

	if outputs.Outputs["answer"] != "initial" {
		t.Error("Should return initial output when max_iterations=1")
	}
}

// TestRefine_GeneratePrediction_WithPreviousOutput tests the refinement prompt building
func TestRefine_GeneratePrediction_WithPreviousOutput(t *testing.T) {
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "Refined answer", "confidence": 0.95}`,
			}, nil
		},
	}

	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "The question").
		AddInput("feedback", core.FieldTypeString, "Refinement feedback").
		AddOutput("answer", core.FieldTypeString, "The answer").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence score")

	refine := NewRefine(sig, lm)

	tests := []struct {
		name           string
		inputs         map[string]any
		previousOutput map[string]any
		wantErr        bool
		checkResult    func(*testing.T, *core.Prediction)
	}{
		{
			name: "with previous output",
			inputs: map[string]any{
				"question": "What is AI?",
				"feedback": "Be more specific",
			},
			previousOutput: map[string]any{
				"answer":     "AI is intelligence",
				"confidence": 0.6,
			},
			wantErr: false,
			checkResult: func(t *testing.T, pred *core.Prediction) {
				if pred == nil {
					t.Fatal("expected prediction, got nil")
				}
				if pred.Outputs["answer"] != "Refined answer" {
					t.Errorf("expected refined answer, got %v", pred.Outputs["answer"])
				}
			},
		},
		{
			name: "first iteration (no previous)",
			inputs: map[string]any{
				"question": "What is AI?",
				"feedback": "Be clear",
			},
			previousOutput: nil,
			wantErr:        false,
			checkResult: func(t *testing.T, pred *core.Prediction) {
				if pred == nil {
					t.Fatal("expected prediction, got nil")
				}
			},
		},
		{
			name: "with refinement field",
			inputs: map[string]any{
				"question": "What is AI?",
				"feedback": "Add examples",
			},
			previousOutput: map[string]any{
				"answer": "AI is machine learning",
			},
			wantErr: false,
			checkResult: func(t *testing.T, pred *core.Prediction) {
				if pred == nil {
					t.Fatal("expected prediction, got nil")
				}
			},
		},
		{
			name: "multiple output fields with previous output",
			inputs: map[string]any{
				"question": "Explain quantum computing",
			},
			previousOutput: map[string]any{
				"answer":     "Quantum computing uses qubits",
				"confidence": 0.7,
			},
			wantErr: false,
			checkResult: func(t *testing.T, pred *core.Prediction) {
				if pred == nil {
					t.Fatal("expected prediction, got nil")
				}
				if _, ok := pred.Outputs["answer"]; !ok {
					t.Error("expected answer field in outputs")
				}
				if _, ok := pred.Outputs["confidence"]; !ok {
					t.Error("expected confidence field in outputs")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := refine.generatePrediction(context.Background(), tt.inputs, tt.previousOutput)
			if (err != nil) != tt.wantErr {
				t.Errorf("generatePrediction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

// TestRefine_GenerateRefinement_EdgeCases tests the refinement generation with edge cases
func TestRefine_GenerateRefinement_EdgeCases(t *testing.T) {
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "Improved answer"}`,
			}, nil
		},
	}

	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question text").
		AddOutput("answer", core.FieldTypeString, "Answer text")

	refine := NewRefine(sig, lm).WithRefinementField("feedback")

	result, err := refine.generateRefinement(context.Background(),
		map[string]any{"question": "test"},
		map[string]any{"answer": "test answer"},
		"Please improve clarity")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestRefine_GeneratePrediction_WithOptionalFields tests optional field handling
func TestRefine_GeneratePrediction_WithOptionalFields(t *testing.T) {
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "Test answer"}`,
			}, nil
		},
	}

	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Required question").
		AddOptionalInput("context", core.FieldTypeString, "Optional context").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOptionalOutput("source", core.FieldTypeString, "Source")

	refine := NewRefine(sig, lm)

	tests := []struct {
		name           string
		inputs         map[string]any
		previousOutput map[string]any
	}{
		{
			name: "with optional input missing",
			inputs: map[string]any{
				"question": "What is Go?",
			},
			previousOutput: nil,
		},
		{
			name: "with all inputs",
			inputs: map[string]any{
				"question": "What is Go?",
				"context":  "Programming languages",
			},
			previousOutput: nil,
		},
		{
			name: "with previous output and optional fields",
			inputs: map[string]any{
				"question": "What is Go?",
			},
			previousOutput: map[string]any{
				"answer": "Go is a programming language",
				"source": "golang.org",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := refine.generatePrediction(context.Background(), tt.inputs, tt.previousOutput)
			if err != nil {
				t.Errorf("generatePrediction() error = %v", err)
				return
			}
			if result == nil {
				t.Error("expected prediction, got nil")
			}
		})
	}
}

// TestRefine_GeneratePrediction_WithClassFields tests class field handling in refinement
func TestRefine_GeneratePrediction_WithClassFields(t *testing.T) {
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"sentiment": "positive", "answer": "Great product"}`,
			}, nil
		},
	}

	sig := core.NewSignature("Test").
		AddInput("text", core.FieldTypeString, "Input text").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification").
		AddOutput("answer", core.FieldTypeString, "Explanation")

	refine := NewRefine(sig, lm)

	tests := []struct {
		name           string
		previousOutput map[string]any
	}{
		{
			name:           "first iteration with class field",
			previousOutput: nil,
		},
		{
			name: "refinement with previous class output",
			previousOutput: map[string]any{
				"sentiment": "neutral",
				"answer":    "Not sure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := refine.generatePrediction(context.Background(),
				map[string]any{"text": "This is amazing!"},
				tt.previousOutput)
			if err != nil {
				t.Errorf("generatePrediction() error = %v", err)
				return
			}
			if result == nil {
				t.Error("expected prediction, got nil")
			}
		})
	}
}

// TestRefine_GenerateRefinement_ComplexPrompt tests complex refinement scenarios
func TestRefine_GenerateRefinement_ComplexPrompt(t *testing.T) {
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "Improved answer", "confidence": 0.9}`,
			}, nil
		},
	}

	sig := core.NewSignature("Complex Task").
		AddInput("question", core.FieldTypeString, "The question").
		AddInput("context", core.FieldTypeString, "Additional context").
		AddOutput("answer", core.FieldTypeString, "The answer").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence score")

	refine := NewRefine(sig, lm)

	result, err := refine.generateRefinement(context.Background(),
		map[string]any{
			"question": "Explain machine learning",
			"context":  "For beginners",
		},
		map[string]any{
			"answer":     "ML is about teaching computers",
			"confidence": 0.7,
		},
		"Add more details and examples")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Outputs["answer"] != "Improved answer" {
		t.Errorf("expected improved answer, got %v", result.Outputs["answer"])
	}
}

// TestRefine_WithAdapter tests adapter configuration
func TestRefine_WithAdapter(t *testing.T) {
	sig := core.NewSignature("test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	lm := &MockLM{}
	adapter := core.NewChatAdapter()

	refine := NewRefine(sig, lm).WithAdapter(adapter)
	if refine.Adapter != adapter {
		t.Error("WithAdapter should set custom adapter")
	}
}

// TestRefine_FinishReasonHandling tests finish_reason scenarios
func TestRefine_FinishReasonHandling(t *testing.T) {
	sig := core.NewSignature("Test signature").
		AddOutput("answer", core.FieldTypeString, "Answer")

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
			content:      "[[ ## answer ## ]]\nRefined answer",
			expectError:  false,
		},
		{
			name:          "finish_reason=length (error)",
			finishReason:  "length",
			content:       "[[ ## answer ## ]]\nPartial refinement",
			expectError:   true,
			errorContains: "model hit max_tokens limit (finish_reason=length)",
		},
		{
			name:          "finish_reason=tool_calls (error)",
			finishReason:  "tool_calls",
			content:       "",
			expectError:   true,
			errorContains: "finish_reason=tool_calls) but Refine module doesn't support tool loops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &MockLM{
				GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
					return &core.GenerateResult{
						Content:      tt.content,
						FinishReason: tt.finishReason,
						Usage:        core.Usage{TotalTokens: 10},
					}, nil
				},
			}

			refine := NewRefine(sig, mockLM).WithMaxIterations(1)
			_, err := refine.Forward(context.Background(), map[string]any{"input": "test"})

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
