package dsgo

import "testing"

// TestSignature_AddOptionalInput tests optional input addition
func TestSignature_AddOptionalInput(t *testing.T) {
	sig := NewSignature("Test").
		AddOptionalInput("optional", FieldTypeString, "Optional field")

	if len(sig.InputFields) != 1 {
		t.Errorf("Expected 1 input field, got %d", len(sig.InputFields))
	}

	if !sig.InputFields[0].Optional {
		t.Error("Field should be marked as optional")
	}

	// Validate that missing optional input is OK
	err := sig.ValidateInputs(map[string]any{})
	if err != nil {
		t.Error("Optional input should not cause validation error when missing")
	}

	// Validate that provided optional input is validated
	err = sig.ValidateInputs(map[string]any{"optional": "value"})
	if err != nil {
		t.Errorf("ValidateInputs() with valid optional = %v", err)
	}

	// Validate that wrong type for optional input fails
	err = sig.ValidateInputs(map[string]any{"optional": 123})
	if err == nil {
		t.Error("Optional input with wrong type should fail validation")
	}
}

// TestSignature_ValidateInputs_CompleteScenarios tests comprehensive input validation
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

// TestSignature_ValidateFieldType_AllTypes tests all field type validations
func TestSignature_ValidateFieldType_AllTypes(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		value     any
		optional  bool
		wantErr   bool
	}{
		// String types
		{"string valid", FieldTypeString, "test", false, false},
		{"string invalid", FieldTypeString, 123, false, true},
		{"class valid", FieldTypeClass, "test", false, false},
		{"image valid", FieldTypeImage, "image.png", false, false},
		{"datetime valid", FieldTypeDatetime, "2025-01-01", false, false},

		// Int types - all Go int variants
		{"int valid", FieldTypeInt, int(42), false, false},
		{"int8 valid", FieldTypeInt, int8(42), false, false},
		{"int16 valid", FieldTypeInt, int16(42), false, false},
		{"int32 valid", FieldTypeInt, int32(42), false, false},
		{"int64 valid", FieldTypeInt, int64(42), false, false},
		{"int from float64 valid", FieldTypeInt, float64(42), false, false},
		{"int invalid string", FieldTypeInt, "42", false, true},

		// Float types - all Go float variants
		{"float32 valid", FieldTypeFloat, float32(3.14), false, false},
		{"float64 valid", FieldTypeFloat, float64(3.14), false, false},
		{"float from int valid", FieldTypeFloat, int(42), false, false},
		{"float from int64 valid", FieldTypeFloat, int64(42), false, false},
		{"float invalid string", FieldTypeFloat, "3.14", false, true},

		// Bool type
		{"bool true", FieldTypeBool, true, false, false},
		{"bool false", FieldTypeBool, false, false, false},
		{"bool invalid", FieldTypeBool, "true", false, true},

		// JSON type
		{"json map", FieldTypeJSON, map[string]any{"key": "value"}, false, false},
		{"json slice", FieldTypeJSON, []any{1, 2, 3}, false, false},
		{"json string", FieldTypeJSON, `{"key":"value"}`, false, false},
		{"json invalid", FieldTypeJSON, 123, false, true},

		// Nil handling
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
			// For nil optional, don't include the field
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
