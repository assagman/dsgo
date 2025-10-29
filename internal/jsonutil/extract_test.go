package jsonutil

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    string
		wantErr bool
	}{
		{
			name:  "plain JSON object",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with whitespace",
			input: `  {"key": "value"}  `,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in markdown json block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in generic markdown block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with text before",
			input: `Here is the result: {"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with text after",
			input: `{"key": "value"} and some text`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "nested JSON objects",
			input: `{"outer": {"inner": "value"}}`,
			want:  `{"outer": {"inner": "value"}}`,
		},
		{
			name:  "JSON with string containing braces",
			input: `{"text": "This { is } in a string"}`,
			want:  `{"text": "This { is } in a string"}`,
		},
		{
			name:  "JSON with escaped quotes",
			input: `{"text": "She said \"hello\""}`,
			want:  `{"text": "She said \"hello\""}`,
		},
		{
			name:    "JSON with newlines (no fix) - invalid JSON",
			input:   "{\"text\": \"line1\nline2\"}",
			wantErr: true, // Invalid JSON is now correctly rejected
		},
		{
			name:  "JSON with newlines (with fix)",
			input: "{\"text\": \"line1\nline2\"}",
			opts:  []Option{WithFixNewlines()},
			want:  "{\"text\": \"line1\\nline2\"}",
		},
		{
			name:  "markdown with python code block before JSON",
			input: "```python\nprint('hello')\n```\n\n{\"result\": \"value\"}",
			want:  `{"result": "value"}`,
		},
		{
			name:  "simple brace matching (no string awareness)",
			input: `{"a": "value"} extra text`,
			opts:  []Option{WithSimpleBraceMatching()},
			want:  `{"a": "value"}`,
		},
		{
			name:  "complex nested with string awareness",
			input: `Text before {"a": {"b": "c"}, "d": "e"} text after`,
			want:  `{"a": {"b": "c"}, "d": "e"}`,
		},
		{
			name:  "markdown JSON block with extra text",
			input: "Sure! Here's the JSON:\n```json\n{\"answer\": 42}\n```\nLet me know if you need anything else!",
			want:  `{"answer": 42}`,
		},
		{
			name: "multiple JSON objects - select largest",
			input: `Thought: {"expression":"2"}
{
  "reasoning": "I used the calculator tool",
  "answer": "DSPy is a framework",
  "sources": "search tool"
}`,
			want: `{
  "reasoning": "I used the calculator tool",
  "answer": "DSPy is a framework",
  "sources": "search tool"
}`,
		},
		{
			name:  "multiple JSON objects on same line",
			input: `{"a": 1} {"b": 2, "c": 3}`,
			want:  `{"b": 2, "c": 3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractJSON(tt.input, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []Option
		wantKey string
		wantVal interface{}
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `{"answer": "hello"}`,
			wantKey: "answer",
			wantVal: "hello",
		},
		{
			name:    "JSON in markdown",
			input:   "```json\n{\"score\": 42}\n```",
			wantKey: "score",
			wantVal: float64(42), // JSON numbers are float64
		},
		{
			name:    "JSON with text around",
			input:   "Result: {\"valid\": true} - done",
			wantKey: "valid",
			wantVal: true,
		},
		{
			name:    "invalid JSON",
			input:   "{not valid json}",
			wantErr: true,
		},
		{
			name:    "JSON with fixed newlines",
			input:   "{\"text\": \"line1\nline2\"}",
			opts:    []Option{WithFixNewlines()},
			wantKey: "text",
			wantVal: "line1\nline2", // After parsing, escaped \n becomes actual newline
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJSON(tt.input, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if val, ok := got[tt.wantKey]; !ok {
					t.Errorf("ParseJSON() missing key %v", tt.wantKey)
				} else if val != tt.wantVal {
					t.Errorf("ParseJSON()[%v] = %v (%T), want %v (%T)", tt.wantKey, val, val, tt.wantVal, tt.wantVal)
				}
			}
		})
	}
}

func TestFixJSONNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no newlines",
			input: `{"text": "hello"}`,
			want:  `{"text": "hello"}`,
		},
		{
			name:  "newline in string",
			input: "{\"text\": \"line1\nline2\"}",
			want:  "{\"text\": \"line1\\nline2\"}",
		},
		{
			name:  "carriage return in string",
			input: "{\"text\": \"line1\rline2\"}",
			want:  "{\"text\": \"line1line2\"}",
		},
		{
			name:  "already escaped newline",
			input: `{"text": "line1\nline2"}`,
			want:  `{"text": "line1\nline2"}`,
		},
		{
			name:  "newline outside string (preserved)",
			input: "{\n\"text\": \"value\"\n}",
			want:  "{\n\"text\": \"value\"\n}",
		},
		{
			name:  "multiple newlines in string",
			input: "{\"text\": \"line1\nline2\nline3\"}",
			want:  "{\"text\": \"line1\\nline2\\nline3\"}",
		},
		{
			name:  "escaped quote with newline",
			input: "{\"text\": \"she said \\\"hi\\\"\nbye\"}",
			want:  "{\"text\": \"she said \\\"hi\\\"\\nbye\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixJSONNewlines(tt.input)
			if got != tt.want {
				t.Errorf("fixJSONNewlines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkExtractJSON(b *testing.B) {
	inputs := []string{
		`{"simple": "json"}`,
		"```json\n{\"markdown\": \"block\"}\n```",
		`Text before {"nested": {"deep": "value"}} text after`,
		strings.Repeat(`{"large": "`, 100) + "value" + strings.Repeat(`"}`, 100),
	}

	for i, input := range inputs {
		b.Run(string(rune('A'+i)), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, _ = ExtractJSON(input)
			}
		})
	}
}
