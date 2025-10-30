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

// RepairJSON attempts to repair common JSON errors to make it parseable.
// This handles issues like:
// - Single quotes instead of double quotes for keys/values
// - Missing quotes around keys
// - Trailing commas
// - Smart quotes (""”)
// - Extra whitespace
//
// Returns the repaired JSON string. If repair is not possible, returns original.
func RepairJSON(jsonStr string) string {
	original := jsonStr
	jsonStr = strings.TrimSpace(jsonStr)

	// Strip markdown code fences if present
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		if idx := strings.Index(jsonStr, "```"); idx >= 0 {
			jsonStr = jsonStr[:idx]
		}
	} else if strings.HasPrefix(jsonStr, "```") {
		jsonStr = strings.TrimPrefix(jsonStr, "```")
		if idx := strings.Index(jsonStr, "```"); idx >= 0 {
			jsonStr = jsonStr[:idx]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	// Replace smart quotes with regular quotes
	replacer := strings.NewReplacer(
		"\u201c", "\"", // "
		"\u201d", "\"", // "
		"\u2018", "'", // '
		"\u2019", "'", // '
		"\u2032", "'", // ′ (prime)
		"\u2033", "\"", // ″ (double prime)
	)
	jsonStr = replacer.Replace(jsonStr)

	// Fix single quotes to double quotes (carefully)
	jsonStr = fixSingleQuotes(jsonStr)

	// Fix unquoted keys: {key: "value"} -> {"key": "value"}
	jsonStr = fixUnquotedKeys(jsonStr)

	// Remove trailing commas before } or ]
	jsonStr = removeTrailingCommas(jsonStr)

	// Find outermost {...} if multiple JSON objects
	if start := strings.Index(jsonStr, "{"); start >= 0 {
		end := findClosingBraceStringAware(jsonStr, start)
		if end > start {
			jsonStr = jsonStr[start : end+1]
		}
	}

	// Verify repair worked
	var test map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &test); err != nil {
		// Repair failed, return original
		return original
	}

	return jsonStr
}

// fixSingleQuotes replaces single quotes with double quotes for JSON strings
func fixSingleQuotes(jsonStr string) string {
	var result strings.Builder
	inDoubleQuote := false
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

		// Track double quotes
		if ch == '"' && !escape {
			inDoubleQuote = !inDoubleQuote
			result.WriteByte(ch)
			continue
		}

		// Convert single quotes to double quotes when not inside double-quoted strings
		if ch == '\'' && !inDoubleQuote {
			result.WriteByte('"')
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}

// fixUnquotedKeys adds quotes around unquoted object keys
func fixUnquotedKeys(jsonStr string) string {
	var result strings.Builder
	inString := false
	escape := false
	afterBrace := false

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
			afterBrace = false
			continue
		}

		if !inString {
			if ch == '{' || ch == ',' {
				result.WriteByte(ch)
				afterBrace = true
				continue
			}

			// If we see a key character after { or ,, and it's not quoted
			if afterBrace && !isWhitespace(ch) && ch != '"' && ch != '}' {
				// This might be an unquoted key
				keyStart := i
				keyEnd := i

				// Find the end of the key (up to :)
				for keyEnd < len(jsonStr) && jsonStr[keyEnd] != ':' && jsonStr[keyEnd] != '}' {
					keyEnd++
				}

				if keyEnd < len(jsonStr) && jsonStr[keyEnd] == ':' {
					// Extract the key
					key := strings.TrimSpace(jsonStr[keyStart:keyEnd])

					// Check if it's already quoted or is a number/boolean
					if !strings.HasPrefix(key, "\"") && !isJSONLiteral(key) {
						// Add quotes around the key
						result.WriteByte('"')
						result.WriteString(key)
						result.WriteByte('"')
						i = keyEnd - 1 // -1 because loop will increment
						afterBrace = false
						continue
					}
				}
			}
		}

		result.WriteByte(ch)

		if !inString && !isWhitespace(ch) {
			afterBrace = false
		}
	}

	return result.String()
}

// removeTrailingCommas removes trailing commas before } or ]
func removeTrailingCommas(jsonStr string) string {
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

		if !inString && ch == ',' {
			// Look ahead to see if only whitespace before } or ]
			j := i + 1
			for j < len(jsonStr) && isWhitespace(jsonStr[j]) {
				j++
			}
			if j < len(jsonStr) && (jsonStr[j] == '}' || jsonStr[j] == ']') {
				// Skip this comma
				continue
			}
		}

		result.WriteByte(ch)
	}

	return result.String()
}

// isWhitespace checks if a character is JSON whitespace
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// isJSONLiteral checks if a string is a JSON literal (true, false, null, number)
func isJSONLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	// Check if it's a number
	var f float64
	return json.Unmarshal([]byte(s), &f) == nil
}
