package core

import (
	"fmt"
	"regexp"
	"strings"
)

// StreamingBuffer handles accumulation and recovery of streaming LM responses
// Detects incomplete field markers and repairs them before final parsing
type StreamingBuffer struct {
	content strings.Builder
}

// NewStreamingBuffer creates a new streaming buffer
func NewStreamingBuffer() *StreamingBuffer {
	return &StreamingBuffer{}
}

// Write appends a chunk to the buffer
func (sb *StreamingBuffer) Write(chunk string) {
	sb.content.WriteString(chunk)
}

// String returns the current accumulated content
func (sb *StreamingBuffer) String() string {
	return sb.content.String()
}

// Len returns the length of accumulated content
func (sb *StreamingBuffer) Len() int {
	return sb.content.Len()
}

// Finalize performs recovery and returns the final content
// Fixes incomplete markers, trailing artifacts, and other streaming issues
func (sb *StreamingBuffer) Finalize() string {
	content := sb.content.String()

	// Apply recovery fixes
	content = sb.repairIncompleteMarkers(content)
	content = sb.cleanTrailingArtifacts(content)

	return content
}

// repairIncompleteMarkers detects and repairs incomplete field markers
// Common streaming failures:
// - "[[ ## field ##" (missing closing brackets)
// - "[[ ## field ## ]" (single closing bracket)
// - "[[ ## field" (incomplete marker)
func (sb *StreamingBuffer) repairIncompleteMarkers(content string) string {
	// Pattern 1: Incomplete marker at end of stream: [[ ## field ##
	// Repair by adding closing brackets
	pattern1 := regexp.MustCompile(`\[\[\s*##\s*(\w+)\s*##\s*$`)
	if pattern1.MatchString(content) {
		content = content + " ]]"
	}

	// Pattern 2: Incomplete marker with single bracket: [[ ## field ## ]
	// This pattern is in the middle of content, replace with complete marker
	pattern2 := regexp.MustCompile(`\[\[\s*##\s*(\w+)\s*##\s*\](?:[^]\n])`)
	content = pattern2.ReplaceAllString(content, "[[ ## $1 ## ]]$2")

	// Pattern 3: Multiple incomplete markers in content
	// Find all instances of incomplete markers and repair them
	lines := strings.Split(content, "\n")
	var repaired []string

	for _, line := range lines {
		// Check if line contains incomplete marker pattern
		if strings.Contains(line, "[[ ##") && !strings.Contains(line, "## ]]") {
			// Check for pattern: [[ ## fieldname ##
			re := regexp.MustCompile(`\[\[\s*##\s*(\w+)\s*##\s*$`)
			if re.MatchString(strings.TrimSpace(line)) {
				line = line + " ]]"
			}

			// Check for pattern: [[ ## fieldname ## ]
			re2 := regexp.MustCompile(`\[\[\s*##\s*(\w+)\s*##\s*\]\s*$`)
			if re2.MatchString(strings.TrimSpace(line)) {
				line = re2.ReplaceAllString(line, "[[ ## $1 ## ]]")
			}
		}
		repaired = append(repaired, line)
	}

	return strings.Join(repaired, "\n")
}

// cleanTrailingArtifacts removes incomplete content at the end of stream
// that might have been cut off mid-generation
func (sb *StreamingBuffer) cleanTrailingArtifacts(content string) string {
	content = strings.TrimRight(content, " \t\n\r")

	// Remove trailing commas or braces that might indicate incomplete JSON
	content = strings.TrimSuffix(content, ",")
	content = strings.TrimSuffix(content, "{")

	return content
}

// DetectIncompleteMarker checks if the current buffer ends with an incomplete marker
// This can be used during streaming to detect issues early
func (sb *StreamingBuffer) DetectIncompleteMarker() (bool, string) {
	content := sb.content.String()

	// Check last 100 characters for incomplete markers
	tail := content
	if len(content) > 100 {
		tail = content[len(content)-100:]
	}

	// Pattern: [[ ## fieldname (no closing)
	re := regexp.MustCompile(`\[\[\s*##\s*(\w+)(?:\s*##?\s*)?$`)
	matches := re.FindStringSubmatch(tail)
	if len(matches) > 1 {
		return true, matches[1] // Return field name
	}

	return false, ""
}

// GetFieldMarkerCompletion returns the expected completion for an incomplete marker
func (sb *StreamingBuffer) GetFieldMarkerCompletion(fieldName string) string {
	content := sb.content.String()

	// Check what's already present
	tail := content
	if len(content) > 50 {
		tail = content[len(content)-50:]
	}

	// Case 1: [[ ## fieldname
	if strings.HasSuffix(strings.TrimSpace(tail), fmt.Sprintf("[[ ## %s", fieldName)) {
		return " ## ]]"
	}

	// Case 2: [[ ## fieldname ##
	if strings.HasSuffix(strings.TrimSpace(tail), fmt.Sprintf("[[ ## %s ##", fieldName)) {
		return " ]]"
	}

	// Case 3: [[ ## fieldname ## ]
	if strings.HasSuffix(strings.TrimSpace(tail), fmt.Sprintf("[[ ## %s ## ]", fieldName)) {
		return "]"
	}

	return ""
}
