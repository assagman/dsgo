package dsgo

import (
	"testing"
)

// TestSignature_ValidateOutputsPartial_Comprehensive tests all code paths
func TestSignature_ValidateOutputsPartial_Comprehensive(t *testing.T) {
	tests := []struct {
		name              string
		sig               *Signature
		outputs           map[string]any
		wantMissingCount  int
		wantTypeErrCount  int
		wantClassErrCount int
	}{
		{
			name: "all fields present and valid",
			sig: NewSignature("Test").
				AddOutput("answer", FieldTypeString, "").
				AddOutput("confidence", FieldTypeFloat, ""),
			outputs: map[string]any{
				"answer":     "Yes",
				"confidence": 0.95,
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "missing required field",
			sig: NewSignature("Test").
				AddOutput("answer", FieldTypeString, "").
				AddOutput("confidence", FieldTypeFloat, ""),
			outputs: map[string]any{
				"answer": "Yes",
			},
			wantMissingCount:  1,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "missing optional field (should not error)",
			sig: NewSignature("Test").
				AddOutput("answer", FieldTypeString, "").
				AddOptionalOutput("note", FieldTypeString, ""),
			outputs: map[string]any{
				"answer": "Yes",
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "type error",
			sig: NewSignature("Test").
				AddOutput("count", FieldTypeInt, ""),
			outputs: map[string]any{
				"count": "not_a_number",
			},
			wantMissingCount:  0,
			wantTypeErrCount:  1,
			wantClassErrCount: 0,
		},
		{
			name: "class validation error",
			sig: NewSignature("Test").
				AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, ""),
			outputs: map[string]any{
				"sentiment": "invalid_class",
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 1,
		},
		{
			name: "nil value for optional field (should skip validation)",
			sig: NewSignature("Test").
				AddOptionalOutput("note", FieldTypeString, ""),
			outputs: map[string]any{
				"note": nil,
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "class normalization success",
			sig: NewSignature("Test").
				AddClassOutput("sentiment", []string{"positive", "negative"}, ""),
			outputs: map[string]any{
				"sentiment": "POSITIVE", // Should normalize to "positive"
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "class with alias",
			sig: NewSignature("Test").
				AddOutput("sentiment", FieldTypeClass, ""),
			outputs: map[string]any{
				"sentiment": "pos", // Should map to "positive" via alias
			},
			wantMissingCount:  0,
			wantTypeErrCount:  0,
			wantClassErrCount: 0,
		},
		{
			name: "multiple errors",
			sig: NewSignature("Test").
				AddOutput("answer", FieldTypeString, "").
				AddOutput("count", FieldTypeInt, "").
				AddClassOutput("category", []string{"a", "b"}, ""),
			outputs: map[string]any{
				"count":    "not_int",
				"category": "invalid",
			},
			wantMissingCount:  1, // missing "answer"
			wantTypeErrCount:  1, // "count" type error
			wantClassErrCount: 1, // "category" class error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For class with alias test, set up aliases
			if tt.name == "class with alias" {
				tt.sig.OutputFields[0].Classes = []string{"positive", "negative"}
				tt.sig.OutputFields[0].ClassAliases = map[string]string{"pos": "positive", "neg": "negative"}
			}

			diag := tt.sig.ValidateOutputsPartial(tt.outputs)

			if len(diag.MissingFields) != tt.wantMissingCount {
				t.Errorf("Expected %d missing fields, got %d: %v", tt.wantMissingCount, len(diag.MissingFields), diag.MissingFields)
			}

			if len(diag.TypeErrors) != tt.wantTypeErrCount {
				t.Errorf("Expected %d type errors, got %d: %v", tt.wantTypeErrCount, len(diag.TypeErrors), diag.TypeErrors)
			}

			if len(diag.ClassErrors) != tt.wantClassErrCount {
				t.Errorf("Expected %d class errors, got %d: %v", tt.wantClassErrCount, len(diag.ClassErrors), diag.ClassErrors)
			}

			// Verify missing fields are set to nil
			for _, field := range diag.MissingFields {
				if val, exists := tt.outputs[field]; !exists || val != nil {
					t.Errorf("Missing field %s should be set to nil in outputs", field)
				}
			}

			// Verify normalized values for class normalization test
			if tt.name == "class normalization success" {
				if sentiment, ok := tt.outputs["sentiment"].(string); !ok || sentiment != "positive" {
					t.Errorf("Expected sentiment to be normalized to 'positive', got '%v'", tt.outputs["sentiment"])
				}
			}
		})
	}
}

// TestSignature_ValidateOutputsPartial_NilValue tests nil value handling
func TestSignature_ValidateOutputsPartial_NilValue(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required", FieldTypeString, "").
		AddOptionalOutput("optional", FieldTypeString, "")

	outputs := map[string]any{
		"required": "value",
		"optional": nil,
	}

	diag := sig.ValidateOutputsPartial(outputs)

	if diag.HasErrors() {
		t.Errorf("Expected no errors with nil optional value: %+v", diag)
	}
}

// TestSignature_ValidateOutputsPartial_EmptyClass tests empty class list
func TestSignature_ValidateOutputsPartial_EmptyClass(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("category", FieldTypeClass, "")

	// Empty class list - should not validate
	sig.OutputFields[0].Classes = []string{}

	outputs := map[string]any{
		"category": "anything",
	}

	diag := sig.ValidateOutputsPartial(outputs)

	// Should not have class errors when class list is empty
	if len(diag.ClassErrors) > 0 {
		t.Errorf("Expected no class errors with empty class list: %+v", diag)
	}
}
