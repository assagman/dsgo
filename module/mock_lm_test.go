package module

import (
	"context"

	"github.com/assagman/dsgo/core"
)

type MockLM struct {
	GenerateFunc     func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error)
	NameValue        string
	SupportsJSONVal  bool
	SupportsToolsVal bool
}

func (m *MockLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, messages, options)
	}
	return &core.GenerateResult{Content: "{}"}, nil
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

func (m *MockLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 1)
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
		chunkChan <- core.Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
}
