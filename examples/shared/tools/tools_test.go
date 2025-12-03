package tools

import (
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	// Test that we can find the project root
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}

	// The root should contain dsgo.go file
	if root == "" {
		t.Fatal("FindProjectRoot() returned empty string")
	}

	t.Logf("Found project root: %s", root)
}

func TestValidatePath(t *testing.T) {
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"valid relative path", "go.mod", false},
		{"valid absolute path", root, false},
		{"path traversal attempt", "../../../etc/passwd", true},
		{"empty path", "", false}, // Should resolve to project root
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validatePath(root, tt.path)
			if (err != nil) != tt.expectError {
				t.Errorf("validatePath() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestGetAllFilesystemTools(t *testing.T) {
	tools := GetAllFilesystemTools()

	if len(tools) != 3 {
		t.Fatalf("GetAllFilesystemTools() returned %d tools, expected 3", len(tools))
	}

	expectedNames := map[string]bool{
		"list_files":   false,
		"read_file":    false,
		"search_files": false,
	}

	for _, tool := range tools {
		if _, exists := expectedNames[tool.Name]; !exists {
			t.Errorf("Unexpected tool name: %s", tool.Name)
		}
		expectedNames[tool.Name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected tool %s not found", name)
		}
	}
}
