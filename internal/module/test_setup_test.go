package module

import (
	"os"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

func init() {
	// Disable structured outputs for all module tests to test legacy behavior
	// This is called before TestMain and before any tests run
	core.Configure(core.WithStructuredOutputEnabled(false))
}

// TestMain disables structured outputs for all module tests to test legacy behavior
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
