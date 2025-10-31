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

func TestPredict_JSONSchemaAutoGeneration(t *testing.T) {
	// Create a signature with multiple output fields of different types
	sig := dsgo.NewSignature("Classification and analysis").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", dsgo.FieldTypeString, "Sentiment classification").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score").
		AddOutput("word_count", dsgo.FieldTypeInt, "Number of words").
		AddClassOutput("category", []string{"business", "technology", "sports"}, "Content category")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			// Verify ResponseSchema was auto-generated
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be auto-generated when using JSONAdapter with JSON-capable LM")
				return &dsgo.GenerateResult{Content: `{}`}, nil
			}

			// Verify schema structure
			schema := options.ResponseSchema
			if schema["type"] != "object" {
				t.Errorf("Expected schema type 'object', got %v", schema["type"])
			}

			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatal("Expected properties map in schema")
			}

			// Verify sentiment field (string)
			if sentimentProp, ok := props["sentiment"].(map[string]any); !ok {
				t.Error("Expected sentiment in schema properties")
			} else if sentimentProp["type"] != "string" {
				t.Errorf("Expected sentiment type 'string', got %v", sentimentProp["type"])
			}

			// Verify confidence field (number)
			if confProp, ok := props["confidence"].(map[string]any); !ok {
				t.Error("Expected confidence in schema properties")
			} else if confProp["type"] != "number" {
				t.Errorf("Expected confidence type 'number', got %v", confProp["type"])
			}

			// Verify word_count field (integer)
			if wcProp, ok := props["word_count"].(map[string]any); !ok {
				t.Error("Expected word_count in schema properties")
			} else if wcProp["type"] != "integer" {
				t.Errorf("Expected word_count type 'integer', got %v", wcProp["type"])
			}

			// Verify category field (enum)
			if catProp, ok := props["category"].(map[string]any); !ok {
				t.Error("Expected category in schema properties")
			} else {
				if catProp["type"] != "string" {
					t.Errorf("Expected category type 'string', got %v", catProp["type"])
				}
				if enum, ok := catProp["enum"].([]string); !ok {
					t.Error("Expected enum array in category")
				} else if len(enum) != 3 {
					t.Errorf("Expected 3 enum values, got %d", len(enum))
				}
			}

			// Verify required fields
			required, ok := schema["required"].([]string)
			if !ok {
				t.Fatal("Expected required array in schema")
			}
			if len(required) != 4 {
				t.Errorf("Expected 4 required fields, got %d", len(required))
			}

			// Return mock response
			return &dsgo.GenerateResult{
				Content: `{"sentiment": "positive", "confidence": 0.95, "word_count": 42, "category": "technology"}`,
			}, nil
		},
	}

	// Use JSONAdapter explicitly to trigger schema generation
	p := NewPredict(sig, lm).WithAdapter(dsgo.NewJSONAdapter())
	outputs, err := p.Forward(context.Background(), map[string]interface{}{
		"text": "This is a great technology article!",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify outputs were parsed correctly
	if outputs.Outputs["sentiment"] != "positive" {
		t.Errorf("Expected sentiment='positive', got %v", outputs.Outputs["sentiment"])
	}
	if outputs.Outputs["confidence"] != 0.95 {
		t.Errorf("Expected confidence=0.95, got %v", outputs.Outputs["confidence"])
	}
	if outputs.Outputs["word_count"] != 42 {
		t.Errorf("Expected word_count=42, got %v", outputs.Outputs["word_count"])
	}
	if outputs.Outputs["category"] != "technology" {
		t.Errorf("Expected category='technology', got %v", outputs.Outputs["category"])
	}
}

func TestPredict_JSONSchemaWithOptionalFields(t *testing.T) {
	sig := dsgo.NewSignature("Optional fields test").
		AddInput("query", dsgo.FieldTypeString, "Query").
		AddOutput("required_field", dsgo.FieldTypeString, "Required").
		AddOptionalOutput("optional_field", dsgo.FieldTypeString, "Optional")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be auto-generated")
				return &dsgo.GenerateResult{Content: `{}`}, nil
			}

			schema := options.ResponseSchema
			required, ok := schema["required"].([]string)
			if !ok {
				t.Fatal("Expected required array in schema")
			}

			// Should only have required_field, not optional_field
			if len(required) != 1 {
				t.Errorf("Expected 1 required field, got %d", len(required))
			}
			if len(required) > 0 && required[0] != "required_field" {
				t.Errorf("Expected required_field to be required, got %v", required[0])
			}

			return &dsgo.GenerateResult{
				Content: `{"required_field": "value"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithAdapter(dsgo.NewJSONAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"query": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestPredict_JSONSchemaNotGeneratedWithChatAdapter(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			// When using ChatAdapter, ResponseFormat should NOT be "json" even if LM supports it
			if options.ResponseFormat == "json" {
				t.Error("ResponseFormat should not be 'json' when using ChatAdapter")
			}
			if options.ResponseSchema != nil {
				t.Error("ResponseSchema should not be set when using ChatAdapter")
			}
			// ChatAdapter expects markers
			return &dsgo.GenerateResult{Content: `[[ ## answer ## ]]\n42`}, nil
		},
	}

	// Use ChatAdapter explicitly - should NOT trigger JSON mode
	p := NewPredict(sig, lm).WithAdapter(dsgo.NewChatAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestPredict_CustomSchemaOverridesAutoGeneration(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddOutput("result", dsgo.FieldTypeString, "Result")

	customSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"custom_field": map[string]any{"type": "string"},
		},
	}

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be present")
				return &dsgo.GenerateResult{Content: `{}`}, nil
			}

			// Verify custom schema is used, not auto-generated
			props, ok := options.ResponseSchema["properties"].(map[string]any)
			if !ok {
				t.Fatal("Expected properties in schema")
			}
			if _, ok := props["custom_field"]; !ok {
				t.Error("Expected custom schema to be used, not auto-generated")
			}
			if _, ok := props["result"]; ok {
				t.Error("Auto-generated schema should not be used when custom schema is provided")
			}

			return &dsgo.GenerateResult{Content: `{"custom_field": "value"}`}, nil
		},
	}

	opts := dsgo.DefaultGenerateOptions()
	opts.ResponseSchema = customSchema

	p := NewPredict(sig, lm).WithAdapter(dsgo.NewJSONAdapter()).WithOptions(opts)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"text": "test",
	})

	if err != nil {
		// Expected to fail validation since we're using custom schema
		// but that's OK - we just want to verify the custom schema was used
		t.Logf("Expected validation error: %v", err)
	}
}
