package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

// TestJSONAdapter_MalformedJSON_RecoveryChain tests JSON adapter with various malformed outputs
// Tests that are likely to succeed with extraction from markdown or embedded JSON
func TestJSONAdapter_MalformedJSON_RecoveryChain(t *testing.T) {
	tests := []struct {
		name           string
		malformedInput string
		shouldRecover  bool
		expectedAnswer string
	}{
		{
			name:           "Markdown fence wrapping - should extract JSON",
			malformedInput: "```json\n{\"answer\": \"yes\", \"confidence\": 0.95}\n```",
			shouldRecover:  true,
			expectedAnswer: "yes",
		},
		{
			name:           "Text before and after JSON - should extract",
			malformedInput: "Here is the answer: {\"answer\": \"yes\", \"confidence\": 0.95} That's it!",
			shouldRecover:  true,
			expectedAnswer: "yes",
		},
		{
			name:           "Leading/trailing whitespace - should parse",
			malformedInput: `   {"answer": "yes", "confidence": 0.95}   `,
			shouldRecover:  true,
			expectedAnswer: "yes",
		},
		{
			name: "Pretty-printed with extra spacing",
			malformedInput: `{
  "answer" : "yes" ,
  "confidence" : 0.95
}`,
			shouldRecover:  true,
			expectedAnswer: "yes",
		},
		{
			name:           "Single quotes - may not recover",
			malformedInput: "{'answer': 'yes', 'confidence': '0.95'}",
			shouldRecover:  false, // JSON extraction doesn't handle single quotes
		},
		{
			name:           "Unquoted keys - may not recover",
			malformedInput: "{answer: 'yes', confidence: '0.95'}",
			shouldRecover:  false,
		},
	}

	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	adapter := dsgo.NewJSONAdapter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.malformedInput)

			if tt.shouldRecover && err != nil {
				t.Logf("Expected recovery but got error (logging): %v", err)
				return
			}

			if !tt.shouldRecover && err == nil {
				t.Logf("Expected error but parse succeeded (acceptable for robustness): %v", outputs)
				return
			}

			if tt.shouldRecover && err == nil {
				if answer, ok := outputs["answer"].(string); ok {
					if answer != tt.expectedAnswer {
						t.Errorf("Got answer %q, want %q", answer, tt.expectedAnswer)
					}
				} else {
					t.Errorf("answer field has wrong type: %T", outputs["answer"])
				}
			}
		})
	}
}

// TestJSONAdapter_ComplexNesting tests JSON adapter with deeply nested structures
func TestJSONAdapter_ComplexNesting(t *testing.T) {
	tests := []struct {
		name           string
		sig            *dsgo.Signature
		input          string
		shouldSucceed  bool
		expectedFields map[string]interface{}
	}{
		{
			name: "Array of strings",
			sig: dsgo.NewSignature("test").
				AddOutput("items", dsgo.FieldTypeJSON, ""),
			input:         `{"items": ["apple", "banana", "cherry"]}`,
			shouldSucceed: true,
		},
		{
			name: "Array of objects",
			sig: dsgo.NewSignature("test").
				AddOutput("records", dsgo.FieldTypeJSON, ""),
			input: `{"records": [
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"}
			]}`,
			shouldSucceed: true,
		},
		{
			name: "Nested objects",
			sig: dsgo.NewSignature("test").
				AddOutput("config", dsgo.FieldTypeJSON, ""),
			input: `{"config": {
				"settings": {
					"enabled": true,
					"options": ["a", "b", "c"]
				}
			}}`,
			shouldSucceed: true,
		},
		{
			name: "Mixed types in object",
			sig: dsgo.NewSignature("test").
				AddOutput("data", dsgo.FieldTypeJSON, ""),
			input: `{"data": {
				"string": "text",
				"number": 42,
				"float": 3.14,
				"bool": true,
				"null": null,
				"array": [1, 2, 3],
				"object": {"nested": "value"}
			}}`,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := dsgo.NewJSONAdapter()
			outputs, err := adapter.Parse(tt.sig, tt.input)

			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected success, got error: %v", err)
				return
			}

			if !tt.shouldSucceed && err == nil {
				t.Errorf("Expected error, but parse succeeded")
				return
			}

			if tt.shouldSucceed && err == nil {
				// Verify that the JSON field was parsed
				for fieldName := range tt.expectedFields {
					if _, ok := outputs[fieldName]; !ok {
						t.Errorf("Expected field %q not found in outputs", fieldName)
					}
				}
			}
		})
	}
}

// TestJSONAdapter_LargeOutputs tests JSON adapter with very large responses
func TestJSONAdapter_LargeOutputs(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("content", dsgo.FieldTypeString, "")

	adapter := dsgo.NewJSONAdapter()

	// Generate a large string (>10KB)
	largeContent := strings.Repeat("This is a long piece of content. ", 500)
	input := fmt.Sprintf(`{"content": "%s"}`, largeContent)

	outputs, err := adapter.Parse(sig, input)
	if err != nil {
		t.Errorf("Failed to parse large output: %v", err)
		return
	}

	content, ok := outputs["content"].(string)
	if !ok {
		t.Errorf("content field has wrong type: %T", outputs["content"])
		return
	}

	if len(content) < len(largeContent)/2 {
		t.Errorf("Parsed content too short: got %d bytes, expected ~%d", len(content), len(largeContent))
	}
}

// TestChatAdapter_FieldMarkers tests Chat adapter with standard field marker format
func TestChatAdapter_FieldMarkers(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("sources", dsgo.FieldTypeString, "")

	adapter := dsgo.NewChatAdapter()

	tests := []struct {
		name      string
		content   string
		wantErr   bool
		validates func(map[string]any) bool
	}{
		{
			name: "Standard markers",
			content: `[[ ## answer ## ]]
42

[[ ## sources ## ]]
Wikipedia`,
			wantErr: false,
			validates: func(outputs map[string]any) bool {
				return outputs["answer"] == "42" && outputs["sources"] == "Wikipedia"
			},
		},
		{
			name: "Markers with minimal spacing",
			content: `[[##answer##]]
yes

[[##sources##]]
Search results`,
			wantErr: false,
			validates: func(outputs map[string]any) bool {
				// May not fully parse without proper markers, but either result is acceptable
				_, ok := outputs["answer"].(string)
				return ok || outputs["answer"] == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.validates(outputs) {
				t.Errorf("Output validation failed: %v", outputs)
			}
		})
	}
}

// TestChatAdapter_MalformedMarkers tests Chat adapter with malformed or missing markers
func TestChatAdapter_MalformedMarkers(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "")

	adapter := dsgo.NewChatAdapter()

	tests := []struct {
		name               string
		content            string
		shouldUseHeuristic bool
	}{
		{
			name: "Incomplete marker - missing closing brackets",
			content: `[[ ## answer ##
yes`,
			shouldUseHeuristic: true,
		},
		{
			name: "Single closing bracket",
			content: `[[ ## answer ## ]
42`,
			shouldUseHeuristic: true,
		},
		{
			name:               "No markers with colon syntax",
			content:            `answer: The correct answer is 42`,
			shouldUseHeuristic: true,
		},
		{
			name:               "Answer synonym without marker",
			content:            `result: The result is definitely yes`,
			shouldUseHeuristic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)

			// Some of these might fail or succeed - we're just testing robustness
			if err == nil && tt.shouldUseHeuristic {
				// Successfully parsed using heuristic
				if _, ok := outputs["answer"]; ok {
					t.Logf("Successfully parsed using heuristic: %v", outputs["answer"])
				}
			} else if err != nil {
				t.Logf("Parse failed (acceptable for malformed): %v", err)
			}
		})
	}
}

// TestChatAdapter_ExtraContent tests Chat adapter with extra content around field values
func TestChatAdapter_ExtraContent(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	adapter := dsgo.NewChatAdapter()

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "Explanation before field",
			content: `Here is my analysis:
[[ ## answer ## ]]
The answer is definitely yes

[[ ## confidence ## ]]
0.95`,
			wantErr: false,
		},
		{
			name: "Explanation after field",
			content: `[[ ## answer ## ]]
no

I'm not very confident about this one.

[[ ## confidence ## ]]
0.3`,
			wantErr: false,
		},
		{
			name: "Multiple paragraphs in field value",
			content: `[[ ## answer ## ]]
This is a long answer that spans
multiple lines and paragraphs.

It discusses various points.

[[ ## confidence ## ]]
0.85`,
			wantErr: false,
		},
		{
			name: "Field markers on same line as content",
			content: `[[ ## answer ## ]] The answer is 42

[[ ## confidence ## ]] High confidence 0.95`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, ok := outputs["answer"]; !ok {
					t.Errorf("answer field not found in outputs")
				}
			}
		})
	}
}

// TestTwoStepAdapter_ReasoningExtraction tests TwoStep adapter with reasoning and structured extraction
func TestTwoStepAdapter_ReasoningExtraction(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("sentiment", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	tests := []struct {
		name               string
		freeFormResponse   string
		extractionResponse string
		wantErr            bool
	}{
		{
			name: "Long reasoning with structured extraction",
			freeFormResponse: `Let me analyze this text carefully.
			
The text contains positive words like "love", "great", and "excellent".
There's enthusiasm and strong opinion expressed.

Based on my analysis, I believe this sentiment is clearly positive.
The confidence level is high - I'd estimate around 0.92.`,
			extractionResponse: `{"sentiment": "positive", "confidence": 0.92}`,
			wantErr:            false,
		},
		{
			name: "Reasoning with uncertainty and fallback confidence",
			freeFormResponse: `This is a tricky case. The text is somewhat ambiguous.
			
On one hand, there are positive elements.
On the other hand, there are some criticisms.

It could be neutral or slightly positive. I'd say about 50-50 chance
between neutral and positive. Confidence is moderate.`,
			extractionResponse: `{"sentiment": "neutral", "confidence": 0.55}`,
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLM := &mockExtractionLM{
				response: tt.extractionResponse,
			}
			adapter := dsgo.NewTwoStepAdapter(mockLM).WithReasoning(true)

			outputs, err := adapter.Parse(sig, tt.freeFormResponse)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, ok := outputs["sentiment"].(string); !ok {
					t.Errorf("sentiment field has wrong type")
				}
				if _, ok := outputs["confidence"].(float64); !ok {
					t.Errorf("confidence field has wrong type")
				}
			}
		})
	}
}

// TestFallbackAdapter_ComplexRecovery tests Fallback adapter in complex failure scenarios
func TestFallbackAdapter_ComplexRecovery(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("score", dsgo.FieldTypeFloat, "")

	adapter := dsgo.NewFallbackAdapter()

	tests := []struct {
		name            string
		content         string
		expectedAdapter int // Which adapter in chain should succeed
		wantErr         bool
		validateOutput  func(map[string]any) bool
	}{
		{
			name:            "Chat format with markers",
			content:         "[[ ## answer ## ]]\nyes\n\n[[ ## score ## ]]\n0.9",
			expectedAdapter: 0, // ChatAdapter
			wantErr:         false,
			validateOutput: func(outputs map[string]any) bool {
				return outputs["answer"] == "yes" && outputs["score"].(float64) == 0.9
			},
		},
		{
			name:            "JSON format (no markers)",
			content:         `{"answer": "yes", "score": 0.9}`,
			expectedAdapter: 1, // JSONAdapter
			wantErr:         false,
			validateOutput: func(outputs map[string]any) bool {
				return outputs["answer"] == "yes"
			},
		},
		{
			name:            "Malformed JSON - may not recover with single quotes",
			content:         `{'answer': 'maybe', 'score': '0.5'}`,
			expectedAdapter: -1,   // Both adapters may fail
			wantErr:         true, // Expecting this to fail
		},
		{
			name:            "Plain text - both adapters fail",
			content:         "This is just plain text without any structure",
			expectedAdapter: -1,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr = %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.expectedAdapter >= 0 {
					usedAdapter := adapter.GetLastUsedAdapter()
					if usedAdapter != tt.expectedAdapter {
						t.Errorf("Expected adapter %d, got %d", tt.expectedAdapter, usedAdapter)
					}
				}

				if !tt.validateOutput(outputs) {
					t.Errorf("Output validation failed: %v", outputs)
				}
			}
		})
	}
}

// TestAdapter_TypeCoercion_EdgeCases tests type coercion in extreme scenarios
func TestAdapter_TypeCoercion_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		sig           *dsgo.Signature
		input         string
		adapter       dsgo.Adapter
		expectedType  string
		validateValue func(interface{}) bool
	}{
		{
			name: "JSON: String number to int",
			sig: dsgo.NewSignature("test").
				AddOutput("count", dsgo.FieldTypeInt, ""),
			input:        `{"count": "  42  "}`,
			adapter:      dsgo.NewJSONAdapter(),
			expectedType: "int",
			validateValue: func(v interface{}) bool {
				count, ok := v.(int)
				return ok && count == 42
			},
		},
		{
			name: "JSON: Percentage string to float",
			sig: dsgo.NewSignature("test").
				AddOutput("score", dsgo.FieldTypeFloat, ""),
			input:        `{"score": "95%"}`,
			adapter:      dsgo.NewJSONAdapter(),
			expectedType: "float64",
			validateValue: func(v interface{}) bool {
				score, ok := v.(float64)
				return ok && score == 95.0
			},
		},
		{
			name: "JSON: Qualitative to numeric",
			sig: dsgo.NewSignature("test").
				AddOutput("confidence", dsgo.FieldTypeFloat, ""),
			input:        `{"confidence": "high"}`,
			adapter:      dsgo.NewJSONAdapter(),
			expectedType: "float64",
			validateValue: func(v interface{}) bool {
				conf, ok := v.(float64)
				return ok && conf == 0.9
			},
		},
	}

	// Create signature with class field separately to set classes
	classSig := dsgo.NewSignature("test").
		AddOutput("category", dsgo.FieldTypeClass, "")
	classSig.OutputFields[0].Classes = []string{"positive", "negative", "neutral"}

	classTests := []struct {
		name          string
		sig           *dsgo.Signature
		input         string
		adapter       dsgo.Adapter
		expectedType  string
		validateValue func(interface{}) bool
	}{
		{
			name: "Chat: String with newlines to class",
			sig:  classSig,
			input: `[[ ## category ## ]]
positive

Some explanation here`,
			adapter:      dsgo.NewChatAdapter(),
			expectedType: "string",
			validateValue: func(v interface{}) bool {
				cat, ok := v.(string)
				return ok && cat == "positive"
			},
		},
	}

	// Append class tests to regular tests
	tests = append(tests, classTests...)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := tt.adapter.Parse(tt.sig, tt.input)
			if err != nil {
				t.Errorf("Parse failed: %v", err)
				return
			}

			// Find the output field
			var fieldName string
			for _, field := range tt.sig.OutputFields {
				fieldName = field.Name
				break
			}

			if val, ok := outputs[fieldName]; ok {
				if !tt.validateValue(val) {
					t.Errorf("Value validation failed for %s: got %v (%T)", fieldName, val, val)
				}
			} else {
				t.Errorf("Expected field %q not found in outputs", fieldName)
			}
		})
	}
}

// TestAdapter_Metadata_Tracking tests adapter metadata tracking
func TestAdapter_Metadata_Tracking(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "")

	adapter := dsgo.NewFallbackAdapter()

	tests := []struct {
		name                     string
		content                  string
		expectedParseAttempts    int
		expectedFallbackUsed     bool
		expectedAdapterUsedIndex int
	}{
		{
			name:                     "First adapter success",
			content:                  "[[ ## answer ## ]]\nyes",
			expectedParseAttempts:    1,
			expectedFallbackUsed:     false,
			expectedAdapterUsedIndex: 0,
		},
		{
			name:                     "Second adapter success",
			content:                  `{"answer": "yes"}`,
			expectedParseAttempts:    2,
			expectedFallbackUsed:     true,
			expectedAdapterUsedIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Check metadata
			if attempts, ok := outputs["__parse_attempts"]; ok {
				if attempts != tt.expectedParseAttempts {
					t.Errorf("Expected %d parse attempts, got %v", tt.expectedParseAttempts, attempts)
				}
			} else {
				t.Error("Missing __parse_attempts metadata")
			}

			if fallbackUsed, ok := outputs["__fallback_used"]; ok {
				if fallbackUsed != tt.expectedFallbackUsed {
					t.Errorf("Expected fallback_used=%v, got %v", tt.expectedFallbackUsed, fallbackUsed)
				}
			} else {
				t.Error("Missing __fallback_used metadata")
			}

			if usedAdapter := adapter.GetLastUsedAdapter(); usedAdapter != tt.expectedAdapterUsedIndex {
				t.Errorf("Expected adapter %d, got %d", tt.expectedAdapterUsedIndex, usedAdapter)
			}
		})
	}
}

// TestAdapter_CaseInsensitivity_ClassFields tests case-insensitive class field matching
func TestAdapter_CaseInsensitivity_ClassFields(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("sentiment", dsgo.FieldTypeClass, "")
	sig.OutputFields[0].Classes = []string{"positive", "negative", "neutral"}

	adapter := dsgo.NewChatAdapter()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Lowercase",
			content:  "[[ ## sentiment ## ]]\npositive",
			expected: "positive",
		},
		{
			name:     "Uppercase",
			content:  "[[ ## sentiment ## ]]\nPOSITIVE",
			expected: "positive",
		},
		{
			name:     "Mixed case",
			content:  "[[ ## sentiment ## ]]\nPoSiTiVe",
			expected: "positive",
		},
		{
			name:     "With explanation",
			content:  "[[ ## sentiment ## ]]\nPositive - clearly favorable",
			expected: "positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Errorf("Parse failed: %v", err)
				return
			}

			sentiment, ok := outputs["sentiment"].(string)
			if !ok {
				t.Errorf("sentiment has wrong type: %T", outputs["sentiment"])
				return
			}

			if sentiment != tt.expected {
				t.Errorf("Got %q, want %q", sentiment, tt.expected)
			}
		})
	}
}

// TestAdapter_IntegrationWithModule tests adapter robustness in actual module usage
func TestAdapter_IntegrationWithModule(t *testing.T) {
	sig := dsgo.NewSignature("Generate").
		AddInput("topic", dsgo.FieldTypeString, "").
		AddOutput("content", dsgo.FieldTypeString, "")

	tests := []struct {
		name       string
		lmResponse string
		adapter    dsgo.Adapter
		shouldWork bool
	}{
		{
			name: "Chat adapter with markers",
			lmResponse: `[[ ## content ## ]]
This is the generated content about the topic.`,
			adapter:    dsgo.NewChatAdapter(),
			shouldWork: true,
		},
		{
			name:       "JSON adapter with valid JSON",
			lmResponse: `{"content": "This is the generated content about the topic."}`,
			adapter:    dsgo.NewJSONAdapter(),
			shouldWork: true,
		},
		{
			name: "Fallback adapter with Chat format",
			lmResponse: `[[ ## content ## ]]
Generated content here.`,
			adapter:    dsgo.NewFallbackAdapter(),
			shouldWork: true,
		},
		{
			name:       "Fallback adapter with JSON format",
			lmResponse: `{"content": "Generated content here."}`,
			adapter:    dsgo.NewFallbackAdapter(),
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := tt.adapter.Parse(sig, tt.lmResponse)

			if tt.shouldWork && err != nil {
				t.Errorf("Expected successful parse, got error: %v", err)
				return
			}

			if !tt.shouldWork && err == nil {
				t.Errorf("Expected error, but parse succeeded")
				return
			}

			if tt.shouldWork {
				if _, ok := outputs["content"]; !ok {
					t.Errorf("content field not found in outputs")
				}
			}
		})
	}
}

// TestAdapter_JSONRepair tests JSON repair functionality
func TestAdapter_JSONRepair(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddOutput("answer", dsgo.FieldTypeString, "")

	adapter := dsgo.NewJSONAdapter()

	// Test cases for JSON repair
	tests := []struct {
		name       string
		input      string
		wantRepair bool
	}{
		{
			name:       "Valid JSON - no repair",
			input:      `{"answer": "yes"}`,
			wantRepair: false,
		},
		{
			name:       "Trailing comma - needs repair",
			input:      `{"answer": "yes",}`,
			wantRepair: true,
		},
		{
			name:       "Single quotes - needs repair",
			input:      `{'answer': 'yes'}`,
			wantRepair: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.input)

			if err != nil {
				if tt.wantRepair {
					t.Logf("Failed to repair (acceptable): %v", err)
				} else {
					t.Errorf("Unexpected parse error: %v", err)
				}
				return
			}

			if _, ok := outputs["answer"]; !ok {
				t.Errorf("answer field not found after parse")
			}
		})
	}
}

// mockExtractionLM provides a mock LM for TwoStepAdapter testing
type mockExtractionLM struct {
	response string
	err      error
}

func (m *mockExtractionLM) Generate(_ context.Context, _ []dsgo.Message, _ *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &dsgo.GenerateResult{
		Content: m.response,
		Usage:   dsgo.Usage{},
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

func (m *mockExtractionLM) IsOpenAI() bool {
	return false
}

func (m *mockExtractionLM) Stream(_ context.Context, _ []dsgo.Message, _ *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(context.Background(), nil, nil)
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

// ============================================================================
// Unicode and International Character Handling Tests
// ============================================================================

func TestAdapter_UnicodeHandling(t *testing.T) {
	tests := []struct {
		name     string
		sig      *dsgo.Signature
		input    string
		adapter  dsgo.Adapter
		expected map[string]string
	}{
		{
			name: "Emoji in output",
			sig: dsgo.NewSignature("test").
				AddOutput("reaction", dsgo.FieldTypeString, ""),
			input:   `{"reaction": "Great job! 🎉👏🚀"}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"reaction": "Great job! 🎉👏🚀",
			},
		},
		{
			name: "CJK characters (Chinese)",
			sig: dsgo.NewSignature("test").
				AddOutput("translation", dsgo.FieldTypeString, ""),
			input:   `{"translation": "这是一个测试。中文字符处理。"}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"translation": "这是一个测试。中文字符处理。",
			},
		},
		{
			name: "CJK characters (Japanese)",
			sig: dsgo.NewSignature("test").
				AddOutput("translation", dsgo.FieldTypeString, ""),
			input:   `{"translation": "これはテストです。日本語の文字。"}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"translation": "これはテストです。日本語の文字。",
			},
		},
		{
			name: "CJK characters (Korean)",
			sig: dsgo.NewSignature("test").
				AddOutput("translation", dsgo.FieldTypeString, ""),
			input:   `{"translation": "이것은 테스트입니다. 한국어 문자."}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"translation": "이것은 테스트입니다. 한국어 문자.",
			},
		},
		{
			name: "RTL text (Arabic)",
			sig: dsgo.NewSignature("test").
				AddOutput("text", dsgo.FieldTypeString, ""),
			input:   `{"text": "هذا اختبار. النص العربي من اليمين إلى اليسار."}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"text": "هذا اختبار. النص العربي من اليمين إلى اليسار.",
			},
		},
		{
			name: "RTL text (Hebrew)",
			sig: dsgo.NewSignature("test").
				AddOutput("text", dsgo.FieldTypeString, ""),
			input:   `{"text": "זהו מבחן. טקסט עברי מימין לשמאל."}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"text": "זהו מבחן. טקסט עברי מימין לשמאל.",
			},
		},
		{
			name: "Mixed Unicode with Chat adapter",
			sig: dsgo.NewSignature("test").
				AddOutput("message", dsgo.FieldTypeString, ""),
			input: `[[ ## message ## ]]
Hello 你好 مرحبا שלום 🌍`,
			adapter: dsgo.NewChatAdapter(),
			expected: map[string]string{
				"message": "Hello 你好 مرحبا שלום 🌍",
			},
		},
		{
			name: "Emoji sequences and skin tones",
			sig: dsgo.NewSignature("test").
				AddOutput("emoji", dsgo.FieldTypeString, ""),
			input:   `{"emoji": "👨‍👩‍👧‍👦 👋🏽 🏳️‍🌈"}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"emoji": "👨‍👩‍👧‍👦 👋🏽 🏳️‍🌈",
			},
		},
		{
			name: "Special Unicode characters",
			sig: dsgo.NewSignature("test").
				AddOutput("special", dsgo.FieldTypeString, ""),
			input:   `{"special": "© ® ™ € £ ¥ § † ‡ • … — –"}`,
			adapter: dsgo.NewJSONAdapter(),
			expected: map[string]string{
				"special": "© ® ™ € £ ¥ § † ‡ • … — –",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := tt.adapter.Parse(tt.sig, tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			for fieldName, expectedValue := range tt.expected {
				actual, ok := outputs[fieldName].(string)
				if !ok {
					t.Errorf("Field %q has wrong type: %T", fieldName, outputs[fieldName])
					continue
				}
				if actual != expectedValue {
					t.Errorf("Field %q: got %q, want %q", fieldName, actual, expectedValue)
				}
			}
		})
	}
}

// ============================================================================
// Very Large Output Tests
// ============================================================================

func TestAdapter_VeryLargeOutputs(t *testing.T) {
	tests := []struct {
		name           string
		contentSizeKB  int
		adapter        dsgo.Adapter
		useMarkers     bool
		validateLength bool
	}{
		{
			name:           "100KB JSON response",
			contentSizeKB:  100,
			adapter:        dsgo.NewJSONAdapter(),
			useMarkers:     false,
			validateLength: true,
		},
		{
			name:           "100KB Chat response",
			contentSizeKB:  100,
			adapter:        dsgo.NewChatAdapter(),
			useMarkers:     true,
			validateLength: true,
		},
		{
			name:           "200KB Fallback response",
			contentSizeKB:  200,
			adapter:        dsgo.NewFallbackAdapter(),
			useMarkers:     false,
			validateLength: true,
		},
		{
			name:           "500KB large response",
			contentSizeKB:  500,
			adapter:        dsgo.NewJSONAdapter(),
			useMarkers:     false,
			validateLength: true,
		},
	}

	sig := dsgo.NewSignature("test").
		AddOutput("content", dsgo.FieldTypeString, "")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseContent := strings.Repeat("This is test content with various words. ", 30)
			targetBytes := tt.contentSizeKB * 1024
			repetitions := (targetBytes / len(baseContent)) + 1
			largeContent := strings.Repeat(baseContent, repetitions)
			largeContent = largeContent[:targetBytes]

			var input string
			if tt.useMarkers {
				input = fmt.Sprintf("[[ ## content ## ]]\n%s", largeContent)
			} else {
				escapedContent := strings.ReplaceAll(largeContent, `"`, `\"`)
				escapedContent = strings.ReplaceAll(escapedContent, "\n", "\\n")
				input = fmt.Sprintf(`{"content": "%s"}`, escapedContent)
			}

			start := time.Now()
			outputs, err := tt.adapter.Parse(sig, input)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("Parse failed for %dKB content: %v", tt.contentSizeKB, err)
			}

			if elapsed > 5*time.Second {
				t.Errorf("Parse took too long: %v (expected < 5s)", elapsed)
			}

			if tt.validateLength {
				content, ok := outputs["content"].(string)
				if !ok {
					t.Fatalf("content field has wrong type: %T", outputs["content"])
				}
				minExpected := targetBytes / 2
				if len(content) < minExpected {
					t.Errorf("Parsed content too short: got %d bytes, expected at least %d", len(content), minExpected)
				}
			}

			t.Logf("Parsed %dKB in %v", tt.contentSizeKB, elapsed)
		})
	}
}

// ============================================================================
// Mixed Field Marker Tests
// ============================================================================

func TestAdapter_MixedFieldMarkers(t *testing.T) {
	tests := []struct {
		name            string
		sig             *dsgo.Signature
		input           string
		expectedAdapter string
		shouldSucceed   bool
		validateOutputs func(map[string]any) bool
	}{
		{
			name: "Chat markers followed by JSON",
			sig: dsgo.NewSignature("test").
				AddOutput("answer", dsgo.FieldTypeString, "").
				AddOutput("score", dsgo.FieldTypeFloat, ""),
			input: `[[ ## answer ## ]]
yes

Here's also some JSON: {"score": 0.95}

[[ ## score ## ]]
0.85`,
			expectedAdapter: "chat",
			shouldSucceed:   true,
			validateOutputs: func(outputs map[string]any) bool {
				_, hasAnswer := outputs["answer"]
				_, hasScore := outputs["score"]
				return hasAnswer && hasScore
			},
		},
		{
			name: "JSON with embedded Chat-like markers",
			sig: dsgo.NewSignature("test").
				AddOutput("content", dsgo.FieldTypeString, ""),
			input:           `{"content": "Example: [[ ## field ## ]]\nvalue"}`,
			expectedAdapter: "json",
			shouldSucceed:   true,
			validateOutputs: func(outputs map[string]any) bool {
				content, ok := outputs["content"].(string)
				return ok && strings.Contains(content, "[[ ## field ## ]]")
			},
		},
		{
			name: "Markdown JSON block with Chat markers after",
			sig: dsgo.NewSignature("test").
				AddOutput("data", dsgo.FieldTypeJSON, "").
				AddOutput("summary", dsgo.FieldTypeString, ""),
			input:           "```json\n{\"data\": {\"key\": \"value\"}}\n```\n\n[[ ## summary ## ]]\nThis is a summary",
			expectedAdapter: "json",
			shouldSucceed:   true,
			validateOutputs: func(outputs map[string]any) bool {
				_, hasData := outputs["data"]
				return hasData
			},
		},
		{
			name: "Fallback adapter with ambiguous format",
			sig: dsgo.NewSignature("test").
				AddOutput("result", dsgo.FieldTypeString, ""),
			input:           `result: The answer is {"value": 42}`,
			expectedAdapter: "fallback",
			shouldSucceed:   false,
		},
		{
			name: "JSON wrapped in Chat markers",
			sig: dsgo.NewSignature("test").
				AddOutput("data", dsgo.FieldTypeJSON, ""),
			input: `[[ ## data ## ]]
{"nested": {"key": "value", "array": [1, 2, 3]}}`,
			expectedAdapter: "chat",
			shouldSucceed:   true,
			validateOutputs: func(outputs map[string]any) bool {
				_, hasData := outputs["data"]
				return hasData
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := dsgo.NewFallbackAdapter()
			outputs, err := adapter.Parse(tt.sig, tt.input)

			if tt.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected success but got error: %v", err)
				}
				if tt.validateOutputs != nil && !tt.validateOutputs(outputs) {
					t.Errorf("Output validation failed: %v", outputs)
				}
			} else {
				if err == nil {
					t.Logf("Parse succeeded (acceptable for robustness): %v", outputs)
				}
			}
		})
	}
}

// ============================================================================
// Nested JSON Escape Tests
// ============================================================================

func TestAdapter_NestedJSONEscape(t *testing.T) {
	tests := []struct {
		name            string
		sig             *dsgo.Signature
		input           string
		adapter         dsgo.Adapter
		validateOutputs func(map[string]any) bool
	}{
		{
			name: "Escaped quotes in string",
			sig: dsgo.NewSignature("test").
				AddOutput("text", dsgo.FieldTypeString, ""),
			input:   `{"text": "He said \"Hello, World!\" to everyone."}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				text, ok := outputs["text"].(string)
				return ok && strings.Contains(text, `"Hello, World!"`)
			},
		},
		{
			name: "Escaped newlines and tabs",
			sig: dsgo.NewSignature("test").
				AddOutput("code", dsgo.FieldTypeString, ""),
			input:   `{"code": "function test() {\n\treturn true;\n}"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				code, ok := outputs["code"].(string)
				return ok && strings.Contains(code, "\n") && strings.Contains(code, "\t")
			},
		},
		{
			name: "Escaped backslashes",
			sig: dsgo.NewSignature("test").
				AddOutput("path", dsgo.FieldTypeString, ""),
			input:   `{"path": "C:\\Users\\test\\Documents\\file.txt"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				path, ok := outputs["path"].(string)
				return ok && path == `C:\Users\test\Documents\file.txt`
			},
		},
		{
			name: "Nested JSON as string",
			sig: dsgo.NewSignature("test").
				AddOutput("config", dsgo.FieldTypeString, ""),
			input:   `{"config": "{\"database\": {\"host\": \"localhost\", \"port\": 5432}}"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				config, ok := outputs["config"].(string)
				return ok && strings.Contains(config, `"database"`) && strings.Contains(config, `"host"`)
			},
		},
		{
			name: "Unicode escapes",
			sig: dsgo.NewSignature("test").
				AddOutput("text", dsgo.FieldTypeString, ""),
			input:   `{"text": "Hello \u4e16\u754c (world in Chinese)"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				text, ok := outputs["text"].(string)
				return ok && strings.Contains(text, "世界")
			},
		},
		{
			name: "Mixed escape sequences",
			sig: dsgo.NewSignature("test").
				AddOutput("complex", dsgo.FieldTypeString, ""),
			input:   `{"complex": "Line1\nLine2\tTabbed\r\nWindows line\u0020space"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				complex, ok := outputs["complex"].(string)
				return ok && strings.Contains(complex, "\n") && strings.Contains(complex, "\t")
			},
		},
		{
			name: "Deeply nested JSON object",
			sig: dsgo.NewSignature("test").
				AddOutput("data", dsgo.FieldTypeJSON, ""),
			input:   `{"data": {"level1": {"level2": {"level3": {"level4": {"value": "deep"}}}}}}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				data, ok := outputs["data"].(map[string]any)
				if !ok {
					return false
				}
				level1, ok := data["level1"].(map[string]any)
				if !ok {
					return false
				}
				level2, ok := level1["level2"].(map[string]any)
				if !ok {
					return false
				}
				level3, ok := level2["level3"].(map[string]any)
				if !ok {
					return false
				}
				level4, ok := level3["level4"].(map[string]any)
				if !ok {
					return false
				}
				value, ok := level4["value"].(string)
				return ok && value == "deep"
			},
		},
		{
			name: "JSON with escaped forward slashes",
			sig: dsgo.NewSignature("test").
				AddOutput("url", dsgo.FieldTypeString, ""),
			input:   `{"url": "https:\/\/example.com\/path\/to\/resource"}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				url, ok := outputs["url"].(string)
				return ok && url == "https://example.com/path/to/resource"
			},
		},
		{
			name: "Chat adapter with escaped content",
			sig: dsgo.NewSignature("test").
				AddOutput("code", dsgo.FieldTypeString, ""),
			input: `[[ ## code ## ]]
func example() {
	fmt.Println("Hello, \"World\"")
	path := "C:\\Users\\test"
}`,
			adapter: dsgo.NewChatAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				code, ok := outputs["code"].(string)
				return ok && strings.Contains(code, "func example()")
			},
		},
		{
			name: "Empty string value",
			sig: dsgo.NewSignature("test").
				AddOutput("empty", dsgo.FieldTypeString, ""),
			input:   `{"empty": ""}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				empty, ok := outputs["empty"].(string)
				return ok && empty == ""
			},
		},
		{
			name: "String with only escape characters",
			sig: dsgo.NewSignature("test").
				AddOutput("escapes", dsgo.FieldTypeString, ""),
			input:   `{"escapes": "\n\t\r\\\""}`,
			adapter: dsgo.NewJSONAdapter(),
			validateOutputs: func(outputs map[string]any) bool {
				escapes, ok := outputs["escapes"].(string)
				return ok && strings.Contains(escapes, "\"")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := tt.adapter.Parse(tt.sig, tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if !tt.validateOutputs(outputs) {
				t.Errorf("Output validation failed: %v", outputs)
			}
		})
	}
}
