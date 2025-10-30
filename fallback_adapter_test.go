package dsgo

import (
	"strings"
	"testing"
)

// TestFallbackAdapter_DefaultChain tests the default adapter chain
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
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response that neither adapter can parse
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
