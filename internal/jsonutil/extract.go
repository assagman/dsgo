// Package jsonutil provides utilities for extracting and parsing JSON from LM responses.
package jsonutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractOptions configures JSON extraction behavior
type ExtractOptions struct {
	// FixNewlines enables automatic fixing of unescaped newlines in strings
	FixNewlines bool
	// IgnoreStringContext if true, uses simpler brace matching without string awareness
	IgnoreStringContext bool
}

// Option is a functional option for ExtractJSON
type Option func(*ExtractOptions)

// WithFixNewlines enables automatic fixing of unescaped newlines in JSON strings
func WithFixNewlines() Option {
	return func(o *ExtractOptions) {
		o.FixNewlines = true
	}
}

// WithSimpleBraceMatching uses simpler brace matching that doesn't track string context
func WithSimpleBraceMatching() Option {
	return func(o *ExtractOptions) {
		o.IgnoreStringContext = true
	}
}

// ExtractJSON extracts JSON from markdown code blocks or raw text.
// It handles:
// - JSON markdown code blocks (```json ... ```)
// - Generic markdown code blocks (``` ... ```)
// - Raw JSON objects in text
// - Brace matching to find complete JSON objects
// - Multiple JSON objects (returns the largest/most complete one)
func ExtractJSON(content string, opts ...Option) (string, error) {
	options := &ExtractOptions{}
	for _, opt := range opts {
		opt(options)
	}

	content = strings.TrimSpace(content)

	// Try to extract JSON from markdown code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else if strings.Contains(content, "```") {
		// Skip non-JSON code blocks (like python, javascript, etc.)
		start := strings.Index(content, "```")
		// Check if this is a code block with a language identifier
		lineEnd := strings.Index(content[start:], "\n")
		if lineEnd > 0 {
			lang := strings.TrimSpace(content[start+3 : start+lineEnd])
			// If it's not JSON and it looks like a programming language, skip this block
			if lang != "" && lang != "json" && !strings.Contains(lang, "{") {
				// Try to find JSON after the code block
				end := strings.Index(content[start+3:], "```")
				if end > 0 {
					afterBlock := content[start+3+end+3:]
					if idx := strings.Index(afterBlock, "{"); idx >= 0 {
						content = afterBlock
					}
				}
			} else {
				// Extract from generic code block
				start := strings.Index(content, "```") + 3
				end := strings.Index(content[start:], "```")
				if end > 0 {
					content = content[start : start+end]
				}
			}
		}
	}

	// Extract all JSON objects and select the largest/most complete one
	candidates := extractAllJSONObjects(content, options)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no JSON object found in content")
	}

	// Return the largest JSON object (likely the most complete)
	content = selectBestJSON(candidates)

	content = strings.TrimSpace(content)

	return content, nil
}

// extractAllJSONObjects finds all complete JSON objects in the content
func extractAllJSONObjects(content string, options *ExtractOptions) []string {
	var candidates []string
	pos := 0

	for {
		start := strings.Index(content[pos:], "{")
		if start < 0 {
			break
		}
		start += pos

		// Find matching closing brace
		var end int
		if options.IgnoreStringContext {
			end = findClosingBraceSimple(content, start)
		} else {
			end = findClosingBraceStringAware(content, start)
		}

		if end > start {
			jsonStr := content[start : end+1]

			// Fix newlines if requested before validation
			if options.FixNewlines {
				jsonStr = fixJSONNewlines(jsonStr)
			}

			// Validate that it's actually valid JSON
			var test map[string]any
			if json.Unmarshal([]byte(jsonStr), &test) == nil {
				candidates = append(candidates, jsonStr)
			}
			pos = end + 1
		} else {
			pos = start + 1
		}
	}

	return candidates
}

// findClosingBraceSimple finds the matching closing brace without string awareness
func findClosingBraceSimple(content string, start int) int {
	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// findClosingBraceStringAware finds the matching closing brace with string awareness
func findClosingBraceStringAware(content string, start int) int {
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(content); i++ {
		if escape {
			escape = false
			continue
		}
		if content[i] == '\\' {
			escape = true
			continue
		}
		if content[i] == '"' {
			inString = !inString
		}
		if !inString {
			switch content[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return -1
}

// selectBestJSON selects the best JSON object from candidates
// Prefers larger objects with more fields
func selectBestJSON(candidates []string) string {
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Parse all candidates and select the one with the most fields
	bestIdx := 0
	maxFields := 0

	for i, candidate := range candidates {
		var obj map[string]any
		if err := json.Unmarshal([]byte(candidate), &obj); err == nil {
			if len(obj) > maxFields {
				maxFields = len(obj)
				bestIdx = i
			}
		}
	}

	return candidates[bestIdx]
}

// ParseJSON extracts and parses JSON from content
func ParseJSON(content string, opts ...Option) (map[string]any, error) {
	jsonStr, err := ExtractJSON(content, opts...)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, jsonStr)
	}

	return result, nil
}

// fixJSONNewlines fixes unescaped newlines in JSON strings
func fixJSONNewlines(jsonStr string) string {
	var result strings.Builder
	inString := false
	escape := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]

		if escape {
			result.WriteByte(ch)
			escape = false
			continue
		}

		if ch == '\\' {
			result.WriteByte(ch)
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		// If we're inside a string and encounter a newline, escape it
		if inString && (ch == '\n' || ch == '\r') {
			if ch == '\n' {
				result.WriteString("\\n")
			}
			// Skip \r as it's often paired with \n
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}
