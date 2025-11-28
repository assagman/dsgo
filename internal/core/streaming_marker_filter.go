package core

import (
	"regexp"
	"strings"
)

// StreamingMarkerFilter buffers streaming chunks and filters out field markers
// as they arrive, handling cases where markers are split across multiple chunks
type StreamingMarkerFilter struct {
	buffer           strings.Builder
	outputBuffer     strings.Builder
	possibleMarker   strings.Builder
	inPossibleMarker bool
}

// NewStreamingMarkerFilter creates a new filter
func NewStreamingMarkerFilter() *StreamingMarkerFilter {
	return &StreamingMarkerFilter{}
}

// ProcessChunk processes an incoming chunk and returns the displayable content
// Markers are buffered until complete, then discarded
func (f *StreamingMarkerFilter) ProcessChunk(chunkContent string) string {
	f.outputBuffer.Reset()

	for _, char := range chunkContent {
		f.buffer.WriteRune(char)

		// Check if we might be starting a marker
		if char == '[' && !f.inPossibleMarker {
			// Could be start of [[ ## field ## ]]
			f.possibleMarker.Reset()
			f.possibleMarker.WriteRune(char)
			f.inPossibleMarker = true
			continue
		}

		if f.inPossibleMarker {
			f.possibleMarker.WriteRune(char)
			current := f.possibleMarker.String()

			// Check if it matches a complete marker pattern
			if isCompleteMarker(current) {
				// It's a complete marker - discard it
				f.possibleMarker.Reset()
				f.inPossibleMarker = false
				continue
			}

			// Check if it could still become a marker
			if couldBeMarker(current) {
				// Keep buffering
				continue
			}

			// It's not a marker - flush the buffer
			f.outputBuffer.WriteString(f.possibleMarker.String())
			f.possibleMarker.Reset()
			f.inPossibleMarker = false
		} else {
			// Normal content
			f.outputBuffer.WriteRune(char)
		}
	}

	return f.outputBuffer.String()
}

// Flush returns any remaining buffered content
// Call this when the stream is complete
func (f *StreamingMarkerFilter) Flush() string {
	if f.inPossibleMarker && f.possibleMarker.Len() > 0 {
		// If we have incomplete marker buffer, it's not a marker - return it
		result := f.possibleMarker.String()
		f.possibleMarker.Reset()
		f.inPossibleMarker = false
		return result
	}
	return ""
}

// isCompleteMarker checks if the string is a complete field marker
func isCompleteMarker(s string) bool {
	// Pattern: [[ ## field ## ]] or [[## field ##]] (with flexible spacing)
	matched, _ := regexp.MatchString(`^\[\[\s*##\s*\w+\s*##\s*\]\]$`, s)
	return matched
}

// couldBeMarker checks if the string could potentially become a marker with more characters
func couldBeMarker(s string) bool {
	// Prefixes of valid markers:
	// [
	// [[
	// [[ #
	// [[ ##
	// [[ ## f
	// [[ ## field
	// [[ ## field #
	// [[ ## field ##
	// [[ ## field ## ]
	// [[ ## field ## ]]

	if len(s) == 0 {
		return false
	}

	// Must start with [
	if s[0] != '[' {
		return false
	}

	// Check if it could be a prefix of [[ ## field ## ]]
	// Allow flexible spacing
	pattern := `^\[\[?\s*#?#?\s*\w*\s*#?#?\s*\]?]?$`
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}
