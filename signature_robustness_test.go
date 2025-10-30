package dsgo

import (
	"testing"
)

func TestField_ClassAliases(t *testing.T) {
	field := Field{
		Type:    FieldTypeClass,
		Classes: []string{"positive", "negative", "neutral"},
		ClassAliases: map[string]string{
			"pos":  "positive",
			"neg":  "negative",
			"neut": "neutral",
		},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"exact match", "positive", "positive"},
		{"case insensitive", "POSITIVE", "positive"},
		{"whitespace", " positive ", "positive"},
		{"alias", "pos", "positive"},
		{"alias uppercase", "POS", "positive"},
		{"no match", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeClassValue(tt.input, field)
			if got != tt.want {
				t.Errorf("normalizeClassValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSignature_ValidateOutputsPartial(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required1", FieldTypeString, "").
		AddOutput("required2", FieldTypeInt, "")

	// Add optional field
	sig.OutputFields = append(sig.OutputFields, Field{
		Name:     "optional",
		Type:     FieldTypeString,
		Optional: true,
	})

	tests := []struct {
		name          string
		outputs       map[string]any
		wantMissing   []string
		wantTypeError bool
	}{
		{
			name: "all fields present",
			outputs: map[string]any{
				"required1": "value",
				"required2": 42,
				"optional":  "opt",
			},
			wantMissing: []string{},
		},
		{
			name: "missing required field",
			outputs: map[string]any{
				"required1": "value",
			},
			wantMissing: []string{"required2"},
		},
		{
			name: "missing optional field",
			outputs: map[string]any{
				"required1": "value",
				"required2": 42,
			},
			wantMissing: []string{},
		},
		{
			name: "type error",
			outputs: map[string]any{
				"required1": "value",
				"required2": "not_an_int",
			},
			wantTypeError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := copyTestMap(tt.outputs)
			diag := sig.ValidateOutputsPartial(outputs)

			// Check missing fields
			if len(diag.MissingFields) != len(tt.wantMissing) {
				t.Errorf("ValidateOutputsPartial() missing fields = %v, want %v", diag.MissingFields, tt.wantMissing)
			}

			// Check that missing fields are set to nil
			for _, field := range diag.MissingFields {
				if _, exists := outputs[field]; !exists {
					t.Errorf("Missing field %q not set to nil in outputs", field)
				}
				if outputs[field] != nil {
					t.Errorf("Missing field %q set to %v, want nil", field, outputs[field])
				}
			}

			// Check type errors
			if tt.wantTypeError && len(diag.TypeErrors) == 0 {
				t.Error("Expected type errors but got none")
			}
			if !tt.wantTypeError && len(diag.TypeErrors) > 0 {
				t.Errorf("Unexpected type errors: %v", diag.TypeErrors)
			}
		})
	}
}

func TestSignature_ValidateOutputs_WithNormalization(t *testing.T) {
	sig := NewSignature("Test")
	sig.OutputFields = []Field{
		{
			Name:    "sentiment",
			Type:    FieldTypeClass,
			Classes: []string{"positive", "negative", "neutral"},
			ClassAliases: map[string]string{
				"pos": "positive",
				"neg": "negative",
			},
		},
	}

	tests := []struct {
		name      string
		outputs   map[string]any
		wantValue string
		wantErr   bool
	}{
		{
			name:      "exact match",
			outputs:   map[string]any{"sentiment": "positive"},
			wantValue: "positive",
			wantErr:   false,
		},
		{
			name:      "case insensitive",
			outputs:   map[string]any{"sentiment": "POSITIVE"},
			wantValue: "positive",
			wantErr:   false,
		},
		{
			name:      "alias",
			outputs:   map[string]any{"sentiment": "pos"},
			wantValue: "positive",
			wantErr:   false,
		},
		{
			name:      "alias uppercase",
			outputs:   map[string]any{"sentiment": "POS"},
			wantValue: "positive",
			wantErr:   false,
		},
		{
			name:      "invalid value",
			outputs:   map[string]any{"sentiment": "invalid"},
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := copyTestMap(tt.outputs)
			err := sig.ValidateOutputs(outputs)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateOutputs() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateOutputs() unexpected error: %v", err)
				}
				if outputs["sentiment"] != tt.wantValue {
					t.Errorf("ValidateOutputs() normalized value = %v, want %v", outputs["sentiment"], tt.wantValue)
				}
			}
		})
	}
}

func TestValidationDiagnostics_HasErrors(t *testing.T) {
	tests := []struct {
		name string
		diag *ValidationDiagnostics
		want bool
	}{
		{
			name: "no errors",
			diag: &ValidationDiagnostics{
				MissingFields: []string{},
				TypeErrors:    map[string]error{},
				ClassErrors:   map[string]error{},
			},
			want: false,
		},
		{
			name: "has missing fields",
			diag: &ValidationDiagnostics{
				MissingFields: []string{"field1"},
				TypeErrors:    map[string]error{},
				ClassErrors:   map[string]error{},
			},
			want: true,
		},
		{
			name: "has type errors",
			diag: &ValidationDiagnostics{
				MissingFields: []string{},
				TypeErrors:    map[string]error{"field1": &ValidationError{}},
				ClassErrors:   map[string]error{},
			},
			want: true,
		},
		{
			name: "has class errors",
			diag: &ValidationDiagnostics{
				MissingFields: []string{},
				TypeErrors:    map[string]error{},
				ClassErrors:   map[string]error{"field1": &ValidationError{}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diag.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to copy a map
func copyTestMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Mock error type for testing
type ValidationError struct{}

func (e *ValidationError) Error() string {
	return "validation error"
}
