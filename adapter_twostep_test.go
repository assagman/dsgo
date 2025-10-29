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
