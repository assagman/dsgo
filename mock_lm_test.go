package dsgo

import (
	"context"
	"encoding/json"
	"testing"
)

// MockLM is a mock language model for testing
type MockLM struct {
	GenerateFunc     func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)
	NameValue        string
	SupportsJSONVal  bool
	SupportsToolsVal bool
}

func (m *MockLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, messages, options)
	}
	return &GenerateResult{Content: "{}"}, nil
}

func (m *MockLM) Name() string {
	if m.NameValue != "" {
		return m.NameValue
	}
	return "mock-lm"
}

func (m *MockLM) SupportsJSON() bool {
	return m.SupportsJSONVal
}

func (m *MockLM) SupportsTools() bool {
	return m.SupportsToolsVal
}

// NewMockLM creates a new mock LM with default behavior
func NewMockLM() *MockLM {
	return &MockLM{
		NameValue:        "mock-lm",
		SupportsJSONVal:  true,
		SupportsToolsVal: false,
	}
}

// WithJSONResponse configures the mock to return a JSON response
func (m *MockLM) WithJSONResponse(data map[string]interface{}) *MockLM {
	m.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		jsonBytes, _ := json.Marshal(data)
		return &GenerateResult{
			Content: string(jsonBytes),
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}, nil
	}
	return m
}

// WithError configures the mock to return an error
func (m *MockLM) WithError(err error) *MockLM {
	m.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		return nil, err
	}
	return m
}

// WithTextResponse configures the mock to return a text response
func (m *MockLM) WithTextResponse(text string) *MockLM {
	m.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		return &GenerateResult{Content: text}, nil
	}
	return m
}

// WithToolCalls configures the mock to return tool calls
func (m *MockLM) WithToolCalls(toolCalls []ToolCall) *MockLM {
	m.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		return &GenerateResult{
			Content:   "Let me use a tool",
			ToolCalls: toolCalls,
		}, nil
	}
	return m
}

func TestDefaultGenerateOptions(t *testing.T) {
	opts := DefaultGenerateOptions()
	if opts == nil {
		t.Fatal("DefaultGenerateOptions should not return nil")
	}
	if opts.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", opts.Temperature)
	}
	if opts.MaxTokens != 2048 {
		t.Errorf("Expected max tokens 2048, got %d", opts.MaxTokens)
	}
}

func TestNewGenerateOptions(t *testing.T) {
	opts := NewGenerateOptions()
	if opts == nil {
		t.Fatal("NewGenerateOptions should not return nil")
	}
	if opts.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", opts.Temperature)
	}
}
