package module

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestReAct_Forward_NoTools(t *testing.T) {
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:   `{"reasoning": "thinking", "answer": "result"}`,
				ToolCalls: []core.ToolCall{},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{
					Content: "Let me search",
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			}
			return &core.GenerateResult{
				Content: `{"answer": "final answer"}`,
			}, nil
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool})
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
	sig := core.NewSignature("Test").
		AddInput("required", core.FieldTypeString, "Required")

	lm := &MockLM{}
	react := NewReAct(sig, lm, []core.Tool{})

	_, err := react.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestReAct_Forward_LMError(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestReAct_Forward_MaxIterations(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "thinking",
				ToolCalls: []core.ToolCall{
					{ID: "1", Name: "search", Arguments: map[string]interface{}{}},
				},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{}).WithMaxIterations(2)
	result, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	// With the extraction phase, ReAct should now return a result instead of erroring
	if err != nil {
		t.Errorf("Forward() should not error when max iterations exceeded, got: %v", err)
	}
	if result == nil {
		t.Error("Forward() should return a result via extraction")
	}
	// Verify that extraction was called (should have made additional LM call)
	if result != nil {
		answer, _ := result.GetString("answer")
		if answer == "" {
			t.Error("Extraction should have produced an answer")
		}
	}
}

func TestReAct_Forward_ToolNotFound(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{
					Content: "Using tool",
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "nonexistent", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &core.GenerateResult{
				Content: `{"answer": "recovered"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Test").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{
					Content: "Using tool",
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "failing_tool", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &core.GenerateResult{
				Content: `{"answer": "recovered from error"}`,
			}, nil
		},
	}

	failingTool := core.NewTool("failing_tool", "Fails", func(ctx context.Context, args map[string]any) (any, error) {
		return nil, errors.New("tool failed")
	})

	react := NewReAct(sig, lm, []core.Tool{*failingTool})
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
	sig := core.NewSignature("Test")
	lm := &MockLM{}
	react := NewReAct(sig, lm, []core.Tool{})

	customOpts := &core.GenerateOptions{Temperature: 0.9}
	react.WithOptions(customOpts)

	if react.Options.Temperature != 0.9 {
		t.Error("WithOptions should set custom options")
	}
}

func TestReAct_WithMaxIterations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})
	react.WithMaxIterations(5)

	if react.MaxIterations != 5 {
		t.Error("WithMaxIterations should set max iterations")
	}
}

func TestReAct_WithVerbose(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})
	react.WithVerbose(true)

	if !react.Verbose {
		t.Error("WithVerbose should enable verbose mode")
	}
}

func TestReAct_GetSignature(t *testing.T) {
	sig := core.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	if react.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

// TestReAct_FixJSONNewlines removed - functionality moved to internal/jsonutil package
// See internal/jsonutil/extract_test.go for comprehensive JSON extraction and newline fixing tests

func TestReAct_BuildSystemPrompt_NoTools(t *testing.T) {
	sig := core.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	prompt := react.buildSystemPrompt()
	if prompt != "" {
		t.Error("System prompt should be empty when no tools")
	}
}

func TestReAct_BuildSystemPrompt_WithTools(t *testing.T) {
	sig := core.NewSignature("Test")
	tool := core.NewTool("test", "Test tool", nil)
	react := NewReAct(sig, &MockLM{}, []core.Tool{*tool})

	prompt := react.buildSystemPrompt()
	if prompt == "" {
		t.Error("System prompt should not be empty with tools")
	}

	if !contains(prompt, "tools") {
		t.Error("System prompt should mention tools")
	}
	if !contains(prompt, "finish") {
		t.Error("System prompt should mention finish tool")
	}
}

func TestReAct_FindTool(t *testing.T) {
	tool1 := core.NewTool("search", "Search", nil)
	tool2 := core.NewTool("calculate", "Calculate", nil)

	sig := core.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []core.Tool{*tool1, *tool2})

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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	var capturedMessages []core.Message
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			capturedMessages = messages

			switch callCount {
			case 1:
				// First call: make a tool call
				return &core.GenerateResult{
					Content: "Let me search",
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			case 2:
				// Second call: make same tool call (stagnation)
				return &core.GenerateResult{
					Content: "Let me search again",
					ToolCalls: []core.ToolCall{
						{ID: "2", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			default:
				// After stagnation message: provide final answer
				return &core.GenerateResult{
					Content: `{"answer": "forced final answer"}`,
				}, nil
			}
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "same result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool}).WithMaxIterations(10)
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "final answer with history"}`,
			}, nil
		},
	}

	history := core.NewHistory()
	history.Add(core.Message{Role: "user", Content: "previous question"})
	history.Add(core.Message{Role: "assistant", Content: "previous answer"})

	react := NewReAct(sig, lm, []core.Tool{}).WithHistory(history)
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence")

	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "I have the answer",
				ToolCalls: []core.ToolCall{
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

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("score", core.FieldTypeInt, "Score")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// First call: finish tool with invalid outputs (missing score)
				return &core.GenerateResult{
					Content: "Trying to finish",
					ToolCalls: []core.ToolCall{
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
			return &core.GenerateResult{
				Content: `{"answer": "complete answer", "score": 85}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"reasoning": "Let me think about this...", "answer": "The answer"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("reasoning", core.FieldTypeString, "Reasoning").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"reasoning": "Thinking step by step", "answer": "42"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	optionsCaptured := false
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			if options.ResponseFormat == "json" {
				optionsCaptured = true
			}
			return &core.GenerateResult{
				Content: `{"answer": "json mode answer"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{}).WithAdapter(core.NewJSONAdapter())
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &core.GenerateResult{
					Content: "Using multiple tools",
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test1"}},
						{ID: "2", Name: "calculate", Arguments: map[string]interface{}{"expr": "2+2"}},
					},
				}, nil
			}
			return &core.GenerateResult{
				Content: `{"answer": "combined result"}`,
			}, nil
		},
	}

	searchTool := core.NewTool("search", "Search", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})
	calcTool := core.NewTool("calculate", "Calculate", func(ctx context.Context, args map[string]any) (any, error) {
		return "4", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool, *calcTool})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "demo-informed answer"}`,
			}, nil
		},
	}

	demos := []core.Example{
		{
			Inputs:  map[string]any{"question": "What is 2+2?"},
			Outputs: map[string]any{"answer": "4"},
		},
	}

	react := NewReAct(sig, lm, []core.Tool{}).WithDemos(demos)
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"answer": "test", "__adapter_used": "JSONAdapter", "__parse_attempts": 2, "__fallback_used": true}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
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
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("score", core.FieldTypeInt, "Required score")

	callCount := 0
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// Missing required "score" field - will trigger extraction
				return &core.GenerateResult{
					Content: `{"answer": "incomplete"}`,
				}, nil
			}
			// Extraction call - provide complete answer
			return &core.GenerateResult{
				Content: `{"answer": "extracted answer", "score": 42}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{})
	result, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	// With extraction, validation failures should be handled gracefully
	if err != nil {
		t.Errorf("Forward() should not error with extraction fallback, got: %v", err)
	}

	if result == nil {
		t.Error("Forward() should return a result via extraction")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 LM calls (initial + extraction), got %d", callCount)
	}
}

func TestReAct_ExtractTextOutputs_ShortContent(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("answer", core.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	// Test with short content (< 10 chars)
	messages := []core.Message{}
	outputs := react.extractTextOutputs("short", messages)

	// Should synthesize from history even though there's no history
	if outputs == nil {
		t.Error("extractTextOutputs should return outputs for short content")
	}
}

func TestReAct_ExtractTextOutputs_NoStringFields(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("count", core.FieldTypeInt, "Count")

	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	messages := []core.Message{}
	outputs := react.extractTextOutputs("long enough content here", messages)

	if outputs != nil {
		t.Error("extractTextOutputs should return nil when no string output fields")
	}
}

func TestReAct_ExtractTextOutputs_SingleField(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("answer", core.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	content := "This is the final answer to the question"
	messages := []core.Message{}
	outputs := react.extractTextOutputs(content, messages)

	if outputs == nil {
		t.Fatal("extractTextOutputs should extract single field")
	}

	if answer, ok := outputs["answer"].(string); !ok || answer != content {
		t.Errorf("Expected answer='%s', got %v", content, outputs["answer"])
	}
}

func TestReAct_ExtractTextOutputs_MultipleFields(t *testing.T) {
	sig := core.NewSignature("Test").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("reasoning", core.FieldTypeString, "Reasoning")

	react := NewReAct(sig, &MockLM{}, []core.Tool{})

	content := "Based on my analysis, the final answer is 42"
	messages := []core.Message{}
	outputs := react.extractTextOutputs(content, messages)

	if outputs == nil {
		t.Fatal("extractTextOutputs should extract multiple fields")
	}

	// First field should get the content
	if answer, ok := outputs["answer"].(string); !ok || answer != content {
		t.Errorf("Expected answer to be content, got %v", outputs["answer"])
	}

	// Second required field should get a placeholder
	if reasoning, ok := outputs["reasoning"].(string); !ok || reasoning == "" {
		t.Errorf("Expected reasoning placeholder, got %v", outputs["reasoning"])
	}
}

func TestReAct_SynthesizeAnswerFromHistory_NoObservations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	messages := []core.Message{
		{Role: "user", Content: "test question"},
		{Role: "assistant", Content: "thinking"},
	}

	result := react.synthesizeAnswerFromHistory(messages)
	if result != "No information available from tools" {
		t.Errorf("Expected 'No information available' message, got '%s'", result)
	}
}

func TestReAct_SynthesizeAnswerFromHistory_WithObservations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	messages := []core.Message{
		{Role: "user", Content: "test question"},
		{Role: "tool", Content: "The weather is sunny"},
		{Role: "assistant", Content: "thinking"},
		{Role: "tool", Content: "Temperature is 25 degrees"},
	}

	result := react.synthesizeAnswerFromHistory(messages)

	// Should use recent observations
	if result == "No information available from tools" {
		t.Error("Should synthesize from tool observations")
	}

	// Should contain one of the tool observations
	if !contains(result, "sunny") && !contains(result, "25 degrees") {
		t.Errorf("Result should contain tool observations, got '%s'", result)
	}
}

func TestReAct_SynthesizeAnswerFromHistory_SkipsErrors(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	messages := []core.Message{
		{Role: "tool", Content: "Error: tool failed"},
		{Role: "tool", Content: "Valid observation here and it is definitely longer than 20 characters"},
	}

	result := react.synthesizeAnswerFromHistory(messages)

	// Should not include error messages
	if contains(result, "Error:") {
		t.Error("Should skip error messages in synthesis")
	}

	if !contains(result, "Valid observation") {
		t.Errorf("Should include valid observation, got '%s'", result)
	}
}

func TestReAct_SynthesizeAnswerFromHistory_DeduplicatesObservations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	duplicateObs := "This is a long observation that will be duplicated to test deduplication"
	messages := []core.Message{
		{Role: "tool", Content: duplicateObs},
		{Role: "tool", Content: duplicateObs}, // Duplicate
		{Role: "tool", Content: "Different observation that is also long enough to be considered"},
	}

	result := react.synthesizeAnswerFromHistory(messages)

	// Should only have unique observations (up to 3)
	// Count occurrences of duplicate string
	count := 0
	content := result
	for i := 0; i < len(content); {
		idx := strings.Index(content[i:], "duplicated")
		if idx == -1 {
			break
		}
		count++
		i += idx + 1
	}

	if count > 1 {
		t.Errorf("Should deduplicate observations, found %d occurrences", count)
	}
}

func TestReAct_SynthesizeAnswerFromHistory_LimitsToThreeObservations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	messages := []core.Message{
		{Role: "tool", Content: "First observation is definitely longer than twenty characters"},
		{Role: "tool", Content: "Second observation is definitely longer than twenty characters"},
		{Role: "tool", Content: "Third observation is definitely longer than twenty characters"},
		{Role: "tool", Content: "Fourth observation is definitely longer than twenty characters"},
		{Role: "tool", Content: "Fifth observation is definitely longer than twenty characters"},
	}

	result := react.synthesizeAnswerFromHistory(messages)

	// Should use most recent 3 unique observations
	if contains(result, "First") && contains(result, "Second") {
		t.Error("Should limit to 3 most recent observations")
	}
}

func TestReAct_SynthesizeAnswerFromHistory_SkipsShortObservations(t *testing.T) {
	react := NewReAct(core.NewSignature("Test"), &MockLM{}, []core.Tool{})

	messages := []core.Message{
		{Role: "tool", Content: "short"},
		{Role: "tool", Content: "This is a longer observation that should be included"},
	}

	result := react.synthesizeAnswerFromHistory(messages)

	if contains(result, "short") && !contains(result, "longer observation") {
		t.Errorf("Should skip observations <= 20 chars, got '%s'", result)
	}
}

// TestReAct_ExtractionWithReasoning verifies that runExtract uses reasoning adapter
// and attaches rationale to the prediction when hitting MaxIterations
func TestReAct_ExtractionWithReasoning(t *testing.T) {
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("confidence", core.FieldTypeInt, "Confidence score")

	iterationCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			iterationCount++

			// Always return tool calls to force hitting MaxIterations
			// Use different queries to avoid stagnation detection
			if len(options.Tools) > 0 {
				query := fmt.Sprintf("test query %d", iterationCount)
				return &core.GenerateResult{
					Content: "Using search tool",
					ToolCalls: []core.ToolCall{
						{
							ID:   fmt.Sprintf("call_%d", iterationCount),
							Name: "search",
							Arguments: map[string]any{
								"query": query,
							},
						},
					},
				}, nil
			}

			// No tools mode (final mode or extraction)
			// During final mode (iteration 2): return malformed JSON to force extraction
			// During extraction (iteration 3): return proper JSON with reasoning
			if iterationCount == 2 {
				// Return malformed JSON that will fail parsing and trigger extraction
				return &core.GenerateResult{
					Content: "I'm thinking about it but not formatting correctly",
				}, nil
			}

			// Extraction phase (iteration 3): return proper answer with reasoning
			return &core.GenerateResult{
				Content: `{
					"rationale": "Based on all the tool observations, I can now provide the final answer.",
					"answer": "The answer based on search results",
					"confidence": 95
				}`,
			}, nil
		},
	}

	callNumber := 0
	searchTool := core.NewTool(
		"search",
		"Search for information",
		func(ctx context.Context, args map[string]any) (any, error) {
			callNumber++
			return fmt.Sprintf("Search results %d: relevant information", callNumber), nil
		},
	).AddParameter("query", "string", "Search query", true)

	react := NewReAct(sig, lm, []core.Tool{*searchTool}).
		WithMaxIterations(2).
		WithVerbose(false)

	result, err := react.Forward(context.Background(), map[string]any{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Should have hit MaxIterations and triggered extraction
	// 2 tool-using iterations + 1 extraction call = 3 total
	if iterationCount < 3 {
		t.Errorf("Expected at least 3 LM calls (2 iterations + extraction), got %d", iterationCount)
	}

	// Check that answer was extracted
	answer, ok := result.GetString("answer")
	if !ok {
		t.Error("Expected answer field in result")
	}
	if !contains(answer, "answer based on search") {
		t.Errorf("Expected answer to contain extracted text, got: %s", answer)
	}

	// CRITICAL: Check that rationale was attached to prediction
	if result.Rationale == "" {
		t.Error("Expected non-empty rationale from extraction phase with reasoning adapter")
	}
	if !contains(result.Rationale, "tool observations") {
		t.Errorf("Expected rationale to contain reasoning, got: %s", result.Rationale)
	}

	// Verify rationale was removed from outputs (not part of signature)
	if _, exists := result.Outputs["rationale"]; exists {
		t.Error("Rationale should be removed from outputs map")
	}
	if _, exists := result.Outputs["reasoning"]; exists {
		t.Error("Reasoning should be removed from outputs map")
	}
}

// TestReAct_WithMethods tests all ReAct configuration methods
func TestReAct_WithMethods(t *testing.T) {
	sig := core.NewSignature("test").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	lm := &MockLM{}
	tools := []core.Tool{}
	history := core.NewHistory()
	demos := []core.Example{
		*core.NewExample(
			map[string]any{"question": "test"},
			map[string]any{"answer": "test"},
		),
	}
	adapter := core.NewJSONAdapter()

	react := NewReAct(sig, lm, tools).
		WithAdapter(adapter).
		WithHistory(history).
		WithDemos(demos)

	if react.Adapter != adapter {
		t.Error("WithAdapter should set adapter")
	}
	if react.History != history {
		t.Error("WithHistory should set history")
	}
	if len(react.Demos) != 1 {
		t.Error("WithDemos should set demos")
	}
}
