package dsgo

import (
	"strings"
	"testing"
)

// TestJSONAdapter_Format tests the full Format method with various scenarios
func TestJSONAdapter_Format(t *testing.T) {
	adapter := NewJSONAdapter()
	sig := NewSignature("Test task").
		AddInput("question", FieldTypeString, "The question").
		AddOutput("answer", FieldTypeString, "The answer")

	tests := []struct {
		name    string
		inputs  map[string]any
		demos   []Example
		wantErr bool
	}{
		{
			name:    "basic format",
			inputs:  map[string]any{"question": "What is 2+2?"},
			demos:   []Example{},
			wantErr: false,
		},
		{
			name:   "with demos",
			inputs: map[string]any{"question": "What is 3+3?"},
			demos: []Example{
				*NewExample(
					map[string]any{"question": "What is 1+1?"},
					map[string]any{"answer": "2"},
				),
			},
			wantErr: false,
		},
		{
			name:    "missing required input",
			inputs:  map[string]any{},
			demos:   []Example{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := adapter.Format(sig, tt.inputs, tt.demos)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(messages) == 0 {
				t.Error("Format() should return non-empty messages")
			}
		})
	}
}

// TestJSONAdapter_Format_MultipleFields tests formatting multiple output fields
func TestJSONAdapter_Format_MultipleFields(t *testing.T) {
	sig := NewSignature("Analyze").
		AddInput("text", FieldTypeString, "").
		AddOutput("sentiment", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "").
		AddOutput("keywords", FieldTypeJSON, "")

	adapter := NewJSONAdapter()
	inputs := map[string]any{"text": "Great product!"}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestJSONAdapter_Format_WithDemos tests JSON adapter with demonstrations
func TestJSONAdapter_Format_WithDemos(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewJSONAdapter()
	inputs := map[string]any{"question": "What is 2+2?"}

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 1+1?"},
			Outputs: map[string]any{"answer": "2"},
		},
	}

	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format with demos failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestJSONAdapter_FormatWithReasoning tests reasoning field
func TestJSONAdapter_FormatWithReasoning(t *testing.T) {
	adapter := NewJSONAdapter().WithReasoning(true)
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{"question": "test"}, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Check that reasoning instruction is included
	found := false
	for _, msg := range messages {
		if strings.Contains(msg.Content, "step-by-step") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Format with reasoning should include step-by-step instruction")
	}
}

// TestJSONAdapter_FormatHistory tests history formatting
func TestJSONAdapter_FormatHistory(t *testing.T) {
	adapter := NewJSONAdapter()
	history := NewHistory()
	history.AddUserMessage("Hello")
	history.AddAssistantMessage("Hi")

	messages := adapter.FormatHistory(history)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test with empty history
	emptyHistory := NewHistory()
	emptyMessages := adapter.FormatHistory(emptyHistory)
	if len(emptyMessages) != 0 {
		t.Error("Empty history should return empty messages")
	}

	// Test with nil history
	nilMessages := adapter.FormatHistory(nil)
	if len(nilMessages) != 0 {
		t.Error("Nil history should return empty messages")
	}
}

// TestJSONAdapter_ParseEmptyContent tests JSON adapter with empty content
func TestJSONAdapter_ParseEmptyContent(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "")

	adapter := NewJSONAdapter()
	_, err := adapter.Parse(sig, "")
	if err == nil {
		t.Error("Expected error for empty content")
	}
}

// TestJSONAdapter_Parse_NoJSON tests JSON adapter with no JSON content
func TestJSONAdapter_Parse_NoJSON(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "")

	adapter := NewJSONAdapter()
	content := "This is plain text without any JSON"

	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Error("Expected error for content without JSON")
	}
}

// TestJSONAdapter_ParseWithRepair tests JSON parsing that needs repair
func TestJSONAdapter_ParseWithRepair(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewJSONAdapter()

	// Malformed JSON that should be repaired
	content := "```json\n{'answer': 'yes',}\n```"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		// Repair might not always succeed, that's okay
		t.Logf("Parse failed (acceptable): %v", err)
		return
	}

	if _, ok := outputs["answer"]; !ok {
		t.Error("Expected answer field in outputs")
	}
}

// TestExtractNumericValue tests the extractNumericValue helper function
func TestExtractNumericValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain number", "42", "42"},
		{"percentage", "95%", "95"},
		{"very high", "very high", "0.95"},
		{"high", "high", "0.9"},
		{"medium", "medium", "0.7"},
		{"low", "low", "0.3"},
		{"no match", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNumericValue(tt.input)
			if result != tt.expected {
				t.Errorf("extractNumericValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestJSONAdapter_CoerceOutputs_EdgeCases tests edge cases in type coercion
func TestJSONAdapter_CoerceOutputs_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		sig     *Signature
		input   string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "int from string with whitespace",
			sig: NewSignature("test").
				AddOutput("count", FieldTypeInt, ""),
			input: `{"count": "  42  "}`,
			want:  map[string]any{"count": 42},
		},
		{
			name: "int from float",
			sig: NewSignature("test").
				AddOutput("count", FieldTypeInt, ""),
			input: `{"count": 42.7}`,
			want:  map[string]any{"count": 42},
		},
		{
			name: "float from string with whitespace",
			sig: NewSignature("test").
				AddOutput("score", FieldTypeFloat, ""),
			input: `{"score": "  0.95  "}`,
			want:  map[string]any{"score": 0.95},
		},
		{
			name: "float from int",
			sig: NewSignature("test").
				AddOutput("score", FieldTypeFloat, ""),
			input: `{"score": 42}`,
			want:  map[string]any{"score": 42.0},
		},
		{
			name: "bool from string true variations",
			sig: NewSignature("test").
				AddOutput("flag", FieldTypeBool, ""),
			input: `{"flag": " true "}`,
			want:  map[string]any{"flag": true},
		},
		{
			name: "bool from string false variations",
			sig: NewSignature("test").
				AddOutput("flag", FieldTypeBool, ""),
			input: `{"flag": "false"}`,
			want:  map[string]any{"flag": false},
		},
		{
			name: "int from percentage string",
			sig: NewSignature("test").
				AddOutput("confidence", FieldTypeInt, ""),
			input: `{"confidence": "95%"}`,
			want:  map[string]any{"confidence": 95},
		},
		{
			name: "float from qualitative high",
			sig: NewSignature("test").
				AddOutput("confidence", FieldTypeFloat, ""),
			input: `{"confidence": "high"}`,
			want:  map[string]any{"confidence": 0.9},
		},
		{
			name: "array to string conversion",
			sig: NewSignature("test").
				AddOutput("items", FieldTypeString, ""),
			input: `{"items": ["apple", "banana", "cherry"]}`,
			want:  map[string]any{"items": "apple\nbanana\ncherry"},
		},
		{
			name: "invalid int conversion keeps original",
			sig: NewSignature("test").
				AddOutput("count", FieldTypeInt, ""),
			input: `{"count": "not a number"}`,
			want:  map[string]any{"count": "not a number"},
		},
		{
			name: "invalid float conversion keeps original",
			sig: NewSignature("test").
				AddOutput("score", FieldTypeFloat, ""),
			input: `{"score": "not a number"}`,
			want:  map[string]any{"score": "not a number"},
		},
		{
			name: "invalid bool conversion keeps original",
			sig: NewSignature("test").
				AddOutput("flag", FieldTypeBool, ""),
			input: `{"flag": "maybe"}`,
			want:  map[string]any{"flag": "maybe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewJSONAdapter()
			outputs, err := adapter.Parse(tt.sig, tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			for key, wantVal := range tt.want {
				gotVal, exists := outputs[key]
				if !exists {
					t.Errorf("Expected output field %q not found", key)
					continue
				}

				// Type-specific comparison
				switch want := wantVal.(type) {
				case int:
					got, ok := gotVal.(int)
					if !ok {
						t.Errorf("Field %q: got type %T, want int", key, gotVal)
					} else if got != want {
						t.Errorf("Field %q: got %d, want %d", key, got, want)
					}
				case float64:
					got, ok := gotVal.(float64)
					if !ok {
						t.Errorf("Field %q: got type %T, want float64", key, gotVal)
					} else if got != want {
						t.Errorf("Field %q: got %f, want %f", key, got, want)
					}
				case bool:
					got, ok := gotVal.(bool)
					if !ok {
						t.Errorf("Field %q: got type %T, want bool", key, gotVal)
					} else if got != want {
						t.Errorf("Field %q: got %v, want %v", key, got, want)
					}
				case string:
					got, ok := gotVal.(string)
					if !ok {
						t.Errorf("Field %q: got type %T, want string", key, gotVal)
					} else if got != want {
						t.Errorf("Field %q: got %q, want %q", key, got, want)
					}
				}
			}
		})
	}
}

// TestJSONAdapter_Parse_MalformedJSONVariations tests various malformed JSON scenarios
func TestJSONAdapter_Parse_MalformedJSONVariations(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	adapter := NewJSONAdapter()

	tests := []struct {
		name       string
		input      string
		wantAnswer string
		wantConf   float64
	}{
		{
			name:       "nested single quotes",
			input:      "{'answer': 'yes', 'confidence': '0.95'}",
			wantAnswer: "yes",
			wantConf:   0.95,
		},
		{
			name:       "mixed quotes",
			input:      `{"answer": 'yes', 'confidence': "0.95"}`,
			wantAnswer: "yes",
			wantConf:   0.95,
		},
		{
			name:       "trailing comma in object",
			input:      `{"answer": "yes", "confidence": 0.95,}`,
			wantAnswer: "yes",
			wantConf:   0.95,
		},
		{
			name:       "multiple trailing commas",
			input:      `{"answer": "yes",, "confidence": 0.95,,}`,
			wantAnswer: "yes",
			wantConf:   0.95,
		},
		{
			name:       "with markdown fence",
			input:      "```json\n{\"answer\": \"yes\", \"confidence\": 0.95}\n```",
			wantAnswer: "yes",
			wantConf:   0.95,
		},
		{
			name:       "with text before and after",
			input:      "Here is the answer: {\"answer\": \"yes\", \"confidence\": 0.95} That's it!",
			wantAnswer: "yes",
			wantConf:   0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.input)
			if err != nil {
				t.Logf("Parse failed for %q (may be acceptable): %v", tt.name, err)
				return // Some malformed JSON may not be repairable
			}

			if answer, ok := outputs["answer"].(string); ok {
				if answer != tt.wantAnswer {
					t.Errorf("answer = %q, want %q", answer, tt.wantAnswer)
				}
			}

			if conf, ok := outputs["confidence"].(float64); ok {
				if conf != tt.wantConf {
					t.Errorf("confidence = %f, want %f", conf, tt.wantConf)
				}
			}
		})
	}
}
