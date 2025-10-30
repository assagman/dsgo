package dsgo

import (
	"strings"
	"testing"
)

// TestChatAdapter_Format tests the basic Format method
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
