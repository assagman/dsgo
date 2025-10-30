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

func (m *MockLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
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

func TestGenerateOptions_Copy(t *testing.T) {
	original := &GenerateOptions{
		Temperature:      0.8,
		MaxTokens:        1024,
		TopP:             0.9,
		Stop:             []string{"STOP", "END"},
		ResponseFormat:   "json",
		ToolChoice:       "auto",
		Stream:           true,
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
		Tools: []Tool{
			{Name: "tool1", Description: "Test tool 1"},
			{Name: "tool2", Description: "Test tool 2"},
		},
	}

	copied := original.Copy()

	// Verify all fields are copied
	if copied.Temperature != original.Temperature {
		t.Errorf("Temperature not copied correctly: got %v, want %v", copied.Temperature, original.Temperature)
	}
	if copied.MaxTokens != original.MaxTokens {
		t.Errorf("MaxTokens not copied correctly: got %v, want %v", copied.MaxTokens, original.MaxTokens)
	}
	if copied.TopP != original.TopP {
		t.Errorf("TopP not copied correctly: got %v, want %v", copied.TopP, original.TopP)
	}
	if copied.ResponseFormat != original.ResponseFormat {
		t.Errorf("ResponseFormat not copied correctly: got %v, want %v", copied.ResponseFormat, original.ResponseFormat)
	}
	if copied.ToolChoice != original.ToolChoice {
		t.Errorf("ToolChoice not copied correctly: got %v, want %v", copied.ToolChoice, original.ToolChoice)
	}
	if copied.Stream != original.Stream {
		t.Errorf("Stream not copied correctly: got %v, want %v", copied.Stream, original.Stream)
	}
	if copied.FrequencyPenalty != original.FrequencyPenalty {
		t.Errorf("FrequencyPenalty not copied correctly: got %v, want %v", copied.FrequencyPenalty, original.FrequencyPenalty)
	}
	if copied.PresencePenalty != original.PresencePenalty {
		t.Errorf("PresencePenalty not copied correctly: got %v, want %v", copied.PresencePenalty, original.PresencePenalty)
	}

	// Verify slices are deep copied (not same memory address)
	if len(copied.Stop) != len(original.Stop) {
		t.Errorf("Stop slice length not copied correctly: got %v, want %v", len(copied.Stop), len(original.Stop))
	}
	if len(copied.Tools) != len(original.Tools) {
		t.Errorf("Tools slice length not copied correctly: got %v, want %v", len(copied.Tools), len(original.Tools))
	}

	// Verify modifying the copy doesn't affect the original
	copied.Stop[0] = "MODIFIED"
	if original.Stop[0] == "MODIFIED" {
		t.Error("Modifying copied Stop slice affected original")
	}

	copied.Tools[0].Name = "modified"
	if original.Tools[0].Name == "modified" {
		t.Error("Modifying copied Tools slice affected original")
	}
}

func TestGenerateOptions_Copy_Nil(t *testing.T) {
	var opts *GenerateOptions
	copied := opts.Copy()
	if copied != nil {
		t.Errorf("Copy of nil should return nil, got %v", copied)
	}
}

func TestGenerateOptions_Copy_EmptySlices(t *testing.T) {
	original := &GenerateOptions{
		Temperature: 0.7,
		Stop:        nil,
		Tools:       nil,
	}

	copied := original.Copy()

	if copied.Stop != nil {
		t.Errorf("Expected nil Stop slice, got %v", copied.Stop)
	}
	if copied.Tools != nil {
		t.Errorf("Expected nil Tools slice, got %v", copied.Tools)
	}
}

func TestGenerateOptions_Copy_StreamCallback(t *testing.T) {
	options := DefaultGenerateOptions()
	options.StreamCallback = func(chunk Chunk) {
		// Mock callback
	}

	copied := options.Copy()
	if copied == nil {
		t.Fatal("Expected copy to be non-nil")
	}

	if copied.Temperature != options.Temperature {
		t.Error("Temperature not copied correctly")
	}
	if copied.MaxTokens != options.MaxTokens {
		t.Error("MaxTokens not copied correctly")
	}
}

func TestDefaultGenerateOptions(t *testing.T) {
	opts := DefaultGenerateOptions()

	if opts == nil {
		t.Fatal("DefaultGenerateOptions should not return nil")
	}
	if opts.Temperature != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %v", opts.Temperature)
	}
	if opts.MaxTokens != 2048 {
		t.Errorf("Expected default max tokens 2048, got %v", opts.MaxTokens)
	}
	if opts.TopP != 1.0 {
		t.Errorf("Expected default TopP 1.0, got %v", opts.TopP)
	}
	if opts.ResponseFormat != "text" {
		t.Errorf("Expected default response format 'text', got '%s'", opts.ResponseFormat)
	}
	if opts.ToolChoice != "auto" {
		t.Errorf("Expected default tool choice 'auto', got '%s'", opts.ToolChoice)
	}
	if opts.Stream != false {
		t.Errorf("Expected default stream false, got %v", opts.Stream)
	}
}
