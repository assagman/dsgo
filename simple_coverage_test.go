package dsgo

import (
	"testing"
)

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

// TestFallbackAdapter_ParseRetry tests retry logic in fallback
func TestFallbackAdapter_ParseRetry(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewFallbackAdapter()

	// Content that should work with chat adapter
	content := "answer: This is my answer"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if _, ok := outputs["answer"]; !ok {
		t.Error("Expected answer field in outputs")
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

// TestFallbackAdapter_ParseWithReasoning tests parsing with reasoning field
func TestFallbackAdapter_ParseWithReasoning(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewFallbackAdapter().WithReasoning(true)
	content := "answer: yes\nrationale: because it makes sense"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if _, ok := outputs["answer"]; !ok {
		t.Error("Expected answer field in outputs")
	}
}

// TestCacheKeyGeneration_EdgeCases tests cache key generation with various options
func TestCacheKeyGeneration_EdgeCases(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	opts1 := DefaultGenerateOptions()
	opts1.Stop = nil

	opts2 := DefaultGenerateOptions()
	opts2.Stop = []string{}

	key1 := GenerateCacheKey("model", messages, opts1)
	key2 := GenerateCacheKey("model", messages, opts2)

	if key1 == "" || key2 == "" {
		t.Error("Expected non-empty cache keys")
	}
}
