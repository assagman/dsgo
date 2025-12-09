package module

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

// MockLM for testing
type mockLM struct {
	responses []string
	index     int
	mu        sync.Mutex
}

func (m *mockLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index >= len(m.responses) {
		m.index = 0 // Reset for testing
	}

	response := m.responses[m.index]
	m.index++

	return &core.GenerateResult{
		Content: response,
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			Cost:             0.001,
			Latency:          100,
		},
		FinishReason: "stop",
	}, nil
}

func (m *mockLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	// Not implemented for these tests
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)
	close(chunkChan)
	close(errChan)
	return chunkChan, errChan
}

func (m *mockLM) SupportsJSON() bool {
	return true
}

func (m *mockLM) SupportsTools() bool {
	return false
}

func (m *mockLM) IsOpenAI() bool {
	return false
}

func (m *mockLM) Name() string {
	return "mockLM"
}

func TestNewMultiChainComparison(t *testing.T) {
	// Create base signature
	sig := core.NewSignature("Test task").
		AddInput("question", core.FieldTypeString, "Question to answer").
		AddOutput("answer", core.FieldTypeString, "Answer to question")

	// Create mock LM
	lm := &mockLM{
		responses: []string{
			`{"rationale": "Best synthesis", "answer": "Final answer"}`,
		},
	}

	// Create MultiChainComparison
	mcc := NewMultiChainComparison(sig, lm, 3)

	// Test configuration
	if mcc.M != 3 {
		t.Errorf("Expected M=3, got %d", mcc.M)
	}

	if mcc.BaseSignature != sig {
		t.Error("BaseSignature not set correctly")
	}

	if mcc.internalSignature == nil {
		t.Error("internalSignature not set")
	}

	// Test signature transformation
	internalSig := mcc.internalSignature

	// Should have original input field
	foundInput := false
	for _, field := range internalSig.InputFields {
		if field.Name == "question" {
			foundInput = true
			break
		}
	}
	if !foundInput {
		t.Error("Original input field not found in internal signature")
	}

	// Should have reasoning attempt INPUT fields (not OUTPUT)
	reasoningCount := 0
	for _, field := range internalSig.InputFields {
		if strings.HasPrefix(field.Name, "reasoning_attempt_") {
			reasoningCount++
		}
	}
	if reasoningCount != 3 {
		t.Errorf("Expected 3 reasoning attempt INPUT fields, got %d", reasoningCount)
	}

	// Should have rationale as first OUTPUT field
	if len(internalSig.OutputFields) == 0 || internalSig.OutputFields[0].Name != "rationale" {
		t.Error("Rationale should be first output field")
	}

	// Should have original output field
	foundOutput := false
	for _, field := range internalSig.OutputFields {
		if field.Name == "answer" {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Error("Original output field not found in internal signature")
	}
}

func TestNewMultiChainComparison_Panics(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}

	// Test nil signature
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil signature")
		}
	}()
	NewMultiChainComparison(nil, lm, 3)

	// Test invalid m
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid m")
		}
	}()
	NewMultiChainComparison(sig, lm, 0)

	// Test signature with no outputs
	sigNoOutputs := core.NewSignature("Test").AddInput("input", core.FieldTypeString, "Input")
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for signature with no outputs")
		}
	}()
	NewMultiChainComparison(sigNoOutputs, lm, 3)
}

func TestMultiChainComparison_WithTemperature(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}

	mcc := NewMultiChainComparison(sig, lm, 3)
	mcc = mcc.WithTemperature(0.5)

	if mcc.Options.Temperature != 0.5 {
		t.Errorf("Expected temperature=0.5, got %f", mcc.Options.Temperature)
	}
}

func TestMultiChainComparison_WithAttemptTemplate(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}

	mcc := NewMultiChainComparison(sig, lm, 3)
	template := "Custom template: {rationale} -> {answer}"
	mcc = mcc.WithAttemptTemplate(template)

	if mcc.AttemptTemplate != template {
		t.Error("AttemptTemplate not set correctly")
	}
}

func TestMultiChainComparison_GetSignature(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}

	mcc := NewMultiChainComparison(sig, lm, 3)

	// GetSignature should return base signature, not internal
	returnedSig := mcc.GetSignature()
	if returnedSig != sig {
		t.Error("GetSignature should return base signature")
	}

	// Should not have reasoning attempt fields
	for _, field := range returnedSig.InputFields {
		if strings.HasPrefix(field.Name, "reasoning_attempt_") {
			t.Error("Returned signature should not have reasoning attempt fields")
		}
	}
}

func TestMultiChainComparison_Forward(t *testing.T) {
	sig := core.NewSignature("Answer question").
		AddInput("question", core.FieldTypeString, "Question").
		AddOutput("answer", core.FieldTypeString, "Answer")

	// Mock LM for synthesis
	synthesisLM := &mockLM{
		responses: []string{
			`{"rationale": "This is the best answer", "answer": "Best synthesized answer"}`,
		},
	}

	mcc := NewMultiChainComparison(sig, synthesisLM, 3)

	// Create mock completions
	completions := []*core.Prediction{
		core.NewPrediction(map[string]any{"answer": "Answer 1", "rationale": "Reasoning 1"}),
		core.NewPrediction(map[string]any{"answer": "Answer 2", "rationale": "Reasoning 2"}),
		core.NewPrediction(map[string]any{"answer": "Answer 3", "rationale": "Reasoning 3"}),
	}

	ctx := context.Background()
	inputs := map[string]any{
		"question":    "What is 2+2?",
		"completions": completions,
	}

	prediction, err := mcc.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if prediction.ModuleName != "MultiChainComparison" {
		t.Errorf("Expected module name 'MultiChainComparison', got '%s'", prediction.ModuleName)
	}

	// Should have answer field
	answer, exists := prediction.GetString("answer")
	if !exists {
		t.Error("Answer field not found in prediction")
	} else {
		if answer != "Best synthesized answer" {
			t.Errorf("Expected 'Best synthesized answer', got '%s'", answer)
		}
	}

	// Should have rationale
	rationale, exists := prediction.GetString("rationale")
	if !exists {
		t.Error("Rationale field not found in prediction")
	} else {
		if rationale != "This is the best answer" {
			t.Errorf("Expected 'This is the best answer', got '%s'", rationale)
		}
	}
}

func TestExtractCompletions(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}
	mcc := NewMultiChainComparison(sig, lm, 3)

	// Test []*Prediction format
	predictions := []*core.Prediction{
		core.NewPrediction(map[string]any{"output": "test1"}),
		core.NewPrediction(map[string]any{"output": "test2"}),
	}

	inputs := map[string]any{"completions": predictions}
	completions, err := mcc.extractCompletions(inputs)
	if err != nil {
		t.Fatalf("extractCompletions failed: %v", err)
	}
	if len(completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(completions))
	}

	// Test []map[string]any format
	maps := []map[string]any{
		{"output": "test1"},
		{"output": "test2"},
	}

	inputs = map[string]any{"completions": maps}
	completions, err = mcc.extractCompletions(inputs)
	if err != nil {
		t.Fatalf("extractCompletions failed: %v", err)
	}
	if len(completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(completions))
	}

	// Test []any format
	anys := []any{
		map[string]any{"output": "test1"},
		map[string]any{"output": "test2"},
	}

	inputs = map[string]any{"completions": anys}
	completions, err = mcc.extractCompletions(inputs)
	if err != nil {
		t.Fatalf("extractCompletions failed: %v", err)
	}
	if len(completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(completions))
	}

	// Test missing completions
	inputs = map[string]any{"input": "test"}
	_, err = mcc.extractCompletions(inputs)
	if err == nil {
		t.Error("Expected error for missing completions")
	}

	// Test wrong number of completions
	wrongPredictions := []*core.Prediction{
		core.NewPrediction(map[string]any{"output": "test1"}),
	}
	inputs = map[string]any{"completions": wrongPredictions}
	_, err = mcc.Forward(context.Background(), inputs)
	if err == nil {
		t.Error("Expected error for wrong number of completions")
	}
}

func TestFormatAttempt(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}
	mcc := NewMultiChainComparison(sig, lm, 3)

	// Test with rationale and answer
	completion := map[string]any{
		"rationale": "This is my reasoning",
		"answer":    "This is my answer",
	}

	formatted := mcc.formatAttempt(completion)
	expected := "I'm trying to This is my reasoning I'm not sure but my prediction is This is my answer"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}

	// Test with reasoning alias
	completion = map[string]any{
		"reasoning": "Alternative reasoning",
		"answer":    "Alternative answer",
	}

	formatted = mcc.formatAttempt(completion)
	expected = "I'm trying to Alternative reasoning I'm not sure but my prediction is Alternative answer"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}

	// Test with thought alias
	completion = map[string]any{
		"thought": "Another thought process",
		"answer":  "Another answer",
	}

	formatted = mcc.formatAttempt(completion)
	expected = "I'm trying to Another thought process I'm not sure but my prediction is Another answer"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}

	// Test with multiline rationale/answer
	completion = map[string]any{
		"rationale": "First line\nSecond line",
		"answer":    "First answer\nSecond answer",
	}

	formatted = mcc.formatAttempt(completion)
	expected = "I'm trying to First line I'm not sure but my prediction is First answer"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}
}

func TestFirstLine(t *testing.T) {
	// Test normal case
	if firstLine("hello world") != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", firstLine("hello world"))
	}

	// Test with newline
	if firstLine("first line\nsecond line") != "first line" {
		t.Errorf("Expected 'first line', got '%s'", firstLine("first line\nsecond line"))
	}

	// Test with leading/trailing whitespace
	if firstLine("  trimmed line  \nsecond line") != "trimmed line" {
		t.Errorf("Expected 'trimmed line', got '%s'", firstLine("  trimmed line  \nsecond line"))
	}

	// Test empty string
	if firstLine("") != "" {
		t.Errorf("Expected empty string, got '%s'", firstLine(""))
	}

	// Test with only whitespace
	if firstLine("   \n  ") != "" {
		t.Errorf("Expected empty string, got '%s'", firstLine("   \n  "))
	}
}

func TestMultiChainComparison_Clone(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{responses: []string{`{"output": "test"}`}}

	mcc := NewMultiChainComparison(sig, lm, 3)
	mcc = mcc.WithTemperature(0.8).WithAttemptTemplate("custom template")

	cloned := mcc.Clone().(*MultiChainComparison)

	// Test that cloned module has same configuration
	if cloned.M != mcc.M {
		t.Error("Cloned module has different M value")
	}

	if cloned.Options.Temperature != mcc.Options.Temperature {
		t.Error("Cloned module has different temperature")
	}

	if cloned.AttemptTemplate != mcc.AttemptTemplate {
		t.Error("Cloned module has different attempt template")
	}

	// Test that modules are independent
	if cloned.predict == mcc.predict {
		t.Error("Cloned module shares predict instance")
	}

	if cloned.BaseSignature != mcc.BaseSignature {
		t.Error("Cloned module should share base signature (same reference)")
	}
}

func TestMultiChainComparison_ConcurrentForward(t *testing.T) {
	sig := core.NewSignature("Test").
		AddInput("input", core.FieldTypeString, "Input").
		AddOutput("output", core.FieldTypeString, "Output")

	lm := &mockLM{
		responses: []string{
			`{"rationale": "rationale 1", "output": "output 1"}`,
			`{"rationale": "rationale 2", "output": "output 2"}`,
			`{"rationale": "rationale 3", "output": "output 3"}`,
			`{"rationale": "rationale 4", "output": "output 4"}`,
		},
	}

	mcc := NewMultiChainComparison(sig, lm, 3)

	// Create completions for each goroutine
	completions1 := []*core.Prediction{
		core.NewPrediction(map[string]any{"output": "test1", "rationale": "reason1"}),
		core.NewPrediction(map[string]any{"output": "test2", "rationale": "reason2"}),
		core.NewPrediction(map[string]any{"output": "test3", "rationale": "reason3"}),
	}

	completions2 := []*core.Prediction{
		core.NewPrediction(map[string]any{"output": "test4", "rationale": "reason4"}),
		core.NewPrediction(map[string]any{"output": "test5", "rationale": "reason5"}),
		core.NewPrediction(map[string]any{"output": "test6", "rationale": "reason6"}),
	}

	// Run concurrent forwards
	errChan := make(chan error, 2)

	go func() {
		inputs := map[string]any{
			"input":       "test1",
			"completions": completions1,
		}
		_, err := mcc.Forward(context.Background(), inputs)
		errChan <- err
	}()

	go func() {
		inputs := map[string]any{
			"input":       "test2",
			"completions": completions2,
		}
		_, err := mcc.Forward(context.Background(), inputs)
		errChan <- err
	}()

	// Check for errors
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent Forward failed: %v", err)
		}
	}
}
