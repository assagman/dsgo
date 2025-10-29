package dsgo

import (
	"strings"
	"testing"
)

// Test ChatAdapter formatting
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

// Test ChatAdapter with reasoning
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

// Test ChatAdapter demo formatting with role alternation
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

// Test ChatAdapter parsing with field markers
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

// Test ChatAdapter parsing multiple fields
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

// Test ChatAdapter type coercion
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

// Test FallbackAdapter with successful first adapter
func TestFallbackAdapter_SuccessFirstAdapter(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Content that ChatAdapter can parse
	content := "[[ ## answer ## ]]\n42"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used the first adapter (ChatAdapter)
	if adapter.GetLastUsedAdapter() != 0 {
		t.Errorf("Expected first adapter (0) to succeed, got %d", adapter.GetLastUsedAdapter())
	}
}

// Test FallbackAdapter falling back to second adapter
func TestFallbackAdapter_FallbackToSecond(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Content that only JSONAdapter can parse (no field markers)
	content := `{"answer": "42"}`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used the second adapter (JSONAdapter)
	if adapter.GetLastUsedAdapter() != 1 {
		t.Errorf("Expected second adapter (1) to succeed, got %d", adapter.GetLastUsedAdapter())
	}
}

// Test FallbackAdapter with all adapters failing
func TestFallbackAdapter_AllFail(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Content that neither adapter can parse
	content := "This is just plain text with no structure"

	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Fatal("Expected error when all adapters fail, got nil")
	}

	if !strings.Contains(err.Error(), "all adapters failed") {
		t.Errorf("Expected 'all adapters failed' error, got: %v", err)
	}
}

// Test FallbackAdapter with custom chain
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

// Test FallbackAdapter Format uses first adapter
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

// Test ChatAdapter with optional fields
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
