package dsgo

import (
	"context"
	"errors"
)

// Common errors
var (
	ErrLMGeneration = errors.New("LM generation failed")
)

// StreamCallback is called for each chunk during streaming
type StreamCallback func(Chunk)

// Message represents a single message in a conversation
type Message struct {
	Role      string // "system", "user", "assistant", "tool"
	Content   string
	ToolID    string     // For tool responses
	ToolCalls []ToolCall // For assistant messages with tool calls
}

// GenerateOptions contains options for LM generation
type GenerateOptions struct {
	Temperature      float64
	MaxTokens        int
	TopP             float64
	Stop             []string
	ResponseFormat   string // "text" or "json"
	Tools            []Tool
	ToolChoice       string // "auto", "none", or specific tool name
	Stream           bool
	StreamCallback   StreamCallback // Optional callback for each streaming chunk
	FrequencyPenalty float64
	PresencePenalty  float64
}

// GenerateResult represents the result of an LM generation
type GenerateResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        Usage
}

// ToolCall represents a tool call made by the LM
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Chunk represents a streaming response chunk from the LM
type Chunk struct {
	Content      string     // Incremental content delta
	ToolCalls    []ToolCall // Incremental tool call deltas
	FinishReason string     // Set when stream ends ("stop", "length", "tool_calls", etc.)
	Usage        Usage      // Token usage (typically only set in final chunk)
}

// LM represents a language model interface
type LM interface {
	// Generate generates a response from the LM
	Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)

	// Stream generates a streaming response from the LM
	// Returns a channel that emits chunks and an error channel
	// The chunk channel will be closed when the stream completes
	// If an error occurs, it will be sent to the error channel
	Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error)

	// Name returns the name/identifier of the LM
	Name() string

	// SupportsJSON indicates if the LM supports native JSON mode
	SupportsJSON() bool

	// SupportsTools indicates if the LM supports tool/function calling
	SupportsTools() bool
}

// DefaultGenerateOptions returns default generation options
func DefaultGenerateOptions() *GenerateOptions {
	return &GenerateOptions{
		Temperature:      0.7,
		MaxTokens:        2048,
		TopP:             1.0,
		Stop:             []string{},
		ResponseFormat:   "text",
		Tools:            []Tool{},
		ToolChoice:       "auto",
		Stream:           false,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
	}
}

// Copy creates a deep copy of GenerateOptions
func (o *GenerateOptions) Copy() *GenerateOptions {
	if o == nil {
		return nil
	}

	copied := &GenerateOptions{
		Temperature:      o.Temperature,
		MaxTokens:        o.MaxTokens,
		TopP:             o.TopP,
		ResponseFormat:   o.ResponseFormat,
		ToolChoice:       o.ToolChoice,
		Stream:           o.Stream,
		FrequencyPenalty: o.FrequencyPenalty,
		PresencePenalty:  o.PresencePenalty,
	}

	// Copy slices
	if o.Stop != nil {
		copied.Stop = make([]string, len(o.Stop))
		copy(copied.Stop, o.Stop)
	}

	if o.Tools != nil {
		copied.Tools = make([]Tool, len(o.Tools))
		copy(copied.Tools, o.Tools)
	}

	return copied
}
