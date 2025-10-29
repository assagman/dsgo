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

	if outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
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
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `invalid json`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error on parse failure")
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

func TestPredict_ParseOutput_JSONCodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := "```json\n{\"answer\": \"test\"}\n```"
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "test" {
		t.Errorf("Expected answer='test', got %v", outputs["answer"])
	}
}

func TestPredict_ParseOutput_GenericCodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := "```\n{\"answer\": \"test\"}\n```"
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "test" {
		t.Errorf("Expected answer='test', got %v", outputs["answer"])
	}
}

func TestPredict_ParseOutput_EmbeddedJSON(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := "Some text before {\"answer\": \"embedded\"} and after"
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "embedded" {
		t.Errorf("Expected answer='embedded', got %v", outputs["answer"])
	}
}

func TestPredict_ParseOutput_NonJSONCodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := "```python\nprint('hello')\n```\n{\"answer\": \"after code\"}"
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "after code" {
		t.Errorf("Expected answer='after code', got %v", outputs["answer"])
	}
}

func TestPredict_CoerceTypes_ArrayToString(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	outputs := map[string]interface{}{
		"answer": []interface{}{"line1", "line2", "line3"},
	}

	coerced := p.coerceTypes(outputs)

	if str, ok := coerced["answer"].(string); !ok || str != "line1\nline2\nline3" {
		t.Errorf("Expected array to be joined into string, got %v", coerced["answer"])
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
				t.Error("ResponseFormat should be 'json' when LM supports JSON")
			}
			return &dsgo.GenerateResult{Content: `{"answer": "ok"}`}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Errorf("Forward() error = %v", err)
	}
}

func TestPredict_ParseOutput_NestedJSON(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := `Some text {"answer": "value with {nested} braces"} more text`
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	if outputs["answer"] != "value with {nested} braces" {
		t.Errorf("Expected nested braces to be handled, got %v", outputs["answer"])
	}
}

func TestPredict_ParseOutput_EscapedQuotes(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	p := NewPredict(sig, nil)

	content := `{"answer": "value with \"quotes\""}`
	outputs, err := p.parseOutput(content)

	if err != nil {
		t.Fatalf("parseOutput() error = %v", err)
	}

	expected := `value with "quotes"`
	if outputs["answer"] != expected {
		t.Errorf("Expected escaped quotes to be handled, got %v", outputs["answer"])
	}
}

func TestPredict_CoerceTypes_NonArrayField(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer").
		AddOutput("count", dsgo.FieldTypeInt, "Count")

	p := NewPredict(sig, nil)

	outputs := map[string]interface{}{
		"answer": "text",
		"count":  42,
		"extra":  []interface{}{"should", "not", "be", "coerced"},
	}

	coerced := p.coerceTypes(outputs)

	if str, ok := coerced["answer"].(string); !ok || str != "text" {
		t.Error("String field should not be changed")
	}

	if count, ok := coerced["count"].(int); !ok || count != 42 {
		t.Error("Int field should not be changed")
	}

	if arr, ok := coerced["extra"].([]interface{}); !ok || len(arr) != 4 {
		t.Error("Unknown fields should not be coerced")
	}
}
