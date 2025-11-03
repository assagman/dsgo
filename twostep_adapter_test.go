package dsgo

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Mock LM for testing TwoStepAdapter extraction
type mockExtractionLM struct {
	response string
	err      error
}

func (m *mockExtractionLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &GenerateResult{
		Content: m.response,
		Usage:   Usage{},
	}, nil
}

func (m *mockExtractionLM) Name() string {
	return "mock-extraction"
}

func (m *mockExtractionLM) SupportsJSON() bool {
	return true
}

func (m *mockExtractionLM) SupportsTools() bool {
	return false
}

func (m *mockExtractionLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
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

// TestTwoStepAdapter_Format tests the formatting of stage 1 (free-form) prompts
func TestTwoStepAdapter_Format(t *testing.T) {
	adapter := NewTwoStepAdapter(nil) // No extraction LM needed for Format
	sig := NewSignature("Analyze sentiment").
		AddInput("text", FieldTypeString, "Text to analyze").
		AddOutput("sentiment", FieldTypeString, "Sentiment classification").
		AddOutput("confidence", FieldTypeFloat, "Confidence score")

	inputs := map[string]any{
		"text": "I love this product!",
	}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	content := messages[0].Content

	// Should include description
	if !strings.Contains(content, "Analyze sentiment") {
		t.Errorf("Expected description in content")
	}

	// Should include natural response instruction (not structured)
	if !strings.Contains(content, "natural response") {
		t.Errorf("Expected natural response instruction")
	}

	// Should include input value
	if !strings.Contains(content, "I love this product!") {
		t.Errorf("Expected input value in content")
	}

	// Should mention expected outputs WITHOUT forcing structure
	if !strings.Contains(content, "sentiment") {
		t.Errorf("Expected 'sentiment' mentioned in guidance")
	}
	if !strings.Contains(content, "confidence") {
		t.Errorf("Expected 'confidence' mentioned in guidance")
	}

	// Should NOT have strict JSON formatting requirement (that's stage 2)
	if strings.Contains(content, "ONLY valid JSON") {
		t.Errorf("Stage 1 should not require JSON format")
	}
}

// TestTwoStepAdapter_FormatWithDemos tests demo formatting
func TestTwoStepAdapter_FormatWithDemos(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)
	sig := NewSignature("Classify").
		AddInput("text", FieldTypeString, "").
		AddOutput("category", FieldTypeString, "")

	demos := []Example{
		*NewExample(
			map[string]any{"text": "Great service!"},
			map[string]any{"category": "positive"},
		),
	}

	inputs := map[string]any{"text": "Good product"}
	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	content := messages[0].Content
	if !strings.Contains(content, "Examples") {
		t.Errorf("Expected examples section")
	}
	if !strings.Contains(content, "Great service!") {
		t.Errorf("Expected demo input in content")
	}
}

// TestTwoStepAdapter_Parse tests the two-stage extraction process
func TestTwoStepAdapter_Parse(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("sentiment", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionResult string
		extractionError  error
		expected         map[string]any
		wantErr          bool
	}{
		{
			name: "Successful extraction",
			freeFormResponse: "This text has a positive sentiment. I'm quite confident about this assessment, " +
				"I'd say around 0.95 confidence level.",
			extractionResult: `{"sentiment": "positive", "confidence": 0.95}`,
			expected:         map[string]any{"sentiment": "positive", "confidence": 0.95},
			wantErr:          false,
		},
		{
			name: "Extraction with reasoning",
			freeFormResponse: "After careful analysis, this appears to be negative. " +
				"The confidence is moderate at 0.7.",
			extractionResult: `{
				"reasoning": "Analyzed word choice and tone",
				"sentiment": "negative",
				"confidence": 0.7
			}`,
			expected: map[string]any{
				"reasoning":  "Analyzed word choice and tone",
				"sentiment":  "negative",
				"confidence": 0.7,
			},
			wantErr: false,
		},
		{
			name:             "Extraction LM failure",
			freeFormResponse: "Some response",
			extractionError:  fmt.Errorf("LM API error"),
			wantErr:          true,
		},
		{
			name:             "Invalid extraction JSON",
			freeFormResponse: "Some response",
			extractionResult: "not json",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{
				response: tt.extractionResult,
				err:      tt.extractionError,
			}
			adapter := NewTwoStepAdapter(mockLM).WithReasoning(true)

			outputs, err := adapter.Parse(sig, tt.freeFormResponse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for key, expectedVal := range tt.expected {
					actualVal, ok := outputs[key]
					if !ok {
						t.Errorf("Missing expected output key: %s", key)
						continue
					}
					if fmt.Sprintf("%v", actualVal) != fmt.Sprintf("%v", expectedVal) {
						t.Errorf("For key %s: expected %v, got %v", key, expectedVal, actualVal)
					}
				}
			}
		})
	}
}

// TestTwoStepAdapter_ParseWithoutExtractionLM tests error when no extraction LM provided
func TestTwoStepAdapter_ParseWithoutExtractionLM(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)
	sig := NewSignature("test").AddOutput("answer", FieldTypeString, "")

	_, err := adapter.Parse(sig, "some response")
	if err == nil {
		t.Error("Expected error when parsing without extraction LM")
	}
	if !strings.Contains(err.Error(), "extraction LM") {
		t.Errorf("Expected error about missing extraction LM, got: %v", err)
	}
}

// TestTwoStepAdapter_TypeCoercion tests that extraction maintains type coercion
func TestTwoStepAdapter_TypeCoercion(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "").
		AddOutput("active", FieldTypeBool, "")

	mockLM := &mockExtractionLM{
		response: `{"count": "42", "score": "0.95", "active": "true"}`,
	}
	adapter := NewTwoStepAdapter(mockLM)

	outputs, err := adapter.Parse(sig, "The count is 42, score is 0.95, active status is true")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify type coercion
	if count, ok := outputs["count"].(int); !ok || count != 42 {
		t.Errorf("Expected count to be int 42, got %v (%T)", outputs["count"], outputs["count"])
	}
	if score, ok := outputs["score"].(float64); !ok || score != 0.95 {
		t.Errorf("Expected score to be float64 0.95, got %v (%T)", outputs["score"], outputs["score"])
	}
	if active, ok := outputs["active"].(bool); !ok || !active {
		t.Errorf("Expected active to be bool true, got %v (%T)", outputs["active"], outputs["active"])
	}
}

// TestTwoStepAdapter_WithReasoning tests reasoning flag
func TestTwoStepAdapter_WithReasoning(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)

	// Default should include reasoning
	if !adapter.IncludeReasoning {
		t.Error("Expected reasoning to be enabled by default")
	}

	// Can disable
	adapter.WithReasoning(false)
	if adapter.IncludeReasoning {
		t.Error("Expected reasoning to be disabled")
	}

	// Can re-enable
	adapter.WithReasoning(true)
	if !adapter.IncludeReasoning {
		t.Error("Expected reasoning to be enabled")
	}
}

// TestTwoStepAdapter_Format_MultipleInputs tests formatting with multiple inputs
func TestTwoStepAdapter_Format_MultipleInputs(t *testing.T) {
	sig := NewSignature("Summarize").
		AddInput("title", FieldTypeString, "").
		AddInput("body", FieldTypeString, "").
		AddOutput("summary", FieldTypeString, "")

	adapter := NewTwoStepAdapter(nil)
	inputs := map[string]any{
		"title": "Important News",
		"body":  "This is the full article text...",
	}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestTwoStepAdapter_FormatHistory_Coverage tests TwoStepAdapter FormatHistory
func TestTwoStepAdapter_FormatHistory_Coverage(t *testing.T) {
	adapter := NewTwoStepAdapter(nil)

	history := NewHistory()
	history.Add(Message{Role: "user", Content: "Question 1"})
	history.Add(Message{Role: "assistant", Content: "Answer 1"})
	history.Add(Message{Role: "user", Content: "Question 2"})

	messages := adapter.FormatHistory(history)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	for i, msg := range messages {
		if msg.Role == "" {
			t.Errorf("Message %d has empty role", i)
		}
		if msg.Content == "" {
			t.Errorf("Message %d has empty content", i)
		}
	}
}

// TestTwoStepAdapter_Parse_ErrorCases tests various error scenarios in TwoStepAdapter.Parse
func TestTwoStepAdapter_Parse_ErrorCases(t *testing.T) {
	// Use multiple fields to prevent JSONAdapter fallback for single string fields
	sig := NewSignature("test").
		AddOutput("result", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionResult string
		extractionError  error
		wantErr          bool
		errContains      string
	}{
		{
			name:             "Extraction LM network error",
			freeFormResponse: "Some response",
			extractionError:  fmt.Errorf("connection refused"),
			wantErr:          true,
			errContains:      "extraction LM failed",
		},
		{
			name:             "Extraction LM returns malformed JSON",
			freeFormResponse: "Some response",
			extractionResult: "{invalid json",
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns empty response",
			freeFormResponse: "Some response",
			extractionResult: "",
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns non-JSON object",
			freeFormResponse: "Some response",
			extractionResult: `"just a string"`,
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
		{
			name:             "Extraction LM returns array instead of object",
			freeFormResponse: "Some response",
			extractionResult: `["item1", "item2"]`,
			wantErr:          true,
			errContains:      "failed to parse extraction result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{
				response: tt.extractionResult,
				err:      tt.extractionError,
			}
			adapter := NewTwoStepAdapter(mockLM)

			_, err := adapter.Parse(sig, tt.freeFormResponse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errContains, err)
			}
		})
	}
}

// TestTwoStepAdapter_Parse_TypeCoercion_EdgeCases tests type coercion edge cases
func TestTwoStepAdapter_Parse_TypeCoercion_EdgeCases(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "").
		AddOutput("flag", FieldTypeBool, "").
		AddOutput("text", FieldTypeString, "")

	tests := []struct {
		name             string
		freeFormResponse string
		extractionJSON   string
		expected         map[string]any
	}{
		{
			name:             "Numeric strings to numbers",
			freeFormResponse: "Count is 42, score is 3.14, flag is true",
			extractionJSON:   `{"count": "42", "score": "3.14", "flag": "true", "text": "hello"}`,
			expected:         map[string]any{"count": 42, "score": 3.14, "flag": true, "text": "hello"},
		},
		{
			name:             "Mixed quotes and whitespace",
			freeFormResponse: "Data with quotes",
			extractionJSON:   `{"count": " 42 ", "score": " 3.14\t", "flag": " false ", "text": " test "}`,
			expected:         map[string]any{"count": 42, "score": 3.14, "flag": false, "text": "test"}, // Note: some processing may trim whitespace
		},
		{
			name:             "Percentage strings",
			freeFormResponse: "95% confidence",
			extractionJSON:   `{"score": "95%"}`,
			expected:         map[string]any{"score": 95.0},
		},
		{
			name:             "Qualitative scores",
			freeFormResponse: "High confidence",
			extractionJSON:   `{"score": "high"}`,
			expected:         map[string]any{"score": 0.9}, // "high" maps to 0.9
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{response: tt.extractionJSON}
			adapter := NewTwoStepAdapter(mockLM)

			outputs, err := adapter.Parse(sig, tt.freeFormResponse)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			for key, expectedVal := range tt.expected {
				actualVal, ok := outputs[key]
				if !ok {
					t.Errorf("Missing expected output key: %s", key)
					continue
				}

				// Check type and value
				switch expected := expectedVal.(type) {
				case int:
					if actual, ok := actualVal.(int); !ok || actual != expected {
						t.Errorf("For key %s: expected int %d, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case float64:
					if actual, ok := actualVal.(float64); !ok || actual != expected {
						t.Errorf("For key %s: expected float64 %f, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case bool:
					if actual, ok := actualVal.(bool); !ok || actual != expected {
						t.Errorf("For key %s: expected bool %t, got %v (%T)", key, expected, actualVal, actualVal)
					}
				case string:
					if actual, ok := actualVal.(string); !ok || actual != expected {
						t.Errorf("For key %s: expected string %q, got %v (%T)", key, expected, actualVal, actualVal)
					}
				}
			}
		})
	}
}

// TestTwoStepAdapter_Parse_WithFallbackChains tests integration with fallback chains
func TestTwoStepAdapter_Parse_WithFallbackChains(t *testing.T) {
	sig := NewSignature("test").
		AddOutput("result", FieldTypeString, "")

	// Mock LM that fails on extraction
	failingLM := &mockExtractionLM{
		err: fmt.Errorf("extraction failed"),
	}

	// Test with TwoStepAdapter as part of a fallback chain
	fallbackAdapter := NewFallbackAdapterWithChain(
		NewTwoStepAdapter(failingLM), // This will fail
		NewJSONAdapter(),             // This should succeed
	)

	// Content that JSONAdapter can parse
	content := `{"result": "success"}`

	outputs, err := fallbackAdapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Fallback should have succeeded: %v", err)
	}

	if outputs["result"] != "success" {
		t.Errorf("Expected result='success', got %v", outputs["result"])
	}

	// Should have used the second adapter (JSONAdapter)
	if fallbackAdapter.GetLastUsedAdapter() != 1 {
		t.Errorf("Expected fallback to adapter 1, got %d", fallbackAdapter.GetLastUsedAdapter())
	}

	// Check metadata
	if outputs["__fallback_used"] != true {
		t.Errorf("Expected fallback_used=true, got %v", outputs["__fallback_used"])
	}
}
