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
