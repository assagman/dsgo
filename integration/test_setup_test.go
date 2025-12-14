package integration

import (
	"os"
	"testing"

	dsgo "github.com/assagman/dsgo"
)

func init() {
	// Disable structured outputs for all integration tests to test legacy behavior
	// This is called before TestMain and before any tests run
	dsgo.Configure(dsgo.WithStructuredOutputEnabled(false))
}

// TestMain disables structured outputs for all integration tests to test legacy behavior
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
