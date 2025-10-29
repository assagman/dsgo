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
