package core

import (
	"context"
	"encoding/json"
	"testing"
)

// TestTool_BooleanCoercion tests boolean parameter coercion
func TestTool_BooleanCoercion(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		expected   bool
		shouldPass bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, false, true},
		{"string true", "true", true, true},
		{"string TRUE", "TRUE", true, true},
		{"string false", "false", false, true},
		{"string FALSE", "FALSE", false, true},
		{"string 1", "1", true, true},
		{"string 0", "0", false, true},
		{"int 1", 1, true, true},
		{"int 0", 0, false, true},
		{"int64 1", int64(1), true, true},
		{"int64 0", int64(0), false, true},
		{"uint 1", uint(1), true, true},
		{"uint 0", uint(0), false, true},
		{"float64 1.0", 1.0, true, true},
		{"float64 0.0", 0.0, false, true},
		{"invalid string yes", "yes", false, false},
		{"invalid string no", "no", false, false},
		{"invalid int 2", 2, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue bool
			tool := NewTool(
				"test_bool",
				"Test boolean coercion",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["enabled"].(bool)
					return receivedValue, nil
				},
			).AddParameter("enabled", "bool", "Enabled flag", true)

			result, err := tool.Execute(context.Background(), map[string]any{
				"enabled": tt.input,
			})

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if receivedValue != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, receivedValue)
				}
				if result != tt.expected {
					t.Errorf("Expected result %v, got %v", tt.expected, result)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for invalid boolean value")
				}
			}
		})
	}
}

// TestTool_IntValidation tests integer parameter validation
func TestTool_IntValidation(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		expected   int64
		shouldPass bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(42), 42, true},
		{"int32", int32(42), 42, true},
		{"int16", int16(42), 42, true},
		{"int8", int8(42), 42, true},
		{"uint", uint(42), 42, true},
		{"uint32", uint32(42), 42, true},
		{"string int", "42", 42, true},
		{"string negative", "-100", -100, true},
		{"float64 integral", 42.0, 42, true},
		{"float32 integral", float32(42.0), 42, true},
		{"json.Number", json.Number("42"), 42, true},
		{"float64 decimal", 42.5, 0, false},
		{"invalid string", "not-a-number", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue int64
			tool := NewTool(
				"test_int",
				"Test int validation",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["count"].(int64)
					return receivedValue, nil
				},
			).AddParameter("count", "int", "Count parameter", true)

			result, err := tool.Execute(context.Background(), map[string]any{
				"count": tt.input,
			})

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if receivedValue != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, receivedValue)
				}
				if result != tt.expected {
					t.Errorf("Expected result %v, got %v", tt.expected, result)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for invalid int value")
				}
			}
		})
	}
}

// TestTool_FloatValidation tests float parameter validation
func TestTool_FloatValidation(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		expected   float64
		shouldPass bool
		tolerance  float64
	}{
		{"float64", 3.14, 3.14, true, 0},
		{"float32", float32(3.14), 3.14, true, 0.001}, // float32 has less precision
		{"int", 42, 42.0, true, 0},
		{"int64", int64(42), 42.0, true, 0},
		{"uint", uint(42), 42.0, true, 0},
		{"string float", "3.14", 3.14, true, 0},
		{"string int", "42", 42.0, true, 0},
		{"string scientific", "1.5e2", 150.0, true, 0},
		{"json.Number", json.Number("3.14"), 3.14, true, 0},
		{"invalid string", "not-a-number", 0, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue float64
			tool := NewTool(
				"test_float",
				"Test float validation",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["amount"].(float64)
					return receivedValue, nil
				},
			).AddParameter("amount", "float", "Amount parameter", true)

			result, err := tool.Execute(context.Background(), map[string]any{
				"amount": tt.input,
			})

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if tt.tolerance > 0 {
					// Check with tolerance for float32 precision
					diff := receivedValue - tt.expected
					if diff < 0 {
						diff = -diff
					}
					if diff > tt.tolerance {
						t.Errorf("Expected %v, got %v (diff: %v, tolerance: %v)", tt.expected, receivedValue, diff, tt.tolerance)
					}
				} else {
					if receivedValue != tt.expected {
						t.Errorf("Expected %v, got %v", tt.expected, receivedValue)
					}
					if result != tt.expected {
						t.Errorf("Expected result %v, got %v", tt.expected, result)
					}
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for invalid float value")
				}
			}
		})
	}
}

// TestTool_JSONParsing tests JSON parameter parsing
func TestTool_JSONParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		shouldPass bool
		validate   func(t *testing.T, value any)
	}{
		{
			name:       "map directly",
			input:      map[string]any{"key": "value"},
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				m, ok := value.(map[string]any)
				if !ok {
					t.Errorf("Expected map[string]any, got %T", value)
				}
				if m["key"] != "value" {
					t.Errorf("Expected key=value, got %v", m["key"])
				}
			},
		},
		{
			name:       "slice directly",
			input:      []any{1, 2, 3},
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				s, ok := value.([]any)
				if !ok {
					t.Errorf("Expected []any, got %T", value)
				}
				if len(s) != 3 {
					t.Errorf("Expected length 3, got %d", len(s))
				}
			},
		},
		{
			name:       "JSON string object",
			input:      `{"name":"John","age":30}`,
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				m, ok := value.(map[string]any)
				if !ok {
					t.Errorf("Expected map[string]any, got %T", value)
				}
				if m["name"] != "John" {
					t.Errorf("Expected name=John, got %v", m["name"])
				}
			},
		},
		{
			name:       "JSON string array",
			input:      `[1,2,3]`,
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				s, ok := value.([]any)
				if !ok {
					t.Errorf("Expected []any, got %T", value)
				}
				if len(s) != 3 {
					t.Errorf("Expected length 3, got %d", len(s))
				}
			},
		},
		{
			name:       "invalid JSON string",
			input:      `{invalid json}`,
			shouldPass: false,
			validate:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue any
			tool := NewTool(
				"test_json",
				"Test JSON parsing",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["data"]
					return receivedValue, nil
				},
			).AddParameter("data", "json", "JSON data parameter", true)

			result, err := tool.Execute(context.Background(), map[string]any{
				"data": tt.input,
			})

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if tt.validate != nil {
					tt.validate(t, receivedValue)
					tt.validate(t, result)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for invalid JSON")
				}
			}
		})
	}
}

// TestTool_ArrayHandling tests array parameter handling
func TestTool_ArrayHandling(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		shouldPass bool
		validate   func(t *testing.T, value any)
	}{
		{
			name:       "slice of strings",
			input:      []string{"a", "b", "c"},
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				s, ok := value.([]string)
				if !ok {
					t.Errorf("Expected []string, got %T", value)
				}
				if len(s) != 3 {
					t.Errorf("Expected length 3, got %d", len(s))
				}
			},
		},
		{
			name:       "slice of ints",
			input:      []int{1, 2, 3},
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				_, ok := value.([]int)
				if !ok {
					t.Errorf("Expected []int, got %T", value)
				}
			},
		},
		{
			name:       "slice of any",
			input:      []any{1, "two", 3.0},
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				s, ok := value.([]any)
				if !ok {
					t.Errorf("Expected []any, got %T", value)
				}
				if len(s) != 3 {
					t.Errorf("Expected length 3, got %d", len(s))
				}
			},
		},
		{
			name:       "CSV string to array",
			input:      "a,b,c",
			shouldPass: true,
			validate: func(t *testing.T, value any) {
				s, ok := value.([]string)
				if !ok {
					t.Errorf("Expected []string, got %T", value)
				}
				if len(s) != 3 {
					t.Errorf("Expected length 3, got %d", len(s))
				}
				if s[0] != "a" || s[1] != "b" || s[2] != "c" {
					t.Errorf("Expected [a,b,c], got %v", s)
				}
			},
		},
		{
			name:       "invalid non-array value",
			input:      42,
			shouldPass: false,
			validate:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue any
			tool := NewTool(
				"test_array",
				"Test array handling",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["items"]
					return receivedValue, nil
				},
			).AddArrayParameter("items", "Array of items", "", true)

			result, err := tool.Execute(context.Background(), map[string]any{
				"items": tt.input,
			})

			if tt.shouldPass {
				if err != nil {
					t.Fatalf("Execute() error = %v, expected success", err)
				}
				if tt.validate != nil {
					tt.validate(t, receivedValue)
					tt.validate(t, result)
				}
			} else {
				if err == nil {
					t.Error("Expected validation error for non-array value")
				}
			}
		})
	}
}

// TestTool_TypeSynonyms tests that type synonyms are normalized correctly
func TestTool_TypeSynonyms(t *testing.T) {
	tests := []struct {
		name        string
		paramType   string
		input       any
		expectedVal any
		checkType   bool
	}{
		{"number to float", "number", 42, 42.0, true},
		{"integer to int", "integer", 42, int64(42), true},
		{"boolean to bool", "boolean", true, true, true},
		{"object to json", "object", map[string]any{"key": "val"}, nil, false}, // Check type only
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedValue any
			tool := NewTool(
				"test_synonyms",
				"Test type synonyms",
				func(ctx context.Context, args map[string]any) (any, error) {
					receivedValue = args["value"]
					return receivedValue, nil
				},
			).AddParameter("value", tt.paramType, "Test parameter", true)

			_, err := tool.Execute(context.Background(), map[string]any{
				"value": tt.input,
			})

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if tt.checkType {
				if receivedValue != tt.expectedVal {
					t.Errorf("Expected %v (%T), got %v (%T)", tt.expectedVal, tt.expectedVal, receivedValue, receivedValue)
				}
			} else {
				// For maps, just check the type and content
				if m, ok := receivedValue.(map[string]any); ok {
					if m["key"] != "val" {
						t.Errorf("Expected map with key=val, got %v", m)
					}
				} else {
					t.Errorf("Expected map[string]any, got %T", receivedValue)
				}
			}
		})
	}
}

// TestTool_EnumRestrictionToString tests that enums are only validated for string parameters
func TestTool_EnumRestrictionToString(t *testing.T) {
	tool := NewTool(
		"test_enum",
		"Test enum restriction",
		func(ctx context.Context, args map[string]any) (any, error) {
			return args["status"], nil
		},
	).AddEnumParameter("status", "Status value", []string{"active", "inactive", "pending"}, true)

	// Valid enum value
	_, err := tool.Execute(context.Background(), map[string]any{
		"status": "active",
	})
	if err != nil {
		t.Errorf("Expected success for valid enum value, got: %v", err)
	}

	// Invalid enum value
	_, err = tool.Execute(context.Background(), map[string]any{
		"status": "unknown",
	})
	if err == nil {
		t.Error("Expected error for invalid enum value")
	}
}

// TestTool_BackwardCompatibility tests backward compatibility with existing code
func TestTool_BackwardCompatibility(t *testing.T) {
	// Test that "number" type still works (maps to float)
	tool := NewTool(
		"convert_currency",
		"Convert currency",
		func(ctx context.Context, args map[string]any) (any, error) {
			amount := args["amount"].(float64)
			return amount * 1.1, nil
		},
	).AddParameter("amount", "number", "Amount to convert", true)

	result, err := tool.Execute(context.Background(), map[string]any{
		"amount": "100",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check with small tolerance for floating point arithmetic
	resultFloat, ok := result.(float64)
	if !ok {
		t.Fatalf("Expected float64 result, got %T", result)
	}
	expected := 110.0
	diff := resultFloat - expected
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("Expected ~%v, got %v (diff: %v)", expected, resultFloat, diff)
	}

	// Test that arrays to CSV still works for string parameters
	csvTool := NewTool(
		"test_csv",
		"Test CSV conversion",
		func(ctx context.Context, args map[string]any) (any, error) {
			return args["data"].(string), nil
		},
	).AddParameter("data", "string", "Data parameter", true)

	result, err = csvTool.Execute(context.Background(), map[string]any{
		"data": []string{"a", "b", "c"},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result != "a,b,c" {
		t.Errorf("Expected 'a,b,c', got %v", result)
	}
}
