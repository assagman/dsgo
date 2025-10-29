package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestRefine_Forward_NoFeedback(t *testing.T) {
	sig := dsgo.NewSignature("Generate answer").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
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

	if outputs["answer"] != "initial" {
		t.Errorf("Expected answer='initial', got %v", outputs["answer"])
	}
}

func TestRefine_Forward_WithFeedback(t *testing.T) {
	sig := dsgo.NewSignature("Generate answer").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{Content: `{"answer": "initial"}`}, nil
			}
			return &dsgo.GenerateResult{Content: `{"answer": "refined"}`}, nil
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

	if outputs["answer"] != "refined" {
		t.Errorf("Expected refined answer, got %v", outputs["answer"])
	}

	if callCount != 2 {
		t.Errorf("Expected 2 LM calls, got %d", callCount)
	}
}

func TestRefine_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	refine := NewRefine(sig, lm)

	_, err := refine.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestRefine_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
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
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	refine := NewRefine(sig, lm)

	customOpts := &dsgo.GenerateOptions{Temperature: 0.8}
	refine.WithOptions(customOpts)

	if refine.Options.Temperature != 0.8 {
		t.Error("WithOptions should set custom options")
	}
}

func TestRefine_WithMaxIterations(t *testing.T) {
	refine := NewRefine(dsgo.NewSignature("Test"), &MockLM{})
	refine.WithMaxIterations(5)

	if refine.MaxIterations != 5 {
		t.Error("WithMaxIterations should set max iterations")
	}
}

func TestRefine_WithRefinementField(t *testing.T) {
	refine := NewRefine(dsgo.NewSignature("Test"), &MockLM{})
	refine.WithRefinementField("custom_feedback")

	if refine.RefinementField != "custom_feedback" {
		t.Error("WithRefinementField should set refinement field")
	}
}

func TestRefine_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	refine := NewRefine(sig, &MockLM{})

	if refine.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestRefine_RefinementError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{Content: `{"answer": "initial"}`}, nil
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

	if outputs["answer"] != "initial" {
		t.Error("Should return initial output when refinement fails")
	}
}

func TestRefine_ParseOutput_CodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	refine := NewRefine(sig, &MockLM{})

	content := "```json\n{\"answer\": \"test\"}\n```"
	outputs, err := refine.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "test" {
		t.Error("Should parse JSON code block")
	}
}

func TestRefine_ParseOutput_GenericBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	refine := NewRefine(sig, &MockLM{})

	content := "```\n{\"answer\": \"result\"}\n```"
	outputs, err := refine.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "result" {
		t.Error("Should parse generic code block")
	}
}

func TestRefine_ParseOutput_EmbeddedJSON(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	refine := NewRefine(sig, &MockLM{})

	content := "Text before {\"answer\": \"embedded\"} after"
	outputs, err := refine.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "embedded" {
		t.Error("Should extract embedded JSON")
	}
}

func TestRefine_Forward_MaxIterations1(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			return &dsgo.GenerateResult{Content: `{"answer": "initial"}`}, nil
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

	if outputs["answer"] != "initial" {
		t.Error("Should return initial output when max_iterations=1")
	}
}

func TestRefine_GeneratePrediction_SkipFeedbackField(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			content := messages[0].Content
			if contains(content, "feedback") {
				t.Error("Initial prompt should not contain feedback field")
			}
			return &dsgo.GenerateResult{Content: `{"answer": "ok"}`}, nil
		},
	}

	refine := NewRefine(sig, lm)
	_, err := refine.generatePrediction(context.Background(), map[string]interface{}{
		"question": "test",
		"feedback": "should be skipped",
	}, nil)

	if err != nil {
		t.Fatalf("generatePrediction() error = %v", err)
	}
}

func TestRefine_GenerateRefinement(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			content := messages[0].Content
			if !contains(content, "Refine") {
				t.Error("Refinement prompt should mention refining")
			}
			if !contains(content, "Previous Output") {
				t.Error("Refinement prompt should show previous output")
			}
			return &dsgo.GenerateResult{Content: `{"answer": "refined"}`}, nil
		},
	}

	refine := NewRefine(sig, lm)
	outputs, err := refine.generateRefinement(
		context.Background(),
		map[string]interface{}{"question": "test"},
		map[string]interface{}{"answer": "initial"},
		"make it better",
	)

	if err != nil {
		t.Fatalf("generateRefinement() error = %v", err)
	}

	if outputs["answer"] != "refined" {
		t.Error("Should return refined output")
	}
}
