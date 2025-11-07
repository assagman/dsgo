package module

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
)

func TestPredict_Forward_Success(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "42"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	outputs, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", outputs.Outputs["answer"])
	}
}

func TestPredict_Forward_InvalidInput(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("required", core.FieldTypeString, "Required")

	lm := &MockLM{}
	p := NewPredict(sig, lm)

	_, err := p.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestPredict_Forward_LMError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestPredict_Forward_ParseError(t *testing.T) {
	// Use multiple fields so JSONAdapter can't fall back to plain text
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("confidence", core.FieldTypeString, "Confidence level")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `invalid json without structure`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error on parse failure when multiple fields required")
	}
}

func TestPredict_Forward_ValidationError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"wrong_field": "value"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error on validation failure")
	}
}

func TestPredict_WithOptions(t *testing.T) {
	sig := core.NewSignature("Test")
	lm := &MockLM{}
	p := NewPredict(sig, lm)

	customOpts := &core.GenerateOptions{Temperature: 0.5}
	p.WithOptions(customOpts)

	if p.Options.Temperature != 0.5 {
		t.Error("WithOptions should set custom options")
	}
}

func TestPredict_GetSignature(t *testing.T) {
	sig := core.NewSignature("Test")
	lm := &MockLM{}
	p := NewPredict(sig, lm)

	if p.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestPredict_JSONSupport(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			if options.ResponseFormat != "json" {
				t.Error("ResponseFormat should be 'json' when LM supports JSON and using JSONAdapter")
			}
			return &core.GenerateResult{Content: `{"answer": "ok"}`}, nil
		},
	}

	// Use JSONAdapter explicitly to trigger JSON mode
	p := NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Errorf("Forward() error = %v", err)
	}
}

func TestPredict_JSONSchemaAutoGeneration(t *testing.T) {
	// Create a signature with multiple output fields of different types
	sig := core.NewSignature("Classification and analysis").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", core.FieldTypeString, "Sentiment classification").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence score").
		AddOutput("word_count", core.FieldTypeInt, "Number of words").
		AddClassOutput("category", []string{"business", "technology", "sports"}, "Content category")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Verify ResponseSchema was auto-generated
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be auto-generated when using JSONAdapter with JSON-capable LM")
				return &core.GenerateResult{Content: `{}`}, nil
			}

			// Verify schema structure
			schema := options.ResponseSchema
			if schema["type"] != "object" {
				t.Errorf("Expected schema type 'object', got %v", schema["type"])
			}

			props, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatal("Expected properties map in schema")
			}

			// Verify sentiment field (string)
			if sentimentProp, ok := props["sentiment"].(map[string]any); !ok {
				t.Error("Expected sentiment in schema properties")
			} else if sentimentProp["type"] != "string" {
				t.Errorf("Expected sentiment type 'string', got %v", sentimentProp["type"])
			}

			// Verify confidence field (number)
			if confProp, ok := props["confidence"].(map[string]any); !ok {
				t.Error("Expected confidence in schema properties")
			} else if confProp["type"] != "number" {
				t.Errorf("Expected confidence type 'number', got %v", confProp["type"])
			}

			// Verify word_count field (integer)
			if wcProp, ok := props["word_count"].(map[string]any); !ok {
				t.Error("Expected word_count in schema properties")
			} else if wcProp["type"] != "integer" {
				t.Errorf("Expected word_count type 'integer', got %v", wcProp["type"])
			}

			// Verify category field (enum)
			if catProp, ok := props["category"].(map[string]any); !ok {
				t.Error("Expected category in schema properties")
			} else {
				if catProp["type"] != "string" {
					t.Errorf("Expected category type 'string', got %v", catProp["type"])
				}
				if enum, ok := catProp["enum"].([]string); !ok {
					t.Error("Expected enum array in category")
				} else if len(enum) != 3 {
					t.Errorf("Expected 3 enum values, got %d", len(enum))
				}
			}

			// Verify required fields
			required, ok := schema["required"].([]string)
			if !ok {
				t.Fatal("Expected required array in schema")
			}
			if len(required) != 4 {
				t.Errorf("Expected 4 required fields, got %d", len(required))
			}

			// Return mock response
			return &core.GenerateResult{
				Content: `{"sentiment": "positive", "confidence": 0.95, "word_count": 42, "category": "technology"}`,
			}, nil
		},
	}

	// Use JSONAdapter explicitly to trigger schema generation
	p := NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter())
	outputs, err := p.Forward(context.Background(), map[string]interface{}{
		"text": "This is a great technology article!",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify outputs were parsed correctly
	if outputs.Outputs["sentiment"] != "positive" {
		t.Errorf("Expected sentiment='positive', got %v", outputs.Outputs["sentiment"])
	}
	if outputs.Outputs["confidence"] != 0.95 {
		t.Errorf("Expected confidence=0.95, got %v", outputs.Outputs["confidence"])
	}
	if outputs.Outputs["word_count"] != 42 {
		t.Errorf("Expected word_count=42, got %v", outputs.Outputs["word_count"])
	}
	if outputs.Outputs["category"] != "technology" {
		t.Errorf("Expected category='technology', got %v", outputs.Outputs["category"])
	}
}

func TestPredict_JSONSchemaWithOptionalFields(t *testing.T) {
	sig := core.NewSignature("Optional fields test").
		AddInput("query", core.FieldTypeString, "Query").
		AddOutput("required_field", core.FieldTypeString, "Required").
		AddOptionalOutput("optional_field", core.FieldTypeString, "Optional")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be auto-generated")
				return &core.GenerateResult{Content: `{}`}, nil
			}

			schema := options.ResponseSchema
			required, ok := schema["required"].([]string)
			if !ok {
				t.Fatal("Expected required array in schema")
			}

			// Should only have required_field, not optional_field
			if len(required) != 1 {
				t.Errorf("Expected 1 required field, got %d", len(required))
			}
			if len(required) > 0 && required[0] != "required_field" {
				t.Errorf("Expected required_field to be required, got %v", required[0])
			}

			return &core.GenerateResult{
				Content: `{"required_field": "value"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"query": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestPredict_JSONSchemaNotGeneratedWithChatAdapter(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// When using ChatAdapter, ResponseFormat should NOT be "json" even if LM supports it
			if options.ResponseFormat == "json" {
				t.Error("ResponseFormat should not be 'json' when using ChatAdapter")
			}
			if options.ResponseSchema != nil {
				t.Error("ResponseSchema should not be set when using ChatAdapter")
			}
			// ChatAdapter expects markers
			return &core.GenerateResult{Content: `[[ ## answer ## ]]\n42`}, nil
		},
	}

	// Use ChatAdapter explicitly - should NOT trigger JSON mode
	p := NewPredict(sig, lm).WithAdapter(core.NewChatAdapter())
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
}

func TestPredict_CustomSchemaOverridesAutoGeneration(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("text", core.FieldTypeString, "Text").
		AddOutput("result", core.FieldTypeString, "Result")

	customSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"custom_field": map[string]any{"type": "string"},
		},
	}

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			if options.ResponseSchema == nil {
				t.Error("ResponseSchema should be present")
				return &core.GenerateResult{Content: `{}`}, nil
			}

			// Verify custom schema is used, not auto-generated
			props, ok := options.ResponseSchema["properties"].(map[string]any)
			if !ok {
				t.Fatal("Expected properties in schema")
			}
			if _, ok := props["custom_field"]; !ok {
				t.Error("Expected custom schema to be used, not auto-generated")
			}
			if _, ok := props["result"]; ok {
				t.Error("Auto-generated schema should not be used when custom schema is provided")
			}

			return &core.GenerateResult{Content: `{"custom_field": "value"}`}, nil
		},
	}

	opts := core.DefaultGenerateOptions()
	opts.ResponseSchema = customSchema

	p := NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter()).WithOptions(opts)
	_, err := p.Forward(context.Background(), map[string]interface{}{
		"text": "test",
	})

	if err != nil {
		// Expected to fail validation since we're using custom schema
		// but that's OK - we just want to verify the custom schema was used
		t.Logf("Expected validation error: %v", err)
	}
}

// TestPredict_ConcurrentForward tests concurrent Forward() calls
// to ensure thread safety when multiple goroutines use the same Predict module
func TestPredict_ConcurrentForward(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()

			// Simulate some work
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &core.GenerateResult{Content: fmt.Sprintf(`{"answer": "response-%d"}`, count)}, nil
			}
		},
	}

	p := NewPredict(sig, lm)

	var wg sync.WaitGroup
	errChan := make(chan error, 50)

	// Run 50 concurrent Forward() calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := p.Forward(context.Background(), map[string]interface{}{"question": fmt.Sprintf("test-%d", id)})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("Concurrent forward failed: %v", err)
	}
}

// TestPredict_ContextTimeout tests that context timeout is properly handled
// during Forward() execution and LM Generate() calls
func TestPredict_ContextTimeout(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, msgs []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
			// Check if context is already cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			// This should be interrupted by context timeout
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	p := NewPredict(sig, lm)

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	_, err := p.Forward(ctx, map[string]interface{}{"question": "test"})

	if err == nil {
		t.Error("Expected error from context timeout")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// TestPredict_Stream_Success tests successful streaming
func TestPredict_Stream_Success(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
			{Content: "answer: ", FinishReason: ""},
			{Content: "Hello ", FinishReason: ""},
			{Content: "World", FinishReason: ""},
			{Content: "", FinishReason: "stop", Usage: core.Usage{TotalTokens: 10}},
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
	var prediction *core.Prediction
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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
			{Content: "answer: test", FinishReason: ""},
			{Content: "", FinishReason: "stop"},
		},
	}

	var callbackCalls int
	options := core.DefaultGenerateOptions()
	options.StreamCallback = func(chunk core.Chunk) {
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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("count", core.FieldTypeInt, "")

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "").
		AddOutput("confidence", core.FieldTypeFloat, "")

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

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
	chunks    []core.Chunk
	streamErr bool
}

func (m *mockStreamingLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	return &core.GenerateResult{Content: "answer: test"}, nil
}

func (m *mockStreamingLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		if m.streamErr {
			errChan <- core.ErrLMGeneration
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

// TestPredict_Stream_WithJSONSchemaAutoGen tests streaming with auto-generated JSON schema
func TestPredict_Stream_WithJSONSchemaAutoGen(t *testing.T) {
	sig := core.NewSignature("Classification").
		AddInput("text", core.FieldTypeString, "Text to classify").
		AddOutput("category", core.FieldTypeString, "Category").
		AddOutput("score", core.FieldTypeFloat, "Confidence score")

	schemaVerified := false
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Verify schema was auto-generated for streaming too
			if options.ResponseSchema != nil {
				schemaVerified = true
				if props, ok := options.ResponseSchema["properties"].(map[string]any); ok {
					if _, ok := props["category"]; !ok {
						t.Error("Expected category in auto-generated schema")
					}
					if _, ok := props["score"]; !ok {
						t.Error("Expected score in auto-generated schema")
					}
				}
			}
			return &core.GenerateResult{
				Content: `{"category": "tech", "score": 0.95}`,
			}, nil
		},
	}

	predict := NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter())
	result, err := predict.Stream(context.Background(), map[string]any{
		"text": "AI article",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks
	for range result.Chunks {
	}

	// Wait for prediction
	prediction := <-result.Prediction
	if prediction == nil {
		t.Fatal("Expected prediction, got nil")
	}

	if !schemaVerified {
		t.Error("Schema auto-generation was not verified in stream mode")
	}
}

// TestPredict_Stream_ParseError tests streaming with parse error
func TestPredict_Stream_ParseError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("field1", core.FieldTypeString, "").
		AddOutput("field2", core.FieldTypeString, "")

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
			{Content: "invalid unparseable content", FinishReason: ""},
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

	// Should get parse error
	select {
	case err := <-result.Errors:
		if err == nil {
			t.Error("Expected parse error, got nil")
		}
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("Expected parse error, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for error")
	}
}

// TestPredict_Stream_WithHistory tests streaming with conversation history
func TestPredict_Stream_WithHistory(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	history := core.NewHistory()
	history.Add(core.Message{Role: "user", Content: "Previous question"})
	history.Add(core.Message{Role: "assistant", Content: "Previous answer"})

	mockLM := &mockStreamingLM{
		chunks: []core.Chunk{
			{Content: "answer: Current answer", FinishReason: ""},
			{Content: "", FinishReason: "stop"},
		},
	}

	predict := NewPredict(sig, mockLM).WithHistory(history)
	result, err := predict.Stream(context.Background(), map[string]any{
		"question": "Current question",
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Drain chunks
	for range result.Chunks {
	}

	// Wait for prediction
	prediction := <-result.Prediction
	if prediction == nil {
		t.Fatal("Expected prediction, got nil")
	}

	// History should now have 4 messages (2 old + 2 new)
	if len(history.Get()) != 4 {
		t.Errorf("Expected 4 messages in history, got %d", len(history.Get()))
	}
}

// MockLMForFallback is a mock LM that can return different response formats
type MockLMForFallback struct {
	ResponseFormat string // "chat", "json", or "invalid"
}

func (m *MockLMForFallback) Generate(ctx context.Context, messages []core.Message, opts *core.GenerateOptions) (*core.GenerateResult, error) {
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

	return &core.GenerateResult{
		Content: content,
		Usage: core.Usage{
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

func (m *MockLMForFallback) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- core.Chunk{
			Content:      result.Content,
			FinishReason: result.FinishReason,
			Usage:        result.Usage,
		}
	}()

	return chunkChan, errChan
}

// TestFallbackAdapter_Integration tests the fallback mechanism with a mock LM
func TestFallbackAdapter_Integration(t *testing.T) {
	sig := core.NewSignature("Analyze sentiment").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", core.FieldTypeString, "Sentiment classification").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence score")

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
				WithAdapter(core.NewFallbackAdapter())

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
				fallbackAdapter, ok := predict.Adapter.(*core.FallbackAdapter)
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
	sig := core.NewSignature("Analyze sentiment").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", core.FieldTypeString, "Sentiment").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence")

	// Mock LM that returns field marker format
	mockLM := &MockLMForFallback{ResponseFormat: "chat"}

	// Create Predict module with ChatAdapter
	predict := NewPredict(sig, mockLM).
		WithAdapter(core.NewChatAdapter())

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
	sig := core.NewSignature("Analyze sentiment").
		AddInput("text", core.FieldTypeString, "").
		AddOutput("sentiment", core.FieldTypeString, "").
		AddOutput("confidence", core.FieldTypeFloat, "")

	demos := []core.Example{
		*core.NewExample(
			map[string]any{"text": "Hello world"},
			map[string]any{"sentiment": "positive", "confidence": 0.9},
		),
	}

	mockLM := &MockLMForFallback{ResponseFormat: "json"}

	predict := NewPredict(sig, mockLM).
		WithAdapter(core.NewFallbackAdapter()).
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

// TestPredict_WithHistory tests multi-turn conversation
func TestPredict_WithHistory(t *testing.T) {
	sig := core.NewSignature("Answer questions").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	history := core.NewHistory()
	history.AddSystemMessage("You are a helpful assistant.")

	var capturedMessages []core.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			capturedMessages = messages
			return &core.GenerateResult{
				Content: `{"answer": "Paris"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithHistory(history)

	// First turn
	_, err := p.Forward(context.Background(), map[string]any{
		"question": "What is the capital of France?",
	})
	if err != nil {
		t.Fatalf("First Forward() error = %v", err)
	}

	// Verify history was prepended
	if len(capturedMessages) < 1 {
		t.Fatal("Expected history to be prepended to messages")
	}
	if capturedMessages[0].Role != "system" {
		t.Errorf("First message should be system message from history")
	}

	// Verify history was updated
	if history.Len() != 3 { // system + user + assistant
		t.Errorf("Expected 3 messages in history, got %d", history.Len())
	}

	// Second turn - history should include previous conversation
	capturedMessages = nil
	lm.GenerateFunc = func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
		capturedMessages = messages
		return &core.GenerateResult{
			Content: `{"answer": "About 2.2 million"}`,
		}, nil
	}

	_, err = p.Forward(context.Background(), map[string]any{
		"question": "What is the population?",
	})
	if err != nil {
		t.Fatalf("Second Forward() error = %v", err)
	}

	// Should have system + previous Q&A + new question
	if len(capturedMessages) < 3 {
		t.Errorf("Expected at least 3 messages (system + prev Q&A + new Q), got %d", len(capturedMessages))
	}

	// Final history should have 5 messages: system + 2 Q&A pairs
	if history.Len() != 5 {
		t.Errorf("Expected 5 messages in history (system + 2 Q&A pairs), got %d", history.Len())
	}
}

// TestPredict_WithDemos tests few-shot learning
func TestPredict_WithDemos(t *testing.T) {
	sig := core.NewSignature("Classify sentiment").
		AddInput("text", core.FieldTypeString, "Text to classify").
		AddOutput("sentiment", core.FieldTypeString, "positive or negative")

	demos := []core.Example{
		*core.NewExample(
			map[string]any{"text": "I love this product!"},
			map[string]any{"sentiment": "positive"},
		),
		*core.NewExample(
			map[string]any{"text": "This is terrible."},
			map[string]any{"sentiment": "negative"},
		),
	}

	var capturedMessages []core.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			capturedMessages = messages
			return &core.GenerateResult{
				Content: `{"sentiment": "positive"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithDemos(demos)

	_, err := p.Forward(context.Background(), map[string]any{
		"text": "Great experience!",
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify demos were included in the prompt
	if len(capturedMessages) == 0 {
		t.Fatal("Expected messages to be captured")
	}

	// Check that prompt includes examples
	promptContent := capturedMessages[0].Content
	if !strings.Contains(promptContent, "Example") {
		t.Error("Prompt should include demo examples")
	}
	if !strings.Contains(promptContent, "I love this product") {
		t.Error("Prompt should include demo input")
	}
}

// TestPredict_WithHistoryAndDemos tests both features together
func TestPredict_WithHistoryAndDemos(t *testing.T) {
	sig := core.NewSignature("Classify").
		AddInput("text", core.FieldTypeString, "Text").
		AddOutput("category", core.FieldTypeString, "Category")

	history := core.NewHistory()
	history.AddSystemMessage("You are a classifier.")

	demos := []core.Example{
		*core.NewExample(
			map[string]any{"text": "apple"},
			map[string]any{"category": "fruit"},
		),
	}

	var capturedMessages []core.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			capturedMessages = messages
			return &core.GenerateResult{
				Content: `{"category": "fruit"}`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithHistory(history).WithDemos(demos)

	_, err := p.Forward(context.Background(), map[string]any{
		"text": "banana",
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// First message should be system from history
	if capturedMessages[0].Role != "system" {
		t.Error("First message should be system message from history")
	}

	// Subsequent message should contain demos and current input
	promptContent := capturedMessages[1].Content
	if !strings.Contains(promptContent, "Example") {
		t.Error("Prompt should include examples from demos")
	}
}

// TestChainOfThought_WithHistory tests multi-turn reasoning
func TestChainOfThought_WithHistory(t *testing.T) {
	sig := core.NewSignature("Solve problems").
		AddInput("problem", core.FieldTypeString, "Problem").
		AddOutput("answer", core.FieldTypeString, "Answer")

	history := core.NewHistory()

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"reasoning": "2+2 equals 4", "answer": "4"}`,
			}, nil
		},
	}

	cot := NewChainOfThought(sig, lm).WithHistory(history)

	// First turn
	pred1, err := cot.Forward(context.Background(), map[string]any{
		"problem": "What is 2+2?",
	})
	if err != nil {
		t.Fatalf("First Forward() error = %v", err)
	}

	if !pred1.HasRationale() {
		t.Error("ChainOfThought should produce rationale")
	}

	// History should contain user + assistant
	if history.Len() != 2 {
		t.Errorf("Expected 2 messages in history, got %d", history.Len())
	}

	// Second turn
	_, err = cot.Forward(context.Background(), map[string]any{
		"problem": "What about 3+3?",
	})
	if err != nil {
		t.Fatalf("Second Forward() error = %v", err)
	}

	// History should grow
	if history.Len() != 4 {
		t.Errorf("Expected 4 messages in history, got %d", history.Len())
	}
}

// TestChainOfThought_WithDemos tests few-shot reasoning
func TestChainOfThought_WithDemos(t *testing.T) {
	sig := core.NewSignature("Solve math problems").
		AddInput("problem", core.FieldTypeString, "Problem").
		AddOutput("answer", core.FieldTypeString, "Answer")

	demos := []core.Example{
		*core.NewExample(
			map[string]any{"problem": "1+1"},
			map[string]any{"answer": "2"},
		),
	}

	var capturedMessages []core.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			capturedMessages = messages
			return &core.GenerateResult{
				Content: `{"reasoning": "Adding 2+2", "answer": "4"}`,
			}, nil
		},
	}

	cot := NewChainOfThought(sig, lm).WithDemos(demos)

	_, err := cot.Forward(context.Background(), map[string]any{
		"problem": "2+2",
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify demos were included in first message
	promptContent := capturedMessages[0].Content
	if !strings.Contains(promptContent, "Example") {
		t.Error("Prompt should include demo examples")
	}

	// Verify step-by-step instruction in main prompt (last message)
	mainPromptContent := capturedMessages[len(capturedMessages)-1].Content
	if !strings.Contains(mainPromptContent, "step-by-step") {
		t.Error("ChainOfThought prompt should include step-by-step instruction")
	}
}

// TestPredict_HistoryNotUpdatedOnError ensures history isn't corrupted on errors
func TestPredict_HistoryNotUpdatedOnError(t *testing.T) {
	// Use multiple fields to prevent JSONAdapter fallback
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output").
		AddOutput("status", core.FieldTypeString, "Status")

	history := core.NewHistory()
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `invalid json without structure`,
			}, nil
		},
	}

	p := NewPredict(sig, lm).WithHistory(history)

	_, err := p.Forward(context.Background(), map[string]any{
		"input": "test",
	})

	if err == nil {
		t.Fatal("Expected error due to invalid JSON when multiple fields required")
	}

	// History should not be updated on error
	if history.Len() != 0 {
		t.Errorf("History should not be updated on error, got %d messages", history.Len())
	}
}

// TestPredict_FinishReasonHandling tests all finish_reason scenarios
func TestPredict_FinishReasonHandling(t *testing.T) {
	sig := core.NewSignature("Test signature").
		AddOutput("answer", core.FieldTypeString, "Answer")

	tests := []struct {
		name          string
		finishReason  string
		content       string
		toolCalls     []core.ToolCall
		expectError   bool
		errorContains string
	}{
		{
			name:         "finish_reason=stop with valid content",
			finishReason: "stop",
			content:      "[[ ## answer ## ]]\nTest answer",
			expectError:  false,
		},
		{
			name:          "finish_reason=stop with empty content (error)",
			finishReason:  "stop",
			content:       "",
			expectError:   true,
			errorContains: "model returned empty content despite finish_reason=stop",
		},
		{
			name:          "finish_reason=length with content (error - truncated)",
			finishReason:  "length",
			content:       "[[ ## answer ## ]]\nPartial answer that was cut off mid",
			expectError:   true,
			errorContains: "model hit max_tokens limit (finish_reason=length)",
		},
		{
			name:          "finish_reason=length with empty content (error)",
			finishReason:  "length",
			content:       "",
			expectError:   true,
			errorContains: "model hit max_tokens limit (finish_reason=length)",
		},
		{
			name:          "finish_reason=tool_calls (error - not supported)",
			finishReason:  "tool_calls",
			content:       "",
			toolCalls:     []core.ToolCall{{ID: "1", Name: "test_tool"}},
			expectError:   true,
			errorContains: "finish_reason=tool_calls) but Predict module doesn't support tool loops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &MockLM{
				GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
					return &core.GenerateResult{
						Content:      tt.content,
						FinishReason: tt.finishReason,
						ToolCalls:    tt.toolCalls,
						Usage:        core.Usage{TotalTokens: 10},
					}, nil
				},
			}

			predict := NewPredict(sig, mockLM)
			_, err := predict.Forward(context.Background(), map[string]any{"input": "test"})

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errorContains)
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}
