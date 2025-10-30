package dsgo

import (
	"testing"
)

// TestGenerateOptions_Copy_StreamCallback tests copying with StreamCallback
func TestGenerateOptions_Copy_StreamCallback(t *testing.T) {
	options := DefaultGenerateOptions()
	options.StreamCallback = func(chunk Chunk) {
		// Mock callback
	}

	copied := options.Copy()
	if copied == nil {
		t.Fatal("Expected copy to be non-nil")
	}

	// Callback should not be copied (function pointers can't be deep copied)
	// Just verify basic fields are copied
	if copied.Temperature != options.Temperature {
		t.Error("Temperature not copied correctly")
	}
	if copied.MaxTokens != options.MaxTokens {
		t.Error("MaxTokens not copied correctly")
	}
}

// TestDefaultGenerateOptions_Values tests default values
func TestDefaultGenerateOptions_Values(t *testing.T) {
	opts := DefaultGenerateOptions()

	if opts.Temperature != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %v", opts.Temperature)
	}
	if opts.MaxTokens != 2048 {
		t.Errorf("Expected default max tokens 2048, got %v", opts.MaxTokens)
	}
	if opts.TopP != 1.0 {
		t.Errorf("Expected default TopP 1.0, got %v", opts.TopP)
	}
	if opts.ResponseFormat != "text" {
		t.Errorf("Expected default response format 'text', got '%s'", opts.ResponseFormat)
	}
	if opts.ToolChoice != "auto" {
		t.Errorf("Expected default tool choice 'auto', got '%s'", opts.ToolChoice)
	}
	if opts.Stream != false {
		t.Errorf("Expected default stream false, got %v", opts.Stream)
	}
}
