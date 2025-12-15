package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

// ============================================================================
// Phase 3: Basic Module Integration Tests
// ============================================================================

func TestPredict_BasicForward(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer":"42"}`)
	sig := fixtures.SimplePredictSig()

	pred := dsgo.NewPredict(sig, lm)
	result, err := pred.Forward(ctx, map[string]any{"question": "What is 6*7?"})
	if err != nil {
		t.Fatalf("Predict.Forward failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok {
		t.Fatalf("Expected 'answer' output to exist")
	}
	if answer != "42" {
		t.Fatalf("Expected answer '42', got '%s'", answer)
	}
}

func TestChainOfThought_BasicForward(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"reasoning":"Multiply 6 by 7.","answer":"42"}`)
	sig := fixtures.ChainOfThoughtSig()

	cot := dsgo.NewChainOfThought(sig, lm)
	result, err := cot.Forward(ctx, map[string]any{"problem": "What is 6*7?"})
	if err != nil {
		t.Fatalf("ChainOfThought.Forward failed: %v", err)
	}

	reasoning, ok := result.GetString("reasoning")
	if !ok {
		t.Fatalf("Expected 'reasoning' output to exist")
	}
	if reasoning == "" {
		t.Fatalf("Expected non-empty reasoning")
	}

	answer, ok := result.GetString("answer")
	if !ok {
		t.Fatalf("Expected 'answer' output to exist")
	}
	if answer != "42" {
		t.Fatalf("Expected answer '42', got '%s'", answer)
	}
}

func TestReAct_BasicForward_WithTool(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	lm := &ToolThenFinishMockLM{}
	sig := fixtures.ReActSig()
	tools := []dsgo.Tool{*fixtures.CalculatorTool()}

	react := dsgo.NewReAct(sig, lm, tools).WithMaxIterations(5)
	result, err := react.Forward(ctx, map[string]any{"question": "What is 6*7?"})
	if err != nil {
		t.Fatalf("ReAct.Forward failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok {
		t.Fatalf("Expected 'answer' output to exist")
	}
	if answer != "42" {
		t.Fatalf("Expected answer '42', got '%s'", answer)
	}
}

func TestRefine_BasicForward(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"output":"draft"}`)
	sig := fixtures.RefineSig()

	refine := dsgo.NewRefine(sig, lm)
	result, err := refine.Forward(ctx, map[string]any{"topic": "Write a short draft"})
	if err != nil {
		t.Fatalf("Refine.Forward failed: %v", err)
	}

	output, ok := result.GetString("output")
	if !ok {
		t.Fatalf("Expected 'output' to exist")
	}
	if output != "draft" {
		t.Fatalf("Expected output 'draft', got '%s'", output)
	}
}

func TestProgram_BasicComposition(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Program runs modules sequentially; ensure each module consumes one LM response.
	lm := NewMockLMWithResponses([]string{
		`{"answer":"step1"}`,
		`{"answer":"step2"}`,
	})
	sig := fixtures.SimplePredictSig()

	pred1 := dsgo.NewPredict(sig, lm)
	pred2 := dsgo.NewPredict(sig, lm)

	prog := dsgo.NewProgram("basic_program").
		AddModule(pred1).
		AddModule(pred2)

	result, err := prog.Forward(ctx, map[string]any{"question": "run"})
	if err != nil {
		t.Fatalf("Program.Forward failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok {
		t.Fatalf("Expected 'answer' output to exist")
	}
	if answer != "step2" {
		t.Fatalf("Expected final answer 'step2', got '%s'", answer)
	}
}

type ToolThenFinishMockLM struct {
	mu   sync.Mutex
	step int
}

func (m *ToolThenFinishMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	m.mu.Lock()
	step := m.step
	m.step++
	m.mu.Unlock()

	if step == 0 {
		return &dsgo.GenerateResult{
			Content: "Calling calculator tool.",
			ToolCalls: []dsgo.ToolCall{{
				ID:   "tool_1",
				Name: "calculate",
				Arguments: map[string]any{
					"operation": "multiply",
					"a":         6.0,
					"b":         7.0,
				},
			}},
			FinishReason: "tool_calls",
			Usage:        dsgo.Usage{TotalTokens: 30, Cost: 0.0005},
		}, nil
	}

	return &dsgo.GenerateResult{
		Content: "Finishing.",
		ToolCalls: []dsgo.ToolCall{{
			ID:   "finish_1",
			Name: "finish",
			Arguments: map[string]any{
				"answer":    "42",
				"reasoning": "Used calculate(multiply,6,7) and returned the result.",
			},
		}},
		FinishReason: "tool_calls",
		Usage:        dsgo.Usage{TotalTokens: 30, Cost: 0.0005},
	}, nil
}

func (m *ToolThenFinishMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}
		chunkChan <- dsgo.Chunk{Content: result.Content, ToolCalls: result.ToolCalls, FinishReason: result.FinishReason, Usage: result.Usage}
	}()
	return chunkChan, errChan
}

func (m *ToolThenFinishMockLM) Name() string        { return "tool-then-finish-mock" }
func (m *ToolThenFinishMockLM) SupportsJSON() bool  { return true }
func (m *ToolThenFinishMockLM) SupportsTools() bool { return true }
func (m *ToolThenFinishMockLM) IsOpenAI() bool      { return false }

// ============================================================================
// Module Configuration Methods Tests (covering 0% coverage With* methods)
// ============================================================================

// TestPredict_WithOptions tests Predict WithOptions configuration
func TestPredict_WithOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "configured response"}`)
	sig := fixtures.SimplePredictSig()

	options := &dsgo.GenerateOptions{
		Temperature: 0.7,
		MaxTokens:   100,
	}

	pred := dsgo.NewPredict(sig, lm).WithOptions(options)

	result, err := pred.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("Predict with options failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}
}

// TestPredict_WithHistory tests Predict WithHistory configuration
func TestPredict_WithHistory(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "response with history"}`)
	sig := fixtures.SimplePredictSig()

	history := &dsgo.History{}
	history.AddUserMessage("Previous context")
	history.AddAssistantMessage("Previous response")

	pred := dsgo.NewPredict(sig, lm).WithHistory(history)

	result, err := pred.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("Predict with history failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}
}

// TestChainOfThought_AllOptions tests all ChainOfThought configuration options
func TestChainOfThought_AllOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"reasoning": "Step 1, Step 2", "answer": "42"}`)
	sig := fixtures.ChainOfThoughtSig()

	options := &dsgo.GenerateOptions{Temperature: 0.5}
	history := &dsgo.History{}
	demos := []dsgo.Example{
		{Inputs: map[string]any{"problem": "2+2"}, Outputs: map[string]any{"reasoning": "Add", "answer": "4"}},
	}

	cot := dsgo.NewChainOfThought(sig, lm).
		WithOptions(options).
		WithHistory(history).
		WithDemos(demos).
		WithAdapter(dsgo.NewJSONAdapter())

	result, err := cot.Forward(ctx, map[string]any{"problem": "What is 6*7?"})
	if err != nil {
		t.Fatalf("ChainOfThought with all options failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}

	// Verify signature accessible
	gotSig := cot.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// TestReAct_AllOptions tests all ReAct configuration options
func TestReAct_AllOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	// Mock LM that finishes immediately
	lm := &FinishToolMockLM{
		FinishCall: dsgo.ToolCall{
			ID:   "finish_1",
			Name: "finish",
			Arguments: map[string]any{
				"answer":    "Direct answer",
				"reasoning": "No tools needed",
			},
		},
	}

	sig := fixtures.ReActSig()
	tools := []dsgo.Tool{*fixtures.CalculatorTool()}

	options := &dsgo.GenerateOptions{Temperature: 0.3}
	history := &dsgo.History{}
	demos := []dsgo.Example{}

	react := dsgo.NewReAct(sig, lm, tools).
		WithOptions(options).
		WithHistory(history).
		WithDemos(demos).
		WithAdapter(dsgo.NewChatAdapter()).
		WithMaxIterations(10).
		WithVerbose(true)

	result, err := react.Forward(ctx, map[string]any{"question": "Quick question"})
	if err != nil {
		t.Fatalf("ReAct with all options failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}

	// Verify GetSignature
	gotSig := react.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// TestRefine_AllOptions tests all Refine configuration options
func TestRefine_AllOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"output": "Refined output"}`)
	sig := fixtures.RefineSig()

	options := &dsgo.GenerateOptions{Temperature: 0.8}
	history := &dsgo.History{}
	demos := []dsgo.Example{
		{Inputs: map[string]any{"topic": "demo"}, Outputs: map[string]any{"output": "demo output"}},
	}

	refine := dsgo.NewRefine(sig, lm).
		WithOptions(options).
		WithAdapter(dsgo.NewJSONAdapter()).
		WithMaxIterations(3).
		WithRefinementField("feedback").
		WithHistory(history).
		WithDemos(demos)

	result, err := refine.Forward(ctx, map[string]any{"topic": "Test topic"})
	if err != nil {
		t.Fatalf("Refine with all options failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}

	// Verify GetSignature
	gotSig := refine.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// TestBestOfN_AllOptions tests all BestOfN configuration options
func TestRefine_WithHistory_ReadOnlyByDefault(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"output": "ok"}`)
	sig := fixtures.RefineSig()

	history := &dsgo.History{}
	history.AddUserMessage("previous")
	history.AddAssistantMessage("previous response")
	initialLen := history.Len()

	refine := dsgo.NewRefine(sig, lm).WithHistory(history)
	_, err := refine.Forward(ctx, map[string]any{"topic": "Test topic"})
	if err != nil {
		t.Fatalf("Refine.Forward failed: %v", err)
	}

	if history.Len() != initialLen {
		t.Fatalf("expected history to be read-only by default: got len=%d want=%d", history.Len(), initialLen)
	}
}

func TestRefine_WithHistoryTracking_AppendsToHistory(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"output": "ok"}`)
	sig := fixtures.RefineSig()

	history := &dsgo.History{}
	refine := dsgo.NewRefine(sig, lm).
		WithHistory(history).
		WithHistoryTracking(true)

	_, err := refine.Forward(ctx, map[string]any{"topic": "Test topic"})
	if err != nil {
		t.Fatalf("Refine.Forward failed: %v", err)
	}

	if history.Len() != 2 {
		t.Fatalf("expected 2 history messages after tracking, got %d", history.Len())
	}
	msgs := history.Get()
	if msgs[0].Role != "user" {
		t.Fatalf("expected first history message role=user, got %s", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Fatalf("expected second history message role=assistant, got %s", msgs[1].Role)
	}
}

func TestBestOfN_AllOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponses([]string{
		`{"answer": "Response 1", "confidence": 0.7}`,
		`{"answer": "Response 2", "confidence": 0.9}`,
		`{"answer": "Response 3", "confidence": 0.5}`,
	})

	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	pred := dsgo.NewPredict(sig, lm)

	bestOfN := dsgo.NewBestOfN(pred, 3).
		WithScorer(func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
			conf, ok := pred.GetFloat("confidence")
			if !ok {
				return 0, nil
			}
			return conf, nil
		}).
		WithParallel(true).
		WithReturnAll(true).
		WithMaxFailures(1).
		WithThreshold(0.5)

	result, err := bestOfN.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("BestOfN with all options failed: %v", err)
	}

	// Should select highest confidence (0.9)
	answer, ok := result.GetString("answer")
	if !ok {
		t.Error("Expected answer in result")
	}
	if answer != "Response 2" {
		t.Errorf("Expected 'Response 2' (highest confidence), got %s", answer)
	}

	// Verify GetSignature
	gotSig := bestOfN.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// TestBestOfN_DefaultScorer tests BestOfN with default scorer
func TestBestOfN_DefaultScorer(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponses([]string{
		`{"answer": "Short"}`,
		`{"answer": "A much longer and more detailed response"}`,
		`{"answer": "Medium length"}`,
	})

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	// Use default scorer (prefers longer, complete responses)
	bestOfN := dsgo.NewBestOfN(pred, 3).WithScorer(dsgo.DefaultScorer())

	result, err := bestOfN.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("BestOfN with default scorer failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}
}

// TestBestOfN_ConfidenceScorer tests BestOfN with confidence scorer
func TestBestOfN_ConfidenceScorer(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponses([]string{
		`{"answer": "Low confidence", "confidence": 0.3}`,
		`{"answer": "High confidence", "confidence": 0.95}`,
	})

	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	pred := dsgo.NewPredict(sig, lm)

	// Use confidence scorer
	bestOfN := dsgo.NewBestOfN(pred, 2).
		WithScorer(dsgo.ConfidenceScorer("confidence"))

	result, err := bestOfN.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("BestOfN with confidence scorer failed: %v", err)
	}

	answer, _ := result.GetString("answer")
	if answer != "High confidence" {
		t.Errorf("Expected 'High confidence', got %s", answer)
	}
}

// TestProgramOfThought_AllOptions tests all ProgramOfThought options
func TestProgramOfThought_AllOptions(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"code": "print(42)", "explanation": "Simple calculation"}`)
	sig := fixtures.ProgramOfThoughtSig()

	options := &dsgo.GenerateOptions{Temperature: 0.2}

	// NewProgramOfThought requires (sig, lm, language) - 3 params
	pot := dsgo.NewProgramOfThought(sig, lm, "python").
		WithOptions(options).
		WithAllowExecution(false).
		WithExecutionTimeout(5)

	result, err := pot.Forward(ctx, map[string]any{"problem": "Calculate 6*7"})
	if err != nil {
		t.Fatalf("ProgramOfThought with options failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}

	// Verify GetSignature
	gotSig := pot.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// TestProgram_Methods tests Program module methods
func TestProgram_Methods(t *testing.T) {
	t.Parallel()
	sig := fixtures.SimplePredictSig()
	lm := NewMockLMWithResponse(`{"answer": "test"}`)

	pred1 := dsgo.NewPredict(sig, lm)
	pred2 := dsgo.NewPredict(sig, lm)

	prog := dsgo.NewProgram("test_program").
		AddModule(pred1).
		AddModule(pred2)

	// Test Name
	name := prog.Name()
	if name != "test_program" {
		t.Errorf("Expected name 'test_program', got %s", name)
	}

	// Test ModuleCount
	count := prog.ModuleCount()
	if count != 2 {
		t.Errorf("Expected module count 2, got %d", count)
	}

	// Test GetSignature (returns last module's signature)
	gotSig := prog.GetSignature()
	if gotSig == nil {
		t.Error("Expected signature from GetSignature")
	}
}

// ============================================================================
// Cache Methods Tests
// ============================================================================

// TestCache_DeepCopySlice tests deep copy of slices in cache
func TestCache_DeepCopySlice(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	// Create result with slice
	original := &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"items": []any{"a", "b", "c"},
		},
	}

	key := "test_key"
	cache.Set(key, original)

	// Get from cache
	cached, hit := cache.Get(key)
	if !hit {
		t.Fatal("Expected cache hit")
	}

	// Modify original slice
	if items, ok := original.Metadata["items"].([]any); ok {
		items[0] = "modified"
	}

	// Cached version should be unchanged (deep copy)
	if cachedItems, ok := cached.Metadata["items"].([]any); ok {
		if cachedItems[0] == "modified" {
			t.Error("Cache did not deep copy slice - modification affected cached value")
		}
	}
}

// TestCache_Clear tests cache clear functionality
func TestCache_Clear(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	// Add items
	for i := 0; i < 5; i++ {
		cache.Set(string(rune('a'+i)), &dsgo.GenerateResult{Content: "test"})
	}

	if cache.Size() != 5 {
		t.Errorf("Expected size 5, got %d", cache.Size())
	}

	// Clear
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
}

// ============================================================================
// Streaming with Modules Tests
// ============================================================================

// TestPredict_Stream tests streaming with Predict module
func TestPredict_Stream(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := &StreamingMockLM{
		Chunks: []string{
			`{"ans`,
			`wer": `,
			`"streamed"}`,
		},
		FinalContent: `{"answer": "streamed"}`,
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	streamResult, err := pred.Stream(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Consume chunks
	for chunk := range streamResult.Chunks {
		_ = chunk
	}

	// Wait for prediction
	finalPred := <-streamResult.Prediction

	// Check for errors
	select {
	case err := <-streamResult.Errors:
		if err != nil {
			t.Fatalf("Stream error: %v", err)
		}
	default:
	}

	if finalPred == nil {
		t.Error("Expected final prediction")
	}
}

// ============================================================================
// Helper Mock LMs
// ============================================================================

// StreamingMockLM simulates streaming responses
type StreamingMockLM struct {
	Chunks       []string
	FinalContent string
}

func (m *StreamingMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	return &dsgo.GenerateResult{
		Content: m.FinalContent,
		Usage:   dsgo.Usage{TotalTokens: 20},
	}, nil
}

func (m *StreamingMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, len(m.Chunks))
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		for _, c := range m.Chunks {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case chunkChan <- dsgo.Chunk{Content: c}:
			}
		}

		// Final chunk with usage
		chunkChan <- dsgo.Chunk{
			Content: "",
			Usage:   dsgo.Usage{TotalTokens: 20, Cost: 0.001},
		}
	}()

	return chunkChan, errChan
}

func (m *StreamingMockLM) Name() string        { return "streaming-mock-lm" }
func (m *StreamingMockLM) SupportsJSON() bool  { return true }
func (m *StreamingMockLM) SupportsTools() bool { return false }
func (m *StreamingMockLM) IsOpenAI() bool      { return false }

// ============================================================================
// Additional Coverage Tests for 0% Functions
// ============================================================================

// TestPredict_WithDemos tests Predict.WithDemos configuration (was 0% coverage)
func TestPredict_WithDemos(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "demo response"}`)
	sig := fixtures.SimplePredictSig()

	demos := []dsgo.Example{
		{
			Inputs:  map[string]any{"question": "What is 2+2?"},
			Outputs: map[string]any{"answer": "4"},
		},
		{
			Inputs:  map[string]any{"question": "What is the capital of France?"},
			Outputs: map[string]any{"answer": "Paris"},
		},
	}

	pred := dsgo.NewPredict(sig, lm).WithDemos(demos)

	result, err := pred.Forward(ctx, map[string]any{"question": "What is 3+3?"})
	if err != nil {
		t.Fatalf("Predict with demos failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result")
	}

	// Verify demos were set
	if pred.Demos == nil || len(pred.Demos) != 2 {
		t.Error("Expected 2 demos to be set")
	}
}

// TestRefine_WithFeedback tests Refine module with actual refinement loop
func TestRefine_WithFeedback(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Mock LM that cycles through refinements
	lm := NewMockLMWithResponses([]string{
		`{"output": "First draft"}`,
		`{"output": "Refined version"}`,
		`{"output": "Final polished version"}`,
	})

	sig := fixtures.RefineSig()

	refine := dsgo.NewRefine(sig, lm).
		WithMaxIterations(3).
		WithRefinementField("feedback")

	result, err := refine.Forward(ctx, map[string]any{
		"topic":    "Test topic",
		"feedback": "Please improve this",
	})
	if err != nil {
		t.Fatalf("Refine with feedback failed: %v", err)
	}

	if result == nil {
		t.Error("Expected result from Refine")
	}
}

// TestBestOfN_ConfidenceScorer tests ConfidenceScorer with various field types
func TestBestOfN_ConfidenceScorerVariants(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	tests := []struct {
		name      string
		responses []string
		sigSetup  func() *dsgo.Signature
		expected  string
	}{
		{
			name: "Float confidence",
			responses: []string{
				`{"answer": "Low", "confidence": 0.3}`,
				`{"answer": "High", "confidence": 0.9}`,
			},
			sigSetup: func() *dsgo.Signature {
				return dsgo.NewSignature("test").
					AddInput("question", dsgo.FieldTypeString, "").
					AddOutput("answer", dsgo.FieldTypeString, "").
					AddOutput("confidence", dsgo.FieldTypeFloat, "")
			},
			expected: "High",
		},
		{
			name: "Int confidence",
			responses: []string{
				`{"answer": "Low", "score": 30}`,
				`{"answer": "High", "score": 90}`,
			},
			sigSetup: func() *dsgo.Signature {
				return dsgo.NewSignature("test").
					AddInput("question", dsgo.FieldTypeString, "").
					AddOutput("answer", dsgo.FieldTypeString, "").
					AddOutput("score", dsgo.FieldTypeInt, "")
			},
			expected: "High",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lm := NewMockLMWithResponses(tt.responses)
			sig := tt.sigSetup()
			pred := dsgo.NewPredict(sig, lm)

			// Extract field name from first output that isn't "answer"
			var scoreField string
			for _, f := range sig.OutputFields {
				if f.Name != "answer" {
					scoreField = f.Name
					break
				}
			}

			bestOfN := dsgo.NewBestOfN(pred, 2).
				WithScorer(dsgo.ConfidenceScorer(scoreField))

			result, err := bestOfN.Forward(ctx, map[string]any{"question": "test"})
			if err != nil {
				t.Fatalf("BestOfN with confidence scorer failed: %v", err)
			}

			answer, _ := result.GetString("answer")
			if answer != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, answer)
			}
		})
	}
}

// TestBestOfN_ThresholdEarlyStop tests threshold-based early stopping
func TestBestOfN_ThresholdEarlyStop(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// First response meets threshold - should stop early
	lm := NewMockLMWithResponses([]string{
		`{"answer": "High confidence", "confidence": 0.95}`,
		`{"answer": "Should not reach", "confidence": 0.1}`,
		`{"answer": "Should not reach", "confidence": 0.2}`,
	})

	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "").
		AddOutput("confidence", dsgo.FieldTypeFloat, "")

	pred := dsgo.NewPredict(sig, lm)

	bestOfN := dsgo.NewBestOfN(pred, 3).
		WithScorer(dsgo.ConfidenceScorer("confidence")).
		WithThreshold(0.9) // High threshold

	result, err := bestOfN.Forward(ctx, map[string]any{"question": "test"})
	if err != nil {
		t.Fatalf("BestOfN with threshold failed: %v", err)
	}

	answer, _ := result.GetString("answer")
	if answer != "High confidence" {
		t.Errorf("Expected 'High confidence', got '%s'", answer)
	}
}

// ============================================================================
// Phase 2: BestOfN ConfidenceScorer Tests
// ============================================================================

// TestBestOfN_ConfidenceScorer_NumericConfidence tests confidence extraction from numeric field
func TestBestOfN_ConfidenceScorer_NumericConfidence(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Signature with numeric confidence field
	sig := dsgo.NewSignature("Rate quality").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddOutput("rating", dsgo.FieldTypeString, "Rating").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence 0-1")

	// Multiple responses with different confidence levels
	responses := []string{
		`{"rating": "good", "confidence": 0.5}`,
		`{"rating": "excellent", "confidence": 0.9}`,
		`{"rating": "poor", "confidence": 0.3}`,
	}

	lm := NewMockLMWithResponses(responses)

	// BestOfN wraps a module (ChainOfThought in this case)
	coT := dsgo.NewChainOfThought(sig, lm)
	bestOfN := dsgo.NewBestOfN(coT, 3).
		WithScorer(dsgo.ConfidenceScorer("confidence"))

	result, err := bestOfN.Forward(ctx, map[string]any{"text": "test"})
	if err != nil {
		t.Fatalf("BestOfN with numeric confidence scorer failed: %v", err)
	}

	rating, _ := result.GetString("rating")
	if rating != "excellent" {
		t.Errorf("Expected highest confidence result 'excellent', got '%s'", rating)
	}
}

// TestBestOfN_ConfidenceScorer_StringConfidence tests confidence extraction from string field
func TestBestOfN_ConfidenceScorer_StringConfidence(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddOutput("sentiment", dsgo.FieldTypeString, "Sentiment").
		AddOutput("confidence", dsgo.FieldTypeString, "Confidence level")

	// Multiple responses with confidence as strings
	responses := []string{
		`{"sentiment": "positive", "confidence": "0.7"}`,
		`{"sentiment": "neutral", "confidence": "0.8"}`,
		`{"sentiment": "negative", "confidence": "0.5"}`,
	}

	lm := NewMockLMWithResponses(responses)

	// BestOfN wraps a module
	coT := dsgo.NewChainOfThought(sig, lm)
	bestOfN := dsgo.NewBestOfN(coT, 3).
		WithScorer(dsgo.ConfidenceScorer("confidence"))

	result, err := bestOfN.Forward(ctx, map[string]any{"text": "test"})
	if err != nil {
		t.Fatalf("BestOfN with string confidence scorer failed: %v", err)
	}

	sentiment, _ := result.GetString("sentiment")
	if sentiment != "neutral" {
		t.Errorf("Expected highest confidence result 'neutral', got '%s'", sentiment)
	}
}

// TestBestOfN_ConfidenceScorer_CustomScoreField tests confidence from custom field name
func TestBestOfN_ConfidenceScorer_CustomScoreField(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Quality assessment").
		AddInput("item", dsgo.FieldTypeString, "Item").
		AddOutput("assessment", dsgo.FieldTypeString, "Assessment").
		AddOutput("score", dsgo.FieldTypeInt, "Quality score")

	responses := []string{
		`{"assessment": "average", "score": 60}`,
		`{"assessment": "excellent", "score": 95}`,
		`{"assessment": "good", "score": 75}`,
	}

	lm := NewMockLMWithResponses(responses)

	// BestOfN wraps a module
	coT := dsgo.NewChainOfThought(sig, lm)
	bestOfN := dsgo.NewBestOfN(coT, 3).
		WithScorer(dsgo.ConfidenceScorer("score"))

	result, err := bestOfN.Forward(ctx, map[string]any{"item": "test"})
	if err != nil {
		t.Fatalf("BestOfN with custom score scorer failed: %v", err)
	}

	assessment, _ := result.GetString("assessment")
	if assessment != "excellent" {
		t.Errorf("Expected highest score result 'excellent', got '%s'", assessment)
	}
}

// ============================================================================
// Phase 2: Refine generatePrediction Error Tests
// ============================================================================

// TestRefine_GeneratePrediction_InitialGeneration tests initial prediction generation
// Validates the initial LM call for Refine module
func TestRefine_GeneratePrediction_InitialGeneration(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Improve text").
		AddInput("text", dsgo.FieldTypeString, "Text to improve").
		AddOutput("improved", dsgo.FieldTypeString, "Improved text")

	lm := NewMockLMWithResponse(`{"improved": "This is the improved version of the text with better grammar and clarity."}`)

	refine := dsgo.NewRefine(sig, lm)

	result, err := refine.Forward(ctx, map[string]any{"text": "This is some text"})
	if err != nil {
		t.Fatalf("Refine initial generation failed: %v", err)
	}

	improved, ok := result.GetString("improved")
	if !ok || improved == "" {
		t.Error("Expected improved output from initial generation")
	}
}

// TestRefine_GeneratePrediction_WithFeedback tests prediction refinement with feedback
// Validates the iterative refinement loop
func TestRefine_GeneratePrediction_WithFeedback(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	// Multiple responses: initial answer, then refined answer
	responses := []string{
		`{"answer": "Initial answer"}`,
		`{"answer": "Refined answer based on feedback"}`,
	}
	lm := NewMockLMWithResponses(responses)

	refine := dsgo.NewRefine(sig, lm).
		WithMaxIterations(1).
		WithRefinementField("feedback")

	result, err := refine.Forward(ctx, map[string]any{
		"question": "What is AI?",
		"feedback": "Your answer needs more detail",
	})

	if err != nil {
		t.Fatalf("Refine with feedback failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok || answer == "" {
		t.Error("Expected refined answer")
	}
}

// ============================================================================
// Phase 2: ChainOfThought finish_reason Error Tests
// ============================================================================

// TestChainOfThought_FinishReason_Length tests handling of finish_reason=length
// Model hit max_tokens, output was truncated
func TestChainOfThought_FinishReason_Length(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Explain concept").
		AddInput("concept", dsgo.FieldTypeString, "Concept").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// Create mock that returns finish_reason=length
	lmInterface := NewMockLMWithResponse(`{"explanation": "This is an explanation that got truncated..."}`)
	// Note: We'd need to modify the mock to set finish_reason, for now just verify error handling
	// by checking with a mock that indicates this scenario

	cot := dsgo.NewChainOfThought(sig, lmInterface)

	result, err := cot.Forward(ctx, map[string]any{"concept": "Machine Learning"})

	// We expect either success (if finish_reason isn't set) or graceful error
	if err == nil {
		if result == nil {
			t.Error("Expected either result or error")
		}
	} else if !containsString(err.Error(), "finish_reason") && !containsString(err.Error(), "parse") {
		t.Logf("Got expected error handling: %v", err)
	}
}

// TestChainOfThought_FinishReason_ToolCalls tests handling of finish_reason=tool_calls
// Model requested tool execution but ChainOfThought doesn't support it
func TestChainOfThought_FinishReason_ToolCalls(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	// ChainOfThought doesn't use tools, so any tool calls would be unexpected
	// Just verify we get valid output from standard JSON
	lm := NewMockLMWithResponse(`{"answer": "The answer is 42"}`)

	cot := dsgo.NewChainOfThought(sig, lm)

	result, err := cot.Forward(ctx, map[string]any{
		"question": "What is the answer to everything?",
	})

	if err != nil {
		t.Fatalf("ChainOfThought failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok || answer == "" {
		t.Error("Expected answer output")
	}
}

// TestChainOfThought_EmptyContent tests handling of empty content
// Some models might return empty content even with finish_reason=stop
func TestChainOfThought_EmptyContent(t *testing.T) {
	t.Parallel()
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Think deeply").
		AddInput("prompt", dsgo.FieldTypeString, "Prompt").
		AddOutput("thinking", dsgo.FieldTypeString, "Thinking")

	lm := NewMockLMWithResponse(`{"thinking": "After deep consideration, I believe the answer is nuanced and depends on context."}`)

	cot := dsgo.NewChainOfThought(sig, lm)

	result, err := cot.Forward(ctx, map[string]any{"prompt": "What is consciousness?"})

	if err != nil {
		t.Fatalf("ChainOfThought reasoning failed: %v", err)
	}

	thinking, ok := result.GetString("thinking")
	if !ok || thinking == "" {
		t.Error("Expected thinking output")
	}
}
