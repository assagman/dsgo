package core

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"
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

// RetryConfig contains configuration for HTTP retry behavior
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts (default: 3)
	InitialBackoff time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 30s)
	JitterFactor   float64       // Jitter factor for randomization (default: 0.1)
}

// GenerateOptions contains options for LM generation
type GenerateOptions struct {
	Temperature      float64
	MaxTokens        int
	TopP             float64
	Stop             []string
	ResponseFormat   string         // "text" or "json"
	ResponseSchema   map[string]any // Optional JSON schema for structured outputs
	Tools            []Tool
	ToolChoice       string // "auto", "none", or specific tool name
	Stream           bool
	StreamCallback   StreamCallback `json:"-"` // Optional callback for each streaming chunk
	FrequencyPenalty float64
	PresencePenalty  float64
	ProviderParams   map[string]any // Provider-specific request fields forwarded verbatim by providers that support it
	RetryConfig      *RetryConfig   // Optional retry configuration (nil uses defaults)
}

// GenerateResult represents the result of an LM generation
type GenerateResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        Usage
	Metadata     map[string]any // Provider-specific metadata (cache headers, rate limits, etc.)
	CacheHit     bool           // True if this result was served from cache (DSPy parity)
}

// ToolCall represents a tool call made by the LM
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]interface{}
}

// Usage represents token usage and cost statistics
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             float64 // Total cost in USD
	Latency          int64   // Latency in milliseconds
}

// Chunk represents a streaming response chunk from the LM
type Chunk struct {
	Content      string     // Incremental content delta (cleaned of internal markers by default)
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

	// IsOpenAI indicates if the LM is OpenAI or OpenAI-compatible
	// This is used to determine if OpenAI-specific JSON schema requirements apply
	IsOpenAI() bool
}

// DefaultGenerateOptions returns default generation options
// Respects environment variables (DSGO_* preferred over EXAMPLES_*):
// - DSGO_MAX_TOKENS or EXAMPLES_MAX_TOKENS (default: 10000)
// - DSGO_TEMPERATURE or EXAMPLES_TEMPERATURE (default: 0.7)
func DefaultGenerateOptions() *GenerateOptions {
	maxTokens := 10000 // Default max tokens
	temperature := 0.7

	// Prefer DSGO_* overrides, fallback to EXAMPLES_*
	if v := getEnv("DSGO_MAX_TOKENS"); v != "" {
		if parsed := parseInt(v); parsed > 0 {
			maxTokens = parsed
		}
	} else if v := getEnv("EXAMPLES_MAX_TOKENS"); v != "" {
		if parsed := parseInt(v); parsed > 0 {
			maxTokens = parsed
		}
	}

	if v := getEnv("DSGO_TEMPERATURE"); v != "" {
		if parsed := parseFloat(v); parsed >= 0 {
			temperature = parsed
		}
	} else if v := getEnv("EXAMPLES_TEMPERATURE"); v != "" {
		if parsed := parseFloat(v); parsed >= 0 {
			temperature = parsed
		}
	}

	return &GenerateOptions{
		Temperature:      temperature,
		MaxTokens:        maxTokens,
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
		ResponseSchema:   o.ResponseSchema, // Copy reference (schema is read-only)
		ToolChoice:       o.ToolChoice,
		Stream:           o.Stream,
		StreamCallback:   o.StreamCallback, // Copy reference (function pointer)
		FrequencyPenalty: o.FrequencyPenalty,
		PresencePenalty:  o.PresencePenalty,
		ProviderParams:   deepCopyMap(o.ProviderParams), // Deep copy provider params
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

	// Copy RetryConfig
	if o.RetryConfig != nil {
		copied.RetryConfig = &RetryConfig{
			MaxRetries:     o.RetryConfig.MaxRetries,
			InitialBackoff: o.RetryConfig.InitialBackoff,
			MaxBackoff:     o.RetryConfig.MaxBackoff,
			JitterFactor:   o.RetryConfig.JitterFactor,
		}
	}

	return copied
}

// Helper functions for environment variable parsing
func getEnv(key string) string {
	return os.Getenv(key)
}

func parseInt(s string) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return 0
}

func parseFloat(s string) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return 0
}
