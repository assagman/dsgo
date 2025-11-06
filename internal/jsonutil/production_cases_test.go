package jsonutil

import (
	"testing"
)

// TestExtractJSON_ProductionFailures tests real-world failure cases from production logs
func TestExtractJSON_ProductionFailures(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantJSON bool // true if we expect JSON to be extracted
		wantErr  bool
	}{
		{
			name: "thinking prefix with XML-style tags",
			input: `<think>Let me analyze this problem... The answer is 42.</think>
{"answer": "42", "reasoning": "calculated"}`,
			wantJSON: true,
		},
		{
			name: "reasoning prefix plain text",
			input: `Let me think about this carefully.

First, I need to understand the question.
Then I can provide the answer.

{"answer": "DSPy is a framework", "confidence": "high"}`,
			wantJSON: true,
		},
		{
			name:     "JSON in triple-backtick code block",
			input:    "The result is:\n```\n{\"score\": 95, \"grade\": \"A\"}\n```\nHope this helps!",
			wantJSON: true,
		},
		{
			name: "JSON with explanation after",
			input: `{"result": "success", "value": 123}

This result was calculated using the formula X + Y where X=100 and Y=23.`,
			wantJSON: true,
		},
		{
			name: "multiple JSON objects - should pick largest",
			input: `Step 1: {"step": 1}
Step 2: {"step": 2, "action": "calculate", "result": 42}
Done!`,
			wantJSON: true,
		},
		{
			name: "JSON with Unicode thinking characters",
			input: `🤔 Thinking... 
💭 The answer is:
{"answer": "correct", "emoji": "✓"}`,
			wantJSON: true,
		},
		{
			name: "malformed thinking with partial JSON",
			input: `<think>
Let me calculate... the result is
{"partial": "json"
</think>
{"complete": "json", "fixed": true}`,
			wantJSON: true,
		},
		{
			name:    "completely invalid - no JSON",
			input:   "This is just plain text with no JSON at all.",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSON(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error, got JSON: %s", got)
				}
				return
			}

			if !tc.wantJSON {
				if err == nil {
					t.Errorf("Expected no JSON extraction, but got: %s", got)
				}
				return
			}

			// Should extract valid JSON
			if err != nil {
				t.Errorf("Failed to extract JSON: %v\nInput: %s", err, tc.input)
				return
			}

			// Verify it's actually valid JSON by parsing
			_, parseErr := ParseJSON(got)
			if parseErr != nil {
				t.Errorf("Extracted invalid JSON: %v\nExtracted: %s\nOriginal: %s", parseErr, got, tc.input)
			}
		})
	}
}

// TestExtractJSON_ThinkingTokenVariations tests various thinking token formats
func TestExtractJSON_ThinkingTokenVariations(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "XML think tags",
			input: `<think>reasoning here</think>{"result": "value"}`,
		},
		{
			name:  "nested think tags",
			input: `<think>outer<think>inner</think>outer</think>{"result": "value"}`,
		},
		{
			name:  "thought prefix",
			input: `Thought: Let me process this\nObservation: data is good\n{"result": "value"}`,
		},
		{
			name:  "reasoning block",
			input: "Reasoning:\n- Point 1\n- Point 2\n\nConclusion:\n{\"result\": \"value\"}",
		},
		{
			name:  "chain of thought format",
			input: "Step 1: analyze\nStep 2: compute\nFinal answer: {\"result\": \"value\"}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSON(tc.input)
			if err != nil {
				t.Errorf("Failed to extract JSON from thinking tokens: %v", err)
				return
			}

			// Should find the JSON object
			_, parseErr := ParseJSON(got)
			if parseErr != nil {
				t.Errorf("Extracted invalid JSON: %v\nExtracted: %s", parseErr, got)
			}
		})
	}
}

// TestExtractJSON_MarkdownVariations tests markdown code block extraction
func TestExtractJSON_MarkdownVariations(t *testing.T) {
	testCases := []struct {
		name       string
		input      string
		wantFields map[string]interface{}
	}{
		{
			name:       "json language tag",
			input:      "```json\n{\"key\": \"value\"}\n```",
			wantFields: map[string]interface{}{"key": "value"},
		},
		{
			name:       "no language tag",
			input:      "```\n{\"key\": \"value\"}\n```",
			wantFields: map[string]interface{}{"key": "value"},
		},
		{
			name:       "multiple code blocks - skip python, get JSON",
			input:      "```python\nprint('hello')\n```\n\nResult:\n```json\n{\"key\": \"value\"}\n```",
			wantFields: map[string]interface{}{"key": "value"},
		},
		{
			name:       "JSON after non-JSON code block",
			input:      "```bash\nls -la\n```\n\n{\"key\": \"value\"}",
			wantFields: map[string]interface{}{"key": "value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseJSON(tc.input)
			if err != nil {
				t.Errorf("Failed to parse JSON: %v", err)
				return
			}

			for k, v := range tc.wantFields {
				if parsed[k] != v {
					t.Errorf("Field %s = %v, want %v", k, parsed[k], v)
				}
			}
		})
	}
}

// TestRepairJSON_ProductionCases tests repair on real malformed JSON
func TestRepairJSON_ProductionCases(t *testing.T) {
	testCases := []struct {
		name       string
		input      string
		wantFields map[string]interface{}
	}{
		{
			name:       "single quotes with thinking prefix",
			input:      "Thought: analyzing\n{'answer': 'correct'}",
			wantFields: map[string]interface{}{"answer": "correct"},
		},
		{
			name:       "unquoted keys in code block",
			input:      "```\n{result: 'success', code: 200}\n```",
			wantFields: map[string]interface{}{"result": "success", "code": float64(200)},
		},
		{
			name:       "trailing comma with explanation",
			input:      `{"valid": true,} // this is the result`,
			wantFields: map[string]interface{}{"valid": true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repaired := RepairJSON(tc.input)

			parsed, err := ParseJSON(repaired)
			if err != nil {
				t.Errorf("Repaired JSON is invalid: %v\nRepaired: %s\nOriginal: %s", err, repaired, tc.input)
				return
			}

			for k, v := range tc.wantFields {
				if parsed[k] != v {
					t.Errorf("Field %s = %v (%T), want %v (%T)", k, parsed[k], parsed[k], v, v)
				}
			}
		})
	}
}
