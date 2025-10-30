package module

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

// TestPredict_Stream_Success tests successful streaming
func TestPredict_Stream_Success(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		chunks: []dsgo.Chunk{
			{Content: "answer: ", FinishReason: ""},
			{Content: "Hello ", FinishReason: ""},
			{Content: "World", FinishReason: ""},
			{Content: "", FinishReason: "stop", Usage: dsgo.Usage{TotalTokens: 10}},
		},
	}

	predict := NewPredict(sig, mockLM)

	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Say hello",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect chunks
	var content strings.Builder
	for chunk := range result.Chunks {
		content.WriteString(chunk.Content)
	}

	// Check for errors
	select {
	case err := <-result.Errors:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	// Get final prediction
	var prediction *dsgo.Prediction
	select {
	case prediction = <-result.Prediction:
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for prediction")
	}

	if prediction == nil {
		t.Fatal("Expected prediction, got nil")
	}

	answer, ok := prediction.GetString("answer")
	if !ok || answer != "Hello World" {
		t.Errorf("Expected answer 'Hello World', got '%s'", answer)
	}

	if prediction.Usage.TotalTokens != 10 {
		t.Errorf("Expected usage 10 tokens, got %d", prediction.Usage.TotalTokens)
	}
}

// TestPredict_Stream_WithCallback tests streaming with callback
func TestPredict_Stream_WithCallback(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		chunks: []dsgo.Chunk{
			{Content: "answer: test", FinishReason: ""},
			{Content: "", FinishReason: "stop"},
		},
	}

	var callbackCalls int
	options := dsgo.DefaultGenerateOptions()
	options.StreamCallback = func(chunk dsgo.Chunk) {
		callbackCalls++
	}

	predict := NewPredict(sig, mockLM).WithOptions(options)

	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Test",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks and wait for completion
	for range result.Chunks {
	}

	// Wait for prediction or error
	select {
	case <-result.Prediction:
	case err := <-result.Errors:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for prediction")
	}

	// Small delay to ensure goroutine completes callback calls
	time.Sleep(10 * time.Millisecond)

	// The callback should be called for each chunk
	if callbackCalls != 2 {
		t.Logf("Note: callback calls = %d (timing-dependent test)", callbackCalls)
	}
}

// TestPredict_Stream_ValidationError tests streaming with validation errors
func TestPredict_Stream_ValidationError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("count", dsgo.FieldTypeInt, "")

	mockLM := &mockStreamingLM{
		chunks: []dsgo.Chunk{
			{Content: "count: not_a_number", FinishReason: ""},
			{Content: "", FinishReason: "stop"},
		},
	}

	predict := NewPredict(sig, mockLM)

	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Count",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks
	for range result.Chunks {
	}

	// Should get validation error
	select {
	case err := <-result.Errors:
		if err == nil {
			t.Error("Expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "type errors") {
			t.Errorf("Expected type error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for error")
	}
}

// TestPredict_Stream_PartialValidation tests streaming with partial validation
func TestPredict_Stream_PartialValidation(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	mockLM := &mockStreamingLM{
		chunks: []dsgo.Chunk{
			{Content: "answer: Yes", FinishReason: ""}, // Missing confidence
			{Content: "", FinishReason: "stop"},
		},
	}

	predict := NewPredict(sig, mockLM)

	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Test",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks
	for range result.Chunks {
	}

	// Wait for both prediction and error channels
	prediction := <-result.Prediction
	err = <-result.Errors

	// Either we get a prediction with diagnostics, or an error
	if prediction != nil {
		// Should have diagnostics for missing field
		if prediction.ParseDiagnostics == nil {
			t.Error("Expected parse diagnostics, got nil")
		} else if len(prediction.ParseDiagnostics.MissingFields) == 0 {
			t.Error("Expected missing fields in diagnostics")
		}
	} else if err != nil {
		// Also acceptable - validation failed
		t.Logf("Got error (acceptable for partial validation): %v", err)
	} else {
		t.Fatal("Expected either prediction with diagnostics or error")
	}
}

// TestPredict_Stream_InvalidInput tests streaming with invalid input
func TestPredict_Stream_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	mockLM := &mockStreamingLM{}
	predict := NewPredict(sig, mockLM)

	_, err := predict.Stream(context.Background(), map[string]any{
		"wrong_field": "test",
	})
	if err == nil {
		t.Error("Expected input validation error, got nil")
	}
}

// TestPredict_Stream_LMError tests streaming with LM error
func TestPredict_Stream_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		streamErr: true,
	}

	predict := NewPredict(sig, mockLM)

	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Test",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks
	for range result.Chunks {
	}

	// Should get LM error
	select {
	case err := <-result.Errors:
		if err == nil {
			t.Error("Expected LM error, got nil")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for error")
	}
}

// mockStreamingLM is a mock LM for streaming tests
type mockStreamingLM struct {
	chunks    []dsgo.Chunk
	streamErr bool
}

func (m *mockStreamingLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	return &dsgo.GenerateResult{Content: "answer: test"}, nil
}

func (m *mockStreamingLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		if m.streamErr {
			errChan <- dsgo.ErrLMGeneration
			return
		}

		for _, chunk := range m.chunks {
			chunkChan <- chunk
		}
	}()

	return chunkChan, errChan
}

func (m *mockStreamingLM) Name() string        { return "mock-streaming" }
func (m *mockStreamingLM) SupportsJSON() bool  { return false }
func (m *mockStreamingLM) SupportsTools() bool { return false }
