package dsgo

import (
	"strings"
	"testing"
)

// TestJSONAdapter_Format tests the full Format method
func TestJSONAdapter_Format(t *testing.T) {
	adapter := NewJSONAdapter()
	sig := NewSignature("Test task").
		AddInput("question", FieldTypeString, "The question").
		AddOutput("answer", FieldTypeString, "The answer")

	tests := []struct {
		name    string
		inputs  map[string]any
		demos   []Example
		wantErr bool
	}{
		{
			name:    "basic format",
			inputs:  map[string]any{"question": "What is 2+2?"},
			demos:   []Example{},
			wantErr: false,
		},
		{
			name:   "with demos",
			inputs: map[string]any{"question": "What is 3+3?"},
			demos: []Example{
				*NewExample(
					map[string]any{"question": "What is 1+1?"},
					map[string]any{"answer": "2"},
				),
			},
			wantErr: false,
		},
		{
			name:    "missing required input",
			inputs:  map[string]any{},
			demos:   []Example{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := adapter.Format(sig, tt.inputs, tt.demos)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(messages) == 0 {
				t.Error("Format() should return non-empty messages")
			}
		})
	}
}

// TestJSONAdapter_FormatWithReasoning tests reasoning field
func TestJSONAdapter_FormatWithReasoning(t *testing.T) {
	adapter := NewJSONAdapter().WithReasoning(true)
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{"question": "test"}, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Check that reasoning instruction is included
	found := false
	for _, msg := range messages {
		if strings.Contains(msg.Content, "step-by-step") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Format with reasoning should include step-by-step instruction")
	}
}

// TestJSONAdapter_FormatHistory tests history formatting
func TestJSONAdapter_FormatHistory(t *testing.T) {
	adapter := NewJSONAdapter()
	history := NewHistory()
	history.AddUserMessage("Hello")
	history.AddAssistantMessage("Hi")

	messages := adapter.FormatHistory(history)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test with empty history
	emptyHistory := NewHistory()
	emptyMessages := adapter.FormatHistory(emptyHistory)
	if len(emptyMessages) != 0 {
		t.Error("Empty history should return empty messages")
	}

	// Test with nil history
	nilMessages := adapter.FormatHistory(nil)
	if len(nilMessages) != 0 {
		t.Error("Nil history should return empty messages")
	}
}

// TestChatAdapter_FormatComprehensive tests the full Format method
func TestChatAdapter_FormatComprehensive(t *testing.T) {
	adapter := NewChatAdapter()
	sig := NewSignature("Test task").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	tests := []struct {
		name    string
		inputs  map[string]any
		demos   []Example
		wantErr bool
	}{
		{
			name:    "basic format",
			inputs:  map[string]any{"question": "test"},
			demos:   []Example{},
			wantErr: false,
		},
		{
			name:   "with demos",
			inputs: map[string]any{"question": "test"},
			demos: []Example{
				*NewExample(
					map[string]any{"question": "What is 1+1?"},
					map[string]any{"answer": "2"},
				),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := adapter.Format(sig, tt.inputs, tt.demos)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(messages) == 0 {
				t.Error("Format() should return messages")
			}
		})
	}
}

// TestChatAdapter_FormatHistory tests history formatting
func TestChatAdapter_FormatHistory(t *testing.T) {
	adapter := NewChatAdapter()
	history := NewHistory()
	history.AddUserMessage("Hello")
	history.AddAssistantMessage("Hi")

	messages := adapter.FormatHistory(history)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test with nil history
	nilMessages := adapter.FormatHistory(nil)
	if len(nilMessages) != 0 {
		t.Error("Nil history should return empty messages")
	}
}

// TestFallbackAdapter_FormatDelegation tests format delegation
func TestFallbackAdapter_FormatDelegation(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{"question": "test"}, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if len(messages) == 0 {
		t.Error("Format() should return messages")
	}
}

// TestFallbackAdapter_FormatHistory tests history delegation
func TestFallbackAdapter_FormatHistory(t *testing.T) {
	adapter := NewFallbackAdapter()
	history := NewHistory()
	history.AddUserMessage("test")

	messages := adapter.FormatHistory(history)
	if len(messages) != 1 {
		t.Error("FormatHistory should delegate to first adapter")
	}
}

// TestFallbackAdapter_Parse tests fallback chain
func TestFallbackAdapter_Parse(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	// Valid JSON - should work with first adapter
	outputs, err := adapter.Parse(sig, `{"answer": "test"}`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if outputs["answer"] != "test" {
		t.Error("Parse() should extract answer")
	}

	// Test GetLastUsedAdapter
	if adapter.GetLastUsedAdapter() < 0 {
		t.Error("GetLastUsedAdapter should return valid index after successful parse")
	}

	// Invalid format - should try all adapters and fail
	_, err = adapter.Parse(sig, "completely invalid")
	if err == nil {
		t.Error("Parse() should fail on invalid content")
	}
	if !strings.Contains(err.Error(), "all adapters failed") {
		t.Errorf("Expected 'all adapters failed' error, got: %v", err)
	}
}

// TestFallbackAdapter_WithReasoningPropagation tests reasoning propagation
func TestFallbackAdapter_WithReasoningPropagation(t *testing.T) {
	adapter := NewFallbackAdapter().WithReasoning(true)
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	messages, err := adapter.Format(sig, map[string]any{"question": "test"}, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Should include reasoning instruction
	found := false
	for _, msg := range messages {
		if strings.Contains(msg.Content, "reasoning") {
			found = true
			break
		}
	}
	if !found {
		t.Error("WithReasoning should propagate to adapters")
	}
}

// TestNewFallbackAdapterWithChain tests custom adapter chain
func TestNewFallbackAdapterWithChain(t *testing.T) {
	// Test with custom chain
	jsonAdapter := NewJSONAdapter()
	adapter := NewFallbackAdapterWithChain(jsonAdapter)
	if len(adapter.adapters) != 1 {
		t.Errorf("Expected 1 adapter, got %d", len(adapter.adapters))
	}

	// Test with empty chain (should use defaults)
	defaultAdapter := NewFallbackAdapterWithChain()
	if len(defaultAdapter.adapters) != 2 {
		t.Errorf("Empty chain should use default 2 adapters, got %d", len(defaultAdapter.adapters))
	}
}

// TestCoerceOutputs tests the shared type coercion helper
func TestCoerceOutputs(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("int_field", FieldTypeInt, "").
		AddOutput("float_field", FieldTypeFloat, "").
		AddOutput("bool_field", FieldTypeBool, "").
		AddOutput("string_field", FieldTypeString, "")

	tests := []struct {
		name               string
		outputs            map[string]any
		allowArrayToString bool
		expected           map[string]any
	}{
		{
			name: "string to int",
			outputs: map[string]any{
				"int_field": "42",
			},
			expected: map[string]any{
				"int_field": 42,
			},
		},
		{
			name: "float64 to int",
			outputs: map[string]any{
				"int_field": float64(42),
			},
			expected: map[string]any{
				"int_field": 42,
			},
		},
		{
			name: "string to float",
			outputs: map[string]any{
				"float_field": "3.14",
			},
			expected: map[string]any{
				"float_field": 3.14,
			},
		},
		{
			name: "int to float",
			outputs: map[string]any{
				"float_field": 42,
			},
			expected: map[string]any{
				"float_field": float64(42),
			},
		},
		{
			name: "string to bool",
			outputs: map[string]any{
				"bool_field": "true",
			},
			expected: map[string]any{
				"bool_field": true,
			},
		},
		{
			name: "array to string (allowed)",
			outputs: map[string]any{
				"string_field": []any{"a", "b", "c"},
			},
			allowArrayToString: true,
			expected: map[string]any{
				"string_field": "a\nb\nc",
			},
		},
		{
			name: "array to string (not allowed)",
			outputs: map[string]any{
				"string_field": []any{"a", "b"},
			},
			allowArrayToString: false,
			expected: map[string]any{
				"string_field": []any{"a", "b"},
			},
		},
		{
			name: "unknown field preserved",
			outputs: map[string]any{
				"unknown": "value",
			},
			expected: map[string]any{
				"unknown": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coerceOutputs(sig, tt.outputs, tt.allowArrayToString)
			for key, expectedValue := range tt.expected {
				resultValue := result[key]

				// Special handling for slices (can't use ==)
				if expectedSlice, ok := expectedValue.([]any); ok {
					resultSlice, ok := resultValue.([]any)
					if !ok {
						t.Errorf("Expected slice for %s, got %T", key, resultValue)
						continue
					}
					if len(resultSlice) != len(expectedSlice) {
						t.Errorf("Expected %v for %s, got %v", expectedValue, key, resultValue)
						continue
					}
					for i := range expectedSlice {
						if resultSlice[i] != expectedSlice[i] {
							t.Errorf("Expected %v for %s, got %v", expectedValue, key, resultValue)
							break
						}
					}
				} else if resultValue != expectedValue {
					t.Errorf("Expected %v for %s, got %v", expectedValue, key, resultValue)
				}
			}
		})
	}
}
