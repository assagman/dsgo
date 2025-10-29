package dsgo

import (
	"testing"
)

func TestGenerateOptions_Copy(t *testing.T) {
	original := &GenerateOptions{
		Temperature:      0.8,
		MaxTokens:        1024,
		TopP:             0.9,
		Stop:             []string{"STOP", "END"},
		ResponseFormat:   "json",
		ToolChoice:       "auto",
		Stream:           true,
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
		Tools: []Tool{
			{Name: "tool1", Description: "Test tool 1"},
			{Name: "tool2", Description: "Test tool 2"},
		},
	}

	copied := original.Copy()

	// Verify all fields are copied
	if copied.Temperature != original.Temperature {
		t.Errorf("Temperature not copied correctly: got %v, want %v", copied.Temperature, original.Temperature)
	}
	if copied.MaxTokens != original.MaxTokens {
		t.Errorf("MaxTokens not copied correctly: got %v, want %v", copied.MaxTokens, original.MaxTokens)
	}
	if copied.TopP != original.TopP {
		t.Errorf("TopP not copied correctly: got %v, want %v", copied.TopP, original.TopP)
	}
	if copied.ResponseFormat != original.ResponseFormat {
		t.Errorf("ResponseFormat not copied correctly: got %v, want %v", copied.ResponseFormat, original.ResponseFormat)
	}
	if copied.ToolChoice != original.ToolChoice {
		t.Errorf("ToolChoice not copied correctly: got %v, want %v", copied.ToolChoice, original.ToolChoice)
	}
	if copied.Stream != original.Stream {
		t.Errorf("Stream not copied correctly: got %v, want %v", copied.Stream, original.Stream)
	}
	if copied.FrequencyPenalty != original.FrequencyPenalty {
		t.Errorf("FrequencyPenalty not copied correctly: got %v, want %v", copied.FrequencyPenalty, original.FrequencyPenalty)
	}
	if copied.PresencePenalty != original.PresencePenalty {
		t.Errorf("PresencePenalty not copied correctly: got %v, want %v", copied.PresencePenalty, original.PresencePenalty)
	}

	// Verify slices are deep copied (not same memory address)
	if len(copied.Stop) != len(original.Stop) {
		t.Errorf("Stop slice length not copied correctly: got %v, want %v", len(copied.Stop), len(original.Stop))
	}
	if len(copied.Tools) != len(original.Tools) {
		t.Errorf("Tools slice length not copied correctly: got %v, want %v", len(copied.Tools), len(original.Tools))
	}

	// Verify modifying the copy doesn't affect the original
	copied.Stop[0] = "MODIFIED"
	if original.Stop[0] == "MODIFIED" {
		t.Error("Modifying copied Stop slice affected original")
	}

	copied.Tools[0].Name = "modified"
	if original.Tools[0].Name == "modified" {
		t.Error("Modifying copied Tools slice affected original")
	}
}

func TestGenerateOptions_Copy_Nil(t *testing.T) {
	var opts *GenerateOptions
	copied := opts.Copy()
	if copied != nil {
		t.Errorf("Copy of nil should return nil, got %v", copied)
	}
}

func TestGenerateOptions_Copy_EmptySlices(t *testing.T) {
	original := &GenerateOptions{
		Temperature: 0.7,
		Stop:        nil,
		Tools:       nil,
	}

	copied := original.Copy()

	if copied.Stop != nil {
		t.Errorf("Expected nil Stop slice, got %v", copied.Stop)
	}
	if copied.Tools != nil {
		t.Errorf("Expected nil Tools slice, got %v", copied.Tools)
	}
}
