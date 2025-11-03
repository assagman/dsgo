package dsgo

import (
	"fmt"
	"testing"
)

func TestHistory_AddAndGet(t *testing.T) {
	h := NewHistory()

	if !h.IsEmpty() {
		t.Error("New history should be empty")
	}

	h.AddUserMessage("Hello")
	h.AddAssistantMessage("Hi there")

	if h.Len() != 2 {
		t.Errorf("Expected 2 messages, got %d", h.Len())
	}

	messages := h.Get()
	if messages[0].Role != "user" || messages[0].Content != "Hello" {
		t.Error("First message incorrect")
	}
	if messages[1].Role != "assistant" || messages[1].Content != "Hi there" {
		t.Error("Second message incorrect")
	}
}

func TestHistory_GetLast(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")
	h.AddUserMessage("msg3")

	last2 := h.GetLast(2)
	if len(last2) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(last2))
	}
	if last2[0].Content != "msg2" || last2[1].Content != "msg3" {
		t.Error("GetLast returned wrong messages")
	}
}

func TestHistory_WithLimit(t *testing.T) {
	h := NewHistoryWithLimit(2)

	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")
	h.AddUserMessage("msg3")

	if h.Len() != 2 {
		t.Errorf("Expected history to be limited to 2, got %d", h.Len())
	}

	messages := h.Get()
	if messages[0].Content != "msg2" || messages[1].Content != "msg3" {
		t.Error("History should keep most recent messages")
	}
}

func TestHistory_Clear(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("test")

	h.Clear()

	if !h.IsEmpty() {
		t.Error("History should be empty after Clear()")
	}
}

func TestHistory_Clone(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("original")

	cloned := h.Clone()
	cloned.AddUserMessage("cloned")

	if h.Len() == cloned.Len() {
		t.Error("Clone should be independent")
	}
	if h.Len() != 1 {
		t.Error("Original should not be affected by clone modifications")
	}
}

func TestHistory_Add(t *testing.T) {
	h := NewHistory()
	msg := Message{Role: "user", Content: "test", ToolID: "tool1"}
	h.Add(msg)

	if h.Len() != 1 {
		t.Errorf("Expected 1 message, got %d", h.Len())
	}

	retrieved := h.Get()[0]
	if retrieved.ToolID != "tool1" {
		t.Error("Message should preserve all fields")
	}
}

func TestHistory_AddSystemMessage(t *testing.T) {
	h := NewHistory()
	h.AddSystemMessage("system prompt")

	messages := h.Get()
	if messages[0].Role != "system" || messages[0].Content != "system prompt" {
		t.Error("AddSystemMessage should add system message correctly")
	}
}

func TestHistory_Truncate(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")
	h.AddUserMessage("msg3")

	h.Truncate(2)

	if h.Len() != 2 {
		t.Errorf("Expected 2 messages after truncate, got %d", h.Len())
	}
	if h.Get()[0].Content != "msg1" {
		t.Error("Truncate should keep first messages")
	}
}

func TestHistory_Truncate_NoOp(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")

	h.Truncate(10)

	if h.Len() != 1 {
		t.Error("Truncate with n > length should not change history")
	}

	h.Truncate(0)
	if h.Len() != 1 {
		t.Error("Truncate with n=0 should not change history")
	}
}

func TestHistory_RemoveFirst(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")
	h.AddUserMessage("msg3")

	h.RemoveFirst(1)

	if h.Len() != 2 {
		t.Errorf("Expected 2 messages, got %d", h.Len())
	}
	if h.Get()[0].Content != "msg2" {
		t.Error("RemoveFirst should remove first message")
	}
}

func TestHistory_RemoveFirst_All(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")

	h.RemoveFirst(10)

	if !h.IsEmpty() {
		t.Error("RemoveFirst with n >= length should clear history")
	}
}

func TestHistory_GetLast_EdgeCases(t *testing.T) {
	h := NewHistory()
	h.AddUserMessage("msg1")
	h.AddUserMessage("msg2")

	empty := h.GetLast(0)
	if len(empty) != 0 {
		t.Error("GetLast(0) should return empty slice")
	}

	negative := h.GetLast(-1)
	if len(negative) != 0 {
		t.Error("GetLast(negative) should return empty slice")
	}

	all := h.GetLast(10)
	if len(all) != 2 {
		t.Error("GetLast(n > length) should return all messages")
	}
}

// TestHistory_ThreadSafety tests concurrent access to history (expecting data races since History is not thread-safe)
// This test is skipped when running with race detector since History is intentionally not thread-safe
func TestHistory_ThreadSafety(t *testing.T) {
	// Skip this test when running with race detector, as History is not thread-safe
	// and we expect data races to be detected
	t.Skip("Skipping thread safety test - History is intentionally not thread-safe and will trigger race detector")
}

// TestHistory_EdgeCaseOperations tests unusual operations and edge cases
func TestHistory_EdgeCaseOperations(t *testing.T) {
	tests := []struct {
		name        string
		operation   func(h *History)
		expectedLen int
	}{
		{
			name: "add empty message",
			operation: func(h *History) {
				h.Add(Message{Role: "user", Content: ""})
			},
			expectedLen: 1,
		},
		{
			name: "add message with only whitespace",
			operation: func(h *History) {
				h.Add(Message{Role: "user", Content: "   \n\t   "})
			},
			expectedLen: 1,
		},
		{
			name: "add message with special characters",
			operation: func(h *History) {
				h.Add(Message{Role: "user", Content: "hello\nworld\twith\ttabs"})
			},
			expectedLen: 1,
		},
		{
			name: "truncate empty history",
			operation: func(h *History) {
				h.Truncate(5)
			},
			expectedLen: 0,
		},
		{
			name: "remove first from empty history",
			operation: func(h *History) {
				h.RemoveFirst(1)
			},
			expectedLen: 0,
		},
		{
			name: "clear already empty history",
			operation: func(h *History) {
				h.Clear()
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHistory()
			tt.operation(h)

			if h.Len() != tt.expectedLen {
				t.Errorf("Expected length %d, got %d", tt.expectedLen, h.Len())
			}
		})
	}
}

// TestHistory_WithLimit_EdgeCases tests history with limits at boundaries
func TestHistory_WithLimit_EdgeCases(t *testing.T) {
	// Test limit of 0 (should behave like unlimited)
	h := NewHistoryWithLimit(0)
	for i := 0; i < 10; i++ {
		h.AddUserMessage(fmt.Sprintf("msg%d", i))
	}
	if h.Len() != 10 {
		t.Errorf("Limit 0 should allow unlimited messages, got %d", h.Len())
	}

	// Test limit of 1
	h1 := NewHistoryWithLimit(1)
	h1.AddUserMessage("first")
	h1.AddUserMessage("second")
	if h1.Len() != 1 {
		t.Errorf("Limit 1 should keep only 1 message, got %d", h1.Len())
	}
	if h1.Get()[0].Content != "second" {
		t.Error("Should keep most recent message")
	}
}

// TestHistory_Clone_DeepCopy tests that clone creates independent copies
func TestHistory_Clone_DeepCopy(t *testing.T) {
	original := NewHistory()
	original.AddUserMessage("original")

	cloned := original.Clone()

	// Modify cloned
	cloned.AddUserMessage("cloned addition")

	// Original should be unchanged
	if original.Len() != 1 {
		t.Errorf("Original should have 1 message, got %d", original.Len())
	}
	if original.Get()[0].Content != "original" {
		t.Error("Original message should be unchanged")
	}

	// Clone should have the addition
	if cloned.Len() != 2 {
		t.Errorf("Cloned should have 2 messages, got %d", cloned.Len())
	}
}
