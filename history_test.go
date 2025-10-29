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
