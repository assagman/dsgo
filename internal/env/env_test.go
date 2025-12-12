package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Not parallel: uses t.Setenv (process-wide env).
	// Create a temporary .env file for testing
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	content := `# Test .env file
TEST_KEY1=value1
TEST_KEY2=value2
export TEST_KEY3=value3
TEST_KEY4="quoted value"
TEST_KEY5='single quoted value'
# Comment line
TEST_KEY6=value with spaces
INVALID_LINE
`

	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Clear any existing test environment variables
	t.Setenv("TEST_KEY1", "")
	t.Setenv("TEST_KEY2", "")
	t.Setenv("TEST_KEY3", "")
	t.Setenv("TEST_KEY4", "")
	t.Setenv("TEST_KEY5", "")
	t.Setenv("TEST_KEY6", "")

	// Load the .env file
	err = Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check that environment variables were set correctly
	tests := []struct {
		key   string
		want  string
		found bool
	}{
		{"TEST_KEY1", "value1", true},
		{"TEST_KEY2", "value2", true},
		{"TEST_KEY3", "value3", true},
		{"TEST_KEY4", "quoted value", true},
		{"TEST_KEY5", "single quoted value", true},
		{"TEST_KEY6", "value with spaces", true},
		{"INVALID_LINE", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.key, func(t *testing.T) {
			got := os.Getenv(tt.key)
			if tt.found {
				if got != tt.want {
					t.Errorf("expected %s=%q, got %q", tt.key, tt.want, got)
				}
			} else {
				if got != "" {
					t.Errorf("expected %s to not be set, but got %q", tt.key, got)
				}
			}
		})
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	// Not parallel: this package has tests that mutate env/CWD.
	err := Load("/non/existent/file.env")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestFindEnvFile(t *testing.T) {
	// Not parallel: this package has tests that mutate env/CWD.
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create subdirectories
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	subSubDir := filepath.Join(subDir, "subsubdir")
	err = os.Mkdir(subSubDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subsubdir: %v", err)
	}

	// Create .env file in root
	envFile := filepath.Join(tmpDir, ".env")
	err = os.WriteFile(envFile, []byte("TEST_ROOT=value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Test finding .env file from subsubdir (should find root .env)
	found := findEnvFile(subSubDir, ".env")
	if found != envFile {
		t.Errorf("expected to find %s, got %s", envFile, found)
	}

	// Test finding non-existent file
	found = findEnvFile(subSubDir, ".nonexistent")
	if found != "" {
		t.Errorf("expected empty string for non-existent file, got %s", found)
	}
}

func TestAutoLoadWithCustomPath(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	customEnvFile := filepath.Join(tmpDir, "custom.env")

	content := "AUTOLOAD_CUSTOM=test_value\nTEST_OVERRIDE=custom"
	err := os.WriteFile(customEnvFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create custom .env file: %v", err)
	}

	// Clear test variables
	t.Setenv("AUTOLOAD_CUSTOM", "")
	t.Setenv("TEST_OVERRIDE", "")

	// Set DSGO_ENV_FILE_PATH
	t.Setenv("DSGO_ENV_FILE_PATH", customEnvFile)

	// Run AutoLoad
	err = AutoLoad()
	if err != nil {
		t.Fatalf("AutoLoad failed: %v", err)
	}

	// Verify variables were loaded
	if os.Getenv("AUTOLOAD_CUSTOM") != "test_value" {
		t.Errorf("expected AUTOLOAD_CUSTOM=test_value, got %q", os.Getenv("AUTOLOAD_CUSTOM"))
	}
}

func TestAutoLoadWithNonExistentCustomPath(t *testing.T) {
	// Not parallel: modifies process-wide environment variables.
	t.Setenv("DSGO_ENV_FILE_PATH", "/non/existent/path.env")

	// Run AutoLoad - should return error
	err := AutoLoad()
	if err == nil {
		t.Error("expected error for non-existent DSGO_ENV_FILE_PATH, got nil")
	}
}

func TestAutoLoadWithDiscovery(t *testing.T) {
	// Create temporary directory with .env file
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Create .env file
	envFile := filepath.Join(tmpDir, ".env")
	content := "AUTOLOAD_DISCOVERY=discovered_value\n"
	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Clear test variable
	t.Setenv("AUTOLOAD_DISCOVERY", "")

	// Run AutoLoad (no DSGO_ENV_FILE_PATH set)
	err = AutoLoad()
	if err != nil {
		t.Fatalf("AutoLoad failed: %v", err)
	}

	// Verify variable was loaded
	if os.Getenv("AUTOLOAD_DISCOVERY") != "discovered_value" {
		t.Errorf("expected AUTOLOAD_DISCOVERY=discovered_value, got %q", os.Getenv("AUTOLOAD_DISCOVERY"))
	}
}

func TestAutoLoadWithLocalOverride(t *testing.T) {
	// Create temporary directory with both .env and .env.local
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Create .env file
	envFile := filepath.Join(tmpDir, ".env")
	content := "SHARED_KEY=env_value\nLOCAL_ONLY=env_only\n"
	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Create .env.local file (should override)
	localEnvFile := filepath.Join(tmpDir, ".env.local")
	localContent := "SHARED_KEY=local_value\nLOCAL_ONLY=local_only\n"
	err = os.WriteFile(localEnvFile, []byte(localContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env.local file: %v", err)
	}

	// Clear test variables
	t.Setenv("SHARED_KEY", "")
	t.Setenv("LOCAL_ONLY", "")

	// Run AutoLoad
	err = AutoLoad()
	if err != nil {
		t.Fatalf("AutoLoad failed: %v", err)
	}

	// Verify .env.local takes precedence
	if os.Getenv("SHARED_KEY") != "local_value" {
		t.Errorf("expected SHARED_KEY=local_value (from .env.local), got %q", os.Getenv("SHARED_KEY"))
	}
	if os.Getenv("LOCAL_ONLY") != "local_only" {
		t.Errorf("expected LOCAL_ONLY=local_only (from .env.local), got %q", os.Getenv("LOCAL_ONLY"))
	}
}

func TestAutoLoadNoFiles(t *testing.T) {
	// Create temporary directory with no .env files
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Ensure no DSGO_ENV_FILE_PATH
	t.Setenv("DSGO_ENV_FILE_PATH", "")

	// Run AutoLoad - should succeed with no error
	err := AutoLoad()
	if err != nil {
		t.Errorf("AutoLoad should succeed when no .env files found, got error: %v", err)
	}
}

func TestAutoLoadDoesNotOverrideExisting(t *testing.T) {
	// Create temporary directory with .env file
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Create .env file
	envFile := filepath.Join(tmpDir, ".env")
	content := "EXISTING_VAR=from_env_file\n"
	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// Pre-set the environment variable
	t.Setenv("EXISTING_VAR", "pre_existing")

	// Run AutoLoad
	err = AutoLoad()
	if err != nil {
		t.Fatalf("AutoLoad failed: %v", err)
	}

	// Verify existing value was NOT overridden
	if os.Getenv("EXISTING_VAR") != "pre_existing" {
		t.Errorf("expected EXISTING_VAR to remain 'pre_existing', got %q", os.Getenv("EXISTING_VAR"))
	}
}
