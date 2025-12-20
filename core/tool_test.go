package core

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTool_Execute(t *testing.T) {
	tool := NewTool(
		"test_tool",
		"A test tool",
		func(ctx context.Context, args map[string]any) (any, error) {
			name := args["name"].(string)
			return "Hello, " + name, nil
		},
	).AddParameter("name", "string", "Name parameter", true)

	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]any{
		"name": "World",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result != "Hello, World" {
		t.Errorf("Expected 'Hello, World', got '%v'", result)
	}
}

func TestTool_AddParameter(t *testing.T) {
	tool := NewTool("test", "Test tool", nil).
		AddParameter("param1", "string", "First param", true).
		AddParameter("param2", "number", "Second param", false)

	if len(tool.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(tool.Parameters))
	}

	if tool.Parameters[0].Required != true {
		t.Error("Expected first parameter to be required")
	}

	if tool.Parameters[1].Required != false {
		t.Error("Expected second parameter to be optional")
	}
}

func TestTool_AddEnumParameter(t *testing.T) {
	tool := NewTool("test", "Test tool", nil).
		AddEnumParameter("status", "Status value", []string{"active", "inactive"}, true)

	if len(tool.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(tool.Parameters))
	}

	param := tool.Parameters[0]
	if param.Type != "string" {
		t.Errorf("Expected type 'string', got '%s'", param.Type)
	}

	if len(param.Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(param.Enum))
	}
}

func TestTool_Validate_Success(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("name", "string", "Name parameter", true)
	tool.AddParameter("age", "int", "Age parameter", false)

	args := map[string]any{
		"name": "John",
		"age":  int64(30), // Must be int64 after normalization
	}

	err := tool.Validate(args)
	if err != nil {
		t.Errorf("validation should succeed, got error: %v", err)
	}
}

func TestTool_Validate_MissingRequired(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("name", "string", "Name parameter", true)

	args := map[string]any{}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for missing required parameter")
	}
}

func TestTool_Validate_EnumSuccess(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddEnumParameter("status", "Status value", []string{"active", "inactive"}, true)

	args := map[string]any{
		"status": "active",
	}

	err := tool.Validate(args)
	if err != nil {
		t.Errorf("validation should succeed, got error: %v", err)
	}
}

func TestTool_Validate_EnumInvalid(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddEnumParameter("status", "Status value", []string{"active", "inactive"}, true)

	args := map[string]any{
		"status": "pending",
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for invalid enum value")
	}
}

func TestTool_Execute_WithValidation(t *testing.T) {
	called := false
	tool := NewTool("test", "Test tool", func(ctx context.Context, args map[string]any) (any, error) {
		called = true
		return "success", nil
	})
	tool.AddParameter("name", "string", "Name parameter", true)

	// Should fail validation
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected validation error")
	}
	if called {
		t.Error("function should not be called when validation fails")
	}

	// Should succeed
	result, err := tool.Execute(context.Background(), map[string]any{"name": "John"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("function should be called when validation succeeds")
	}
	if result != "success" {
		t.Errorf("expected 'success', got %v", result)
	}
}

func TestTool_NormalizeArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected string
	}{
		{
			name:     "array of interfaces",
			input:    map[string]any{"data": []interface{}{1, 2, 3}},
			expected: "1,2,3",
		},
		{
			name:     "array of strings",
			input:    map[string]any{"data": []string{"a", "b", "c"}},
			expected: "a,b,c",
		},
		{
			name:     "array of ints",
			input:    map[string]any{"data": []int{10, 20, 30}},
			expected: "10,20,30",
		},
		{
			name:     "array of floats",
			input:    map[string]any{"data": []float64{1.5, 2.5, 3.5}},
			expected: "1.5,2.5,3.5",
		},
		{
			name:     "single string value",
			input:    map[string]any{"data": "single"},
			expected: "single",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue string
			tool := NewTool(
				"test_normalize",
				"Test normalization",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["data"].(string)
					return receivedValue, nil
				},
			).AddParameter("data", "string", "Data parameter", true)

			result, err := tool.Execute(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if receivedValue != tt.expected {
				t.Errorf("Expected normalized value '%s', got '%s'", tt.expected, receivedValue)
			}

			if result != tt.expected {
				t.Errorf("Expected result '%s', got '%v'", tt.expected, result)
			}
		})
	}
}

// TestTool_Validate_EnumNonStringType tests enum validation with non-string parameter
func TestTool_Validate_EnumNonStringType(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddEnumParameter("status", "Status value", []string{"active", "inactive"}, true)

	// Try to validate enum with a non-string value
	args := map[string]any{
		"status": 123, // Not a string
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for enum with non-string value")
	}
	// validateType runs first and rejects the non-string value before enum check
	if err.Error() != "parameter status has invalid type: expected string, got int" {
		t.Errorf("expected type validation error, got: %v", err)
	}
}

// TestTool_Validate_TypeMismatch_String tests validation fails for string type mismatch
func TestTool_Validate_TypeMismatch_String(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("name", "string", "Name parameter", true)

	args := map[string]any{
		"name": 123, // Wrong type
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for type mismatch")
	}
}

// TestTool_Validate_TypeMismatch_Bool tests validation fails for bool type mismatch
func TestTool_Validate_TypeMismatch_Bool(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("enabled", "bool", "Enable flag", true)

	args := map[string]any{
		"enabled": "true", // Wrong type - should be bool, not string
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for bool type mismatch")
	}
}

// TestTool_Validate_TypeMismatch_JSON tests validation fails for JSON type mismatch
func TestTool_Validate_TypeMismatch_JSON(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("data", "json", "JSON data", true)

	// Try to pass a string instead of object/array
	args := map[string]any{
		"data": "not json", // Wrong type
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for JSON type mismatch")
	}
}

// TestTool_Validate_TypeMismatch_Array tests validation fails for array type mismatch
func TestTool_Validate_TypeMismatch_Array(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("items", "array", "Array items", true)

	args := map[string]any{
		"items": "not an array", // Wrong type
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for array type mismatch")
	}
}

// TestTool_NormalizeParamType_CustomType tests normalizeParamType with unknown/custom type
func TestTool_NormalizeParamType_CustomType(t *testing.T) {
	// Custom types should be preserved as-is
	result := normalizeParamType("custom_type")
	if result != ParamType("custom_type") {
		t.Errorf("Expected 'custom_type', got %v", result)
	}
}

// TestTool_Validate_CustomType tests validation skips unknown parameter types
func TestTool_Validate_CustomType(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("custom", "custom_type", "Custom parameter", true)

	args := map[string]any{
		"custom": "any value", // Should accept any value for unknown type
	}

	err := tool.Validate(args)
	if err != nil {
		t.Errorf("Expected validation to pass for unknown type, got error: %v", err)
	}
}

// TestTool_NormalizeInt_UnsignedTypes tests normalization of unsigned integer types
func TestTool_NormalizeInt_UnsignedTypes(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		want  int64
	}{
		{
			name:  "uint8",
			input: map[string]any{"num": uint8(42)},
			want:  42,
		},
		{
			name:  "uint16",
			input: map[string]any{"num": uint16(1000)},
			want:  1000,
		},
		{
			name:  "uint32",
			input: map[string]any{"num": uint32(100000)},
			want:  100000,
		},
		{
			name:  "uint64 within int64 range",
			input: map[string]any{"num": uint64(9223372036854775807)}, // MaxInt64
			want:  9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue int64
			tool := NewTool(
				"test_uint",
				"Test unsigned conversion",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["num"].(int64)
					return receivedValue, nil
				},
			).AddParameter("num", "int", "Number", true)

			result, err := tool.Execute(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if receivedValue != tt.want {
				t.Errorf("Expected %v, got %v", tt.want, receivedValue)
			}

			if result != tt.want {
				t.Errorf("Expected result %v, got %v", tt.want, result)
			}
		})
	}
}

// TestTool_NormalizeInt_UintOverflow tests uint64 overflow handling
func TestTool_NormalizeInt_UintOverflow(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)
	tool.AddParameter("num", "int", "Number", true)

	// uint64 value that exceeds MaxInt64
	args := map[string]any{
		"num": uint64(9223372036854775808), // MaxInt64 + 1
	}

	// Should fail validation because it can't be converted to int64
	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for uint64 overflow")
	}
}

// TestTool_NormalizeToInt_AllTypes tests normalizeToInt with all numeric types
func TestTool_NormalizeToInt_AllTypes(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)

	tests := []struct {
		name        string
		input       any
		checkResult func(t *testing.T, result any)
	}{
		// String cases
		{
			name:  "string valid int",
			input: "42",
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
		{
			name:  "string valid negative",
			input: "-99",
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != -99 {
					t.Errorf("expected -99, got %v", result)
				}
			},
		},
		{
			name:  "string invalid",
			input: "not-an-int",
			checkResult: func(t *testing.T, result any) {
				if s, ok := result.(string); !ok || s != "not-an-int" {
					t.Errorf("expected unchanged string, got %v", result)
				}
			},
		},
		// Integer types
		{
			name:  "int",
			input: int(42),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
		{
			name:  "int8",
			input: int8(10),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 10 {
					t.Errorf("expected 10, got %v", result)
				}
			},
		},
		{
			name:  "int16",
			input: int16(100),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 100 {
					t.Errorf("expected 100, got %v", result)
				}
			},
		},
		{
			name:  "int32",
			input: int32(1000),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 1000 {
					t.Errorf("expected 1000, got %v", result)
				}
			},
		},
		{
			name:  "int64",
			input: int64(10000),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 10000 {
					t.Errorf("expected 10000, got %v", result)
				}
			},
		},
		// Unsigned integer types
		{
			name:  "uint",
			input: uint(42),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
		{
			name:  "uint8",
			input: uint8(10),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 10 {
					t.Errorf("expected 10, got %v", result)
				}
			},
		},
		{
			name:  "uint16",
			input: uint16(100),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 100 {
					t.Errorf("expected 100, got %v", result)
				}
			},
		},
		{
			name:  "uint32",
			input: uint32(1000),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 1000 {
					t.Errorf("expected 1000, got %v", result)
				}
			},
		},
		{
			name:  "uint64 within range",
			input: uint64(10000),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 10000 {
					t.Errorf("expected 10000, got %v", result)
				}
			},
		},
		// Float types - whole numbers only
		{
			name:  "float32 whole number",
			input: float32(42.0),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
		{
			name:  "float32 not whole",
			input: float32(42.5),
			checkResult: func(t *testing.T, result any) {
				// Should return unchanged for non-whole floats
				if _, ok := result.(float32); !ok {
					t.Errorf("expected unchanged float32, got %T", result)
				}
			},
		},
		{
			name:  "float64 whole number",
			input: float64(99.0),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 99 {
					t.Errorf("expected 99, got %v", result)
				}
			},
		},
		{
			name:  "float64 not whole",
			input: float64(99.5),
			checkResult: func(t *testing.T, result any) {
				// Should return unchanged for non-whole floats
				if _, ok := result.(float64); !ok {
					t.Errorf("expected unchanged float64, got %T", result)
				}
			},
		},
		// JSON number
		{
			name:  "json.Number valid",
			input: json.Number("42"),
			checkResult: func(t *testing.T, result any) {
				if i, ok := result.(int64); !ok || i != 42 {
					t.Errorf("expected 42, got %v", result)
				}
			},
		},
		{
			name:  "json.Number invalid",
			input: json.Number("not-a-number"),
			checkResult: func(t *testing.T, result any) {
				// Should return unchanged for invalid json.Number
				if _, ok := result.(json.Number); !ok {
					t.Errorf("expected json.Number unchanged, got %T", result)
				}
			},
		},
		// Default case - returns unchanged
		{
			name:  "bool returns unchanged",
			input: true,
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected unchanged bool true, got %v", result)
				}
			},
		},
		{
			name:  "nil returns unchanged",
			input: nil,
			checkResult: func(t *testing.T, result any) {
				if result != nil {
					t.Errorf("expected nil unchanged, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.normalizeToInt(tt.input)
			tt.checkResult(t, result)
		})
	}
}

// TestTool_NormalizeFloat_StringParse tests float normalization from strings
func TestTool_NormalizeFloat_StringParse(t *testing.T) {
	tests := []struct {
		name           string
		input          map[string]any
		expectedValue  float64
		shouldValidate bool
	}{
		{
			name:           "valid float string",
			input:          map[string]any{"value": "3.14"},
			expectedValue:  3.14,
			shouldValidate: true,
		},
		{
			name:           "int string to float",
			input:          map[string]any{"value": "42"},
			expectedValue:  42.0,
			shouldValidate: true,
		},
		{
			name:           "invalid float string",
			input:          map[string]any{"value": "not-a-float"},
			expectedValue:  0,
			shouldValidate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue float64
			tool := NewTool(
				"test",
				"Test tool",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["value"].(float64)
					return receivedValue, nil
				},
			).AddParameter("value", "float", "Float value", true)

			_, err := tool.Execute(context.Background(), tt.input)

			if tt.shouldValidate {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if receivedValue != tt.expectedValue {
					t.Errorf("Expected %v, got %v", tt.expectedValue, receivedValue)
				}
			} else {
				if err == nil {
					t.Error("expected validation error")
				}
			}
		})
	}
}

// TestTool_NormalizeToFloat_AllTypes tests normalizeToFloat with all numeric types
func TestTool_NormalizeToFloat_AllTypes(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)

	tests := []struct {
		name        string
		input       any
		checkResult func(t *testing.T, result any)
	}{
		// String cases
		{
			name:  "string valid float",
			input: "3.14",
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 3.14 {
					t.Errorf("expected 3.14, got %v", result)
				}
			},
		},
		{
			name:  "string valid int",
			input: "42",
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 42.0 {
					t.Errorf("expected 42.0, got %v", result)
				}
			},
		},
		{
			name:  "string invalid",
			input: "not-a-number",
			checkResult: func(t *testing.T, result any) {
				if s, ok := result.(string); !ok || s != "not-a-number" {
					t.Errorf("expected unchanged string, got %v", result)
				}
			},
		},
		// Integer types
		{
			name:  "int",
			input: int(42),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 42.0 {
					t.Errorf("expected 42.0, got %v", result)
				}
			},
		},
		{
			name:  "int8",
			input: int8(10),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 10.0 {
					t.Errorf("expected 10.0, got %v", result)
				}
			},
		},
		{
			name:  "int16",
			input: int16(100),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 100.0 {
					t.Errorf("expected 100.0, got %v", result)
				}
			},
		},
		{
			name:  "int32",
			input: int32(1000),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 1000.0 {
					t.Errorf("expected 1000.0, got %v", result)
				}
			},
		},
		{
			name:  "int64",
			input: int64(10000),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 10000.0 {
					t.Errorf("expected 10000.0, got %v", result)
				}
			},
		},
		// Unsigned integer types
		{
			name:  "uint",
			input: uint(42),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 42.0 {
					t.Errorf("expected 42.0, got %v", result)
				}
			},
		},
		{
			name:  "uint8",
			input: uint8(10),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 10.0 {
					t.Errorf("expected 10.0, got %v", result)
				}
			},
		},
		{
			name:  "uint16",
			input: uint16(100),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 100.0 {
					t.Errorf("expected 100.0, got %v", result)
				}
			},
		},
		{
			name:  "uint32",
			input: uint32(1000),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 1000.0 {
					t.Errorf("expected 1000.0, got %v", result)
				}
			},
		},
		{
			name:  "uint64",
			input: uint64(10000),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 10000.0 {
					t.Errorf("expected 10000.0, got %v", result)
				}
			},
		},
		// Float types
		{
			name:  "float32",
			input: float32(3.14),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok {
					t.Errorf("expected float64, got %T", result)
				} else if f < 3.13 || f > 3.15 { // Allow for precision loss
					t.Errorf("expected ~3.14, got %v", f)
				}
			},
		},
		{
			name:  "float64",
			input: float64(2.71),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 2.71 {
					t.Errorf("expected 2.71, got %v", result)
				}
			},
		},
		// JSON number
		{
			name:  "json.Number valid",
			input: json.Number("3.14"),
			checkResult: func(t *testing.T, result any) {
				if f, ok := result.(float64); !ok || f != 3.14 {
					t.Errorf("expected 3.14, got %v", result)
				}
			},
		},
		{
			name:  "json.Number invalid",
			input: json.Number("not-a-number"),
			checkResult: func(t *testing.T, result any) {
				// Should return unchanged for invalid json.Number
				if _, ok := result.(json.Number); !ok {
					t.Errorf("expected json.Number unchanged, got %T", result)
				}
			},
		},
		// Default case - returns unchanged
		{
			name:  "bool returns unchanged",
			input: true,
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected unchanged bool true, got %v", result)
				}
			},
		},
		{
			name:  "nil returns unchanged",
			input: nil,
			checkResult: func(t *testing.T, result any) {
				if result != nil {
					t.Errorf("expected nil unchanged, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.normalizeToFloat(tt.input)
			tt.checkResult(t, result)
		})
	}
}

// TestTool_NormalizeToBool_AllTypes tests normalizeToBool with all types
func TestTool_NormalizeToBool_AllTypes(t *testing.T) {
	tool := NewTool("test", "Test tool", nil)

	tests := []struct {
		name        string
		input       any
		checkResult func(t *testing.T, result any)
	}{
		// Bool cases
		{
			name:  "bool true",
			input: true,
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "bool false",
			input: false,
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		// String cases
		{
			name:  "string 'true'",
			input: "true",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "string 'True'",
			input: "True",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "string 'TRUE'",
			input: "TRUE",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "string '1'",
			input: "1",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "string 'false'",
			input: "false",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "string 'False'",
			input: "False",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "string '0'",
			input: "0",
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "string invalid",
			input: "maybe",
			checkResult: func(t *testing.T, result any) {
				if s, ok := result.(string); !ok || s != "maybe" {
					t.Errorf("expected unchanged string, got %v", result)
				}
			},
		},
		// Integer types - 0 and 1 only
		{
			name:  "int 0",
			input: int(0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "int 1",
			input: int(1),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "int other",
			input: int(42),
			checkResult: func(t *testing.T, result any) {
				if _, ok := result.(int); !ok {
					t.Errorf("expected unchanged int, got %T", result)
				}
			},
		},
		{
			name:  "int8 0",
			input: int8(0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "int8 1",
			input: int8(1),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		// Unsigned integer types
		{
			name:  "uint 0",
			input: uint(0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "uint 1",
			input: uint(1),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "uint8 0",
			input: uint8(0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "uint16 1",
			input: uint16(1),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		// Float types
		{
			name:  "float32 0",
			input: float32(0.0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "float32 1",
			input: float32(1.0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		{
			name:  "float64 0",
			input: float64(0.0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || b {
					t.Errorf("expected false, got %v", result)
				}
			},
		},
		{
			name:  "float64 1",
			input: float64(1.0),
			checkResult: func(t *testing.T, result any) {
				if b, ok := result.(bool); !ok || !b {
					t.Errorf("expected true, got %v", result)
				}
			},
		},
		// Default case - returns unchanged
		{
			name:  "nil returns unchanged",
			input: nil,
			checkResult: func(t *testing.T, result any) {
				if result != nil {
					t.Errorf("expected nil unchanged, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.normalizeToBool(tt.input)
			tt.checkResult(t, result)
		})
	}
}

// TestTool_NormalizeBool_Invalid tests bool normalization with invalid values
func TestTool_NormalizeBool_Invalid(t *testing.T) {
	tool := NewTool("test", "Test tool", nil).
		AddParameter("enabled", "bool", "Boolean flag", true)

	args := map[string]any{
		"enabled": "maybe", // Not a valid bool string
	}

	err := tool.Validate(args)
	if err == nil {
		t.Error("expected validation error for invalid bool value")
	}
}

// TestTool_Execute_ContextCancellation tests that context cancellation mid-execution
// is properly handled by Tool.Execute
func TestTool_Execute_ContextCancellation(t *testing.T) {
	tool := NewTool(
		"long_running_tool",
		"A tool that simulates long-running work",
		func(ctx context.Context, args map[string]any) (any, error) {
			// Simulate long-running operation that respects context
			<-ctx.Done()
			return nil, ctx.Err()
		},
	).AddParameter("input", "string", "Input parameter", true)

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tool.Execute(ctx, map[string]any{"input": "test"})

	if err == nil {
		t.Error("Expected error from cancelled context")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestTool_NormalizeArguments_NumberCoercion tests that string-encoded numbers
// are properly coerced to numeric types when parameter type is "number"
func TestTool_NormalizeArguments_NumberCoercion(t *testing.T) {
	tests := []struct {
		name          string
		input         map[string]any
		expectedValue float64
		shouldPass    bool
	}{
		{
			name:          "string to number - integer string",
			input:         map[string]any{"amount": "450"},
			expectedValue: 450.0,
			shouldPass:    true,
		},
		{
			name:          "string to number - decimal string",
			input:         map[string]any{"amount": "450.75"},
			expectedValue: 450.75,
			shouldPass:    true,
		},
		{
			name:          "already float64",
			input:         map[string]any{"amount": 450.0},
			expectedValue: 450.0,
			shouldPass:    true,
		},
		{
			name:          "integer to float64",
			input:         map[string]any{"amount": 450},
			expectedValue: 450.0,
			shouldPass:    true,
		},
		{
			name:          "invalid string - should fail validation",
			input:         map[string]any{"amount": "not-a-number"},
			expectedValue: 0,
			shouldPass:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue float64
			tool := NewTool(
				"convert_currency",
				"Convert currency amounts",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["amount"].(float64)
					return receivedValue, nil
				},
			).AddParameter("amount", "number", "Amount to convert", true)

			result, err := tool.Execute(context.Background(), tt.input)

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}

				if receivedValue != tt.expectedValue {
					t.Errorf("Expected normalized value %v, got %v", tt.expectedValue, receivedValue)
				}

				if result != tt.expectedValue {
					t.Errorf("Expected result %v, got %v", tt.expectedValue, result)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for invalid number string")
				}
			}
		})
	}
}
