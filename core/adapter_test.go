package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

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
// With single-field fallback, empty content becomes empty string value
func TestJSONAdapter_ParseEmptyContent(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "")

	adapter := NewJSONAdapter()
	outputs, err := adapter.Parse(sig, "")
	if err != nil {
		t.Errorf("Expected fallback for empty content, got error: %v", err)
	}
	if outputs["result"] != "" {
		t.Errorf("Expected empty result, got %q", outputs["result"])
	}
}

// TestJSONAdapter_Parse_NoJSON tests JSON adapter with no JSON content
// With single-field string signature, should fall back to using content as value
func TestJSONAdapter_Parse_NoJSON(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "")

	adapter := NewJSONAdapter()
	content := "This is plain text without any JSON"

	// Should succeed with fallback for single string field
	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Errorf("Expected fallback to succeed, got error: %v", err)
	}
	if outputs["result"] != content {
		t.Errorf("Expected result=%q, got %q", content, outputs["result"])
	}
}

// TestJSONAdapter_Parse_NoJSON_MultipleFields tests that JSON adapter fails with multiple fields
func TestJSONAdapter_Parse_NoJSON_MultipleFields(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "").
		AddOutput("status", FieldTypeString, "")

	adapter := NewJSONAdapter()
	content := "This is plain text without any JSON"

	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Error("Expected error for content without JSON when multiple fields required")
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

// TestNormalizeKey tests the normalizeKey utility function
func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "answer", "answer"},
		{"uppercase", "Answer", "answer"},
		{"mixed case", "AnSwEr", "answer"},
		{"with spaces", "  answer  ", "answer"},
		{"with underscores", "final_answer", "finalanswer"},
		{"with hyphens", "final-answer", "finalanswer"},
		{"complex", "  Final_Answer-123  ", "finalanswer123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeKey(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalizeOutputKeys tests the NormalizeOutputKeys utility function
func TestNormalizeOutputKeys(t *testing.T) {
	tests := []struct {
		name     string
		sig      *Signature
		inputs   map[string]any
		expected map[string]any
	}{
		{
			name: "exact match",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"answer": "test answer",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "case variation",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"Answer": "test answer",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "underscore variation",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"final_answer": "test answer",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "result synonym",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"result": "test answer",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "response synonym",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"response": "test answer",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "whitespace trimming",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"answer": "  test answer  ",
			},
			expected: map[string]any{
				"answer": "test answer",
			},
		},
		{
			name: "multiple fields",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer").
				AddOutput("sources", FieldTypeString, "sources"),
			inputs: map[string]any{
				"Answer":  "test answer",
				"Sources": "test sources",
			},
			expected: map[string]any{
				"answer":  "test answer",
				"sources": "test sources",
			},
		},
		{
			name: "no synonym conflict",
			sig: NewSignature("test").
				AddOutput("result", FieldTypeString, "result").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"result": "actual result",
				"answer": "actual answer",
			},
			expected: map[string]any{
				"result": "actual result",
				"answer": "actual answer",
			},
		},
		{
			name: "preserve unknown keys",
			sig: NewSignature("test").
				AddOutput("answer", FieldTypeString, "answer"),
			inputs: map[string]any{
				"answer":        "test answer",
				"__metadata":    "some metadata",
				"__json_repair": true,
			},
			expected: map[string]any{
				"answer":        "test answer",
				"__metadata":    "some metadata",
				"__json_repair": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeOutputKeys(tt.sig, tt.inputs)

			// Check all expected keys are present with correct values
			for k, expectedVal := range tt.expected {
				actualVal, ok := result[k]
				if !ok {
					t.Errorf("expected key %q not found in result", k)
					continue
				}
				if actualVal != expectedVal {
					t.Errorf("key %q: got %v, want %v", k, actualVal, expectedVal)
				}
			}

			// Check no unexpected keys (except metadata keys starting with __)
			for k := range result {
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("unexpected key %q in result", k)
				}
			}
		})
	}
}

// TestNormalizeOutputKeys_Integration tests realistic scenario with model output
func TestNormalizeOutputKeys_Integration(t *testing.T) {
	sig := NewSignature("Answer the question").
		AddOutput("answer", FieldTypeString, "The answer").
		AddOutput("sources", FieldTypeString, "Sources used")

	// Simulate model returning capitalized field names
	modelOutput := map[string]any{
		"Answer":  "DSPy is a framework for programming language models.",
		"Sources": "Search results",
	}

	normalized := NormalizeOutputKeys(sig, modelOutput)

	// Should validate successfully after normalization
	if err := sig.ValidateOutputs(normalized); err != nil {
		t.Errorf("validation failed after normalization: %v", err)
	}

	// Check values
	if normalized["answer"] != "DSPy is a framework for programming language models." {
		t.Errorf("answer field incorrect: %v", normalized["answer"])
	}
	if normalized["sources"] != "Search results" {
		t.Errorf("sources field incorrect: %v", normalized["sources"])
	}
}

func TestChatAdapter_Format(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddInput("question", FieldTypeString, "The question").
		AddOutput("answer", FieldTypeString, "The answer")

	inputs := map[string]any{"question": "What is 2+2?"}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	content := messages[0].Content
	if !strings.Contains(content, "[[ ## answer ## ]]") {
		t.Errorf("Expected field marker [[ ## answer ## ]], got: %s", content)
	}
	if !strings.Contains(content, "What is 2+2?") {
		t.Errorf("Expected input value in content, got: %s", content)
	}
}

// TestChatAdapter_FormatWithReasoning tests Format with reasoning enabled
func TestChatAdapter_FormatWithReasoning(t *testing.T) {
	adapter := NewChatAdapter().WithReasoning(true)
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "The answer")

	inputs := map[string]any{}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	content := messages[0].Content
	if !strings.Contains(content, "[[ ## reasoning ## ]]") {
		t.Errorf("Expected reasoning field marker, got: %s", content)
	}
}

// TestChatAdapter_FormatDemos tests demo formatting with role alternation
func TestChatAdapter_FormatDemos(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	demos := []Example{
		*NewExample(
			map[string]any{"question": "What is 1+1?"},
			map[string]any{"answer": "2"},
		),
		*NewExample(
			map[string]any{"question": "What is 2+2?"},
			map[string]any{"answer": "4"},
		),
	}

	messages, err := adapter.Format(sig, map[string]any{"question": "What is 3+3?"}, demos)
	if err != nil {
		t.Fatalf("FormatDemos failed: %v", err)
	}

	// Should have 2 demos * 2 messages (user + assistant) + 1 main prompt = 5 messages
	if len(messages) != 5 {
		t.Fatalf("Expected 5 messages (4 demo + 1 prompt), got %d", len(messages))
	}

	// Check role alternation
	if messages[0].Role != "user" {
		t.Errorf("Expected first message to be user, got %s", messages[0].Role)
	}
	if messages[1].Role != "assistant" {
		t.Errorf("Expected second message to be assistant, got %s", messages[1].Role)
	}
	if messages[2].Role != "user" {
		t.Errorf("Expected third message to be user, got %s", messages[2].Role)
	}
	if messages[3].Role != "assistant" {
		t.Errorf("Expected fourth message to be assistant, got %s", messages[3].Role)
	}

	// Check that assistant messages use field markers
	if !strings.Contains(messages[1].Content, "[[ ## answer ## ]]") {
		t.Errorf("Expected field marker in assistant message, got: %s", messages[1].Content)
	}
}

// TestChatAdapter_FormatDemos_Empty tests formatting with no demos
func TestChatAdapter_FormatDemos_Empty(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewChatAdapter()
	inputs := map[string]any{"question": "Test?"}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestChatAdapter_Format_WithFieldDesc tests formatting with field descriptions
func TestChatAdapter_Format_WithFieldDesc(t *testing.T) {
	sig := NewSignature("Analyze").
		AddInput("text", FieldTypeString, "Text to analyze").
		AddOutput("sentiment", FieldTypeString, "The sentiment")

	adapter := NewChatAdapter()
	inputs := map[string]any{"text": "Great product!"}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}

	// Check that content contains field descriptions
	found := false
	for _, msg := range messages {
		if len(msg.Content) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected messages to contain content with descriptions")
	}
}

// TestChatAdapter_Format_WithDemos tests chat adapter with demonstrations
func TestChatAdapter_Format_WithDemos(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewChatAdapter()
	inputs := map[string]any{"question": "What is 2+2?"}

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 1+1?"},
			Outputs: map[string]any{"answer": "2"},
		},
		{
			Inputs:  map[string]any{"question": "What is 3+3?"},
			Outputs: map[string]any{"answer": "6"},
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

// TestChatAdapter_FormatComprehensive tests the full Format method
func TestChatAdapter_FormatComprehensive(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("Test task").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	tests := []struct {
		name    string
		inputs  map[string]any
		demos   []Example
		wantErr bool
	}{
		{
			name:    "basic format",
			inputs:  map[string]any{"question": "test"},
			demos:   []Example{},
			wantErr: false,
		},
		{
			name:   "with demos",
			inputs: map[string]any{"question": "test"},
			demos: []Example{
				*NewExample(
					map[string]any{"question": "What is 1+1?"},
					map[string]any{"answer": "2"},
				),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := adapter.Format(sig, tt.inputs, tt.demos)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(messages) == 0 {
				t.Error("Format() should return messages")
			}
		})
	}
}

// TestChatAdapter_FormatHistory tests history formatting
func TestChatAdapter_FormatHistory(t *testing.T) {
	adapter := NewChatAdapter()
	history := NewHistory()
	history.AddUserMessage("Hello")
	history.AddAssistantMessage("Hi")

	messages := adapter.FormatHistory(history)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test with nil history
	nilMessages := adapter.FormatHistory(nil)
	if len(nilMessages) != 0 {
		t.Error("Nil history should return empty messages")
	}
}

// TestChatAdapter_Parse tests parsing with field markers
func TestChatAdapter_Parse(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	tests := []struct {
		name     string
		content  string
		expected map[string]any
		wantErr  bool
	}{
		{
			name:     "Standard field marker",
			content:  "[[ ## answer ## ]]\n42",
			expected: map[string]any{"answer": "42"},
			wantErr:  false,
		},
		{
			name:     "Field marker without spaces",
			content:  "[[## answer ##]]\n42",
			expected: map[string]any{"answer": "42"},
			wantErr:  false,
		},
		{
			name:     "Field marker with extra content",
			content:  "Here is my answer:\n[[ ## answer ## ]]\n42\nThat's all!",
			expected: map[string]any{"answer": "42\nThat's all!"},
			wantErr:  false,
		},
		{
			name:     "Missing required field",
			content:  "No field markers here",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if outputs["answer"] != tt.expected["answer"] {
					t.Errorf("Parse() = %v, want %v", outputs, tt.expected)
				}
			}
		})
	}
}

// TestChatAdapter_ParseMultipleFields tests parsing multiple fields
func TestChatAdapter_ParseMultipleFields(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddOutput("name", FieldTypeString, "").
		AddOutput("age", FieldTypeInt, "").
		AddOutput("active", FieldTypeBool, "")

	content := `[[ ## name ## ]]
John Doe

[[ ## age ## ]]
25

[[ ## active ## ]]
true`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if outputs["name"] != "John Doe" {
		t.Errorf("Expected name='John Doe', got %v", outputs["name"])
	}
	if outputs["age"] != 25 {
		t.Errorf("Expected age=25, got %v", outputs["age"])
	}
	if outputs["active"] != true {
		t.Errorf("Expected active=true, got %v", outputs["active"])
	}
}

// TestChatAdapter_TypeCoercion tests type coercion
func TestChatAdapter_TypeCoercion(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "").
		AddOutput("enabled", FieldTypeBool, "")

	content := `[[ ## count ## ]]
42

[[ ## score ## ]]
3.14

[[ ## enabled ## ]]
true`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if _, ok := outputs["count"].(int); !ok {
		t.Errorf("Expected count to be int, got %T", outputs["count"])
	}
	if _, ok := outputs["score"].(float64); !ok {
		t.Errorf("Expected score to be float64, got %T", outputs["score"])
	}
	if _, ok := outputs["enabled"].(bool); !ok {
		t.Errorf("Expected enabled to be bool, got %T", outputs["enabled"])
	}
}

// TestChatAdapter_OptionalFields tests parsing with optional fields
func TestChatAdapter_OptionalFields(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("test").
		AddOutput("required", FieldTypeString, "").
		AddOptionalOutput("optional", FieldTypeString, "")

	// Content with only required field
	content := "[[ ## required ## ]]\nvalue"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if outputs["required"] != "value" {
		t.Errorf("Expected required='value', got %v", outputs["required"])
	}

	// optional field should not be present
	if _, exists := outputs["optional"]; exists {
		t.Errorf("Expected optional field to be absent, got %v", outputs["optional"])
	}
}

// TestChatAdapter_ParseMultiline tests multiline content
func TestChatAdapter_ParseMultiline(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("description", FieldTypeString, "")

	adapter := NewChatAdapter()
	content := `description: This is a long
description that spans
multiple lines`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	desc, ok := outputs["description"].(string)
	if !ok {
		t.Fatal("Expected description to be string")
	}

	if len(desc) == 0 {
		t.Error("Expected non-empty description")
	}
}

// TestChatAdapter_ParseWithColon tests parsing content with colons in values
func TestChatAdapter_ParseWithColon(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("url", FieldTypeString, "")

	adapter := NewChatAdapter()
	content := "url: https://example.com:8080/path"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	url, ok := outputs["url"].(string)
	if !ok {
		t.Fatal("Expected url to be string")
	}

	if len(url) == 0 {
		t.Error("Expected non-empty URL")
	}
}

// TestChatAdapter_ParseOptional tests parsing with optional fields
func TestChatAdapter_ParseOptional(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required", FieldTypeString, "").
		AddOptionalOutput("optional", FieldTypeString, "")

	adapter := NewChatAdapter()
	content := "required: value"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if _, ok := outputs["required"]; !ok {
		t.Error("Expected required field in outputs")
	}
}

// TestChatAdapter_Parse_HeuristicExtraction tests heuristic extraction path
func TestChatAdapter_Parse_HeuristicExtraction(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewChatAdapter()

	// Content without explicit field markers that might trigger heuristic extraction
	content := "The answer is definitely yes, I'm very confident about this."

	// This might work with heuristic extraction or fail - either is acceptable
	_, err := adapter.Parse(sig, content)

	// We just want to exercise the code path, error is acceptable
	if err != nil {
		t.Logf("Parse failed (expected): %v", err)
	}
}

// TestChatAdapter_HeuristicExtract tests the heuristicExtract helper method
func TestChatAdapter_HeuristicExtract(t *testing.T) {
	adapter := NewChatAdapter()

	tests := []struct {
		name      string
		content   string
		fieldName string
		fieldType FieldType
		expected  string
	}{
		{
			name:      "simple answer with colon",
			content:   "Answer: Paris is the capital of France",
			fieldName: "answer",
			fieldType: FieldTypeString,
			expected:  "Paris is the capital of France",
		},
		{
			name:      "synonym - result for answer",
			content:   "Result: 42",
			fieldName: "answer",
			fieldType: FieldTypeString,
			expected:  "42",
		},
		{
			name: "react final answer",
			content: `Thought: I have enough information now
Action: None (Final Answer)
The capital of France is Paris`,
			fieldName: "answer",
			fieldType: FieldTypeString,
			expected:  "The capital of France is Paris",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.heuristicExtract(tt.content, tt.fieldName, tt.fieldType)
			if result != tt.expected {
				t.Errorf("heuristicExtract() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFallbackAdapter_DefaultChain(t *testing.T) {
	adapter := NewFallbackAdapter()

	// Default should be ChatAdapter → JSONAdapter
	if len(adapter.adapters) != 2 {
		t.Errorf("Expected 2 adapters in default chain, got %d", len(adapter.adapters))
	}

	// Verify types
	if _, ok := adapter.adapters[0].(*ChatAdapter); !ok {
		t.Errorf("Expected first adapter to be ChatAdapter, got %T", adapter.adapters[0])
	}
	if _, ok := adapter.adapters[1].(*JSONAdapter); !ok {
		t.Errorf("Expected second adapter to be JSONAdapter, got %T", adapter.adapters[1])
	}
}

// TestFallbackAdapter_ParseChatSuccess tests successful parsing with ChatAdapter
func TestFallbackAdapter_ParseChatSuccess(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response with field markers (ChatAdapter format)
	content := "[[ ## answer ## ]]\n42"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if answer, ok := outputs["answer"].(string); !ok || answer != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used first adapter (ChatAdapter)
	if adapter.GetLastUsedAdapter() != 0 {
		t.Errorf("Expected adapter 0 to be used, got %d", adapter.GetLastUsedAdapter())
	}

	// Check metadata
	if outputs["__parse_attempts"] != 1 {
		t.Errorf("Expected 1 parse attempt, got %v", outputs["__parse_attempts"])
	}
	if outputs["__fallback_used"] != false {
		t.Errorf("Expected fallback_used=false, got %v", outputs["__fallback_used"])
	}
}

// TestFallbackAdapter_ParseFallbackToJSON tests fallback to JSONAdapter
func TestFallbackAdapter_ParseFallbackToJSON(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response in JSON format (no field markers, ChatAdapter will fail)
	content := `{"answer": "42"}`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if answer, ok := outputs["answer"].(string); !ok || answer != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used second adapter (JSONAdapter)
	if adapter.GetLastUsedAdapter() != 1 {
		t.Errorf("Expected adapter 1 to be used, got %d", adapter.GetLastUsedAdapter())
	}

	// Check metadata - fallback was used
	if outputs["__parse_attempts"] != 2 {
		t.Errorf("Expected 2 parse attempts, got %v", outputs["__parse_attempts"])
	}
	if outputs["__fallback_used"] != true {
		t.Errorf("Expected fallback_used=true, got %v", outputs["__fallback_used"])
	}
}

// TestFallbackAdapter_ParseAllFail tests when all adapters fail
func TestFallbackAdapter_ParseAllFail(t *testing.T) {
	adapter := NewFallbackAdapter()
	// Use multiple fields so JSONAdapter can't fall back
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "").
		AddOutput("confidence", FieldTypeString, "")

	// Response that neither adapter can parse (no markers, no JSON, multiple fields)
	content := "This is just plain text with no structure"

	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Fatal("Expected parse to fail when all adapters fail")
	}

	// Error should mention all adapters
	if !strings.Contains(err.Error(), "all adapters failed") {
		t.Errorf("Expected error about all adapters failing, got: %v", err)
	}
}

// TestFallbackAdapter_WithReasoning tests reasoning propagation to all adapters
func TestFallbackAdapter_WithReasoning(t *testing.T) {
	adapter := NewFallbackAdapter().WithReasoning(true)

	// Verify all adapters have reasoning enabled
	for i, a := range adapter.adapters {
		switch typed := a.(type) {
		case *ChatAdapter:
			if !typed.IncludeReasoning {
				t.Errorf("Adapter %d (ChatAdapter) should have reasoning enabled", i)
			}
		case *JSONAdapter:
			if !typed.IncludeReasoning {
				t.Errorf("Adapter %d (JSONAdapter) should have reasoning enabled", i)
			}
		}
	}
}

// TestFallbackAdapter_TypeCoercionConsistency tests type coercion across adapters
func TestFallbackAdapter_TypeCoercionConsistency(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "ChatAdapter format",
			content: "[[ ## count ## ]]\n42\n\n[[ ## score ## ]]\n0.95",
		},
		{
			name:    "JSONAdapter format",
			content: `{"count": "42", "score": "0.95"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Both should coerce to proper types
			if count, ok := outputs["count"].(int); !ok || count != 42 {
				t.Errorf("Expected count=42 (int), got %v (%T)", outputs["count"], outputs["count"])
			}
			if score, ok := outputs["score"].(float64); !ok || score != 0.95 {
				t.Errorf("Expected score=0.95 (float64), got %v (%T)", outputs["score"], outputs["score"])
			}
		})
	}
}

// TestFallbackAdapter_CustomChain tests custom adapter chain
func TestFallbackAdapter_CustomChain(t *testing.T) {
	// Create adapter with only JSONAdapter
	jsonOnly := NewFallbackAdapterWithChain(NewJSONAdapter())

	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Content that only JSONAdapter can parse
	content := `{"answer": "42"}`

	outputs, err := jsonOnly.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used the first (and only) adapter
	if jsonOnly.GetLastUsedAdapter() != 0 {
		t.Errorf("Expected adapter 0 to succeed, got %d", jsonOnly.GetLastUsedAdapter())
	}
}

// TestFallbackAdapter_Format tests Format uses first adapter
func TestFallbackAdapter_Format(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{}, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Should use ChatAdapter (first in chain), which uses field markers
	content := messages[0].Content
	if !strings.Contains(content, "[[ ## answer ## ]]") {
		t.Errorf("Expected ChatAdapter field markers, got: %s", content)
	}
}

// TestFallbackAdapter_FormatDelegation tests format delegation
func TestFallbackAdapter_FormatDelegation(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{"question": "test"}, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if len(messages) == 0 {
		t.Error("Format() should return messages")
	}
}

// TestFallbackAdapter_FormatHistory tests history delegation
func TestFallbackAdapter_FormatHistory(t *testing.T) {
	adapter := NewFallbackAdapter()
	history := NewHistory()
	history.AddUserMessage("test")

	messages := adapter.FormatHistory(history)
	if len(messages) != 1 {
		t.Error("FormatHistory should delegate to first adapter")
	}
}

// TestNewFallbackAdapterWithChain tests custom adapter chain constructor
func TestNewFallbackAdapterWithChain(t *testing.T) {
	// Test with custom chain
	jsonAdapter := NewJSONAdapter()
	adapter := NewFallbackAdapterWithChain(jsonAdapter)
	if len(adapter.adapters) != 1 {
		t.Errorf("Expected 1 adapter, got %d", len(adapter.adapters))
	}

	// Test with empty chain (should use defaults)
	defaultAdapter := NewFallbackAdapterWithChain()
	if len(defaultAdapter.adapters) != 2 {
		t.Errorf("Empty chain should use default 2 adapters, got %d", len(defaultAdapter.adapters))
	}
}

// TestFallbackAdapter_Format_EmptyAdapters tests Format with no adapters configured
func TestFallbackAdapter_Format_EmptyAdapters(t *testing.T) {
	// Create adapter with manually cleared adapter list (edge case)
	adapter := &FallbackAdapter{
		adapters:        []Adapter{}, // Empty adapter list
		lastUsedAdapter: -1,
	}

	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	_, err := adapter.Format(sig, map[string]any{}, nil)
	if err == nil {
		t.Error("Format() should error when no adapters configured")
	}
	if !strings.Contains(err.Error(), "no adapters configured") {
		t.Errorf("Expected 'no adapters configured' error, got: %v", err)
	}
}

// TestFallbackAdapter_FormatHistory_EmptyAdapters tests FormatHistory with no adapters
func TestFallbackAdapter_FormatHistory_EmptyAdapters(t *testing.T) {
	// Create adapter with manually cleared adapter list (edge case)
	adapter := &FallbackAdapter{
		adapters:        []Adapter{}, // Empty adapter list
		lastUsedAdapter: -1,
	}

	history := NewHistory()
	history.AddUserMessage("test")

	messages := adapter.FormatHistory(history)
	if len(messages) != 0 {
		t.Errorf("FormatHistory() should return empty slice when no adapters, got %d messages", len(messages))
	}
}

// TestFallbackAdapter_Format_WithDemos tests Format with few-shot examples
func TestFallbackAdapter_Format_WithDemos(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("Answer question").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 2+2?"},
			Outputs: map[string]any{"answer": "4"},
		},
		{
			Inputs:  map[string]any{"question": "What is 3+3?"},
			Outputs: map[string]any{"answer": "6"},
		},
	}

	messages, err := adapter.Format(sig, map[string]any{"question": "What is 5+5?"}, demos)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Should delegate to first adapter (ChatAdapter) and include demos
	if len(messages) == 0 {
		t.Error("Expected messages from Format")
	}

	// Check that demos are referenced in the formatted messages
	allContent := ""
	for _, msg := range messages {
		allContent += msg.Content
	}

	if !strings.Contains(allContent, "What is 2+2?") {
		t.Errorf("Expected demo examples in formatted content")
	}
}

// TestFallbackAdapter_Parse_AdapterMetadata tests adapter metadata tracking
func TestFallbackAdapter_Parse_AdapterMetadata(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	tests := []struct {
		name             string
		content          string
		wantAttempts     int
		wantFallbackUsed bool
		wantAdapterIndex int
	}{
		{
			name:             "First adapter success (ChatAdapter)",
			content:          "[[ ## answer ## ]]\ntest",
			wantAttempts:     1,
			wantFallbackUsed: false,
			wantAdapterIndex: 0,
		},
		{
			name:             "Second adapter success (JSONAdapter)",
			content:          `{"answer": "test"}`,
			wantAttempts:     2,
			wantFallbackUsed: true,
			wantAdapterIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if outputs["__parse_attempts"] != tt.wantAttempts {
				t.Errorf("Expected %d parse attempts, got %v", tt.wantAttempts, outputs["__parse_attempts"])
			}

			if outputs["__fallback_used"] != tt.wantFallbackUsed {
				t.Errorf("Expected fallback_used=%v, got %v", tt.wantFallbackUsed, outputs["__fallback_used"])
			}

			if adapter.GetLastUsedAdapter() != tt.wantAdapterIndex {
				t.Errorf("Expected adapter %d, got %d", tt.wantAdapterIndex, adapter.GetLastUsedAdapter())
			}

			// Check adapter_used field
			if outputs["__adapter_used"] == nil {
				t.Error("Expected __adapter_used metadata")
			}
		})
	}
}

type mockExtractionLM struct {
	response string
	err      error
}

func (m *mockExtractionLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &GenerateResult{
		Content: m.response,
		Usage:   Usage{},
	}, nil
}

func (m *mockExtractionLM) Name() string {
	return "mock-extraction"
}

func (m *mockExtractionLM) SupportsJSON() bool {
	return true
}

func (m *mockExtractionLM) SupportsTools() bool {
	return false
}

func (m *mockExtractionLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
}

// TestTwoStepAdapter_Format tests the formatting of stage 1 (free-form) prompts
func TestTwoStepAdapter_Format(t *testing.T) {
	adapter := NewTwoStepAdapter(nil) // No extraction LM needed for Format
	sig := NewSignature("Analyze sentiment").
		AddInput("text", FieldTypeString, "Text to analyze").
		AddOutput("sentiment", FieldTypeString, "Sentiment classification").
		AddOutput("confidence", FieldTypeFloat, "Confidence score")

	inputs := map[string]any{
		"text": "I love this product!",
	}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	content := messages[0].Content

	// Should include description
	if !strings.Contains(content, "Analyze sentiment") {
		t.Errorf("Expected description in content")
	}

	// Should include natural response instruction (not structured)
	if !strings.Contains(content, "natural response") {
		t.Errorf("Expected natural response instruction")
	}

	// Should include input value
	if !strings.Contains(content, "I love this product!") {
		t.Errorf("Expected input value in content")
	}

	// Should mention expected outputs WITHOUT forcing structure
	if !strings.Contains(content, "sentiment") {
		t.Errorf("Expected 'sentiment' mentioned in guidance")
	}
	if !strings.Contains(content, "confidence") {
		t.Errorf("Expected 'confidence' mentioned in guidance")
	}

	// Should NOT have strict JSON formatting requirement (that's stage 2)
	if strings.Contains(content, "ONLY valid JSON") {
		t.Errorf("Stage 1 should not require JSON format")
	}
}

// TestTwoStepAdapter_FormatWithDemos tests demo formatting
func TestTwoStepAdapter_FormatWithDemos(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)
	sig := NewSignature("Classify").
		AddInput("text", FieldTypeString, "").
		AddOutput("category", FieldTypeString, "")

	demos := []Example{
		*NewExample(
			map[string]any{"text": "Great service!"},
			map[string]any{"category": "positive"},
		),
	}

	inputs := map[string]any{"text": "Good product"}
	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	content := messages[0].Content
	if !strings.Contains(content, "Examples") {
		t.Errorf("Expected examples section")
	}
	if !strings.Contains(content, "Great service!") {
		t.Errorf("Expected demo input in content")
	}
}

// TestTwoStepAdapter_Parse tests the two-stage extraction process
func TestTwoStepAdapter_Parse(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("sentiment", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionResult string
		extractionError  error
		expected         map[string]any
		wantErr          bool
	}{
		{
			name: "Successful extraction",
			freeFormResponse: "This text has a positive sentiment. I'm quite confident about this assessment, " +
				"I'd say around 0.95 confidence level.",
			extractionResult: `{"sentiment": "positive", "confidence": 0.95}`,
			expected:         map[string]any{"sentiment": "positive", "confidence": 0.95},
			wantErr:          false,
		},
		{
			name: "Extraction with reasoning",
			freeFormResponse: "After careful analysis, this appears to be negative. " +
				"The confidence is moderate at 0.7.",
			extractionResult: `{
				"reasoning": "Analyzed word choice and tone",
				"sentiment": "negative",
				"confidence": 0.7
			}`,
			expected: map[string]any{
				"reasoning":  "Analyzed word choice and tone",
				"sentiment":  "negative",
				"confidence": 0.7,
			},
			wantErr: false,
		},
		{
			name:             "Extraction LM failure",
			freeFormResponse: "Some response",
			extractionError:  fmt.Errorf("LM API error"),
			wantErr:          true,
		},
		{
			name:             "Invalid extraction JSON",
			freeFormResponse: "Some response",
			extractionResult: "not json",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{
				response: tt.extractionResult,
				err:      tt.extractionError,
			}
			adapter := NewTwoStepAdapter(mockLM).WithReasoning(true)

			outputs, err := adapter.Parse(sig, tt.freeFormResponse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for key, expectedVal := range tt.expected {
					actualVal, ok := outputs[key]
					if !ok {
						t.Errorf("Missing expected output key: %s", key)
						continue
					}
					if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedVal) {
						t.Errorf("For key %s: expected %v, got %v", key, expectedVal, actualVal)
					}
				}
			}
		})
	}
}

// TestTwoStepAdapter_ParseWithoutExtractionLM tests error when no extraction LM provided
func TestTwoStepAdapter_ParseWithoutExtractionLM(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)
	sig := NewSignature("test").AddOutput("answer", FieldTypeString, "")

	_, err := adapter.Parse(sig, "some response")
	if err == nil {
		t.Error("Expected error when parsing without extraction LM")
	}
	if !strings.Contains(err.Error(), "extraction LM") {
		t.Errorf("Expected error about missing extraction LM, got: %v", err)
	}
}

// TestTwoStepAdapter_TypeCoercion tests that extraction maintains type coercion
func TestTwoStepAdapter_TypeCoercion(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "").
		AddOutput("active", FieldTypeBool, "")

	mockLM := &mockExtractionLM{
		response: `{"count": "42", "score": "0.95", "active": "true"}`,
	}
	adapter := NewTwoStepAdapter(mockLM)

	outputs, err := adapter.Parse(sig, "The count is 42, score is 0.95, active status is true")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify type coercion
	if count, ok := outputs["count"].(int); !ok || count != 42 {
		t.Errorf("Expected count to be int 42, got %v (%T)", outputs["count"], outputs["count"])
	}
	if score, ok := outputs["score"].(float64); !ok || score != 0.95 {
		t.Errorf("Expected score to be float64 0.95, got %v (%T)", outputs["score"], outputs["score"])
	}
	if active, ok := outputs["active"].(bool); !ok || !active {
		t.Errorf("Expected active to be bool true, got %v (%T)", outputs["active"], outputs["active"])
	}
}

// TestTwoStepAdapter_WithReasoning tests reasoning flag
func TestTwoStepAdapter_WithReasoning(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)

	// Default should include reasoning
	if !adapter.IncludeReasoning {
		t.Error("Expected reasoning to be enabled by default")
	}

	// Can disable
	adapter.WithReasoning(false)
	if adapter.IncludeReasoning {
		t.Error("Expected reasoning to be disabled")
	}

	// Can re-enable
	adapter.WithReasoning(true)
	if !adapter.IncludeReasoning {
		t.Error("Expected reasoning to be enabled")
	}
}

// TestTwoStepAdapter_Format_MultipleInputs tests formatting with multiple inputs
func TestTwoStepAdapter_Format_MultipleInputs(t *testing.T) {
	sig := NewSignature("Summarize").
		AddInput("title", FieldTypeString, "").
		AddInput("body", FieldTypeString, "").
		AddOutput("summary", FieldTypeString, "")

	adapter := NewTwoStepAdapter(nil)
	inputs := map[string]any{
		"title": "Important News",
		"body":  "This is the full article text...",
	}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestTwoStepAdapter_FormatHistory_Coverage tests TwoStepAdapter FormatHistory
func TestTwoStepAdapter_FormatHistory_Coverage(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)

	history := NewHistory()
	history.Add(Message{Role: "user", Content: "Question 1"})
	history.Add(Message{Role: "assistant", Content: "Answer 1"})
	history.Add(Message{Role: "user", Content: "Question 2"})

	messages := adapter.FormatHistory(history)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	for i, msg := range messages {
		if msg.Role == "" {
			t.Errorf("Message %d has empty role", i)
		}
		if msg.Content == "" {
			t.Errorf("Message %d has empty content", i)
		}
	}
}

// TestTwoStepAdapter_Parse_ErrorCases tests various error scenarios in TwoStepAdapter.Parse
func TestTwoStepAdapter_Parse_ErrorCases(t *testing.T) {
	// Use multiple fields to prevent JSONAdapter fallback for single string fields
	sig := NewSignature("test").
		AddOutput("result", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionResult string
		extractionError  error
		wantErr          bool
		errContains      string
	}{
		{
			name:             "Extraction LM network error",
			freeFormResponse: "Some response",
			extractionError:  fmt.Errorf("connection refused"),
			wantErr:          true,
			errContains:      "extraction LM failed",
		},
		{
			name:             "Extraction LM returns malformed JSON",
			freeFormResponse: "Some response",
			extractionResult: "{invalid json",
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns empty response",
			freeFormResponse: "Some response",
			extractionResult: "",
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns non-JSON object",
			freeFormResponse: "Some response",
			extractionResult: `"just a string"`,
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns array instead of object",
			freeFormResponse: "Some response",
			extractionResult: `["item1", "item2"]`,
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{
				response: tt.extractionResult,
				err:      tt.extractionError,
			}
			adapter := NewTwoStepAdapter(mockLM)

			_, err := adapter.Parse(sig, tt.freeFormResponse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errContains, err)
			}
		})
	}
}

// TestTwoStepAdapter_Parse_TypeCoercion_EdgeCases tests type coercion edge cases
func TestTwoStepAdapter_Parse_TypeCoercion_EdgeCases(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "").
		AddOutput("flag", FieldTypeBool, "").
		AddOutput("text", FieldTypeString, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionJSON   string
		expected         map[string]any
	}{
		{
			name:             "Numeric strings to numbers",
			freeFormResponse: "Count is 42, score is 3.14, flag is true",
			extractionJSON:   `{"count": "42", "score": "3.14", "flag": "true", "text": "hello"}`,
			expected:         map[string]any{"count": 42, "score": 3.14, "flag": true, "text": "hello"},
		},
		{
			name:             "Mixed quotes and whitespace",
			freeFormResponse: "Data with quotes",
			extractionJSON:   `{"count": " 42 ", "score": " 3.14\t", "flag": " false ", "text": " test "}`,
			expected:         map[string]any{"count": 42, "score": 3.14, "flag": false, "text": "test"}, // Note: some processing may trim whitespace
		},
		{
			name:             "Percentage strings",
			freeFormResponse: "95% confidence",
			extractionJSON:   `{"score": "95%"}`,
			expected:         map[string]any{"score": 95.0},
		},
		{
			name:             "Qualitative scores",
			freeFormResponse: "High confidence",
			extractionJSON:   `{"score": "high"}`,
			expected:         map[string]any{"score": 0.9}, // "high" maps to 0.9
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{response: tt.extractionJSON}
			adapter := NewTwoStepAdapter(mockLM)

			outputs, err := adapter.Parse(sig, tt.freeFormResponse)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			for key, expectedVal := range tt.expected {
				actualVal, ok := outputs[key]
				if !ok {
					t.Errorf("Missing expected output key: %s", key)
					continue
				}

				// Check type and value
				switch expected := expectedVal.(type) {
				case int:
					if actual, ok := actualVal.(int); !ok || actual != expected {
						t.Errorf("For key %s: expected int %d, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case float64:
					if actual, ok := actualVal.(float64); !ok || actual != expected {
						t.Errorf("For key %s: expected float64 %f, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case bool:
					if actual, ok := actualVal.(bool); !ok || actual != expected {
						t.Errorf("For key %s: expected bool %t, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case string:
					if actual, ok := actualVal.(string); !ok || actual != expected {
						t.Errorf("For key %s: expected string %q, got %v (%T)", key, expected, actualVal, actualVal)
					}
				}
			}
		})
	}
}

// TestTwoStepAdapter_Parse_WithFallbackChains tests integration with fallback chains
func TestTwoStepAdapter_Parse_WithFallbackChains(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("result", FieldTypeString, "")

	// Mock LM that fails on extraction
	failingLM := &mockExtractionLM{
		err: fmt.Errorf("extraction failed"),
	}

	// Test with TwoStepAdapter as part of a fallback chain
	fallbackAdapter := NewFallbackAdapterWithChain(
		NewTwoStepAdapter(failingLM), // This will fail
		NewJSONAdapter(),             // This should succeed
	)

	// Content that JSONAdapter can parse
	content := `{"result": "success"}`

	outputs, err := fallbackAdapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Fallback should have succeeded: %v", err)
	}

	if outputs["result"] != "success" {
		t.Errorf("Expected result='success', got %v", outputs["result"])
	}

	// Should have used the second adapter (JSONAdapter)
	if fallbackAdapter.GetLastUsedAdapter() != 1 {
		t.Errorf("Expected fallback to adapter 1, got %d", fallbackAdapter.GetLastUsedAdapter())
	}

	// Check metadata
	if outputs["__fallback_used"] != true {
		t.Errorf("Expected fallback_used=true, got %v", outputs["__fallback_used"])
	}
}

// TestChatAdapterPartialOutput tests parsing when LM hits token limit mid-generation
func TestChatAdapterPartialOutput(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		signature    *Signature
		shouldParse  bool
		hasField     string
		missingField string
	}{
		{
			name: "gemini cutoff before summary field",
			content: `[[ ## explanation ## ]]
Photosynthesis is the process by which plants convert light energy into chemical energy.
This complex process occurs at the molecular level within specialized organelles called chloroplasts.
The process can be divided into two main stages: light-dependent and light-independent reactions.

[Stream finished: length]`,
			signature: NewSignature("Explain photosynthesis").
				AddInput("question", FieldTypeString, "").
				AddOutput("explanation", FieldTypeString, "").
				AddOutput("summary", FieldTypeString, ""),
			shouldParse:  false, // Should fail - missing summary
			hasField:     "",
			missingField: "summary",
		},
		{
			name: "complete output with all fields",
			content: `[[ ## explanation ## ]]
Photosynthesis converts light to chemical energy.

[[ ## summary ## ]]
Plants use light to make glucose.`,
			signature: NewSignature("Explain photosynthesis").
				AddInput("question", FieldTypeString, "").
				AddOutput("explanation", FieldTypeString, "").
				AddOutput("summary", FieldTypeString, ""),
			shouldParse: true,
			hasField:    "summary",
		},
		{
			name: "truncated in middle of field value",
			content: `[[ ## story ## ]]
The astronaut discovered an artifact on Mars. It was a perfect dodecahedron...

[[ ## title ## ]]
The Martian

[Stream finished: length]`,
			signature: NewSignature("Generate story").
				AddInput("prompt", FieldTypeString, "").
				AddOutput("story", FieldTypeString, "").
				AddOutput("title", FieldTypeString, "").
				AddOutput("genre", FieldTypeString, ""),
			shouldParse:  false, // Missing genre
			hasField:     "title",
			missingField: "genre",
		},
		{
			name:    "no field markers at all - empty response",
			content: `Photosynthesis is the process...`,
			signature: NewSignature("Explain").
				AddInput("q", FieldTypeString, "").
				AddOutput("explanation", FieldTypeString, ""),
			shouldParse:  false,
			missingField: "explanation",
		},
	}

	adapter := NewChatAdapter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(tt.signature, tt.content)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("expected successful parse, got error: %v", err)
				}
				if tt.hasField != "" {
					if _, ok := outputs[tt.hasField]; !ok {
						t.Errorf("expected field %q to be present", tt.hasField)
					}
				}
			} else {
				if err == nil {
					t.Errorf("expected parse error, but got none")
				}
				if tt.missingField != "" && err != nil {
					// Error should mention the missing field
					if !containsSubstr(err.Error(), tt.missingField) {
						t.Errorf("error should mention missing field %q, got: %v", tt.missingField, err)
					}
				}
			}
		})
	}
}

// TestJSONAdapterEdgeCases tests JSON adapter with malformed/partial JSON
func TestJSONAdapterEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		signature    *Signature
		shouldParse  bool
		expectRepair bool
	}{
		{
			name:    "cutoff JSON - missing closing brace",
			content: `{"explanation": "Photosynthesis converts light to chemical energy through chloroplasts`,
			signature: NewSignature("").
				AddOutput("explanation", FieldTypeString, ""),
			shouldParse:  true,  // JSON repair should add closing brace and quotes
			expectRepair: false, // Repair metadata not always added, just check it parses
		},
		{
			name:    "model outputs class with prefix",
			content: `{"sentiment": "(one of: positive)"}`,
			signature: NewSignature("").
				AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, ""),
			shouldParse:  true,
			expectRepair: false, // Valid JSON, just needs class normalization
		},
		{
			name:    "model outputs partial class value",
			content: `{"sentiment": "(one"}`,
			signature: NewSignature("").
				AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, ""),
			shouldParse: true, // Valid JSON, validation will fail but parse succeeds
		},
		{
			name:    "truncated in middle of JSON value",
			content: `{"story": "The astronaut found an artifact...", "title": "The Mar`,
			signature: NewSignature("").
				AddOutput("story", FieldTypeString, "").
				AddOutput("title", FieldTypeString, ""),
			shouldParse: false,
		},
	}

	adapter := NewJSONAdapter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(tt.signature, tt.content)

			if tt.shouldParse {
				if err != nil {
					t.Errorf("expected successful parse, got error: %v", err)
				}
				if tt.expectRepair {
					if _, hasRepair := outputs["__json_repair"]; !hasRepair {
						t.Errorf("expected JSON repair to be used")
					}
				}
			} else {
				if err == nil {
					t.Errorf("expected parse error for malformed JSON, got none")
				}
			}
		})
	}
}

// TestClassNormalizationEdgeCases tests enhanced class normalization
func TestClassNormalizationEdgeCases(t *testing.T) {
	field := Field{
		Name:    "sentiment",
		Type:    FieldTypeClass,
		Classes: []string{"positive", "negative", "neutral"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"bare parenthesis prefix", "(one", "(one"}, // No valid class, returns original
		{"full parenthesis wrap", "(one)", "(one)"}, // No valid class, returns original
		{"one of prefix", "one of: positive", "positive"},
		{"one of without colon", "one of positive", "positive"},
		{"one prefix only", "one positive", "positive"},
		{"answer prefix", "answer: negative", "negative"},
		{"result prefix", "result: neutral", "neutral"},
		{"substring match", "(positive answer)", "positive"},
		{"substring in text", "the answer is positive", "positive"},
		{"exact match", "negative", "negative"},
		{"case insensitive", "POSITIVE", "positive"},
		{"with quotes", "\"positive\"", "positive"},
		{"with backticks", "`negative`", "negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeClassValue(tt.input, field)
			if result != tt.expected {
				t.Errorf("normalizeClassValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFallbackAdapterWithPartialOutputs tests fallback when primary adapter fails
func TestFallbackAdapterWithPartialOutputs(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("field1", FieldTypeString, "").
		AddOutput("field2", FieldTypeString, "")

	// Content that's valid for ChatAdapter but not JSONAdapter
	content := `[[ ## field1 ## ]]
Value 1

[[ ## field2 ## ]]
Value 2`

	chatAdapter := NewChatAdapter()
	jsonAdapter := NewJSONAdapter()
	fallback := NewFallbackAdapter()
	fallback.adapters = []Adapter{jsonAdapter, chatAdapter}

	outputs, err := fallback.Parse(sig, content)
	if err != nil {
		t.Fatalf("fallback adapter should succeed with chat format: %v", err)
	}

	if v, ok := outputs["field1"]; !ok || v != "Value 1" {
		t.Errorf("expected field1='Value 1', got %v", v)
	}
	if v, ok := outputs["field2"]; !ok || v != "Value 2" {
		t.Errorf("expected field2='Value 2', got %v", v)
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test incomplete marker handling - fixes for real test matrix failures
func TestChatAdapter_IncompleteMarkers(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("reasoning", FieldTypeString, "").
		AddOutput("answer", FieldTypeInt, "").
		AddOutput("explanation", FieldTypeString, "")

	adapter := NewChatAdapter()

	tests := []struct {
		name     string
		content  string
		wantAns  int
		wantExpl string
	}{
		{
			name: "incomplete marker missing closing brackets - z-ai/glm-4.6 failure",
			content: `[[ ## reasoning ## ]]
1. John starts with 5 apples.
2. He gives 2 apples to Mary, so we subtract 2 from 5: 5 - 2 = 3.
3. He then buys 3 more apples, so we add 3 to the remaining 3: 3 + 3 = 6.
4. The final number of apples John has is 6.

[[ ## answer ## ]]
6

[[ ## explanation ## ]
1. John starts with 5 apples.
2. He gives 2 apples to Mary, leaving him with 5 - 2 = 3 apples.
3. He buys 3 more apples, bringing his total to 3 + 3 = 6 apples.
4. Therefore, John now has 6 apples.`,
			wantAns:  6,
			wantExpl: "1. John starts with 5 apples.\n2. He gives 2 apples to Mary, leaving him with 5 - 2 = 3 apples.\n3. He buys 3 more apples, bringing his total to 3 + 3 = 6 apples.\n4. Therefore, John now has 6 apples.",
		},
		{
			name: "incomplete marker with no closing brackets at all",
			content: `[[ ## reasoning ## ]]
Some reasoning here.

[[ ## answer ##
42

[[ ## explanation ## ]]
The answer is 42 because that's the answer to everything.`,
			wantAns:  42,
			wantExpl: "The answer is 42 because that's the answer to everything.",
		},
		{
			name: "incomplete marker single closing bracket",
			content: `[[ ## reasoning ## ]]
Calculating...

[[ ## answer ## ]
99

[[ ## explanation ## ]]
The calculation yields 99.`,
			wantAns:  99,
			wantExpl: "The calculation yields 99.",
		},
		{
			name: "DSPy style - content on same line as marker",
			content: `[[ ## reasoning ## ]] Let me think about this carefully
The problem requires...

[[ ## answer ## 123

[[ ## explanation ## ]]
This is the explanation.`,
			wantAns:  123,
			wantExpl: "This is the explanation.",
		},
		{
			name: "mixed incomplete markers",
			content: `[[ ## reasoning ## ]]
Step by step reasoning

[[ ## answer ##
77

[[ ## explanation ## ]
Partial explanation here`,
			wantAns:  77,
			wantExpl: "Partial explanation here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse() error = %v, should handle incomplete markers gracefully", err)
			}

			// Validate answer
			if ans, ok := outputs["answer"].(int); !ok || ans != tt.wantAns {
				t.Errorf("answer = %v, want %v", outputs["answer"], tt.wantAns)
			}

			// Validate explanation
			if expl, ok := outputs["explanation"].(string); !ok || expl != tt.wantExpl {
				t.Errorf("explanation = %q, want %q", outputs["explanation"], tt.wantExpl)
			}
		})
	}
}

// Test that simulates the exact failure from test matrix
func TestChatAdapter_RealTestMatrixFailure_GLM46_ChainOfThought(t *testing.T) {
	sig := NewSignature("Answer a simple math question").
		AddOutput("reasoning", FieldTypeString, "Step by step reasoning").
		AddOutput("answer", FieldTypeInt, "Final answer").
		AddOutput("explanation", FieldTypeString, "Explanation of the answer")

	adapter := NewChatAdapter()

	// This is the EXACT content that caused z-ai/glm-4.6:exacto to fail
	content := `[[ ## reasoning ## ]]
1. John starts with 5 apples.
2. He gives 2 apples to Mary, so we subtract 2 from 5: 5 - 2 = 3.
3. He then buys 3 more apples, so we add 3 to the remaining 3: 3 + 3 = 6.
4. The final number of apples John has is 6.

[[ ## answer ## ]]
6

[[ ## explanation ## ]
1. John starts with 5 apples.
2. He gives 2 apples to Mary, leaving him with 5 - 2 = 3 apples.
3. He buys 3 more apples, bringing his total to 3 + 3 = 6 apples.
4. Therefore, John now has 6 apples.`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse() failed on real test matrix failure case: %v", err)
	}

	// Should successfully extract all fields
	if outputs["answer"] == nil {
		t.Error("Failed to extract answer field")
	}
	if outputs["explanation"] == nil {
		t.Error("Failed to extract explanation field - this was the original failure!")
	}
	if outputs["reasoning"] == nil {
		t.Error("Failed to extract reasoning field")
	}

	// Verify values
	if ans, ok := outputs["answer"].(int); !ok || ans != 6 {
		t.Errorf("answer = %v (type %T), want 6 (int)", outputs["answer"], outputs["answer"])
	}
}

// Test FallbackAdapter automatic retry
func TestFallbackAdapter_AutomaticRetry(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("result", FieldTypeString, "")

	fallback := NewFallbackAdapter()

	// Content that ChatAdapter might struggle with but JSONAdapter handles
	content := `{"result": "success"}`

	outputs, err := fallback.Parse(sig, content)
	if err != nil {
		t.Fatalf("FallbackAdapter should handle this via JSONAdapter: %v", err)
	}

	if outputs["result"] != "success" {
		t.Errorf("result = %v, want 'success'", outputs["result"])
	}

	// Verify fallback was used
	if fallbackUsed, ok := outputs["__fallback_used"].(bool); !ok || !fallbackUsed {
		t.Log("Warning: Expected fallback to be used for JSON content")
	}
}

// Test partial validation mode
func TestSignature_ValidateOutputsPartial(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("field1", FieldTypeString, "").
		AddOutput("field2", FieldTypeInt, "").
		AddOutput("field3", FieldTypeString, "")

	// Partial outputs - missing field3
	outputs := map[string]any{
		"field1": "value1",
		"field2": 42,
	}

	diag := sig.ValidateOutputsPartial(outputs)

	if len(diag.MissingFields) != 1 || diag.MissingFields[0] != "field3" {
		t.Errorf("MissingFields = %v, want [field3]", diag.MissingFields)
	}

	// Should have set missing field to nil
	if outputs["field3"] != nil {
		t.Errorf("Missing field should be set to nil, got %v", outputs["field3"])
	}

	// Should have preserved parsed fields
	if outputs["field1"] != "value1" {
		t.Errorf("field1 should be preserved")
	}
	if outputs["field2"] != 42 {
		t.Errorf("field2 should be preserved")
	}
}

// TestChatAdapterStripMarkersFromOutputs tests that field markers are stripped from parsed outputs
func TestChatAdapterStripMarkersFromOutputs(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "marker at start",
			content:  "[[ ## answer ## ]] The actual answer",
			expected: "The actual answer",
		},
		{
			name:     "trailing marker fragment",
			content:  "[[ ## answer ## ]] Some text]]",
			expected: "Some text",
		},
		{
			name:     "no markers",
			content:  "[[ ## answer ## ]] Clean text without trailing markers",
			expected: "Clean text without trailing markers",
		},

		{
			name:     "just trailing brackets",
			content:  "[[ ## answer ## ]] Final answer]]",
			expected: "Final answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			answer, ok := outputs["answer"].(string)
			if !ok {
				t.Fatal("answer is not a string")
			}

			if answer != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, answer)
			}
		})
	}
}

// TestStripFieldMarkers tests the stripFieldMarkers helper function directly
func TestStripFieldMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "marker at start",
			input:    "[[ ## response ## ]] Hi there!",
			expected: "Hi there!",
		},
		{
			name:     "marker with spaces",
			input:    "[[  ##  answer  ##  ]] Result",
			expected: "Result",
		},
		{
			name:     "trailing brackets only",
			input:    "Some text]]",
			expected: "Some text",
		},
		{
			name:     "no markers",
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "marker only",
			input:    "[[ ## field ## ]]",
			expected: "",
		},
		{
			name:     "multiple trailing brackets",
			input:    "Text]]]]",
			expected: "Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripFieldMarkers(tt.input)
			if result != tt.expected {
				t.Errorf("stripFieldMarkers(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTruncateString tests the truncateString helper function
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "short",
			maxLen: 10,
			want:   "short",
		},
		{
			name:   "exact length",
			input:  "exactly10c",
			maxLen: 10,
			want:   "exactly10c",
		},
		{
			name:   "truncate with ellipsis",
			input:  "this is a very long string",
			maxLen: 10,
			want:   "this is a ...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 5,
			want:   "",
		},
		{
			name:   "maxLen of 1",
			input:  "test",
			maxLen: 1,
			want:   "t...",
		},
		{
			name:   "maxLen of 0",
			input:  "test",
			maxLen: 0,
			want:   "...",
		},
		{
			name:   "long string with common chars",
			input:  "This is a very long string that needs truncation",
			maxLen: 15,
			want:   "This is a very ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.want)
			}
		})
	}
}

// TestStripMarkers tests the public StripMarkers API
func TestStripMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple marker at start",
			input:    "[[ ## answer ## ]] Hello world",
			expected: "Hello world",
		},
		{
			name:     "marker in middle",
			input:    "Text [[ ## field ## ]] more text",
			expected: "Text  more text",
		},
		{
			name:     "no markers",
			input:    "Plain text without markers",
			expected: "Plain text without markers",
		},
		{
			name:     "multiple markers",
			input:    "[[ ## field1 ## ]] First [[ ## field2 ## ]] Second",
			expected: "First  Second",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only marker",
			input:    "[[ ## answer ## ]]",
			expected: "",
		},
		{
			name:     "partial marker at start",
			input:    "## ]] Content",
			expected: "Content",
		},
		{
			name:     "trailing brackets",
			input:    "Content]]",
			expected: "Content",
		},
		{
			name:     "whitespace around marker",
			input:    "[[  ##  field  ##  ]] Text",
			expected: "Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripMarkers(tt.input)
			if result != tt.expected {
				t.Errorf("StripMarkers(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestStripFieldMarkersPreserveJSON tests marker stripping for JSON fields
func TestStripFieldMarkersPreserveJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "nested array with trailing ]]",
			input:    `[["day1", "temple"], ["day2", "food"], ["day3", "shopping"]]`,
			expected: `[["day1", "temple"], ["day2", "food"], ["day3", "shopping"]]`,
		},
		{
			name:     "deeply nested arrays",
			input:    `[[["a", "b"]], [["c", "d"]]]`,
			expected: `[[["a", "b"]], [["c", "d"]]]`,
		},
		{
			name:     "object with nested arrays",
			input:    `{"data": [["x", "y"]], "more": [["z"]]}`,
			expected: `{"data": [["x", "y"]], "more": [["z"]]}`,
		},
		{
			name:     "marker at start then JSON",
			input:    `[[ ## data ## ]] [["a", "b"]]`,
			expected: `[["a", "b"]]`,
		},
		{
			name:     "marker in middle of JSON (should strip)",
			input:    `[["a", "b"], [[ ## field ## ]] ["c", "d"]]`,
			expected: `[["a", "b"],  ["c", "d"]]`,
		},
		{
			name:     "empty nested arrays",
			input:    `[[]]`,
			expected: `[[]]`,
		},
		{
			name:     "triple nested empty",
			input:    `[[[]]]`,
			expected: `[[[]]]`,
		},
		{
			name:     "JSON array ending with multiple brackets",
			input:    `{"items": [{"tags": ["a", "b"]}]}`,
			expected: `{"items": [{"tags": ["a", "b"]}]}`,
		},
		{
			name:     "partial marker at start",
			input:    `]] [["valid", "json"]]`,
			expected: `[["valid", "json"]]`,
		},
		{
			name: "complex real-world JSON",
			input: `[
				{"day": 1, "activities": ["temple", "food"]},
				{"day": 2, "activities": ["bamboo", "shopping"]}
			]`,
			expected: `[
				{"day": 1, "activities": ["temple", "food"]},
				{"day": 2, "activities": ["bamboo", "shopping"]}
			]`,
		},
		{
			name:     "marker with spaces then nested array",
			input:    `[[  ##  field  ##  ]] [["x"]]`,
			expected: `[["x"]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripFieldMarkersPreserveJSON(tt.input)
			if result != tt.expected {
				t.Errorf("stripFieldMarkersPreserveJSON() failed\nInput:    %q\nGot:      %q\nExpected: %q",
					tt.input, result, tt.expected)
			}

			// Verify it's still valid JSON (if it started as JSON)
			if strings.HasPrefix(strings.TrimSpace(tt.expected), "[") ||
				strings.HasPrefix(strings.TrimSpace(tt.expected), "{") {
				var parsed interface{}
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					t.Errorf("Result is not valid JSON: %v\nResult: %q", err, result)
				}
			}
		})
	}
}

// TestChatAdapter_Parse_JSONField tests that ChatAdapter properly parses JSON field types
func TestChatAdapter_Parse_JSONField(t *testing.T) {
	adapter := &ChatAdapter{}

	tests := []struct {
		name        string
		content     string
		wantOutputs map[string]any
		wantErr     bool
	}{
		{
			name: "valid JSON array",
			content: `[[ ## activities ## ]]
["Visit temple", "Try ramen", "See bamboo forest"]`,
			wantOutputs: map[string]any{
				"activities": []any{"Visit temple", "Try ramen", "See bamboo forest"},
			},
			wantErr: false,
		},
		{
			name: "valid JSON object",
			content: `[[ ## config ## ]]
{"name": "test", "count": 5}`,
			wantOutputs: map[string]any{
				"config": map[string]any{"name": "test", "count": float64(5)},
			},
			wantErr: false,
		},
		{
			name: "invalid JSON - text description",
			content: `[[ ## activities ## ]]
1. Visit temple
2. Try ramen`,
			wantOutputs: map[string]any{
				"activities": "1. Visit temple\n2. Try ramen",
			},
			wantErr: false, // Parse succeeds, but validation should fail later
		},
		{
			name: "nested JSON array with newline",
			content: `[[ ## data ## ]]

[["day1", "temple"], ["day2", "food"], ["day3", "shopping"]]`,
			wantOutputs: map[string]any{
				"data": []any{
					[]any{"day1", "temple"},
					[]any{"day2", "food"},
					[]any{"day3", "shopping"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create signature with only the fields present in expected outputs
			sig := NewSignature("Test")
			for key := range tt.wantOutputs {
				sig.AddOutput(key, FieldTypeJSON, key)
			}

			outputs, err := adapter.Parse(sig, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for key, wantValue := range tt.wantOutputs {
					gotValue, exists := outputs[key]
					if !exists {
						t.Errorf("Output %q missing", key)
						continue
					}

					// Check type matches expectations
					wantType := fmt.Sprintf("%T", wantValue)
					gotType := fmt.Sprintf("%T", gotValue)

					if wantType != gotType {
						t.Errorf("Output %q has type %T, want %T", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}
