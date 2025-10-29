package dsgo

import "testing"

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
