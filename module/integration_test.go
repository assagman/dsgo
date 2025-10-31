package module

import (
	"context"
	"strings"
	"testing"

	"github.com/assagman/dsgo"
)

// TestPredict_WithHistory tests multi-turn conversation
func TestPredict_WithHistory(t *testing.T) {
	sig := dsgo.NewSignature("Answer questions").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	history := dsgo.NewHistory()
	history.AddSystemMessage("You are a helpful assistant.")

	var capturedMessages []dsgo.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			capturedMessages = messages
			return &dsgo.GenerateResult{
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
	lm.GenerateFunc = func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
		capturedMessages = messages
		return &dsgo.GenerateResult{
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
	sig := dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to classify").
		AddOutput("sentiment", dsgo.FieldTypeString, "positive or negative")

	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{"text": "I love this product!"},
			map[string]any{"sentiment": "positive"},
		),
		*dsgo.NewExample(
			map[string]any{"text": "This is terrible."},
			map[string]any{"sentiment": "negative"},
		),
	}

	var capturedMessages []dsgo.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			capturedMessages = messages
			return &dsgo.GenerateResult{
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
	sig := dsgo.NewSignature("Classify").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddOutput("category", dsgo.FieldTypeString, "Category")

	history := dsgo.NewHistory()
	history.AddSystemMessage("You are a classifier.")

	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{"text": "apple"},
			map[string]any{"category": "fruit"},
		),
	}

	var capturedMessages []dsgo.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			capturedMessages = messages
			return &dsgo.GenerateResult{
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
	sig := dsgo.NewSignature("Solve problems").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	history := dsgo.NewHistory()

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
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
	sig := dsgo.NewSignature("Solve math problems").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{"problem": "1+1"},
			map[string]any{"answer": "2"},
		),
	}

	var capturedMessages []dsgo.Message
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			capturedMessages = messages
			return &dsgo.GenerateResult{
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
	sig := dsgo.NewSignature("Test").
		AddInput("input", dsgo.FieldTypeString, "Input").
		AddOutput("output", dsgo.FieldTypeString, "Output").
		AddOutput("status", dsgo.FieldTypeString, "Status")

	history := dsgo.NewHistory()
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
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
