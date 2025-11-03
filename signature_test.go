package dsgo

import (
	"fmt"
	"testing"
)

func TestSignature_AddFields(t *testing.T) {
	sig := NewSignature("Test signature")

	sig.AddInput("text", FieldTypeString, "Input text").
		AddOutput("result", FieldTypeString, "Output result").
		AddClassOutput("category", []string{"A", "B", "C"}, "Category")

	if len(sig.InputFields) != 1 {
		t.Errorf("Expected 1 input field, got %d", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 2 {
		t.Errorf("Expected 2 output fields, got %d", len(sig.OutputFields))
	}

	if sig.OutputFields[1].Type != FieldTypeClass {
		t.Errorf("Expected class type, got %v", sig.OutputFields[1].Type)
	}
}

func TestSignature_ValidateInputs(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("required", FieldTypeString, "Required field")

	tests := []struct {
		name    string
		inputs  map[string]any
		wantErr bool
	}{
		{
			name:    "valid input",
			inputs:  map[string]any{"required": "value"},
			wantErr: false,
		},
		{
			name:    "missing required field",
			inputs:  map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.ValidateInputs(tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_ValidateOutputs(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required", FieldTypeString, "Required field").
		AddOptionalOutput("optional", FieldTypeString, "Optional field").
		AddClassOutput("category", []string{"A", "B", "C"}, "Category")

	tests := []struct {
		name    string
		outputs map[string]any
		wantErr bool
	}{
		{
			name: "valid outputs",
			outputs: map[string]any{
				"required": "value",
				"category": "A",
			},
			wantErr: false,
		},
		{
			name: "valid with optional",
			outputs: map[string]any{
				"required": "value",
				"optional": "opt",
				"category": "B",
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			outputs: map[string]any{
				"category": "A",
			},
			wantErr: true,
		},
		{
			name: "invalid class value",
			outputs: map[string]any{
				"required": "value",
				"category": "D",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.ValidateOutputs(tt.outputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_GetOutputField(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("field1", FieldTypeString, "First field").
		AddOutput("field2", FieldTypeInt, "Second field")

	field := sig.GetOutputField("field1")
	if field == nil || field.Name != "field1" {
		t.Error("GetOutputField should return field1")
	}

	nonexistent := sig.GetOutputField("nonexistent")
	if nonexistent != nil {
		t.Error("GetOutputField should return nil for nonexistent field")
	}
}

func TestSignature_ValidateOutputs_TypeValidation(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		value     interface{}
		wantErr   bool
	}{
		{"string valid", FieldTypeString, "test", false},
		{"string invalid", FieldTypeString, 123, true},
		{"int valid", FieldTypeInt, 42, false},
		{"int from float64", FieldTypeInt, 42.0, false},
		{"int invalid", FieldTypeInt, "not an int", true},
		{"float valid", FieldTypeFloat, 3.14, false},
		{"float from int", FieldTypeFloat, 42, false},
		{"float invalid", FieldTypeFloat, "not a float", true},
		{"bool valid", FieldTypeBool, true, false},
		{"bool invalid", FieldTypeBool, "not a bool", true},
		{"nil optional", FieldTypeString, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := NewSignature("Test")
			if tt.name == "nil optional" {
				sig.AddOptionalOutput("field", tt.fieldType, "Test field")
			} else {
				sig.AddOutput("field", tt.fieldType, "Test field")
			}

			outputs := map[string]any{"field": tt.value}
			if tt.value == nil && tt.name == "nil optional" {
				outputs = map[string]any{}
			}

			err := sig.ValidateOutputs(outputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_AddOptionalOutput(t *testing.T) {
	sig := NewSignature("Test").
		AddOptionalOutput("optional", FieldTypeString, "Optional field")

	if len(sig.OutputFields) != 1 {
		t.Errorf("Expected 1 output field, got %d", len(sig.OutputFields))
	}

	if !sig.OutputFields[0].Optional {
		t.Error("Field should be optional")
	}

	err := sig.ValidateOutputs(map[string]any{})
	if err != nil {
		t.Error("Optional field should not cause validation error when missing")
	}
}

func TestFieldType_Constants(t *testing.T) {
	types := []FieldType{
		FieldTypeString,
		FieldTypeInt,
		FieldTypeFloat,
		FieldTypeBool,
		FieldTypeJSON,
		FieldTypeClass,
		FieldTypeImage,
		FieldTypeDatetime,
	}

	for _, ft := range types {
		if ft == "" {
			t.Errorf("FieldType should not be empty: %v", ft)
		}
	}
}

func TestSignature_ValidateFieldType_NumericTypes(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		value     any
		wantErr   bool
	}{
		// Int validation - accept all int types + float64
		{"int valid", FieldTypeInt, int(42), false},
		{"int8 valid", FieldTypeInt, int8(42), false},
		{"int16 valid", FieldTypeInt, int16(42), false},
		{"int32 valid", FieldTypeInt, int32(42), false},
		{"int64 valid", FieldTypeInt, int64(42), false},
		{"int from float64", FieldTypeInt, float64(42.0), false},
		{"int invalid", FieldTypeInt, "42", true},

		// Float validation - accept float types + int types
		{"float32 valid", FieldTypeFloat, float32(3.14), false},
		{"float64 valid", FieldTypeFloat, float64(3.14), false},
		{"float from int", FieldTypeFloat, int(42), false},
		{"float from int8", FieldTypeFloat, int8(42), false},
		{"float from int16", FieldTypeFloat, int16(42), false},
		{"float from int32", FieldTypeFloat, int32(42), false},
		{"float from int64", FieldTypeFloat, int64(42), false},
		{"float invalid", FieldTypeFloat, "3.14", true},

		// JSON validation
		{"json map", FieldTypeJSON, map[string]any{"key": "value"}, false},
		{"json slice", FieldTypeJSON, []any{1, 2, 3}, false},
		{"json string", FieldTypeJSON, `{"key":"value"}`, false},
		{"json invalid", FieldTypeJSON, 123, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := NewSignature("Test").
				AddOutput("field", tt.fieldType, "Test field")

			err := sig.ValidateOutputs(map[string]any{"field": tt.value})
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_AddOptionalInput(t *testing.T) {
	sig := NewSignature("Test").
		AddOptionalInput("optional", FieldTypeString, "Optional field")

	if len(sig.InputFields) != 1 {
		t.Errorf("Expected 1 input field, got %d", len(sig.InputFields))
	}

	if !sig.InputFields[0].Optional {
		t.Error("Field should be marked as optional")
	}

	err := sig.ValidateInputs(map[string]any{})
	if err != nil {
		t.Error("Optional input should not cause validation error when missing")
	}

	err = sig.ValidateInputs(map[string]any{"optional": "value"})
	if err != nil {
		t.Errorf("ValidateInputs() with valid optional = %v", err)
	}

	err = sig.ValidateInputs(map[string]any{"optional": 123})
	if err == nil {
		t.Error("Optional input with wrong type should fail validation")
	}
}

func TestSignature_ValidateInputs_CompleteScenarios(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("required_str", FieldTypeString, "").
		AddInput("required_int", FieldTypeInt, "").
		AddOptionalInput("optional_str", FieldTypeString, "")

	tests := []struct {
		name    string
		inputs  map[string]any
		wantErr bool
	}{
		{
			name: "all valid",
			inputs: map[string]any{
				"required_str": "test",
				"required_int": 42,
				"optional_str": "opt",
			},
			wantErr: false,
		},
		{
			name: "valid without optional",
			inputs: map[string]any{
				"required_str": "test",
				"required_int": 42,
			},
			wantErr: false,
		},
		{
			name: "missing required",
			inputs: map[string]any{
				"required_str": "test",
			},
			wantErr: true,
		},
		{
			name: "wrong type for required",
			inputs: map[string]any{
				"required_str": 123,
				"required_int": 42,
			},
			wantErr: true,
		},
		{
			name: "wrong type for optional",
			inputs: map[string]any{
				"required_str": "test",
				"required_int": 42,
				"optional_str": 123,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.ValidateInputs(tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_ValidateFieldType_AllTypes(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		value     any
		optional  bool
		wantErr   bool
	}{
		{"string valid", FieldTypeString, "test", false, false},
		{"string invalid", FieldTypeString, 123, false, true},
		{"class valid", FieldTypeClass, "test", false, false},
		{"image valid", FieldTypeImage, "image.png", false, false},
		{"datetime valid", FieldTypeDatetime, "2025-01-01", false, false},

		// Comprehensive int type coverage
		{"int valid", FieldTypeInt, int(42), false, false},
		{"int8 valid", FieldTypeInt, int8(42), false, false},
		{"int16 valid", FieldTypeInt, int16(42), false, false},
		{"int32 valid", FieldTypeInt, int32(42), false, false},
		{"int64 valid", FieldTypeInt, int64(42), false, false},
		{"int from float64 valid", FieldTypeInt, float64(42), false, false},
		{"int invalid string", FieldTypeInt, "42", false, true},

		// Comprehensive float type coverage
		{"float32 valid", FieldTypeFloat, float32(3.14), false, false},
		{"float64 valid", FieldTypeFloat, float64(3.14), false, false},
		{"float from int valid", FieldTypeFloat, int(42), false, false},
		{"float from int8 valid", FieldTypeFloat, int8(42), false, false},
		{"float from int16 valid", FieldTypeFloat, int16(42), false, false},
		{"float from int32 valid", FieldTypeFloat, int32(42), false, false},
		{"float from int64 valid", FieldTypeFloat, int64(42), false, false},
		{"float invalid string", FieldTypeFloat, "3.14", false, true},

		{"bool true", FieldTypeBool, true, false, false},
		{"bool false", FieldTypeBool, false, false, false},
		{"bool invalid", FieldTypeBool, "true", false, true},

		// Comprehensive JSON type coverage
		{"json map", FieldTypeJSON, map[string]any{"key": "value"}, false, false},
		{"json slice", FieldTypeJSON, []any{1, 2, 3}, false, false},
		{"json string", FieldTypeJSON, `{"key":"value"}`, false, false},
		{"json invalid", FieldTypeJSON, 123, false, true},

		{"nil optional", FieldTypeString, nil, true, false},
		{"nil required", FieldTypeString, nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := NewSignature("Test")
			if tt.optional {
				sig.AddOptionalOutput("field", tt.fieldType, "")
			} else {
				sig.AddOutput("field", tt.fieldType, "")
			}

			outputs := map[string]any{"field": tt.value}
			if tt.value == nil && tt.optional {
				outputs = map[string]any{}
			}

			err := sig.ValidateOutputs(outputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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
				"sentiment": "POSITIVE",
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
				"sentiment": "pos",
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
			wantMissingCount:  1,
			wantTypeErrCount:  1,
			wantClassErrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			for _, field := range diag.MissingFields {
				if val, exists := tt.outputs[field]; !exists || val != nil {
					t.Errorf("Missing field %s should be set to nil in outputs", field)
				}
			}

			if tt.name == "class normalization success" {
				if sentiment, ok := tt.outputs["sentiment"].(string); !ok || sentiment != "positive" {
					t.Errorf("Expected sentiment to be normalized to 'positive', got '%v'", tt.outputs["sentiment"])
				}
			}
		})
	}
}

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

func TestSignature_ValidateOutputsPartial_EmptyClass(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("category", FieldTypeClass, "")

	sig.OutputFields[0].Classes = []string{}

	outputs := map[string]any{
		"category": "anything",
	}

	diag := sig.ValidateOutputsPartial(outputs)

	if len(diag.ClassErrors) > 0 {
		t.Errorf("Expected no class errors with empty class list: %+v", diag)
	}
}

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

func TestSignature_ValidateOutputsPartial_WithOptional(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required1", FieldTypeString, "").
		AddOutput("required2", FieldTypeInt, "")

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

			if len(diag.MissingFields) != len(tt.wantMissing) {
				t.Errorf("ValidateOutputsPartial() missing fields = %v, want %v", diag.MissingFields, tt.wantMissing)
			}

			for _, field := range diag.MissingFields {
				if _, exists := outputs[field]; !exists {
					t.Errorf("Missing field %q not set to nil in outputs", field)
				}
				if outputs[field] != nil {
					t.Errorf("Missing field %q set to %v, want nil", field, outputs[field])
				}
			}

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

func copyTestMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

type ValidationError struct{}

func (e *ValidationError) Error() string {
	return "validation error"
}

// TestSignature_validateFieldType_DirectCalls tests validateFieldType directly for complete coverage
func TestSignature_validateFieldType_DirectCalls(t *testing.T) {
	sig := NewSignature("Test")

	tests := []struct {
		name    string
		field   Field
		value   any
		wantErr bool
	}{
		// Int type - test each case individually
		{"int as int", Field{Name: "f", Type: FieldTypeInt}, int(42), false},
		{"int as int8", Field{Name: "f", Type: FieldTypeInt}, int8(42), false},
		{"int as int16", Field{Name: "f", Type: FieldTypeInt}, int16(42), false},
		{"int as int32", Field{Name: "f", Type: FieldTypeInt}, int32(42), false},
		{"int as int64", Field{Name: "f", Type: FieldTypeInt}, int64(42), false},
		{"int as float64", Field{Name: "f", Type: FieldTypeInt}, float64(42.0), false},

		// Float type - test each int case
		{"float as float32", Field{Name: "f", Type: FieldTypeFloat}, float32(3.14), false},
		{"float as float64", Field{Name: "f", Type: FieldTypeFloat}, float64(3.14), false},
		{"float as int", Field{Name: "f", Type: FieldTypeFloat}, int(42), false},
		{"float as int8", Field{Name: "f", Type: FieldTypeFloat}, int8(42), false},
		{"float as int16", Field{Name: "f", Type: FieldTypeFloat}, int16(42), false},
		{"float as int32", Field{Name: "f", Type: FieldTypeFloat}, int32(42), false},
		{"float as int64", Field{Name: "f", Type: FieldTypeFloat}, int64(42), false},

		// JSON type - test each case
		{"json as map", Field{Name: "f", Type: FieldTypeJSON}, map[string]any{"key": "val"}, false},
		{"json as slice", Field{Name: "f", Type: FieldTypeJSON}, []int{1, 2, 3}, false},
		{"json as string", Field{Name: "f", Type: FieldTypeJSON}, `{"json":"string"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.validateFieldType(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFieldType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignatureToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		sig      *Signature
		validate func(t *testing.T, schema map[string]any)
	}{
		{
			name: "basic string and int fields",
			sig: NewSignature("Test signature").
				AddOutput("name", FieldTypeString, "Person's name").
				AddOutput("age", FieldTypeInt, "Person's age"),
			validate: func(t *testing.T, schema map[string]any) {
				if schema["type"] != "object" {
					t.Errorf("expected type object, got %v", schema["type"])
				}
				props, ok := schema["properties"].(map[string]any)
				if !ok {
					t.Fatal("expected properties map")
				}
				nameProp, ok := props["name"].(map[string]any)
				if !ok {
					t.Fatal("expected name property")
				}
				if nameProp["type"] != "string" {
					t.Errorf("expected name type string, got %v", nameProp["type"])
				}
				if nameProp["description"] != "Person's name" {
					t.Errorf("expected description, got %v", nameProp["description"])
				}
				ageProp, ok := props["age"].(map[string]any)
				if !ok {
					t.Fatal("expected age property")
				}
				if ageProp["type"] != "integer" {
					t.Errorf("expected age type integer, got %v", ageProp["type"])
				}
				required, ok := schema["required"].([]string)
				if !ok {
					t.Fatal("expected required array")
				}
				if len(required) != 2 {
					t.Errorf("expected 2 required fields, got %d", len(required))
				}
			},
		},
		{
			name: "class field with enum",
			sig: NewSignature("Classification").
				AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification"),
			validate: func(t *testing.T, schema map[string]any) {
				props := schema["properties"].(map[string]any)
				sentimentProp := props["sentiment"].(map[string]any)
				if sentimentProp["type"] != "string" {
					t.Errorf("expected sentiment type string, got %v", sentimentProp["type"])
				}
				enum, ok := sentimentProp["enum"].([]string)
				if !ok {
					t.Fatal("expected enum array")
				}
				if len(enum) != 3 {
					t.Errorf("expected 3 enum values, got %d", len(enum))
				}
				if enum[0] != "positive" || enum[1] != "negative" || enum[2] != "neutral" {
					t.Errorf("unexpected enum values: %v", enum)
				}
			},
		},
		{
			name: "optional fields",
			sig: NewSignature("Test").
				AddOutput("required_field", FieldTypeString, "Required").
				AddOptionalOutput("optional_field", FieldTypeString, "Optional"),
			validate: func(t *testing.T, schema map[string]any) {
				required, ok := schema["required"].([]string)
				if !ok {
					t.Fatal("expected required array")
				}
				if len(required) != 1 {
					t.Errorf("expected 1 required field, got %d", len(required))
				}
				if required[0] != "required_field" {
					t.Errorf("expected required_field, got %v", required[0])
				}
			},
		},
		{
			name: "all field types",
			sig: NewSignature("All types test").
				AddOutput("str_field", FieldTypeString, "String field").
				AddOutput("int_field", FieldTypeInt, "Int field").
				AddOutput("float_field", FieldTypeFloat, "Float field").
				AddOutput("bool_field", FieldTypeBool, "Bool field").
				AddOutput("json_field", FieldTypeJSON, "JSON field").
				AddOutput("image_field", FieldTypeImage, "Image field").
				AddOutput("datetime_field", FieldTypeDatetime, "Datetime field"),
			validate: func(t *testing.T, schema map[string]any) {
				props := schema["properties"].(map[string]any)

				expectedTypes := map[string]string{
					"str_field":      "string",
					"int_field":      "integer",
					"float_field":    "number",
					"bool_field":     "boolean",
					"json_field":     "object",
					"image_field":    "string",
					"datetime_field": "string",
				}

				for fieldName, expectedType := range expectedTypes {
					prop, ok := props[fieldName].(map[string]any)
					if !ok {
						t.Errorf("expected %s property", fieldName)
						continue
					}
					if prop["type"] != expectedType {
						t.Errorf("%s: expected type %s, got %v", fieldName, expectedType, prop["type"])
					}
				}
			},
		},
		{
			name: "signature description",
			sig:  NewSignature("This is a test signature"),
			validate: func(t *testing.T, schema map[string]any) {
				if schema["description"] != "This is a test signature" {
					t.Errorf("expected description in schema, got %v", schema["description"])
				}
			},
		},
		{
			name: "no required fields when all optional",
			sig: NewSignature("All optional").
				AddOptionalOutput("opt1", FieldTypeString, "").
				AddOptionalOutput("opt2", FieldTypeInt, ""),
			validate: func(t *testing.T, schema map[string]any) {
				_, hasRequired := schema["required"]
				if hasRequired {
					t.Error("expected no required field in schema when all fields are optional")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.sig.SignatureToJSONSchema()
			if schema == nil {
				t.Fatal("expected schema, got nil")
			}
			tt.validate(t, schema)
		})
	}
}

func TestSignatureToJSONSchema_EmptySignature(t *testing.T) {
	sig := NewSignature("Empty")
	schema := sig.SignatureToJSONSchema()

	if schema["type"] != "object" {
		t.Errorf("expected type object, got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if len(props) != 0 {
		t.Errorf("expected empty properties, got %d", len(props))
	}
}

// TestSignature_FieldValidation_EdgeCases tests field validation with exotic types and edge cases
func TestSignature_FieldValidation_EdgeCases(t *testing.T) {
	sig := NewSignature("Edge cases").
		AddOutput("exotic", FieldTypeString, "")

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{"nil value", nil, true}, // nil is not allowed for required fields
		{"empty string", "", false},
		{"very long string", string(make([]byte, 1000000)), false}, // Large string
		{"string with special chars", "hello\n\t\"world\"", false},
		{"unicode string", "héllo wörld 🚀", false},
	}

	// Test with required field
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := map[string]any{"exotic": tt.value}
			err := sig.ValidateOutputs(outputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v for value %v", err, tt.wantErr, tt.value)
			}
		})
	}
}

// TestSignature_JSONSchema_EdgeCases tests JSON schema generation edge cases
func TestSignature_JSONSchema_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		sig      *Signature
		validate func(t *testing.T, schema map[string]any)
	}{
		{
			name: "signature with very long description",
			sig:  NewSignature(string(make([]byte, 10000))), // 10k char description
			validate: func(t *testing.T, schema map[string]any) {
				if len(schema["description"].(string)) != 10000 {
					t.Errorf("expected 10000 char description, got %d", len(schema["description"].(string)))
				}
			},
		},
		{
			name: "fields with special characters in names",
			sig: NewSignature("Special names").
				AddOutput("field-with-dashes", FieldTypeString, "").
				AddOutput("field_with_underscores", FieldTypeString, "").
				AddOutput("field.with.dots", FieldTypeString, ""),
			validate: func(t *testing.T, schema map[string]any) {
				props := schema["properties"].(map[string]any)
				expectedFields := []string{"field-with-dashes", "field_with_underscores", "field.with.dots"}
				for _, field := range expectedFields {
					if _, exists := props[field]; !exists {
						t.Errorf("expected field %s in schema properties", field)
					}
				}
			},
		},
		{
			name: "class field with many enum values",
			sig: func() *Signature {
				sig := NewSignature("Many enums").AddOutput("category", FieldTypeClass, "")
				manyEnums := make([]string, 100)
				for i := range manyEnums {
					manyEnums[i] = fmt.Sprintf("value%d", i)
				}
				sig.OutputFields[0].Classes = manyEnums
				return sig
			}(),
			validate: func(t *testing.T, schema map[string]any) {
				props := schema["properties"].(map[string]any)
				catProp := props["category"].(map[string]any)
				enum := catProp["enum"].([]string)
				if len(enum) != 100 {
					t.Errorf("expected 100 enum values, got %d", len(enum))
				}
			},
		},
		{
			name: "mixed required and optional fields",
			sig: NewSignature("Mixed").
				AddOutput("required1", FieldTypeString, "").
				AddOptionalOutput("optional1", FieldTypeString, "").
				AddOutput("required2", FieldTypeInt, "").
				AddOptionalOutput("optional2", FieldTypeInt, ""),
			validate: func(t *testing.T, schema map[string]any) {
				required := schema["required"].([]string)
				if len(required) != 2 {
					t.Errorf("expected 2 required fields, got %d", len(required))
				}
				// Should only include required fields
				for _, req := range required {
					if req != "required1" && req != "required2" {
						t.Errorf("unexpected required field: %s", req)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.sig.SignatureToJSONSchema()
			if schema == nil {
				t.Fatal("expected schema, got nil")
			}
			tt.validate(t, schema)
		})
	}
}
