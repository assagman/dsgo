package module

import (
	"context"
	"testing"

	"github.com/assagman/dsgo"
)

// MockLMForFallback is a mock LM that can return different response formats
type MockLMForFallback struct {
	ResponseFormat string // "chat", "json", or "invalid"
}

func (m *MockLMForFallback) Generate(ctx context.Context, messages []dsgo.Message, opts *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	// Simulate different response formats based on configuration
	var content string

	switch m.ResponseFormat {
	case "chat":
		// Return response using field markers (ChatAdapter format)
		content = `[[ ## sentiment ## ]]
positive

[[ ## confidence ## ]]
0.95`

	case "json":
		// Return response in JSON format (JSONAdapter format)
		content = `{"sentiment": "positive", "confidence": 0.95}`

	default:
		// Invalid format that neither adapter can parse
		content = "The sentiment is positive with high confidence"
	}

	return &dsgo.GenerateResult{
		Content: content,
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *MockLMForFallback) Name() string {
	return "mock-fallback"
}

func (m *MockLMForFallback) SupportsJSON() bool {
	return false
}

func (m *MockLMForFallback) SupportsTools() bool {
	return false
}

func (m *MockLMForFallback) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- dsgo.Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
}

// TestFallbackAdapter_Integration tests the fallback mechanism with a mock LM
func TestFallbackAdapter_Integration(t *testing.T) {
	sig := dsgo.NewSignature("Analyze sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", dsgo.FieldTypeString, "Sentiment classification").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

	tests := []struct {
		name           string
		responseFormat string
		expectSuccess  bool
		expectAdapter  int // 0=ChatAdapter, 1=JSONAdapter
	}{
		{
			name:           "ChatAdapter succeeds",
			responseFormat: "chat",
			expectSuccess:  true,
			expectAdapter:  0,
		},
		{
			name:           "Fallback to JSONAdapter",
			responseFormat: "json",
			expectSuccess:  true,
			expectAdapter:  1,
		},
		{
			name:           "All adapters fail",
			responseFormat: "invalid",
			expectSuccess:  false,
			expectAdapter:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock LM with specific response format
			mockLM := &MockLMForFallback{ResponseFormat: tt.responseFormat}

			// Create Predict module with FallbackAdapter
			predict := NewPredict(sig, mockLM).
				WithAdapter(dsgo.NewFallbackAdapter())

			// Execute prediction
			inputs := map[string]any{
				"text": "This product is amazing!",
			}

			result, err := predict.Forward(context.Background(), inputs)

			if tt.expectSuccess {
				if err != nil {
					t.Fatalf("Expected success, got error: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}

				// Verify outputs
				sentiment, ok := result.GetString("sentiment")
				if !ok || sentiment != "positive" {
					t.Errorf("Expected sentiment='positive', got %v (ok=%v)", sentiment, ok)
				}

				confidence, ok := result.GetFloat("confidence")
				if !ok || confidence != 0.95 {
					t.Errorf("Expected confidence=0.95, got %v (ok=%v)", confidence, ok)
				}

				// Verify which adapter was used
				fallbackAdapter, ok := predict.Adapter.(*dsgo.FallbackAdapter)
				if !ok {
					t.Fatal("Expected FallbackAdapter")
				}
				if fallbackAdapter.GetLastUsedAdapter() != tt.expectAdapter {
					t.Errorf("Expected adapter %d to be used, got %d",
						tt.expectAdapter, fallbackAdapter.GetLastUsedAdapter())
				}
			} else {
				if err == nil {
					t.Fatal("Expected error when all adapters fail, got nil")
				}
			}
		})
	}
}

// TestChatAdapter_Integration tests ChatAdapter with a mock LM
func TestChatAdapter_Integration(t *testing.T) {
	sig := dsgo.NewSignature("Analyze sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", dsgo.FieldTypeString, "Sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence")

	// Mock LM that returns field marker format
	mockLM := &MockLMForFallback{ResponseFormat: "chat"}

	// Create Predict module with ChatAdapter
	predict := NewPredict(sig, mockLM).
		WithAdapter(dsgo.NewChatAdapter())

	inputs := map[string]any{
		"text": "This product is amazing!",
	}

	result, err := predict.Forward(context.Background(), inputs)

	// This test demonstrates that ChatAdapter works end-to-end
	if err != nil {
		t.Fatalf("ChatAdapter integration failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	sentiment, ok := result.GetString("sentiment")
	if !ok || sentiment != "positive" {
		t.Errorf("Expected sentiment='positive', got %v (ok=%v)", sentiment, ok)
	}
}

// TestFallbackAdapter_WithDemos tests fallback with few-shot examples
func TestFallbackAdapter_WithDemos(t *testing.T) {
	sig := dsgo.NewSignature("Analyze sentiment").
		AddInput("text", dsgo.FieldTypeString, "").
		AddOutput("sentiment", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{"text": "Hello world"},
			map[string]any{"sentiment": "positive", "confidence": 0.9},
		),
	}

	mockLM := &MockLMForFallback{ResponseFormat: "json"}

	predict := NewPredict(sig, mockLM).
		WithAdapter(dsgo.NewFallbackAdapter()).
		WithDemos(demos)

	inputs := map[string]any{
		"text": "Hi there!",
	}

	result, err := predict.Forward(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Forward with demos failed: %v", err)
	}

	// Verify that demos were formatted correctly and result is valid
	// (they should be in ChatAdapter format since it's first in fallback chain)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	sentiment, ok := result.GetString("sentiment")
	if !ok || sentiment != "positive" {
		t.Errorf("Expected sentiment='positive', got %v (ok=%v)", sentiment, ok)
	}
}
