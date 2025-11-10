package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/assagman/dsgo/internal/jsonutil"
)

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractNumericValue extracts the first numeric value (int or float) from a string
// This handles cases where LMs return "High" or "0.95" or "95%" for numeric fields
func extractNumericValue(s string) string {
	// Try to extract a number using regex
	// Matches: optional minus, digits, optional decimal point and more digits
	re := regexp.MustCompile(`-?\d+\.?\d*`)
	match := re.FindString(s)
	if match != "" {
		return match
	}

	// Map common qualitative confidence values to numeric equivalents
	// This handles LMs that return "High", "Low", "Medium" instead of numbers
	lowerS := strings.ToLower(strings.TrimSpace(s))
	confidenceMap := map[string]string{
		"very high": "0.95",
		"high":      "0.9",
		"medium":    "0.7",
		"moderate":  "0.7",
		"low":       "0.3",
		"very low":  "0.1",
	}
	if numericValue, ok := confidenceMap[lowerS]; ok {
		return numericValue
	}

	// If no numeric value found, return original (will fail coercion later)
	return s
}

// coerceOutputs attempts to convert output values to expected types based on signature.
// This is a shared helper used by both JSONAdapter and ChatAdapter to ensure consistent
// type coercion behavior across all adapters.
func coerceOutputs(sig *Signature, outputs map[string]any, allowArrayToString bool) map[string]any {
	result := make(map[string]any)

	for key, value := range outputs {
		field := sig.GetOutputField(key)
		if field == nil {
			result[key] = value
			continue
		}

		switch field.Type {
		case FieldTypeInt:
			// string → int, float64 → int
			if s, ok := value.(string); ok {
				// Extract numeric value first (handles "High (95%)" → "95")
				s = extractNumericValue(s)
				if i, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
					result[key] = i
					continue
				}
			}
			if f, ok := value.(float64); ok {
				result[key] = int(f)
				continue
			}

		case FieldTypeFloat:
			// string → float64, int → float64
			if s, ok := value.(string); ok {
				// Extract numeric value first (handles "High (0.95)" → "0.95")
				s = extractNumericValue(s)
				trimmed := strings.TrimSpace(s)
				if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
					result[key] = f
					continue
				}
			}
			if i, ok := value.(int); ok {
				result[key] = float64(i)
				continue
			}

		case FieldTypeBool:
			// string → bool
			if s, ok := value.(string); ok {
				if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
					result[key] = b
					continue
				}
			}

		case FieldTypeJSON:
			// For JSON fields, if value is a string containing JSON, parse it
			if s, ok := value.(string); ok && s != "" {
				var parsed any
				if err := json.Unmarshal([]byte(s), &parsed); err == nil {
					// Successfully parsed as JSON
					result[key] = parsed
					continue
				}
				// Try to repair the JSON string
				repaired := jsonutil.RepairJSON(s)
				if repaired != s { // Only if repair changed something
					if err := json.Unmarshal([]byte(repaired), &parsed); err == nil {
						result[key] = parsed
						continue
					}
				}
				// If parsing/repair failed, keep as string (validation will catch this)
			}

		case FieldTypeString, FieldTypeClass:
			// Coerce arrays to strings if allowed (JSON adapter needs this)
			if allowArrayToString {
				if arr, ok := value.([]any); ok {
					var parts []string
					for _, item := range arr {
						parts = append(parts, fmt.Sprintf("%v", item))
					}
					result[key] = strings.Join(parts, "\n")
					continue
				}
			}
		}

		result[key] = value
	}

	return result
}

// normalizeKey normalizes a field name for case-insensitive matching
// Converts to lowercase and removes spaces, underscores, and hyphens
func normalizeKey(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	k = strings.ReplaceAll(k, " ", "")
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, "-", "")
	return k
}

// NormalizeOutputKeys normalizes output field names to match signature fields
// This makes parsing resilient to casing variations (Answer vs answer) and formatting
func NormalizeOutputKeys(sig *Signature, outputs map[string]any) map[string]any {
	out := make(map[string]any, len(outputs))

	// Build normalized-to-canonical mapping
	normToCanon := map[string]string{}
	for _, f := range sig.OutputFields {
		normToCanon[normalizeKey(f.Name)] = f.Name
	}

	// Add conservative synonyms for common field names
	// Only add if the canonical field exists in signature
	if _, ok := normToCanon["answer"]; ok {
		for _, syn := range []string{"final", "finalanswer", "finalresult", "result", "response"} {
			// Only map synonym if it doesn't conflict with existing field
			if _, exists := normToCanon[syn]; !exists {
				normToCanon[syn] = "answer"
			}
		}
	}

	// Map output keys to canonical names
	for k, v := range outputs {
		normKey := normalizeKey(k)
		if canon, ok := normToCanon[normKey]; ok {
			// Use canonical name, but don't overwrite if already set
			if _, exists := out[canon]; !exists || out[canon] == nil {
				out[canon] = v
			}
		} else {
			// Keep original key if no canonical mapping found
			out[k] = v
		}
	}

	// Trim string values
	for _, f := range sig.OutputFields {
		if s, ok := out[f.Name].(string); ok {
			out[f.Name] = strings.TrimSpace(s)
		}
	}

	return out
}

// Adapter handles formatting prompts and parsing LM responses
type Adapter interface {
	// Format builds prompt messages from signature, inputs, and optional context
	Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error)

	// Parse extracts structured outputs from LM response
	Parse(sig *Signature, content string) (map[string]any, error)

	// FormatHistory formats conversation history for multi-turn interactions
	FormatHistory(history *History) []Message
}

// JSONAdapter implements Adapter using JSON format for structured I/O
type JSONAdapter struct {
	IncludeReasoning bool // Whether to request reasoning field (for CoT)
}

// NewJSONAdapter creates a new JSON adapter
func NewJSONAdapter() *JSONAdapter {
	return &JSONAdapter{
		IncludeReasoning: false,
	}
}

// WithReasoning enables reasoning field in output format
func (a *JSONAdapter) WithReasoning(include bool) *JSONAdapter {
	a.IncludeReasoning = include
	return a
}

// Format builds prompt messages from signature and inputs
func (a *JSONAdapter) Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error) {
	var prompt strings.Builder

	// Add description
	if sig.Description != "" {
		prompt.WriteString(sig.Description)
		prompt.WriteString("\n\n")
	}

	// Add CoT instruction if reasoning is enabled
	if a.IncludeReasoning {
		prompt.WriteString("Think through this step-by-step before providing your final answer.\n\n")
	}

	// Add demos if provided
	if len(demos) > 0 {
		demoMessages, err := a.formatDemos(sig, demos)
		if err != nil {
			return nil, fmt.Errorf("failed to format demos: %w", err)
		}
		if len(demoMessages) > 0 {
			// Append demo messages as examples
			prompt.WriteString("--- Examples ---\n")
			for _, msg := range demoMessages {
				prompt.WriteString(msg.Content)
				prompt.WriteString("\n")
			}
			prompt.WriteString("\n")
		}
	}

	// Add input fields
	if len(sig.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range sig.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				if !field.Optional {
					return nil, fmt.Errorf("missing required input field: %s", field.Name)
				}
				continue
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add output format specification
	if len(sig.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing:\n")

		// Add reasoning field if enabled
		if a.IncludeReasoning {
			prompt.WriteString("- reasoning (string): Your step-by-step thought process\n")
		}

		for _, field := range sig.OutputFields {
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == FieldTypeClass && len(field.Classes) > 0 {
				classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
			}
		}
		prompt.WriteString("\nIMPORTANT: Return ONLY valid JSON in your response. Do not include any markdown formatting, code blocks, or explanatory text.\n")
	}

	return []Message{{Role: "user", Content: prompt.String()}}, nil
}

// Parse extracts structured outputs from LM response
func (a *JSONAdapter) Parse(sig *Signature, content string) (map[string]any, error) {
	// Extract JSON using unified utility
	jsonStr, err := jsonutil.ExtractJSON(content)
	if err != nil {
		// VERBOSE DEBUG for parsing failures
		if debugEnv := os.Getenv("DSGO_DEBUG_PARSE"); debugEnv == "1" || debugEnv == "true" {
			fmt.Fprintf(os.Stderr, "\n=== JSON PARSE ERROR DEBUG ===\n")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "Content length: %d\n", len(content))
			fmt.Fprintf(os.Stderr, "Content preview (first 500 chars):\n%s\n", truncateString(content, 500))
			fmt.Fprintf(os.Stderr, "==============================\n\n")
		}

		// FALLBACK: If no JSON found and signature has single string field, use content as value
		if len(sig.OutputFields) == 1 && sig.OutputFields[0].Type == FieldTypeString {
			fieldName := sig.OutputFields[0].Name
			outputs := map[string]any{
				fieldName: strings.TrimSpace(content),
			}
			return outputs, nil
		}
		return nil, err
	}

	var outputs map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &outputs); err != nil {
		// Try to repair the JSON before failing
		repairedJSON := jsonutil.RepairJSON(jsonStr)
		if err := json.Unmarshal([]byte(repairedJSON), &outputs); err != nil {
			return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, jsonStr)
		}
		// Track that repair was used
		outputs["__json_repair"] = true
	}

	// Normalize field names for resilient parsing
	outputs = NormalizeOutputKeys(sig, outputs)

	// Coerce types to match signature expectations
	outputs = a.coerceTypes(sig, outputs)

	return outputs, nil
}

// coerceTypes attempts to convert output values to expected types
func (a *JSONAdapter) coerceTypes(sig *Signature, outputs map[string]any) map[string]any {
	return coerceOutputs(sig, outputs, true) // allow array→string coercion
}

// formatDemos formats few-shot examples for inclusion in prompts
func (a *JSONAdapter) formatDemos(sig *Signature, demos []Example) ([]Message, error) {
	var messages []Message

	for i, demo := range demos {
		var demoText strings.Builder
		demoText.WriteString(fmt.Sprintf("Example %d:\n", i+1))

		// Show inputs
		demoText.WriteString("Inputs:\n")
		for k, v := range demo.Inputs {
			demoText.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}

		// Show outputs
		if len(demo.Outputs) > 0 {
			demoText.WriteString("Expected Output:\n")
			outputJSON, err := json.MarshalIndent(demo.Outputs, "  ", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal demo output: %w", err)
			}
			demoText.WriteString(fmt.Sprintf("  %s\n", string(outputJSON)))
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: demoText.String(),
		})
	}

	return messages, nil
}

// FormatHistory formats conversation history for multi-turn interactions
func (a *JSONAdapter) FormatHistory(history *History) []Message {
	if history == nil || history.IsEmpty() {
		return []Message{}
	}
	return history.Get()
}

// ChatAdapter implements Adapter using field markers for structured I/O
// Uses format: [[ ## field_name ## ]] value to mark outputs
// This adapter is more robust for models that struggle with JSON
type ChatAdapter struct {
	IncludeReasoning bool // Whether to request reasoning field (for CoT)
}

// NewChatAdapter creates a new chat adapter
func NewChatAdapter() *ChatAdapter {
	return &ChatAdapter{
		IncludeReasoning: false,
	}
}

// WithReasoning enables reasoning field in output format
func (a *ChatAdapter) WithReasoning(include bool) *ChatAdapter {
	a.IncludeReasoning = include
	return a
}

// Format builds prompt messages from signature and inputs
func (a *ChatAdapter) Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error) {
	var prompt strings.Builder

	// Add description
	if sig.Description != "" {
		prompt.WriteString(sig.Description)
		prompt.WriteString("\n\n")
	}

	// Add CoT instruction if reasoning is enabled
	if a.IncludeReasoning {
		prompt.WriteString("Think through this step-by-step before providing your final answer.\n\n")
	}

	// Add demos if provided (will be added as separate messages)
	var demoMessages []Message
	if len(demos) > 0 {
		var err error
		demoMessages, err = a.formatDemos(sig, demos)
		if err != nil {
			return nil, fmt.Errorf("failed to format demos: %w", err)
		}
	}

	// Add input fields
	if len(sig.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range sig.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				if !field.Optional {
					return nil, fmt.Errorf("missing required input field: %s", field.Name)
				}
				continue
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add output format specification with field markers
	if len(sig.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond using the following format with field markers:\n\n")

		// Add reasoning field if enabled
		if a.IncludeReasoning {
			prompt.WriteString("[[ ## reasoning ## ]]\nYour step-by-step thought process\n\n")
		}

		for _, field := range sig.OutputFields {
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == FieldTypeClass && len(field.Classes) > 0 {
				classInfo = fmt.Sprintf("one of: %s", strings.Join(field.Classes, ", "))
			}
			descInfo := ""
			if field.Description != "" {
				descInfo = field.Description
			}

			// Build hint text (without redundant field name)
			var hints []string
			if classInfo != "" {
				hints = append(hints, classInfo)
			}
			if descInfo != "" {
				hints = append(hints, descInfo)
			}
			if optional != "" {
				hints = append(hints, "optional")
			}

			hintText := ""
			if len(hints) > 0 {
				hintText = " (" + strings.Join(hints, ", ") + ")"
			}

			prompt.WriteString(fmt.Sprintf("[[ ## %s ## ]]%s\n\n", field.Name, hintText))
		}
		prompt.WriteString("IMPORTANT: Use the exact field marker format shown above. Start each field with [[ ## field_name ## ]].\n")
	}

	// Combine demo messages with the main prompt
	messages := demoMessages
	messages = append(messages, Message{Role: "user", Content: prompt.String()})

	return messages, nil
}

// Parse extracts structured outputs from LM response using field markers
func (a *ChatAdapter) Parse(sig *Signature, content string) (map[string]any, error) {
	outputs := make(map[string]any)

	// Build list of fields to extract
	fieldsToExtract := make([]string, 0, len(sig.OutputFields)+1)
	if a.IncludeReasoning {
		fieldsToExtract = append(fieldsToExtract, "reasoning")
	}
	for _, field := range sig.OutputFields {
		fieldsToExtract = append(fieldsToExtract, field.Name)
	}

	// Extract each field using the marker pattern [[ ## field ## ]]
	for _, fieldName := range fieldsToExtract {
		marker := fmt.Sprintf("[[ ## %s ## ]]", fieldName)
		startIdx := strings.Index(content, marker)
		markerLen := len(marker)

		// CRITICAL FIX 1: Try variations with/without spaces
		if startIdx == -1 {
			marker = fmt.Sprintf("[[## %s ##]]", fieldName)
			startIdx = strings.Index(content, marker)
			markerLen = len(marker)
		}
		if startIdx == -1 {
			marker = fmt.Sprintf("[[##%s##]]", fieldName)
			startIdx = strings.Index(content, marker)
			markerLen = len(marker)
		}

		// CRITICAL FIX 2: Try incomplete markers (common LM error)
		// Models often emit [[ ## field ## ] or [[ ## field ## instead of [[ ## field ## ]]
		if startIdx == -1 {
			// Try marker missing closing brackets
			lenientMarker := fmt.Sprintf("[[ ## %s ##", fieldName)
			startIdx = strings.Index(content, lenientMarker)
			if startIdx >= 0 {
				markerLen = len(lenientMarker)
				// Check if content is on same line (DSPy style)
				lineEnd := strings.Index(content[startIdx:], "\n")
				if lineEnd > markerLen {
					sameLine := strings.TrimSpace(content[startIdx+markerLen : startIdx+lineEnd])
					if sameLine != "" && !strings.HasPrefix(sameLine, "]") {
						// Found inline content after incomplete marker
						outputs[fieldName] = sameLine
						continue
					}
				}
			}
		}

		// Try single closing bracket variant
		if startIdx == -1 {
			singleBracketMarker := fmt.Sprintf("[[ ## %s ## ]", fieldName)
			startIdx = strings.Index(content, singleBracketMarker)
			if startIdx >= 0 {
				markerLen = len(singleBracketMarker)
			}
		}

		if startIdx == -1 {
			// Field not found with markers - try heuristic extraction for required fields
			field := sig.GetOutputField(fieldName)
			if field != nil && !field.Optional {
				// Attempt heuristic extraction before failing
				extracted := a.heuristicExtract(content, fieldName, field.Type)
				if extracted != "" {
					outputs[fieldName] = extracted
					continue
				}
				return nil, fmt.Errorf("required field '%s' not found in response (expected marker: [[ ## %s ## ]])", fieldName, fieldName)
			}
			continue
		}

		// Move past the marker
		valueStart := startIdx + markerLen

		// Find the next marker or end of string
		valueEnd := len(content)
		for _, nextField := range fieldsToExtract {
			if nextField == fieldName {
				continue
			}
			nextMarker := fmt.Sprintf("[[ ## %s ## ]]", nextField)
			nextIdx := strings.Index(content[valueStart:], nextMarker)
			if nextIdx != -1 {
				absIdx := valueStart + nextIdx
				if absIdx < valueEnd {
					valueEnd = absIdx
				}
			}
		}

		// Extract and clean the value
		value := strings.TrimSpace(content[valueStart:valueEnd])

		// Get field info early for type-specific processing
		field := sig.GetOutputField(fieldName)

		// Clean up artifacts from incomplete markers
		value = strings.TrimPrefix(value, "]")
		value = strings.TrimSpace(value)

		// Strip field markers from value (removes markers for clean internal data flow)
		// For JSON fields, do this BEFORE parsing to clean up markers but preserve JSON syntax
		if field != nil && field.Type == FieldTypeJSON {
			// For JSON fields, use lighter stripping that preserves JSON structure
			value = stripFieldMarkersPreserveJSON(value)
		} else {
			// For other fields, use full stripping
			value = stripFieldMarkers(value)
		}

		// For class fields, extract only the first word/line to avoid explanatory text
		if field != nil && field.Type == FieldTypeClass {
			// Take only the first line or first word
			lines := strings.Split(value, "\n")
			if len(lines) > 0 {
				firstLine := strings.TrimSpace(lines[0])
				// Take first word if line contains spaces
				words := strings.Fields(firstLine)
				if len(words) > 0 {
					value = strings.ToLower(words[0]) // Normalize to lowercase for matching
				}
			}
		}

		// For float/int fields, extract only numeric values
		if field != nil && (field.Type == FieldTypeFloat || field.Type == FieldTypeInt) {
			// Extract first numeric value from the text
			value = extractNumericValue(value)
		}

		// For JSON fields, attempt to parse as JSON
		if field != nil && field.Type == FieldTypeJSON {
			// Try to parse as JSON
			var parsed any
			if err := json.Unmarshal([]byte(value), &parsed); err == nil {
				// Successfully parsed as JSON
				outputs[fieldName] = parsed
				continue
			}
			// If parsing failed, keep as string (validation will catch this later)
		}

		outputs[fieldName] = value
	}

	// Normalize field names for resilient parsing
	outputs = NormalizeOutputKeys(sig, outputs)

	// Coerce types to match signature expectations
	outputs = a.coerceTypes(sig, outputs)

	return outputs, nil
}

// heuristicExtract attempts to extract a field value using simple heuristics when markers aren't found
func (a *ChatAdapter) heuristicExtract(content string, fieldName string, fieldType FieldType) string {
	// Try common field name synonyms
	synonyms := map[string][]string{
		"answer":      {"answer", "final answer", "final_answer", "result", "output", "solution", "conclusion", "response"},
		"title":       {"title", "heading", "name"},
		"summary":     {"summary", "synopsis", "overview"},
		"explanation": {"explanation", "reasoning", "rationale"},
		"sources":     {"sources", "source", "references", "citations"},
	}

	searchTerms := []string{fieldName}
	if syns, ok := synonyms[strings.ToLower(fieldName)]; ok {
		searchTerms = append(searchTerms, syns...)
	}

	// Try to find field name followed by colon (common in free-form output)
	for _, term := range searchTerms {
		patterns := []string{
			term + ":",
			toTitle(term) + ":",
			strings.ToUpper(term) + ":",
		}

		for _, pattern := range patterns {
			idx := strings.Index(strings.ToLower(content), strings.ToLower(pattern))
			if idx != -1 {
				// Found the pattern, extract value after it
				valueStart := idx + len(pattern)
				if valueStart >= len(content) {
					continue
				}

				// Extract until newline or end
				remaining := content[valueStart:]
				lines := strings.SplitN(remaining, "\n", 2)
				value := strings.TrimSpace(lines[0])

				if value != "" {
					return value
				}
			}
		}
	}

	// ReAct-style final answer detection (only when content has ReAct markers)
	hasReActStructure := strings.Contains(strings.ToLower(content), "thought:") ||
		strings.Contains(strings.ToLower(content), "action:") ||
		strings.Contains(strings.ToLower(content), "observation:")

	if hasReActStructure && (strings.ToLower(fieldName) == "answer" || strings.ToLower(fieldName) == "result") {
		// Look for "Action: None (Final Answer)" or similar patterns
		finalMarkers := []string{
			"action: none (final answer)",
			"action: none",
			"final answer:",
		}

		for _, marker := range finalMarkers {
			idx := strings.Index(strings.ToLower(content), marker)
			if idx != -1 {
				// Extract everything after this marker
				afterMarker := content[idx+len(marker):]
				lines := strings.Split(afterMarker, "\n")

				// Collect non-empty lines after the marker, skipping ReAct scaffolding
				var extracted strings.Builder
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					// Skip empty lines and ReAct structural elements
					if trimmed == "" ||
						strings.HasPrefix(strings.ToLower(trimmed), "thought:") ||
						strings.HasPrefix(strings.ToLower(trimmed), "action:") ||
						strings.HasPrefix(strings.ToLower(trimmed), "observation:") ||
						strings.Contains(trimmed, "[[ ##") {
						continue
					}
					if extracted.Len() > 0 {
						extracted.WriteString(" ")
					}
					extracted.WriteString(trimmed)
				}

				result := strings.TrimSpace(extracted.String())
				if result != "" {
					return result
				}
			}
		}

		// Fallback: use last substantial paragraph for answer fields (only in ReAct context)
		paragraphs := strings.Split(content, "\n\n")
		for i := len(paragraphs) - 1; i >= 0; i-- {
			p := strings.TrimSpace(paragraphs[i])
			// Skip empty, markers, or ReAct scaffolding
			if p != "" &&
				!strings.Contains(p, "[[ ##") &&
				!strings.HasPrefix(strings.ToLower(p), "thought:") &&
				!strings.HasPrefix(strings.ToLower(p), "action:") &&
				len(p) > 20 {
				return p
			}
		}
	}

	// For "story" field specifically, if content is long and no other fields found,
	// assume the entire content is the story
	if strings.ToLower(fieldName) == "story" && len(content) > 100 {
		return strings.TrimSpace(content)
	}

	// For "title" field, try first non-empty line
	if strings.ToLower(fieldName) == "title" {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip empty lines and lines that look like markers
			if line != "" && !strings.Contains(line, "##") && len(line) < 200 {
				return line
			}
		}
	}

	return ""
}

// coerceTypes attempts to convert output values to expected types
func (a *ChatAdapter) coerceTypes(sig *Signature, outputs map[string]any) map[string]any {
	return coerceOutputs(sig, outputs, false) // no array→string coercion
}

// formatDemos formats few-shot examples with role alternation (user/assistant pairs)
func (a *ChatAdapter) formatDemos(sig *Signature, demos []Example) ([]Message, error) {
	var messages []Message

	for i, demo := range demos {
		// User message with inputs
		var userText strings.Builder
		userText.WriteString(fmt.Sprintf("--- Example %d (Inputs) ---\n", i+1))
		for k, v := range demo.Inputs {
			userText.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: userText.String(),
		})

		// Assistant message with outputs using field markers
		if len(demo.Outputs) > 0 {
			var assistantText strings.Builder
			for _, field := range sig.OutputFields {
				if value, exists := demo.Outputs[field.Name]; exists {
					assistantText.WriteString(fmt.Sprintf("[[ ## %s ## ]]\n%v\n\n", field.Name, value))
				}
			}

			messages = append(messages, Message{
				Role:    "assistant",
				Content: assistantText.String(),
			})
		}
	}

	return messages, nil
}

// FormatHistory formats conversation history for multi-turn interactions
func (a *ChatAdapter) FormatHistory(history *History) []Message {
	if history == nil || history.IsEmpty() {
		return []Message{}
	}
	return history.Get()
}

// FallbackAdapter tries multiple adapters in sequence until one succeeds
// This implements the critical fallback chain: ChatAdapter → JSONAdapter → Salvage
type FallbackAdapter struct {
	adapters        []Adapter
	mu              sync.RWMutex
	lastUsedAdapter int // Track which adapter succeeded (for debugging)
}

// NewFallbackAdapter creates a new fallback adapter with the default chain
// Default chain: ChatAdapter → JSONAdapter
func NewFallbackAdapter() *FallbackAdapter {
	return &FallbackAdapter{
		adapters: []Adapter{
			NewChatAdapter(),
			NewJSONAdapter(),
		},
		lastUsedAdapter: -1,
	}
}

// NewFallbackAdapterWithChain creates a fallback adapter with custom adapters
func NewFallbackAdapterWithChain(adapters ...Adapter) *FallbackAdapter {
	if len(adapters) == 0 {
		// Default to ChatAdapter → JSONAdapter
		adapters = []Adapter{
			NewChatAdapter(),
			NewJSONAdapter(),
		}
	}
	return &FallbackAdapter{
		adapters:        adapters,
		lastUsedAdapter: -1,
	}
}

// WithReasoning enables reasoning field in all adapters that support it
func (f *FallbackAdapter) WithReasoning(include bool) *FallbackAdapter {
	for _, adapter := range f.adapters {
		switch a := adapter.(type) {
		case *ChatAdapter:
			a.WithReasoning(include)
		case *JSONAdapter:
			a.WithReasoning(include)
		}
	}
	return f
}

// Format uses the first adapter in the chain for formatting
func (f *FallbackAdapter) Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error) {
	if len(f.adapters) == 0 {
		return nil, fmt.Errorf("no adapters configured")
	}
	// Always use the first adapter for formatting
	return f.adapters[0].Format(sig, inputs, demos)
}

// Parse tries each adapter in sequence until one succeeds
// Returns outputs with metadata about which adapter succeeded and how many attempts were made
func (f *FallbackAdapter) Parse(sig *Signature, content string) (map[string]any, error) {
	var parseErrors []error

	for i, adapter := range f.adapters {
		outputs, err := adapter.Parse(sig, content)
		if err == nil {
			f.mu.Lock()
			f.lastUsedAdapter = i
			f.mu.Unlock()
			// Add adapter metadata to outputs for tracking
			// This will be picked up by modules to add to Prediction
			outputs["__adapter_used"] = fmt.Sprintf("%T", adapter)
			outputs["__parse_attempts"] = i + 1
			outputs["__fallback_used"] = i > 0
			return outputs, nil
		}
		parseErrors = append(parseErrors, fmt.Errorf("adapter %d (%T): %w", i, adapter, err))
	}

	// All adapters failed - return combined error with raw content debug
	var errMsg strings.Builder
	errMsg.WriteString("all adapters failed to parse response:\n")
	for _, err := range parseErrors {
		errMsg.WriteString(fmt.Sprintf("  - %v\n", err))
	}
	errMsg.WriteString(fmt.Sprintf("\nRAW RESPONSE (length=%d):\n%s\n", len(content), content))
	return nil, fmt.Errorf("%s", errMsg.String())
}

// FormatHistory uses the first adapter in the chain
func (f *FallbackAdapter) FormatHistory(history *History) []Message {
	if len(f.adapters) == 0 {
		return []Message{}
	}
	return f.adapters[0].FormatHistory(history)
}

// GetLastUsedAdapter returns the index of the adapter that last succeeded in Parse
// Returns -1 if Parse hasn't been called or all adapters failed
func (f *FallbackAdapter) GetLastUsedAdapter() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastUsedAdapter
}

// TwoStepAdapter implements a two-stage generation approach for reasoning models
// Stage 1: Free-form generation without structured output constraints (reasoning model)
// Stage 2: Extraction model parses the free-form response into structured outputs
// This is critical for reasoning models (o1/o3/gpt-5) that struggle with structured outputs
type TwoStepAdapter struct {
	extractionLM     LM   // The LM to use for extraction (stage 2)
	IncludeReasoning bool // Whether to preserve reasoning from stage 1
}

// NewTwoStepAdapter creates a new two-step adapter
// extractionLM is used in stage 2 to parse the free-form response
// If nil, you must call Parse with the original LM-generated content
func NewTwoStepAdapter(extractionLM LM) *TwoStepAdapter {
	return &TwoStepAdapter{
		extractionLM:     extractionLM,
		IncludeReasoning: true,
	}
}

// WithReasoning controls whether to preserve reasoning from stage 1
func (a *TwoStepAdapter) WithReasoning(include bool) *TwoStepAdapter {
	a.IncludeReasoning = include
	return a
}

// Format builds prompt messages for stage 1 (free-form generation)
// This allows the reasoning model to work without structured output constraints
func (a *TwoStepAdapter) Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error) {
	var prompt strings.Builder

	// Add description
	if sig.Description != "" {
		prompt.WriteString(sig.Description)
		prompt.WriteString("\n\n")
	}

	// Add instruction for natural response
	prompt.WriteString("Please provide a thorough, natural response to the following inputs.\n")
	prompt.WriteString("Think carefully and explain your reasoning.\n\n")

	// Add demos if provided (show what good reasoning looks like)
	if len(demos) > 0 {
		prompt.WriteString("--- Examples ---\n")
		for i, demo := range demos {
			prompt.WriteString(fmt.Sprintf("\nExample %d:\n", i+1))
			prompt.WriteString("Inputs:\n")
			for k, v := range demo.Inputs {
				prompt.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
			}
			if len(demo.Outputs) > 0 {
				prompt.WriteString("Response:\n")
				for k, v := range demo.Outputs {
					prompt.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
				}
			}
		}
		prompt.WriteString("\n")
	}

	// Add input fields
	if len(sig.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range sig.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				if !field.Optional {
					return nil, fmt.Errorf("missing required input field: %s", field.Name)
				}
				continue
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add gentle guidance about expected outputs (without forcing structure)
	if len(sig.OutputFields) > 0 {
		prompt.WriteString("--- Please Address ---\n")
		for _, field := range sig.OutputFields {
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("- %s: %s\n", field.Name, field.Description))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s\n", field.Name))
			}
		}
		prompt.WriteString("\nProvide your response in a clear, natural format.\n")
	}

	return []Message{{Role: "user", Content: prompt.String()}}, nil
}

// Parse implements a two-stage extraction process
// Stage 1 output (free-form) should already be in content
// Stage 2: Use extraction LM to parse into structured format
func (a *TwoStepAdapter) Parse(sig *Signature, content string) (map[string]any, error) {
	// If no extraction LM, we can't perform stage 2
	if a.extractionLM == nil {
		return nil, fmt.Errorf("TwoStepAdapter requires an extraction LM for Parse")
	}

	// Build extraction prompt
	var extractPrompt strings.Builder
	extractPrompt.WriteString("Extract structured information from the following response.\n\n")
	extractPrompt.WriteString("--- Original Response ---\n")
	extractPrompt.WriteString(content)
	extractPrompt.WriteString("\n\n")

	// Specify extraction format (use JSON for reliable parsing)
	extractPrompt.WriteString("--- Required Output Format ---\n")
	extractPrompt.WriteString("Extract the following fields as a JSON object:\n")

	if a.IncludeReasoning {
		extractPrompt.WriteString("- reasoning (string): The reasoning or thought process from the response\n")
	}

	for _, field := range sig.OutputFields {
		optional := ""
		if field.Optional {
			optional = " (optional)"
		}
		classInfo := ""
		if field.Type == FieldTypeClass && len(field.Classes) > 0 {
			classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
		}
		if field.Description != "" {
			extractPrompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
		} else {
			extractPrompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
		}
	}

	extractPrompt.WriteString("\nIMPORTANT: Return ONLY valid JSON. Extract information accurately from the original response.\n")

	// Call extraction LM (use context.Background() for extraction call)
	extractMsg := []Message{{Role: "user", Content: extractPrompt.String()}}
	result, err := a.extractionLM.Generate(context.Background(), extractMsg, DefaultGenerateOptions())
	if err != nil {
		return nil, fmt.Errorf("extraction LM failed: %w", err)
	}

	// Parse the extraction result using JSONAdapter logic
	jsonAdapter := NewJSONAdapter()
	outputs, err := jsonAdapter.Parse(sig, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extraction result: %w", err)
	}

	return outputs, nil
}

// FormatHistory formats conversation history for multi-turn interactions
func (a *TwoStepAdapter) FormatHistory(history *History) []Message {
	if history == nil || history.IsEmpty() {
		return []Message{}
	}
	return history.Get()
}

// toTitle converts the first rune of a string to uppercase (replacement for deprecated strings.Title)
func toTitle(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// StripMarkers removes field marker delimiters from text
// Markers like [[ ## field ## ]] are for internal parsing only
// and should not appear in final outputs
// This is a public function that can be used to clean display strings
func StripMarkers(s string) string {
	return stripFieldMarkers(s)
}

// stripFieldMarkers is the internal implementation
func stripFieldMarkers(s string) string {
	// Pattern 1: [[ ## any_field_name ## ]] at start of string (with flexible spacing)
	re := regexp.MustCompile(`^\[\[\s*##\s*\w+\s*##\s*\]\]\s*`)
	s = re.ReplaceAllString(s, "")

	// Pattern 2: Also strip markers in the middle of text (streaming can break them)
	// This catches things like "text [[ ## field ## ]] more text"
	re2 := regexp.MustCompile(`\[\[\s*##\s*\w+\s*##\s*\]\]`)
	s = re2.ReplaceAllString(s, "")

	// Pattern 3: Partial markers at start (from streaming chunks)
	// e.g., "## ]]" or "]] " at the beginning
	re3 := regexp.MustCompile(`^(?:##\s*\]\]|\]\])\s*`)
	s = re3.ReplaceAllString(s, "")

	// Pattern 4: trailing ]] fragments (from incomplete markers)
	// Keep trimming until no more ]] at the end
	for strings.HasSuffix(strings.TrimSpace(s), "]]") {
		s = strings.TrimSuffix(strings.TrimSpace(s), "]]")
	}
	s = strings.TrimSpace(s)

	return s
}

// stripFieldMarkersPreserveJSON strips field markers but preserves JSON array/object syntax
// This is used for JSON fields where trailing ]] might be part of the JSON structure
func stripFieldMarkersPreserveJSON(s string) string {
	// Pattern 1: [[ ## any_field_name ## ]] at start of string (with flexible spacing)
	re := regexp.MustCompile(`^\[\[\s*##\s*\w+\s*##\s*\]\]\s*`)
	s = re.ReplaceAllString(s, "")

	// Pattern 2: Strip markers in the middle of text
	re2 := regexp.MustCompile(`\[\[\s*##\s*\w+\s*##\s*\]\]`)
	s = re2.ReplaceAllString(s, "")

	// Pattern 3: Partial markers at start (from streaming chunks)
	re3 := regexp.MustCompile(`^(?:##\s*\]\]|\]\])\s*`)
	s = re3.ReplaceAllString(s, "")

	// For JSON fields, do NOT strip trailing ]] as it might be part of the JSON
	// Just trim whitespace
	s = strings.TrimSpace(s)

	return s
}
