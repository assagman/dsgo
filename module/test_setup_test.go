package module

import (
	"fmt"
	"os"
	"testing"

	"github.com/assagman/dsgo/core"
)

// TestMain disables structured outputs for all module tests to test legacy behavior
func TestMain(m *testing.M) {
	// Disable structured outputs for all module tests to test legacy behavior.
	// Configure must succeed before running tests.
	if err := os.Setenv("DSGO_OPENAI_API_KEY", "test-openai-key"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set test API key env var: %v\n", err)
		os.Exit(1)
	}
	if err := core.Configure(core.WithStructuredOutputEnabled(false)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure module tests: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	os.Exit(code)
}
