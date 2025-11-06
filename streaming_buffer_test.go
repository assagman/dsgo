package dsgo

import (
	"testing"
)

func TestStreamingBuffer_BasicAccumulation(t *testing.T) {
	sb := NewStreamingBuffer()

	sb.Write("Hello ")
	sb.Write("world")

	if sb.String() != "Hello world" {
		t.Errorf("String() = %q, want %q", sb.String(), "Hello world")
	}

	if sb.Len() != 11 {
		t.Errorf("Len() = %d, want 11", sb.Len())
	}
}

func TestStreamingBuffer_IncompleteMarkerAtEnd(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "missing closing brackets",
			input: `[[ ## reasoning ## ]]
Step by step

[[ ## answer ##`,
			expected: `[[ ## reasoning ## ]]
Step by step

[[ ## answer ## ]]`,
		},
		{
			name: "single closing bracket",
			input: `[[ ## reasoning ## ]]
Thinking...

[[ ## answer ## ]
42`,
			expected: `[[ ## reasoning ## ]]
Thinking...

[[ ## answer ## ]]
42`,
		},
		{
			name: "complete markers - no change",
			input: `[[ ## reasoning ## ]]
Done

[[ ## answer ## ]]
Complete`,
			expected: `[[ ## reasoning ## ]]
Done

[[ ## answer ## ]]
Complete`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStreamingBuffer()
			sb.Write(tt.input)
			result := sb.Finalize()

			if result != tt.expected {
				t.Errorf("Finalize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStreamingBuffer_RealTestMatrixFailure(t *testing.T) {
	// This simulates the exact failure mode from minimax/minimax-m2 020_streaming
	sb := NewStreamingBuffer()

	// Simulate chunked streaming
	sb.Write("[[ ## story ## ]]\n")
	sb.Write("On a morning when the sun rose...\n")
	sb.Write("...the artifact was a question.\n\n")
	sb.Write("[[ ## title ## ]]\n")
	sb.Write("The Stone That Dreamed\n\n")
	sb.Write("[[ ## genre ## ]]\n")
	sb.Write("Science Fiction\n\n")
	// Stream cuts off here - incomplete explanation marker
	sb.Write("[[ ## explanation ##")

	result := sb.Finalize()

	// Should add closing brackets
	if !Contains(result, "[[ ## explanation ## ]]") {
		t.Errorf("Failed to repair incomplete marker, got: %s", result[len(result)-50:])
	}
}

func TestStreamingBuffer_DetectIncompleteMarker(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantFound bool
		wantField string
	}{
		{
			name:      "incomplete at end",
			content:   "Some text\n[[ ## answer",
			wantFound: true,
			wantField: "answer",
		},
		{
			name:      "incomplete with ##",
			content:   "Text here\n[[ ## explanation ##",
			wantFound: true,
			wantField: "explanation",
		},
		{
			name:      "complete marker",
			content:   "Text\n[[ ## answer ## ]]",
			wantFound: false,
			wantField: "",
		},
		{
			name:      "no marker",
			content:   "Just plain text",
			wantFound: false,
			wantField: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStreamingBuffer()
			sb.Write(tt.content)

			found, field := sb.DetectIncompleteMarker()

			if found != tt.wantFound {
				t.Errorf("DetectIncompleteMarker() found = %v, want %v", found, tt.wantFound)
			}

			if field != tt.wantField {
				t.Errorf("DetectIncompleteMarker() field = %q, want %q", field, tt.wantField)
			}
		})
	}
}

func TestStreamingBuffer_MultipleIncompleteMarkers(t *testing.T) {
	sb := NewStreamingBuffer()
	sb.Write(`[[ ## field1 ## ]]
Value 1

[[ ## field2 ##
Value 2

[[ ## field3 ## ]
Value 3`)

	result := sb.Finalize()

	// All markers should be repaired
	if !Contains(result, "[[ ## field2 ## ]]") {
		t.Error("Failed to repair field2 marker")
	}
	if !Contains(result, "[[ ## field3 ## ]]") {
		t.Error("Failed to repair field3 marker")
	}
}

func TestStreamingBuffer_GetFieldMarkerCompletion(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		field          string
		wantCompletion string
	}{
		{
			name:           "missing ## ]]",
			content:        "text\n[[ ## answer",
			field:          "answer",
			wantCompletion: " ## ]]",
		},
		{
			name:           "missing ]]",
			content:        "text\n[[ ## answer ##",
			field:          "answer",
			wantCompletion: " ]]",
		},
		{
			name:           "missing ]",
			content:        "text\n[[ ## answer ## ]",
			field:          "answer",
			wantCompletion: "]",
		},
		{
			name:           "complete - no completion needed",
			content:        "text\n[[ ## answer ## ]]",
			field:          "answer",
			wantCompletion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStreamingBuffer()
			sb.Write(tt.content)

			completion := sb.GetFieldMarkerCompletion(tt.field)

			if completion != tt.wantCompletion {
				t.Errorf("GetFieldMarkerCompletion() = %q, want %q", completion, tt.wantCompletion)
			}
		})
	}
}

func TestStreamingBuffer_TrailingArtifacts(t *testing.T) {
	sb := NewStreamingBuffer()
	sb.Write("[[ ## answer ## ]]\n42\n\n  \n\t")

	result := sb.Finalize()

	// Should trim trailing whitespace
	if result[len(result)-1] == ' ' || result[len(result)-1] == '\n' || result[len(result)-1] == '\t' {
		t.Error("Failed to clean trailing artifacts")
	}
}

// Helper function
func Contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && ContainsSubstring(s, substr))
}

func ContainsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
