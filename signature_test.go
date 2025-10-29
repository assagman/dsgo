package dsgo

import (
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
