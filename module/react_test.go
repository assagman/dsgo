package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestReAct_Forward_NoTools(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content:   `{"reasoning": "thinking", "answer": "result"}`,
				ToolCalls: []dsgo.ToolCall{},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "result" {
		t.Errorf("Expected answer='result', got %v", outputs.Outputs["answer"])
	}
}

func TestReAct_Forward_WithToolCalls(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Let me search",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "final answer"}`,
			}, nil
		},
	}

	searchTool := dsgo.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*searchTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "final answer" {
		t.Errorf("Expected final answer, got %v", outputs.Outputs["answer"])
	}
}

func TestReAct_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	react := NewReAct(sig, lm, []dsgo.Tool{})

	_, err := react.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestReAct_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestReAct_Forward_MaxIterations(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: "thinking",
				ToolCalls: []dsgo.ToolCall{
					{ID: "1", Name: "search", Arguments: map[string]interface{}{}},
				},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{}).WithMaxIterations(2)
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error when max iterations exceeded")
	}
}

func TestReAct_Forward_ToolNotFound(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Using tool",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "nonexistent", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "recovered"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should handle missing tool gracefully, got error: %v", err)
	}

	if outputs.Outputs["answer"] != "recovered" {
		t.Error("Should recover from tool not found error")
	}
}

func TestReAct_Forward_ToolError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Using tool",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "failing_tool", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "recovered from error"}`,
			}, nil
		},
	}

	failingTool := dsgo.NewTool("failing_tool", "Fails", func(ctx context.Context, args map[string]any) (any, error) {
		return nil, errors.New("tool failed")
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*failingTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should handle tool errors, got: %v", err)
	}

	if outputs.Outputs["answer"] != "recovered from error" {
		t.Error("Should recover from tool execution error")
	}
}

func TestReAct_WithOptions(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	react := NewReAct(sig, lm, []dsgo.Tool{})

	customOpts := &dsgo.GenerateOptions{Temperature: 0.9}
	react.WithOptions(customOpts)

	if react.Options.Temperature != 0.9 {
		t.Error("WithOptions should set custom options")
	}
}

func TestReAct_WithMaxIterations(t *testing.T) {
	react := NewReAct(dsgo.NewSignature("Test"), &MockLM{}, []dsgo.Tool{})
	react.WithMaxIterations(5)

	if react.MaxIterations != 5 {
		t.Error("WithMaxIterations should set max iterations")
	}
}

func TestReAct_WithVerbose(t *testing.T) {
	react := NewReAct(dsgo.NewSignature("Test"), &MockLM{}, []dsgo.Tool{})
	react.WithVerbose(true)

	if !react.Verbose {
		t.Error("WithVerbose should enable verbose mode")
	}
}

func TestReAct_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	if react.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

// TestReAct_FixJSONNewlines removed - functionality moved to internal/jsonutil package
// See internal/jsonutil/extract_test.go for comprehensive JSON extraction and newline fixing tests

func TestReAct_BuildSystemPrompt_NoTools(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	prompt := react.buildSystemPrompt()
	if prompt != "" {
		t.Error("System prompt should be empty when no tools")
	}
}

func TestReAct_BuildSystemPrompt_WithTools(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	tool := dsgo.NewTool("test", "Test tool", nil)
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{*tool})

	prompt := react.buildSystemPrompt()
	if prompt == "" {
		t.Error("System prompt should not be empty with tools")
	}

	if !contains(prompt, "ReAct") {
		t.Error("System prompt should mention ReAct")
	}
}

func TestReAct_FindTool(t *testing.T) {
	tool1 := dsgo.NewTool("search", "Search", nil)
	tool2 := dsgo.NewTool("calculate", "Calculate", nil)

	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{*tool1, *tool2})

	found := react.findTool("search")
	if found == nil || found.Name != "search" {
		t.Error("Should find existing tool")
	}

	notFound := react.findTool("nonexistent")
	if notFound != nil {
		t.Error("Should return nil for missing tool")
	}
}

func TestReAct_StagnationDetection(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	var capturedMessages []dsgo.Message
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			capturedMessages = messages

			switch callCount {
			case 1:
				// First call: make a tool call
				return &dsgo.GenerateResult{
					Content: "Let me search",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			case 2:
				// Second call: make same tool call (stagnation)
				return &dsgo.GenerateResult{
					Content: "Let me search again",
					ToolCalls: []dsgo.ToolCall{
						{ID: "2", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			default:
				// After stagnation message: provide final answer
				return &dsgo.GenerateResult{
					Content: `{"answer": "forced final answer"}`,
				}, nil
			}
		},
	}

	searchTool := dsgo.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "same result", nil
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*searchTool}).WithMaxIterations(10)
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify that the final answer was forced after stagnation
	if outputs.Outputs["answer"] != "forced final answer" {
		t.Errorf("Expected forced final answer after stagnation, got %v", outputs.Outputs["answer"])
	}

	// Verify that a stagnation prevention message was injected
	stagnationMessageFound := false
	for _, msg := range capturedMessages {
		if msg.Role == "user" && contains(msg.Content, "same observation twice") {
			stagnationMessageFound = true
			break
		}
	}

	if !stagnationMessageFound {
		t.Error("Expected stagnation prevention message to be injected")
	}

	// Verify the model was called at least 3 times (2 tool calls + 1 final answer after stagnation)
	if callCount < 3 {
		t.Errorf("Expected at least 3 LM calls (stagnation + recovery), got %d", callCount)
	}
}

// TestReAct_Forward_WithHistory tests history management
func TestReAct_Forward_WithHistory(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"answer": "final answer with history"}`,
			}, nil
		},
	}

	history := dsgo.NewHistory()
	history.Add(dsgo.Message{Role: "user", Content: "previous question"})
	history.Add(dsgo.Message{Role: "assistant", Content: "previous answer"})

	react := NewReAct(sig, lm, []dsgo.Tool{}).WithHistory(history)
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "current question",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "final answer with history" {
		t.Errorf("Expected answer with history, got %v", outputs.Outputs["answer"])
	}

	// Verify history was updated
	if history.Len() != 4 { // 2 previous + 1 user + 1 assistant
		t.Errorf("Expected 4 messages in history, got %d", history.Len())
	}
}

// TestReAct_Forward_WithFinishTool tests the "finish" tool detection
func TestReAct_Forward_WithFinishTool(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence")

	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: "I have the answer",
				ToolCalls: []dsgo.ToolCall{
					{
						ID:   "finish-1",
						Name: "finish",
						Arguments: map[string]interface{}{
							"answer":     "The answer is 42",
							"confidence": 0.95,
						},
					},
				},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "The answer is 42" {
		t.Errorf("Expected finish tool answer, got %v", outputs.Outputs["answer"])
	}

	if outputs.Outputs["confidence"] != 0.95 {
		t.Errorf("Expected confidence 0.95, got %v", outputs.Outputs["confidence"])
	}
}

// TestReAct_Forward_WithFinishTool_InvalidOutputs tests finish tool with validation errors
func TestReAct_Forward_WithFinishTool_InvalidOutputs(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer").
		AddOutput("score", dsgo.FieldTypeInt, "Score")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// First call: finish tool with invalid outputs (missing score)
				return &dsgo.GenerateResult{
					Content: "Trying to finish",
					ToolCalls: []dsgo.ToolCall{
						{
							ID:   "finish-1",
							Name: "finish",
							Arguments: map[string]interface{}{
								"answer": "incomplete",
							},
						},
					},
				}, nil
			}
			// Second call: proper final answer
			return &dsgo.GenerateResult{
				Content: `{"answer": "complete answer", "score": 85}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "complete answer" {
		t.Error("Should recover from invalid finish tool and provide proper answer")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (invalid finish + recovery), got %d", callCount)
	}
}

// TestReAct_Forward_WithReasoning tests reasoning field extraction and cleanup
func TestReAct_Forward_WithReasoning(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"reasoning": "Let me think about this...", "answer": "The answer"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Reasoning should be extracted to rationale
	if outputs.Rationale != "Let me think about this..." {
		t.Errorf("Expected rationale to be set, got %q", outputs.Rationale)
	}

	// Reasoning should be removed from outputs if not in signature
	if _, exists := outputs.Outputs["reasoning"]; exists {
		t.Error("Reasoning should be removed from outputs when not in signature")
	}
}

// TestReAct_Forward_WithReasoningInSignature tests when reasoning is part of the signature
func TestReAct_Forward_WithReasoningInSignature(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("reasoning", dsgo.FieldTypeString, "Reasoning").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"reasoning": "Thinking step by step", "answer": "42"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Reasoning should be in rationale
	if outputs.Rationale != "Thinking step by step" {
		t.Errorf("Expected rationale to be set, got %q", outputs.Rationale)
	}

	// Reasoning should remain in outputs when it's in the signature
	if _, exists := outputs.Outputs["reasoning"]; !exists {
		t.Error("Reasoning should remain in outputs when it's part of the signature")
	}
}

// TestReAct_Forward_JSONModeWithJSONAdapter tests JSON mode enablement with JSONAdapter
func TestReAct_Forward_JSONModeWithJSONAdapter(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	optionsCaptured := false
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			if options.ResponseFormat == "json" {
				optionsCaptured = true
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "json mode answer"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{}).WithAdapter(dsgo.NewJSONAdapter())
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if !optionsCaptured {
		t.Error("JSON mode should be enabled when using JSONAdapter and LM supports JSON")
	}

	if outputs.Outputs["answer"] != "json mode answer" {
		t.Errorf("Expected answer, got %v", outputs.Outputs["answer"])
	}
}

// TestReAct_Forward_MultipleToolCalls tests multiple tool calls in one iteration
func TestReAct_Forward_MultipleToolCalls(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Using multiple tools",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test1"}},
						{ID: "2", Name: "calculate", Arguments: map[string]interface{}{"expr": "2+2"}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "combined result"}`,
			}, nil
		},
	}

	searchTool := dsgo.NewTool("search", "Search", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})
	calcTool := dsgo.NewTool("calculate", "Calculate", func(ctx context.Context, args map[string]any) (any, error) {
		return "4", nil
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*searchTool, *calcTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "combined result" {
		t.Error("Should handle multiple tool calls in one iteration")
	}
}

// TestReAct_Forward_WithDemos tests few-shot examples
func TestReAct_Forward_WithDemos(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"answer": "demo-informed answer"}`,
			}, nil
		},
	}

	demos := []dsgo.Example{
		{
			Inputs:  map[string]any{"question": "What is 2+2?"},
			Outputs: map[string]any{"answer": "4"},
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{}).WithDemos(demos)
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "What is 3+3?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "demo-informed answer" {
		t.Errorf("Expected demo-informed answer, got %v", outputs.Outputs["answer"])
	}
}

// TestReAct_Forward_AdapterMetrics tests adapter metadata extraction
func TestReAct_Forward_AdapterMetrics(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: `{"answer": "test", "__adapter_used": "JSONAdapter", "__parse_attempts": 2, "__fallback_used": true}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.AdapterUsed == "" {
		t.Error("Expected adapter metadata to be extracted")
	}

	if outputs.ParseAttempts != 2 {
		t.Errorf("Expected 2 parse attempts, got %d", outputs.ParseAttempts)
	}

	if !outputs.FallbackUsed {
		t.Error("Expected fallback_used to be true")
	}
}

// TestReAct_Forward_OutputValidationError tests validation errors after parsing
func TestReAct_Forward_OutputValidationError(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer").
		AddOutput("score", dsgo.FieldTypeInt, "Required score")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			// Missing required "score" field
			return &dsgo.GenerateResult{
				Content: `{"answer": "incomplete"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error when output validation fails")
	}

	if !contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got %v", err)
	}
}
