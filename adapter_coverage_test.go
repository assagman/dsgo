package dsgo

import (
	"testing"
)

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

// TestFallbackAdapter_FormatWithHistory tests fallback adapter's FormatHistory
func TestFallbackAdapter_FormatWithHistory(t *testing.T) {
	adapter := NewFallbackAdapter()

	history := NewHistory()
	history.Add(Message{Role: "user", Content: "Hello"})
	history.Add(Message{Role: "assistant", Content: "Hi"})

	messages := adapter.FormatHistory(history)

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
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
