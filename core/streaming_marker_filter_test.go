package core

import (
	"testing"
)

func TestStreamingMarkerFilter_SingleChunkWithMarker(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	// Complete marker in one chunk
	result := filter.ProcessChunk("[[ ## response ## ]] Hello world")
	if result != " Hello world" {
		t.Errorf("Expected ' Hello world', got %q", result)
	}
}

func TestStreamingMarkerFilter_MarkerSplitAcrossChunks(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	// Marker split across multiple chunks (simulates real streaming)
	chunks := []string{
		"[[",
		"##",
		" response ",
		"##",
		"]] ",
		"Hello",
		" world",
	}

	var result string
	for _, chunk := range chunks {
		result += filter.ProcessChunk(chunk)
	}
	result += filter.Flush()

	// Note: There's a space after "]] " in the chunks
	expected := " Hello world"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStreamingMarkerFilter_MultipleMarkers(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	result := filter.ProcessChunk("[[ ## field1 ## ]] ")
	result += filter.ProcessChunk("Text ")
	result += filter.ProcessChunk("[[ ## field2 ## ]] ")
	result += filter.ProcessChunk("More text")
	result += filter.Flush()

	expected := " Text  More text"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStreamingMarkerFilter_NoMarkers(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	chunks := []string{"Hello", " ", "world", "!"}
	var result string
	for _, chunk := range chunks {
		result += filter.ProcessChunk(chunk)
	}
	result += filter.Flush()

	expected := "Hello world!"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStreamingMarkerFilter_IncompleteMarkerAtEnd(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	// Start of a marker that never completes
	result := filter.ProcessChunk("Hello ")
	result += filter.ProcessChunk("[[")
	result += filter.ProcessChunk("##")
	// Stream ends before marker completes
	result += filter.Flush()

	// Incomplete marker should be returned as content
	expected := "Hello [[##"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStreamingMarkerFilter_FalseStartBracket(t *testing.T) {
	filter := NewStreamingMarkerFilter()

	// Single bracket that's not part of a marker
	result := filter.ProcessChunk("Hello [world]")
	result += filter.Flush()

	expected := "Hello [world]"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestStreamingMarkerFilter_MarkerWithVariableSpacing(t *testing.T) {
	// Marker with different spacing patterns
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "compact marker",
			input:    "[[##response##]] Hello",
			expected: " Hello",
		},
		{
			name:     "spaced marker",
			input:    "[[  ##  response  ##  ]] Hello",
			expected: " Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewStreamingMarkerFilter()
			result := f.ProcessChunk(tt.input)
			result += f.Flush()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsCompleteMarker(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"[[ ## response ## ]]", true},
		{"[[## response ##]]", true},
		{"[[  ##  response  ##  ]]", true},
		{"[[ ## answer ## ]]", true},
		{"[[", false},
		{"[[ ##", false},
		{"[[ ## response", false},
		{"[[ ## response ##", false},
		{"[[ ## response ## ]", false},
		{"Hello", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isCompleteMarker(tt.input)
			if result != tt.expected {
				t.Errorf("isCompleteMarker(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCouldBeMarker(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"[", true},
		{"[[", true},
		{"[[ ", true},
		{"[[ #", true},
		{"[[ ##", true},
		{"[[ ## ", true},
		{"[[ ## r", true},
		{"[[ ## response", true},
		{"[[ ## response ", true},
		{"[[ ## response #", true},
		{"[[ ## response ##", true},
		{"[[ ## response ## ", true},
		{"[[ ## response ## ]", true},
		{"Hello", false},
		{"# ##", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := couldBeMarker(tt.input)
			if result != tt.expected {
				t.Errorf("couldBeMarker(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
