package dsgo

// History manages conversation history for multi-turn interactions
type History struct {
	messages []Message
	maxSize  int // 0 = unlimited
}

// NewHistory creates a new conversation history
func NewHistory() *History {
	return &History{
		messages: []Message{},
		maxSize:  0,
	}
}

// NewHistoryWithLimit creates a history with a maximum size
func NewHistoryWithLimit(maxSize int) *History {
	return &History{
		messages: []Message{},
		maxSize:  maxSize,
	}
}

// Add appends a message to the history
func (h *History) Add(message Message) {
	h.messages = append(h.messages, message)
	
	// Trim if exceeds max size (keep most recent)
	if h.maxSize > 0 && len(h.messages) > h.maxSize {
		h.messages = h.messages[len(h.messages)-h.maxSize:]
	}
}

// AddUserMessage adds a user message to history
func (h *History) AddUserMessage(content string) {
	h.Add(Message{Role: "user", Content: content})
}

// AddAssistantMessage adds an assistant message to history
func (h *History) AddAssistantMessage(content string) {
	h.Add(Message{Role: "assistant", Content: content})
}

// AddSystemMessage adds a system message to history
func (h *History) AddSystemMessage(content string) {
	h.Add(Message{Role: "system", Content: content})
}

// Get returns all messages in the history
func (h *History) Get() []Message {
	return h.messages
}

// GetLast returns the last n messages from history
func (h *History) GetLast(n int) []Message {
	if n <= 0 || n >= len(h.messages) {
		return h.messages
	}
	return h.messages[len(h.messages)-n:]
}

// Clear removes all messages from history
func (h *History) Clear() {
	h.messages = []Message{}
}

// Len returns the number of messages in history
func (h *History) Len() int {
	return len(h.messages)
}

// IsEmpty returns true if history has no messages
func (h *History) IsEmpty() bool {
	return len(h.messages) == 0
}

// Clone creates a deep copy of the history
func (h *History) Clone() *History {
	cloned := &History{
		messages: make([]Message, len(h.messages)),
		maxSize:  h.maxSize,
	}
	copy(cloned.messages, h.messages)
	return cloned
}

// Truncate keeps only the first n messages
func (h *History) Truncate(n int) {
	if n > 0 && n < len(h.messages) {
		h.messages = h.messages[:n]
	}
}

// RemoveFirst removes the first n messages
func (h *History) RemoveFirst(n int) {
	if n > 0 && n < len(h.messages) {
		h.messages = h.messages[n:]
	} else if n >= len(h.messages) {
		h.Clear()
	}
}
