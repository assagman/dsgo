package module

import (
	"context"

	"github.com/assagman/dsgo"
)

type MockLM struct {
	GenerateFunc     func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error)
	NameValue        string
	SupportsJSONVal  bool
	SupportsToolsVal bool
}

func (m *MockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, messages, options)
	}
	return &dsgo.GenerateResult{Content: "{}"}, nil
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

func (m *MockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		// Generate response using GenerateFunc
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		// Send content as a single chunk
		chunkChan <- dsgo.Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
}
