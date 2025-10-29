package dsgo

import "context"

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

// LM represents a language model interface
type LM interface {
	// Generate generates a response from the LM
	Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)

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

// NewGenerateOptions creates GenerateOptions with custom values
func NewGenerateOptions() *GenerateOptions {
	return DefaultGenerateOptions()
}
