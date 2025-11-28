package core

import "time"

// HistoryEntry represents a rich structured event for LM interactions
type HistoryEntry struct {
	ID        string    `json:"id"`         // UUID for this call
	Timestamp time.Time `json:"timestamp"`  // Call timestamp
	SessionID string    `json:"session_id"` // Conversation session identifier

	// Provider and model info
	Provider string `json:"provider"` // "openrouter", "openai", etc.
	Model    string `json:"model"`    // Model identifier

	// Request metadata
	Request RequestMeta `json:"request"`

	// Response metadata
	Response ResponseMeta `json:"response"`

	// Usage and cost
	Usage Usage `json:"usage"`

	// Cache metadata
	Cache CacheMeta `json:"cache"`

	// Provider-specific metadata (request IDs, rate limits, headers, etc.)
	ProviderMeta map[string]any `json:"provider_meta,omitempty"`

	// Error details (if failed)
	Error *ErrorMeta `json:"error,omitempty"`
}

// RequestMeta contains metadata about the request
type RequestMeta struct {
	Messages       []Message        `json:"messages"`
	Options        *GenerateOptions `json:"options,omitempty"`
	PromptLength   int              `json:"prompt_length"`   // Character count
	MessageCount   int              `json:"message_count"`   // Number of messages
	HasTools       bool             `json:"has_tools"`       // Whether tools were provided
	ToolCount      int              `json:"tool_count"`      // Number of tools
	ResponseFormat string           `json:"response_format"` // "text" or "json"
}

// ResponseMeta contains metadata about the response
type ResponseMeta struct {
	Content        string     `json:"content"`
	ToolCalls      []ToolCall `json:"tool_calls,omitempty"`
	FinishReason   string     `json:"finish_reason"`
	ResponseLength int        `json:"response_length"` // Character count
	ToolCallCount  int        `json:"tool_call_count"` // Number of tool calls
}

// CacheMeta contains cache-related metadata
type CacheMeta struct {
	Hit    bool   `json:"hit"`              // Whether this was a cache hit
	Source string `json:"source,omitempty"` // Cache source ("memory", "disk", "provider")
	TTL    int64  `json:"ttl,omitempty"`    // Time-to-live in seconds
}

// ErrorMeta contains error details if the call failed
type ErrorMeta struct {
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"`
	Type       string `json:"type,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

// Collector is the interface for collecting history entries
type Collector interface {
	// Collect records a history entry
	Collect(entry *HistoryEntry) error

	// Close closes the collector and flushes any pending entries
	Close() error
}
