package module

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

func TestReAct_Forward_NoTools(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++

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

	// Stagnation triggers early termination into extraction; no extra prompt injection.

	// Verify the model was called at least 3 times (2 tool calls + 1 final answer after stagnation)
	if callCount < 3 {
		t.Errorf("Expected at least 3 LM calls (stagnation + recovery), got %d", callCount)
	}
}

// TestReAct_Forward_WithHistory tests history management
func TestReAct_Forward_WithHistory(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// Loop call: finish is a termination signal.
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
			}
			// Extraction call: produce signature-valid JSON.
			return &core.GenerateResult{Content: `{"answer":"The answer is 42","confidence":0.95}`}, nil
		},
	}

	dummyTool := core.NewTool("dummy", "unused", func(ctx context.Context, args map[string]any) (any, error) {
		return "unused", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*dummyTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer?",
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "The answer is 42" {
		t.Errorf("expected answer, got %v", outputs.Outputs["answer"])
	}
	if outputs.Outputs["confidence"] != 0.95 {
		t.Errorf("expected confidence 0.95, got %v", outputs.Outputs["confidence"])
	}
	if callCount != 1 {
		t.Errorf("expected 1 LM call (finish args validated), got %d", callCount)
	}
}

// TestReAct_Forward_WithFinishTool_InvalidOutputs tests finish tool with validation errors
func TestReAct_Forward_WithFinishTool_InvalidOutputs(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("score", core.FieldTypeInt, "Score")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// First call: finish tool (possibly invalid args). The loop terminates and extraction produces outputs.
				return &core.GenerateResult{
					Content: "Trying to finish",
					ToolCalls: []core.ToolCall{
						{
							ID:        "finish-1",
							Name:      "finish",
							Arguments: map[string]interface{}{"answer": "incomplete"},
						},
					},
				}, nil
			}
			// Extraction call: proper final answer.
			return &core.GenerateResult{Content: `{"answer":"complete answer","score":85}`}, nil
		},
	}

	dummyTool := core.NewTool("dummy", "unused", func(ctx context.Context, args map[string]any) (any, error) {
		return "unused", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*dummyTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{"question": "test"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "complete answer" {
		t.Errorf("expected recovered answer, got %v", outputs.Outputs["answer"])
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (loop + extraction), got %d", callCount)
	}
}

// TestReAct_Forward_WithReasoning tests reasoning field extraction and cleanup
func TestReAct_Forward_WithReasoning(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// JSON format succeeds on first try (JSONAdapter is now first in fallback chain)
			return &core.GenerateResult{
				Content: `{"answer": "test"}`,
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

	// JSON format should succeed on first attempt (no fallback needed)
	if outputs.ParseAttempts != 1 {
		t.Errorf("Expected 1 parse attempt, got %d", outputs.ParseAttempts)
	}

	if outputs.FallbackUsed {
		t.Error("Expected fallback_used to be false for JSON format")
	}
}

// TestReAct_Forward_OutputValidationError tests validation errors after parsing
func TestReAct_Forward_OutputValidationError(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer").
		AddOutput("score", core.FieldTypeInt, "Required score")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// Loop call (implicit finish): missing required field.
				return &core.GenerateResult{Content: `{"answer":"incomplete"}`}, nil
			}
			// Extraction call - provide complete answer.
			return &core.GenerateResult{Content: `{"answer":"extracted answer","score":42}`}, nil
		},
	}

	dummyTool := core.NewTool("dummy", "unused", func(ctx context.Context, args map[string]any) (any, error) { return "unused", nil })

	react := NewReAct(sig, lm, []core.Tool{*dummyTool})
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

			// Check ToolChoice to determine mode (tools are now always present for provider compatibility)
			// ToolChoice == "auto" means tool-using mode, ToolChoice == "none" means final/extraction mode
			toolsEnabled := options.ToolChoice != "none" && len(options.Tools) > 0

			// Tool-using mode: return tool calls to force hitting MaxIterations
			// Use different queries to avoid stagnation detection
			if toolsEnabled {
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

// TestReAct_ImplicitFinish tests that ReAct accepts direct answers without tool calls.
// This validates the "Implicit Finish" pattern where the model provides a valid answer
// directly instead of using tools, which is correct behavior for native tool calling APIs.
func TestReAct_ImplicitFinish(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			// Model returns valid JSON without making any tool calls (implicit finish)
			return &core.GenerateResult{
				Content:   `{"answer": "42"}`,
				ToolCalls: []core.ToolCall{}, // Empty - no tool calls
			}, nil
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		t.Error("Tool should not be executed in implicit finish scenario")
		return "search result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool})
	result, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "What is the answer to life?",
	})

	// Verify: err == nil (success)
	if err != nil {
		t.Fatalf("Forward() error = %v, want nil", err)
	}

	// Direct answer was signature-valid; no extraction call needed.
	if callCount != 1 {
		t.Errorf("Expected 1 LM call, got %d", callCount)
	}

	// Verify: result.Outputs["answer"] == "42"
	if result.Outputs["answer"] != "42" {
		t.Errorf("Expected answer='42', got %v", result.Outputs["answer"])
	}
}

// TestReAct_ImplicitFinish_MalformedRetry tests the retry mechanism when implicit finish
// fails validation in early iterations. The model should be guided to use tools.
// Note: This test uses int fields to ensure malformed content fails validation,
// triggering the retry mechanism. String-only signatures would use text extraction
// as a fallback and accept malformed content.
func TestReAct_ImplicitFinish_MalformedRetry(t *testing.T) {
	t.Parallel()
	// Use an int output field so malformed text fails validation
	sig := core.NewSignature("Calculate something").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("count", core.FieldTypeInt, "Count result")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++

			if callCount == 1 {
				// First call: return malformed text without tool calls
				// This fails validation because "count" expects int, gets no valid int
				return &core.GenerateResult{
					Content:   "thinking about this problem without any numbers",
					ToolCalls: []core.ToolCall{},
				}, nil
			}
			// Second call: return valid JSON with int (recovery)
			return &core.GenerateResult{
				Content:   `{"count": 42}`,
				ToolCalls: []core.ToolCall{},
			}, nil
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool}).WithMaxIterations(5)
	result, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "What is the count?",
	})

	// Verify: err == nil (success after retry)
	if err != nil {
		t.Fatalf("Forward() error = %v, want nil", err)
	}

	// Loop terminates and extractor produces the final structured output.
	if callCount != 2 {
		t.Errorf("Expected 2 LM calls (loop + extraction), got %d", callCount)
	}

	// Verify: result.Outputs["count"] == 42
	count, ok := result.GetInt("count")
	if !ok || count != 42 {
		t.Errorf("Expected count=42, got %v", result.Outputs["count"])
	}
}

// TestReAct_WithMethods tests all ReAct configuration methods
func TestReAct_WithMethods(t *testing.T) {
	t.Parallel()
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

// TestReAct_UsageAccumulation tests that usage (tokens, cost, latency) accumulates correctly across multiple iterations
func TestReAct_UsageAccumulation(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		SupportsJSONVal:  true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			switch callCount {
			case 1:
				// Loop: tool call
				return &core.GenerateResult{
					Content:   "Let me search",
					ToolCalls: []core.ToolCall{{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}}},
					Usage: core.Usage{
						PromptTokens:     100,
						CompletionTokens: 50,
						TotalTokens:      150,
						Cost:             0.001,
						Latency:          500 * 1_000_000,
					},
				}, nil
			case 2:
				// Loop: invalid direct answer (forces extraction).
				return &core.GenerateResult{
					Content: `{"wrong":"field"}`,
					Usage: core.Usage{
						PromptTokens:     200,
						CompletionTokens: 100,
						TotalTokens:      300,
						Cost:             0.002,
						Latency:          600 * 1_000_000,
					},
				}, nil
			default:
				// Extraction: final structured answer.
				return &core.GenerateResult{
					Content: `{"answer":"final answer"}`,
					Usage: core.Usage{
						PromptTokens:     50,
						CompletionTokens: 25,
						TotalTokens:      75,
						Cost:             0.0005,
						Latency:          250 * 1_000_000,
					},
				}, nil
			}
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool})
	pred, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify usage accumulation across loop + extraction (3 LM calls)
	expectedPromptTokens := 100 + 200 + 50
	expectedCompletionTokens := 50 + 100 + 25
	expectedTotalTokens := 150 + 300 + 75
	expectedCost := 0.001 + 0.002 + 0.0005
	expectedLatency := 500 + 600 + 250

	if pred.Usage.PromptTokens != expectedPromptTokens {
		t.Errorf("PromptTokens: expected %d, got %d", expectedPromptTokens, pred.Usage.PromptTokens)
	}
	if pred.Usage.CompletionTokens != expectedCompletionTokens {
		t.Errorf("CompletionTokens: expected %d, got %d", expectedCompletionTokens, pred.Usage.CompletionTokens)
	}
	if pred.Usage.TotalTokens != expectedTotalTokens {
		t.Errorf("TotalTokens: expected %d, got %d", expectedTotalTokens, pred.Usage.TotalTokens)
	}
	if pred.Usage.Cost != expectedCost {
		t.Errorf("Cost: expected %.6f, got %.6f", expectedCost, pred.Usage.Cost)
	}

	// Ensure extraction answer wins.
	if pred.Outputs["answer"] != "final answer" {
		t.Errorf("expected final answer, got %v", pred.Outputs["answer"])
	}

	expectedLatencyNs := int64(expectedLatency) * 1_000_000
	if pred.Usage.Latency != expectedLatencyNs {
		t.Errorf("Latency: expected %d ns (%.2fms), got %d ns (%.2fms)",
			expectedLatencyNs, float64(expectedLatencyNs)/1_000_000,
			pred.Usage.Latency, float64(pred.Usage.Latency)/1_000_000)
	}
}

// TestReAct_ToolsSliceNotMutated tests that the caller's tools slice is not mutated by NewReAct
func TestReAct_ToolsSliceNotMutated(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	lm := &MockLM{}

	// Create tools slice with specific capacity to detect append mutation
	originalTools := make([]core.Tool, 1, 10) // capacity > len to allow in-place append
	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "result", nil
	})
	originalTools[0] = *searchTool

	// Capture original length
	originalLen := len(originalTools)

	// Create ReAct which auto-injects finish tool
	_ = NewReAct(sig, lm, originalTools)

	// Verify caller's slice was NOT modified
	if len(originalTools) != originalLen {
		t.Errorf("Caller's tools slice was mutated: expected len %d, got %d", originalLen, len(originalTools))
	}

	// Verify finish tool was NOT appended to caller's slice
	for _, tool := range originalTools {
		if strings.ToLower(tool.Name) == "finish" {
			t.Error("Finish tool should not appear in caller's original tools slice")
		}
	}
}

// TestReAct_NonToolLM_NoToolsPassedInFinalMode tests that tools are not passed to LMs that don't support them in final mode
func TestReAct_NonToolLM_NoToolsPassedInFinalMode(t *testing.T) {
	t.Parallel()
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	var finalModeOptions *core.GenerateOptions
	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: false, // LM does NOT support tools
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				// First call: return something that doesn't parse well to trigger iteration
				return &core.GenerateResult{
					Content: "I need to think about this...",
				}, nil
			}
			if callCount == 2 {
				// Second call: still no good answer, will trigger final mode
				return &core.GenerateResult{
					Content: "Still thinking...",
				}, nil
			}
			// Third call onward: final mode - capture options here
			finalModeOptions = options
			return &core.GenerateResult{
				Content: `{"answer": "final answer"}`,
			}, nil
		},
	}

	searchTool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "result", nil
	})

	react := NewReAct(sig, lm, []core.Tool{*searchTool}).WithMaxIterations(4)
	_, err := react.Forward(context.Background(), map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	// Verify final mode options: tools should NOT be passed since LM doesn't support them
	if finalModeOptions != nil {
		if len(finalModeOptions.Tools) > 0 {
			t.Errorf("Tools should not be passed to non-tool LM in final mode, got %d tools", len(finalModeOptions.Tools))
		}
		if finalModeOptions.ToolChoice == "none" {
			t.Errorf("ToolChoice should not be 'none' for non-tool LM, got %q", finalModeOptions.ToolChoice)
		}
	}
}

// ============================================================================
// reactTrajectory tests
// ============================================================================

func TestReActTrajectory_Render_EmptySteps(t *testing.T) {
	t.Parallel()
	base := []core.Message{{Role: "system", Content: "You are helpful."}}
	traj := newReActTrajectory(base)

	msgs := traj.Render(1000)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected system message, got %s", msgs[0].Role)
	}
}

func TestReActTrajectory_Render_AlwaysIncludesNewestStep(t *testing.T) {
	t.Parallel()
	base := []core.Message{{Role: "system", Content: "base"}}
	traj := newReActTrajectory(base)

	// Add a step that exceeds the budget by itself.
	largeThought := string(make([]byte, 500))
	traj.AddStep(largeThought, nil)

	// Budget is 100 bytes, base is ~4 bytes, but newest step should still be included.
	msgs := traj.Render(100)
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages (base + newest step), got %d", len(msgs))
	}
}

func TestReActTrajectory_Render_SelectsSuffix(t *testing.T) {
	t.Parallel()
	base := []core.Message{{Role: "system", Content: "x"}}
	traj := newReActTrajectory(base)

	// Add 5 steps, each ~10 bytes.
	for i := 0; i < 5; i++ {
		traj.AddStep("thought123", nil)
	}

	// Budget allows base (~1 byte) + ~30 bytes = ~3 steps.
	msgs := traj.Render(35)

	// Should get base + some suffix of steps.
	// We expect at least 2 messages (base + at least 1 step).
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages, got %d", len(msgs))
	}

	// The last message should be from the newest step (step 5).
	found := false
	for _, m := range msgs {
		if m.Content == "thought123" && m.Role == "assistant" {
			found = true
		}
	}
	if !found {
		t.Error("expected newest step content to be present")
	}
}

func TestReActTrajectory_Render_BaseExceedsBudget(t *testing.T) {
	t.Parallel()
	// Base itself exceeds budget.
	largeBase := []core.Message{{Role: "system", Content: string(make([]byte, 1000))}}
	traj := newReActTrajectory(largeBase)
	traj.AddStep("small", nil)

	// Budget is only 100 bytes, base is 1000 bytes.
	msgs := traj.Render(100)

	// Should only return base, no steps (remaining budget is <= 0).
	if len(msgs) != 1 {
		t.Errorf("expected only base message, got %d messages", len(msgs))
	}
}

func TestReActTrajectory_DropOldestSteps(t *testing.T) {
	t.Parallel()
	base := []core.Message{{Role: "system", Content: "x"}}
	traj := newReActTrajectory(base)

	for i := 0; i < 5; i++ {
		traj.AddStep("step", nil)
	}

	// Drop 2 oldest.
	dropped := traj.DropOldestSteps(2)
	if dropped != 2 {
		t.Errorf("expected 2 dropped, got %d", dropped)
	}

	// Should have 3 steps remaining.
	msgs := traj.Render(100000)
	// Base (1) + 3 steps (3 assistant messages) = 4.
	if len(msgs) != 4 {
		t.Errorf("expected 4 messages, got %d", len(msgs))
	}
}

func TestReActTrajectory_DropOldestSteps_MoreThanAvailable(t *testing.T) {
	t.Parallel()
	traj := newReActTrajectory(nil)
	traj.AddStep("s1", nil)
	traj.AddStep("s2", nil)

	dropped := traj.DropOldestSteps(10)
	if dropped != 2 {
		t.Errorf("expected 2 dropped (all available), got %d", dropped)
	}

	msgs := traj.Render(100000)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after dropping all, got %d", len(msgs))
	}
}

func TestReActTrajectory_DropOldestSteps_Zero(t *testing.T) {
	t.Parallel()
	traj := newReActTrajectory(nil)
	traj.AddStep("s1", nil)

	dropped := traj.DropOldestSteps(0)
	if dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", dropped)
	}
}

func TestReActTrajectory_HasToolContent(t *testing.T) {
	t.Parallel()

	t.Run("no tool content", func(t *testing.T) {
		traj := newReActTrajectory(nil)
		traj.AddStep("thought only", nil)
		if traj.HasToolContent() {
			t.Error("expected no tool content")
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		traj := newReActTrajectory(nil)
		traj.AddStep("thought", []core.ToolCall{{ID: "1", Name: "search"}})
		if !traj.HasToolContent() {
			t.Error("expected tool content from tool calls")
		}
	})

	t.Run("with tool results", func(t *testing.T) {
		traj := newReActTrajectory(nil)
		step := traj.AddStep("thought", nil)
		step.AddToolResult(reactToolResult{ToolCallID: "1", Content: "result"})
		if !traj.HasToolContent() {
			t.Error("expected tool content from tool results")
		}
	})
}

func TestReActStep_ToMessages(t *testing.T) {
	t.Parallel()
	step := &reactStep{
		Thought:   "I'm thinking",
		ToolCalls: []core.ToolCall{{ID: "1", Name: "search"}},
		ToolResults: []reactToolResult{
			{ToolCallID: "1", Content: `{"tool":"search","ok":true}`},
		},
		Errors: []string{"error message"},
	}

	msgs := step.toMessages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", msgs[0].Role)
	}
	if msgs[1].Role != "tool" {
		t.Errorf("expected tool role, got %s", msgs[1].Role)
	}
	if msgs[2].Role != "system" {
		t.Errorf("expected system role for error, got %s", msgs[2].Role)
	}
}

// ============================================================================
// encodeToolResult tests
// ============================================================================

func TestEncodeToolResult_SmallResult(t *testing.T) {
	t.Parallel()
	content, truncated, hash := encodeToolResult("search", "call-1", "hello", nil, 1000)

	if truncated {
		t.Error("expected not truncated")
	}

	var env toolResultEnvelope
	if err := json.Unmarshal([]byte(content), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Tool != "search" {
		t.Errorf("expected tool=search, got %s", env.Tool)
	}
	if !env.OK {
		t.Error("expected OK=true")
	}
	if env.Result != "hello" {
		t.Errorf("expected result=hello, got %v", env.Result)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestEncodeToolResult_WithError(t *testing.T) {
	t.Parallel()
	content, _, _ := encodeToolResult("search", "call-1", nil, errors.New("tool failed"), 1000)

	var env toolResultEnvelope
	if err := json.Unmarshal([]byte(content), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.OK {
		t.Error("expected OK=false for error")
	}
	if env.Error != "tool failed" {
		t.Errorf("expected error message, got %s", env.Error)
	}
}

func TestEncodeToolResult_Truncation(t *testing.T) {
	t.Parallel()
	largeResult := string(make([]byte, 10000))
	content, truncated, _ := encodeToolResult("search", "call-1", largeResult, nil, 200)

	if !truncated {
		t.Error("expected truncated=true")
	}
	if len(content) > 200 {
		t.Errorf("expected content <= 200 bytes, got %d", len(content))
	}

	var env toolResultEnvelope
	if err := json.Unmarshal([]byte(content), &env); err != nil {
		t.Fatalf("truncated result should still be valid JSON: %v", err)
	}
	if !env.Truncated {
		t.Error("expected Truncated=true in envelope")
	}
	if env.OriginalSize == 0 {
		t.Error("expected OriginalSize to be set")
	}
}

func TestEncodeToolResult_StableHashIgnoresToolCallID(t *testing.T) {
	t.Parallel()
	// Same tool, same result, different tool_call_id should produce same hash.
	_, _, hash1 := encodeToolResult("search", "call-1", "result", nil, 1000)
	_, _, hash2 := encodeToolResult("search", "call-2", "result", nil, 1000)

	if hash1 != hash2 {
		t.Errorf("expected same hash for same content, different call IDs: %s vs %s", hash1, hash2)
	}
}

func TestEncodeToolResult_DifferentResultsDifferentHashes(t *testing.T) {
	t.Parallel()
	_, _, hash1 := encodeToolResult("search", "call-1", "result1", nil, 1000)
	_, _, hash2 := encodeToolResult("search", "call-1", "result2", nil, 1000)

	if hash1 == hash2 {
		t.Error("expected different hashes for different results")
	}
}

func TestEncodeToolResult_UTF8SafeTruncation(t *testing.T) {
	t.Parallel()
	// Use a multi-byte UTF-8 character.
	result := "Hello 世界 🌍"
	content, _, _ := encodeToolResult("search", "call-1", result, nil, 50)

	var env toolResultEnvelope
	if err := json.Unmarshal([]byte(content), &env); err != nil {
		t.Fatalf("should produce valid JSON: %v", err)
	}
}

func TestEncodeToolResult_MinimalEnvelope(t *testing.T) {
	t.Parallel()
	// Extremely small budget.
	content, truncated, _ := encodeToolResult("x", "1", "huge data", nil, 50)

	if !truncated {
		t.Error("expected truncated")
	}

	var env toolResultEnvelope
	if err := json.Unmarshal([]byte(content), &env); err != nil {
		t.Fatalf("should produce valid JSON even with tiny budget: %v", err)
	}
}

// ============================================================================
// reactTermination tests
// ============================================================================

func TestReActTermination_MarkDone_Idempotent(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	term.MarkDone(terminationRepeatedToolCall)
	term.MarkDone(terminationStagnation) // Should be ignored.

	if term.Reason() != terminationRepeatedToolCall {
		t.Errorf("expected reason %s, got %s", terminationRepeatedToolCall, term.Reason())
	}
}

func TestReActTermination_ObserveToolCall_RepeatedTriggers(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	tc := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test"}}

	// First call: sets fingerprint.
	term.ObserveToolCall(tc)
	if term.ShouldStop() {
		t.Error("should not stop after first call")
	}

	// Second call: increments count to 1.
	term.ObserveToolCall(tc)
	if term.ShouldStop() {
		t.Error("should not stop after second call")
	}

	// Third call: count becomes 2, triggers termination.
	term.ObserveToolCall(tc)
	if !term.ShouldStop() {
		t.Error("should stop after third repeated call")
	}
	if term.Reason() != terminationRepeatedToolCall {
		t.Errorf("expected reason %s, got %s", terminationRepeatedToolCall, term.Reason())
	}
}

func TestReActTermination_ObserveToolCall_DifferentCallsReset(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	tc1 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test1"}}
	tc2 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test2"}}

	term.ObserveToolCall(tc1)
	term.ObserveToolCall(tc1) // repeat count = 1
	term.ObserveToolCall(tc2) // Different call, resets.

	if term.ShouldStop() {
		t.Error("should not stop after changing tool call args")
	}
}

func TestReActTermination_ObserveToolResult_Stagnation(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	tc := core.ToolCall{Name: "search"}
	hash := "same-hash-123"

	// First result: sets hash.
	term.ObserveToolResult(tc, hash, nil)
	if term.ShouldStop() {
		t.Error("should not stop after first observation")
	}

	// Second result: increments.
	term.ObserveToolResult(tc, hash, nil)
	if term.ShouldStop() {
		t.Error("should not stop after second observation")
	}

	// Third result: triggers stagnation.
	term.ObserveToolResult(tc, hash, nil)
	if !term.ShouldStop() {
		t.Error("should stop after third repeated observation")
	}
	if term.Reason() != terminationStagnation {
		t.Errorf("expected reason %s, got %s", terminationStagnation, term.Reason())
	}
}

func TestReActTermination_ObserveError_RepeatedErrors(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	term.ObserveError(errors.New("err1"))
	if term.ShouldStop() {
		t.Error("should not stop after first error")
	}

	term.ObserveError(errors.New("err2"))
	if !term.ShouldStop() {
		t.Error("should stop after second consecutive error")
	}
	if term.Reason() != terminationRepeatedErrors {
		t.Errorf("expected reason %s, got %s", terminationRepeatedErrors, term.Reason())
	}
}

func TestReActTermination_ObserveToolResult_ErrorResetsOnSuccess(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	tc := core.ToolCall{Name: "search"}

	term.ObserveToolResult(tc, "h1", errors.New("err"))
	term.ObserveToolResult(tc, "h2", nil) // Success resets error count.
	term.ObserveToolResult(tc, "h3", errors.New("err"))

	if term.ShouldStop() {
		t.Error("should not stop; error count was reset by success")
	}
}

func TestReActTermination_SetFinalToolArgs(t *testing.T) {
	t.Parallel()
	term := newReActTermination()

	args := map[string]any{"answer": "42"}
	term.SetFinalToolArgs(args)

	// Modify original to ensure copy was made.
	args["answer"] = "modified"

	if term.FinalToolArgs()["answer"] != "42" {
		t.Error("expected args to be copied, not referenced")
	}
}

func TestReActTermination_SetFinalContent(t *testing.T) {
	t.Parallel()
	term := newReActTermination()
	term.SetFinalContent(`{"answer":"42"}`)

	if term.FinalContent() != `{"answer":"42"}` {
		t.Errorf("unexpected final content: %s", term.FinalContent())
	}
}

// ============================================================================
// generateWithContextRetry / isContextOverflowError tests
// ============================================================================

func TestIsContextOverflowError_Sentinel(t *testing.T) {
	t.Parallel()
	err := contextLengthSentinel{}
	if !isContextOverflowError(err) {
		t.Error("expected sentinel to be detected as overflow")
	}
}

func TestIsContextOverflowError_Patterns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg  string
		want bool
	}{
		{"context_length_exceeded", true},
		{"maximum context length", true},
		{"max context length exceeded", true},
		{"maximum context window exceeded", true},
		{"context window exceeded", true},
		{"too many tokens", true},
		{"exceeded the context", true},
		{"please reduce the length of the messages", true},
		{"prompt is too long", true},
		{"tokens exceeded", true},
		{"input is too long", true},
		{"some other error", false},
		{"network timeout", false},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			err := errors.New(tc.msg)
			if got := isContextOverflowError(err); got != tc.want {
				t.Errorf("isContextOverflowError(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestIsContextOverflowError_Nil(t *testing.T) {
	t.Parallel()
	if isContextOverflowError(nil) {
		t.Error("expected nil to not be overflow error")
	}
}

func TestGenerateWithContextRetry_NoOverflow(t *testing.T) {
	t.Parallel()
	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			return &core.GenerateResult{Content: "success"}, nil
		},
	}

	r := &ReAct{LM: lm, MaxPromptBytes: 10000}
	traj := newReActTrajectory([]core.Message{{Role: "user", Content: "hi"}})

	result, err := r.generateWithContextRetry(context.Background(), traj, &core.GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "success" {
		t.Errorf("unexpected content: %s", result.Content)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestGenerateWithContextRetry_OverflowRetry(t *testing.T) {
	t.Parallel()
	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			if callCount < 3 {
				return nil, contextLengthSentinel{}
			}
			return &core.GenerateResult{Content: "success after retry"}, nil
		},
	}

	r := &ReAct{LM: lm, MaxPromptBytes: 10000}
	traj := newReActTrajectory([]core.Message{{Role: "user", Content: "hi"}})
	traj.AddStep("step1", nil)
	traj.AddStep("step2", nil)
	traj.AddStep("step3", nil)

	result, err := r.generateWithContextRetry(context.Background(), traj, &core.GenerateOptions{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "success after retry" {
		t.Errorf("unexpected content: %s", result.Content)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestGenerateWithContextRetry_ExhaustedRetries(t *testing.T) {
	t.Parallel()
	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			return nil, contextLengthSentinel{}
		},
	}

	r := &ReAct{LM: lm, MaxPromptBytes: 10000}
	traj := newReActTrajectory([]core.Message{{Role: "user", Content: "hi"}})
	traj.AddStep("step1", nil)
	traj.AddStep("step2", nil)
	traj.AddStep("step3", nil)

	_, err := r.generateWithContextRetry(context.Background(), traj, &core.GenerateOptions{}, nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// Should have tried reactContextOverflowMaxRetries (3) times.
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestGenerateWithContextRetry_NoStepsToDropReturnsImmediately(t *testing.T) {
	t.Parallel()
	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			return nil, contextLengthSentinel{}
		},
	}

	r := &ReAct{LM: lm, MaxPromptBytes: 10000}
	traj := newReActTrajectory([]core.Message{{Role: "user", Content: "hi"}})
	// No steps added - nothing to drop.

	_, err := r.generateWithContextRetry(context.Background(), traj, &core.GenerateOptions{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should return immediately after first failure since no steps to drop.
	if callCount != 1 {
		t.Errorf("expected 1 call (immediate return), got %d", callCount)
	}
}

func TestGenerateWithContextRetry_NonOverflowErrorReturnsImmediately(t *testing.T) {
	t.Parallel()
	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callCount++
			return nil, errors.New("network error")
		},
	}

	r := &ReAct{LM: lm, MaxPromptBytes: 10000}
	traj := newReActTrajectory([]core.Message{{Role: "user", Content: "hi"}})
	traj.AddStep("step1", nil)

	_, err := r.generateWithContextRetry(context.Background(), traj, &core.GenerateOptions{}, nil)
	if err == nil || err.Error() != "network error" {
		t.Fatalf("expected network error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (immediate return for non-overflow error), got %d", callCount)
	}
}

// ============================================================================
// Helper: approxMessagesBytes / approxBytes tests
// ============================================================================

func TestApproxMessagesBytes(t *testing.T) {
	t.Parallel()
	msgs := []core.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}

	bytes := approxMessagesBytes(msgs)
	if bytes != 10 { // "hello" (5) + "world" (5)
		t.Errorf("expected 10 bytes, got %d", bytes)
	}
}

func TestApproxMessagesBytes_WithToolCalls(t *testing.T) {
	t.Parallel()
	msgs := []core.Message{
		{
			Role:    "assistant",
			Content: "x",
			ToolCalls: []core.ToolCall{
				{ID: "1", Name: "search", Arguments: map[string]any{"q": "test"}},
			},
		},
	}

	bytes := approxMessagesBytes(msgs)
	// Content "x" (1 byte) + JSON of tool calls.
	if bytes <= 1 {
		t.Errorf("expected > 1 byte due to tool calls, got %d", bytes)
	}
}

func TestReActStep_ApproxBytes(t *testing.T) {
	t.Parallel()
	step := &reactStep{
		Thought: "thinking",
		ToolResults: []reactToolResult{
			{Content: "result"},
		},
		Errors: []string{"error"},
	}

	bytes := step.approxBytes()
	// "thinking" (8) + "result" (6) + "error" (5) = 19
	if bytes != 19 {
		t.Errorf("expected 19 bytes, got %d", bytes)
	}
}

// ============================================================================
// toolFingerprint tests
// ============================================================================

func TestToolFingerprint_SameCallsSameFingerprint(t *testing.T) {
	t.Parallel()
	tc1 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test"}}
	tc2 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test"}}

	fp1 := toolFingerprint(tc1)
	fp2 := toolFingerprint(tc2)

	if fp1 != fp2 {
		t.Errorf("expected same fingerprint for identical calls: %s vs %s", fp1, fp2)
	}
}

func TestToolFingerprint_DifferentArgsDifferentFingerprint(t *testing.T) {
	t.Parallel()
	tc1 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test1"}}
	tc2 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test2"}}

	fp1 := toolFingerprint(tc1)
	fp2 := toolFingerprint(tc2)

	if fp1 == fp2 {
		t.Error("expected different fingerprints for different args")
	}
}

func TestToolFingerprint_IncludesToolName(t *testing.T) {
	t.Parallel()
	tc1 := core.ToolCall{Name: "search", Arguments: map[string]any{"q": "test"}}
	tc2 := core.ToolCall{Name: "calculate", Arguments: map[string]any{"q": "test"}}

	fp1 := toolFingerprint(tc1)
	fp2 := toolFingerprint(tc2)

	if fp1 == fp2 {
		t.Error("expected different fingerprints for different tool names")
	}
}
