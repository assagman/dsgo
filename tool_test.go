package dsgo

import (
	"context"
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
		"age":  30,
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
