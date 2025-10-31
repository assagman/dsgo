package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestPredict_Forward_Success(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"answer": "42"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	outputs, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs.Outputs["answer"])
	}
}

func TestPredict_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	p := NewPredict(sig, lm)

	_, err := p.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestPredict_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestPredict_Forward_ParseError(t *testing.T) {
	// Use multiple fields so JSONAdapter can't fall back to plain text
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer").
		AddOutput("confidence", dsgo.FieldTypeString, "Confidence level")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `invalid json without structure`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error on parse failure when multiple fields required")
	}
}

func TestPredict_Forward_ValidationError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"wrong_field": "value"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error on validation failure")
	}
}

func TestPredict_WithOptions(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	p := NewPredict(sig, lm)

	customOpts := &dsgo.GenerateOptions{Temperature: 0.5}
	p.WithOptions(customOpts)

	if p.Options.Temperature != 0.5 {
		t.Error("WithOptions should set custom options")
	}
}

func TestPredict_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	p := NewPredict(sig, lm)

	if p.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestPredict_JSONSupport(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			if options.ResponseFormat != "json" {
				t.Error("ResponseFormat should be 'json' when LM supports JSON and using JSONAdapter")
			}
			return &dsgo.GenerateResult{Content: `{"answer": "ok"}`}, nil
		},
	}

	// Use JSONAdapter explicitly to trigger JSON mode
	p := NewPredict(sig, lm).WithAdapter(dsgo.NewJSONAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Errorf("Forward() error = %v", err)
	}
}
