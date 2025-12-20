package core

import (
	"context"
	"testing"
)

func ensureAPIKeyEnv(t *testing.T) {
	t.Helper()
	if hasAPIKeyEnv() {
		return
	}
	t.Setenv("DSGO_OPENAI_API_KEY", "test-openai-key")
}

// mockLM is a lightweight LM stub for tests that only need an LM instance.
type mockLM struct{}

func (m *mockLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	return &GenerateResult{Content: "mock"}, nil
}

func (m *mockLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk, 1)
	errChan := make(chan error, 1)
	close(chunkChan)
	close(errChan)
	return chunkChan, errChan
}

func (m *mockLM) Name() string {
	return "mock"
}

func (m *mockLM) SupportsJSON() bool {
	return true
}

func (m *mockLM) SupportsTools() bool {
	return false
}

func (m *mockLM) IsOpenAI() bool {
	return false
}
