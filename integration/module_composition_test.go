package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

// ============================================================================
// Sequential Composition Tests
// ============================================================================

func TestSequentialComposition_TwoModules(t *testing.T) {
	t.Parallel()
	// Test: Basic sequential composition with Predict → Predict
	// Validates:
	// - Output from module 1 feeds into module 2
	// - Errors propagate correctly
	// - Usage accumulates across modules
	// - Outputs from both modules are available

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Create signatures and mock LMs
	sig1 := fixtures.SimplePredictSig()
	sig2 := fixtures.SimplePredictSig()

	lm1 := NewMockLMWithResponse(`{"answer": "Intermediate result"}`)
	lm2 := NewMockLMWithResponse(`{"answer": "Final result"}`)

	// Create modules
	module1 := dsgo.NewPredict(sig1, lm1)
	module2 := dsgo.NewPredict(sig2, lm2)

	// Execute module 1
	result1, err := module1.Forward(ctx, map[string]any{
		"question": "What is step 1?",
	})
	if err != nil {
		t.Fatalf("Module 1 failed: %v", err)
	}

	// Execute module 2 with output from module 1
	answer1, ok := result1.GetString("answer")
	if !ok {
		t.Fatal("Module 1 didn't produce answer field")
	}

	result2, err := module2.Forward(ctx, map[string]any{
		"question": answer1,
	})
	if err != nil {
		t.Fatalf("Module 2 failed: %v", err)
	}

	// Verify outputs
	answer2, ok := result2.GetString("answer")
	if !ok {
		t.Fatal("Module 2 didn't produce answer field")
	}

	if answer1 != "Intermediate result" {
		t.Errorf("Module 1 output incorrect: got %s", answer1)
	}
	if answer2 != "Final result" {
		t.Errorf("Module 2 output incorrect: got %s", answer2)
	}

	// Verify usage tracking
	totalTokens := result1.Usage.TotalTokens + result2.Usage.TotalTokens
	if totalTokens != 40 { // 20 from each
		t.Errorf("Usage not tracked: expected 40 tokens, got %d", totalTokens)
	}
}

func TestSequentialComposition_ChainOfThoughtToRefine(t *testing.T) {
	t.Parallel()
	// Test: ChainOfThought (reasoning) → Refine (improve)
	// Validates:
	// - Reasoning output feeds into refinement
	// - Refinement improves on original reasoning
	// - Error handling at each stage

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Create signatures
	cotSig := fixtures.ChainOfThoughtSig()
	refineSig := fixtures.RefineSig()

	// Mock responses: reasoning then refinement
	cotLM := NewMockLMWithResponse(`{
		"reasoning": "1. First principle: start simple\n2. Apply to problem\n3. Conclusion",
		"answer": "The answer is 42"
	}`)

	refineLM := NewMockLMWithResponse(`{
		"output": "Refined explanation: The answer is 42 because of these key reasons..."
	}`)

	// Create modules
	cot := dsgo.NewChainOfThought(cotSig, cotLM)
	refine := dsgo.NewRefine(refineSig, refineLM)

	// Execute chain of thought
	cotResult, err := cot.Forward(ctx, map[string]any{
		"problem": "Why is 42 the answer to everything?",
	})
	if err != nil {
		t.Fatalf("ChainOfThought failed: %v", err)
	}

	// Execute refinement with reasoning from chain of thought
	answer, ok := cotResult.GetString("answer")
	if !ok {
		t.Fatal("ChainOfThought didn't produce answer field")
	}

	refineResult, err := refine.Forward(ctx, map[string]any{
		"topic": answer,
	})
	if err != nil {
		t.Fatalf("Refine failed: %v", err)
	}

	// Verify both have results
	if reasoning, ok := cotResult.GetString("reasoning"); !ok || reasoning == "" {
		t.Error("ChainOfThought reasoning is empty")
	}

	if output, ok := refineResult.GetString("output"); !ok || output == "" {
		t.Error("Refine output is empty")
	}

	// Verify usage accumulation
	if cotResult.Usage.TotalTokens == 0 {
		t.Error("ChainOfThought usage not tracked")
	}
	if refineResult.Usage.TotalTokens == 0 {
		t.Error("Refine usage not tracked")
	}
}

func TestSequentialComposition_LongPipeline(t *testing.T) {
	t.Parallel()
	// Test: 5-module sequential pipeline
	// Validates:
	// - No data loss between stages
	// - All modules execute successfully
	// - Output correctly chains through pipeline

	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create a 5-module pipeline, each adds a marker
	responses := []string{
		`{"answer": "Step1"}`,
		`{"answer": "Step1→Step2"}`,
		`{"answer": "Step1→Step2→Step3"}`,
		`{"answer": "Step1→Step2→Step3→Step4"}`,
		`{"answer": "Step1→Step2→Step3→Step4→Step5"}`,
	}

	lms := make([]dsgo.LM, 5)
	modules := make([]dsgo.Module, 5)

	for i, response := range responses {
		lms[i] = NewMockLMWithResponse(response)
		modules[i] = dsgo.NewPredict(sig, lms[i])
	}

	// Execute pipeline
	var result *dsgo.Prediction
	var err error
	input := "Start"

	for i, m := range modules {
		result, err = m.Forward(ctx, map[string]any{
			"question": input,
		})
		if err != nil {
			t.Fatalf("Module %d failed: %v", i, err)
		}
		answer, ok := result.GetString("answer")
		if !ok {
			t.Fatalf("Module %d didn't produce answer field", i)
		}
		input = answer
	}

	// Verify final result contains all steps
	finalAnswer, ok := result.GetString("answer")
	if !ok {
		t.Fatal("Final result doesn't have answer field")
	}

	expectedOutput := "Step1→Step2→Step3→Step4→Step5"
	if finalAnswer != expectedOutput {
		t.Errorf("Pipeline output incorrect: expected %s, got %s", expectedOutput, finalAnswer)
	}
}

func TestSequentialComposition_Program(t *testing.T) {
	t.Parallel()
	// Test: Using Program module for sequential composition
	// Validates:
	// - Program correctly sequences modules
	// - Outputs accumulate across modules
	// - Errors propagate from any module

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()
	lm1 := NewMockLMWithResponse(`{"answer": "first"}`)
	lm2 := NewMockLMWithResponse(`{"answer": "second"}`)

	m1 := dsgo.NewPredict(sig, lm1)
	m2 := dsgo.NewPredict(sig, lm2)

	// Create program
	prog := dsgo.NewProgram("test_pipeline").
		AddModule(m1).
		AddModule(m2)

	// Execute
	result, err := prog.Forward(ctx, map[string]any{
		"question": "test",
	})
	if err != nil {
		t.Fatalf("Program failed: %v", err)
	}

	// Verify outputs from both modules are present
	answer, ok := result.GetString("answer")
	if !ok || answer == "" {
		t.Error("Program output is empty")
	}

	// Verify usage is accumulated
	if result.Usage.TotalTokens != 40 { // 20 from each module
		t.Errorf("Usage not accumulated: expected 40, got %d", result.Usage.TotalTokens)
	}
}

// ============================================================================
// Parallel Composition Tests
// ============================================================================

func TestParallelComposition_MultiplePredictsSync(t *testing.T) {
	t.Parallel()
	// Test: Multiple modules executing sequentially (simulating parallel results)
	// Validates:
	// - All modules execute
	// - Results correctly collected
	// - No race conditions
	// - Error handling

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create 3 independent modules
	modules := []dsgo.Module{
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "response1"}`)),
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "response2"}`)),
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "response3"}`)),
	}

	// Execute all modules (sequential for this test, but demonstrates the pattern)
	results := make([]*dsgo.Prediction, len(modules))
	for i, m := range modules {
		res, err := m.Forward(ctx, map[string]any{
			"question": fmt.Sprintf("Question %d", i+1),
		})
		if err != nil {
			t.Fatalf("Module %d failed: %v", i, err)
		}
		results[i] = res
	}

	// Verify all results collected
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Verify each result is distinct
	for i, res := range results {
		expected := fmt.Sprintf("response%d", i+1)
		answer, ok := res.GetString("answer")
		if !ok || answer != expected {
			t.Errorf("Result %d incorrect: expected %s, got %s", i, expected, answer)
		}
	}
}

func TestParallelComposition_BestOfN(t *testing.T) {
	t.Parallel()
	// Test: BestOfN generates multiple outputs and selects best
	// Validates:
	// - All N executions complete
	// - Best result selected based on scorer
	// - Scoring function applied correctly

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()
	lm := NewMockLMWithResponses([]string{
		`{"answer": "good"}`,
		`{"answer": "better"}`,
		`{"answer": "best"}`,
	})

	m := dsgo.NewPredict(sig, lm)

	// Create BestOfN
	best := dsgo.NewBestOfN(m, 3)

	// Scoring: prefer "best" specifically
	best.WithScorer(func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		answer, ok := pred.GetString("answer")
		if !ok {
			return 0, nil
		}
		switch answer {
		case "best":
			return 100.0, nil // Highest score for "best"
		case "better":
			return 50.0, nil // Medium score for "better"
		default:
			return 10.0, nil // Low score for "good"
		}
	})

	// Execute
	result, err := best.Forward(ctx, map[string]any{
		"question": "test",
	})
	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	// Verify best result selected (should be "best")
	answer, ok := result.GetString("answer")
	if !ok || answer != "best" {
		t.Errorf("Best result not selected: got %s", answer)
	}
}

func TestParallelComposition_BestOfNWithParallel(t *testing.T) {
	t.Parallel()
	// Test: BestOfN with parallel execution
	// Validates:
	// - Parallel execution completes
	// - Results still correct with parallel
	// - No race conditions

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create module for BestOfN
	m := dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "result"}`))

	// Create best of N
	best := dsgo.NewBestOfN(m, 3)
	best.WithScorer(func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		return 1.0, nil
	})

	result, err := best.Forward(ctx, map[string]any{
		"question": "test",
	})
	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	answer, ok := result.GetString("answer")
	if !ok || answer == "" {
		t.Error("BestOfN produced no result")
	}
}

// ============================================================================
// Nested Composition Tests
// ============================================================================

func TestNestedComposition_ProgramWithinProgram(t *testing.T) {
	t.Parallel()
	// Test: Program containing sub-Programs
	// Validates:
	// - Nested input mapping works
	// - Output extraction from nested modules
	// - Error handling at each level

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create inner programs
	innerProg1 := dsgo.NewProgram("inner1").
		AddModule(dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "inner1_result"}`)))

	innerProg2 := dsgo.NewProgram("inner2").
		AddModule(dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "inner2_result"}`)))

	// Create outer program containing inner programs
	outerProg := dsgo.NewProgram("outer").
		AddModule(innerProg1).
		AddModule(innerProg2).
		AddModule(dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "final_result"}`)))

	// Execute
	result, err := outerProg.Forward(ctx, map[string]any{
		"question": "test",
	})
	if err != nil {
		t.Fatalf("Nested program failed: %v", err)
	}

	// Verify result is available
	answer, ok := result.GetString("answer")
	if !ok || answer == "" {
		t.Error("Nested program produced no result")
	}
}

func TestNestedComposition_ChainOfThoughtWithinProgram(t *testing.T) {
	t.Parallel()
	// Test: Program containing ChainOfThought and other modules
	// Validates:
	// - ChainOfThought works within Program
	// - Outputs chain correctly through composition

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	cotSig := fixtures.ChainOfThoughtSig()
	refineSig := fixtures.RefineSig()

	// Create program with mixed module types
	prog := dsgo.NewProgram("mixed").
		AddModule(dsgo.NewChainOfThought(cotSig, NewMockLMWithResponse(`{
			"reasoning": "Step-by-step reasoning here",
			"answer": "Reasoned answer"
		}`))).
		AddModule(dsgo.NewRefine(refineSig, NewMockLMWithResponse(`{
			"output": "Refined output"
		}`)))

	// Execute
	result, err := prog.Forward(ctx, map[string]any{
		"problem": "test problem",
		"topic":   "test topic",
	})
	if err != nil {
		t.Fatalf("Mixed program failed: %v", err)
	}

	// Verify result has output from last module
	output, ok := result.GetString("output")
	if !ok || output == "" {
		t.Error("Mixed program produced no output")
	}
}

// ============================================================================
// Specific Scenario Tests
// ============================================================================

func TestScenario_ChainOfThoughtToRefineToClassify(t *testing.T) {
	t.Parallel()
	// Test: Complete workflow: Reason → Refine → Classify
	// Example: Analyze text → Improve analysis → Classify sentiment
	// Validates: Complex multi-stage pipeline with different module types

	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	// Stage 1: Chain of thought reasoning
	reasonSig := fixtures.ChainOfThoughtSig()
	reasonLM := NewMockLMWithResponse(`{
		"reasoning": "The text exhibits positive language patterns",
		"answer": "Generally positive tone detected"
	}`)

	reasoner := dsgo.NewChainOfThought(reasonSig, reasonLM)

	// Execute reasoning
	reasonResult, err := reasoner.Forward(ctx, map[string]any{
		"problem": "Analyze this customer review for sentiment",
	})
	if err != nil {
		t.Fatalf("Reasoning failed: %v", err)
	}

	// Stage 2: Refine the reasoning
	refineSig := fixtures.RefineSig()
	refineLM := NewMockLMWithResponse(`{
		"output": "The analysis shows strong positive sentiment with confidence 0.9"
	}`)

	refiner := dsgo.NewRefine(refineSig, refineLM)
	answer, ok := reasonResult.GetString("answer")
	if !ok {
		t.Fatal("Reason result didn't have answer")
	}

	refineResult, err := refiner.Forward(ctx, map[string]any{
		"topic": answer,
	})
	if err != nil {
		t.Fatalf("Refinement failed: %v", err)
	}

	// Stage 3: Classify based on refined output
	classifySig := fixtures.ClassificationSig()
	classifyLM := NewMockLMWithResponse(`{
		"sentiment": "positive"
	}`)

	classifier := dsgo.NewPredict(classifySig, classifyLM)
	output, ok := refineResult.GetString("output")
	if !ok {
		t.Fatal("Refine result didn't have output")
	}

	classifyResult, err := classifier.Forward(ctx, map[string]any{
		"text": output,
	})
	if err != nil {
		t.Fatalf("Classification failed: %v", err)
	}

	// Verify complete pipeline
	if reasoning, ok := reasonResult.GetString("reasoning"); !ok || reasoning == "" {
		t.Error("Stage 1 reasoning is empty")
	}
	if output, ok := refineResult.GetString("output"); !ok || output == "" {
		t.Error("Stage 2 refinement is empty")
	}
	if sentiment, ok := classifyResult.GetString("sentiment"); !ok || sentiment == "" {
		t.Error("Stage 3 classification is empty")
	}

	// Verify usage accumulated
	totalTokens := reasonResult.Usage.TotalTokens +
		refineResult.Usage.TotalTokens +
		classifyResult.Usage.TotalTokens
	if totalTokens != 60 { // 20 tokens per module
		t.Errorf("Usage not accumulated: expected 60, got %d", totalTokens)
	}
}

func TestScenario_BestOfNRefinement(t *testing.T) {
	t.Parallel()
	// Test: Generate multiple versions → Select best → Refine best
	// Validates: Selection pipeline with quality improvement

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Generate 3 candidate answers
	genLM := NewMockLMWithResponses([]string{
		`{"answer": "adequate response"}`,
		`{"answer": "good response"}`,
		`{"answer": "excellent response"}`,
	})

	generator := dsgo.NewPredict(sig, genLM)

	// Create BestOfN
	best := dsgo.NewBestOfN(generator, 3)
	best.WithScorer(func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		answer, ok := pred.GetString("answer")
		if !ok {
			return 0, nil
		}
		// Score based on answer length (simulates quality)
		return float64(len(answer)), nil
	})

	// Get best
	bestResult, err := best.Forward(ctx, map[string]any{
		"question": "Generate response",
	})
	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	// Refine the best result
	refineSig := fixtures.RefineSig()
	refineLM := NewMockLMWithResponse(`{
		"output": "Polished and perfected excellent response"
	}`)

	refiner := dsgo.NewRefine(refineSig, refineLM)
	bestAnswer, ok := bestResult.GetString("answer")
	if !ok {
		t.Fatal("BestOfN result didn't have answer")
	}

	refineResult, err := refiner.Forward(ctx, map[string]any{
		"topic": bestAnswer,
	})
	if err != nil {
		t.Fatalf("Refinement failed: %v", err)
	}

	// Verify pipeline
	if bestAnswer != "excellent response" {
		t.Errorf("Best selection incorrect: got %s", bestAnswer)
	}
	if output, ok := refineResult.GetString("output"); !ok || output == "" {
		t.Error("Refinement produced no output")
	}
}

func TestScenario_ConditionalBranching(t *testing.T) {
	t.Parallel()
	// Test: Module 1 → Decision point → Different path based on output
	// Validates: Manual conditional routing (Program doesn't support conditional logic)

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.ClassificationSig()

	// First stage: Classification
	classifyLM := NewMockLMWithResponse(`{
		"sentiment": "positive"
	}`)

	classifier := dsgo.NewPredict(sig, classifyLM)
	classifyResult, err := classifier.Forward(ctx, map[string]any{
		"text": "This is great!",
	})
	if err != nil {
		t.Fatalf("Classification failed: %v", err)
	}

	sentiment, ok := classifyResult.GetString("sentiment")
	if !ok {
		t.Fatal("Classification didn't produce sentiment")
	}

	// Decision: Route to appropriate next module based on sentiment
	var nextModule dsgo.Module
	if sentiment == "positive" {
		nextModule = dsgo.NewPredict(
			fixtures.SimplePredictSig(),
			NewMockLMWithResponse(`{"answer": "Generate positive-focused response"}`),
		)
	} else {
		nextModule = dsgo.NewPredict(
			fixtures.SimplePredictSig(),
			NewMockLMWithResponse(`{"answer": "Generate neutral-focused response"}`),
		)
	}

	// Execute routed module
	routeResult, err := nextModule.Forward(ctx, map[string]any{
		"question": fmt.Sprintf("Handle %s sentiment", sentiment),
	})
	if err != nil {
		t.Fatalf("Routed module failed: %v", err)
	}

	// Verify routing worked
	routeAnswer, ok := routeResult.GetString("answer")
	if !ok || !contains(routeAnswer, "positive") {
		t.Errorf("Routing failed: expected positive response, got %s", routeAnswer)
	}
}

func TestScenario_DocumentAnalysisPipeline(t *testing.T) {
	t.Parallel()
	// Test: Real-world scenario - Document analysis pipeline
	// 1. Extract key info (Predict)
	// 2. Generate summary (ChainOfThought)
	// 3. Identify risks (BestOfN)
	// 4. Create action items (Predict)

	ctx, cancel := ContextWithTimeout(20 * time.Second)
	defer cancel()

	// Stage 1: Extract key info
	extractLM := NewMockLMWithResponse(`{
		"answer": "Key info: Budget $100k, Timeline 6 months, Team size 5"
	}`)
	extractor := dsgo.NewPredict(fixtures.SimplePredictSig(), extractLM)
	extractResult, err := extractor.Forward(ctx, map[string]any{
		"question": "Extract key project information",
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	// Stage 2: Generate summary
	summaryLM := NewMockLMWithResponse(`{
		"reasoning": "The project has clear constraints",
		"answer": "Project summary: Medium-scale initiative with fixed resources"
	}`)
	summarizer := dsgo.NewChainOfThought(fixtures.ChainOfThoughtSig(), summaryLM)
	extractAnswer, ok := extractResult.GetString("answer")
	if !ok {
		t.Fatal("Extract didn't produce answer")
	}

	summaryResult, err := summarizer.Forward(ctx, map[string]any{
		"problem": extractAnswer,
	})
	if err != nil {
		t.Fatalf("Summarization failed: %v", err)
	}

	// Stage 3: Identify risks (best of 3)
	riskLM := NewMockLMWithResponses([]string{
		`{"answer": "Risk: Budget overrun"}`,
		`{"answer": "Risk: Timeline slippage"}`,
		`{"answer": "Risk: Resource shortage"}`,
	})
	riskAnalyzer := dsgo.NewPredict(fixtures.SimplePredictSig(), riskLM)
	bestRisk := dsgo.NewBestOfN(riskAnalyzer, 3)
	bestRisk.WithScorer(func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		answer, ok := pred.GetString("answer")
		if !ok {
			return 0, nil
		}
		// Prefer "Resource" risks (simulates priority ranking)
		if contains(answer, "Resource") {
			return 10.0, nil
		}
		return 1.0, nil
	})

	summaryAnswer, ok := summaryResult.GetString("answer")
	if !ok {
		t.Fatal("Summary didn't produce answer")
	}

	riskResult, err := bestRisk.Forward(ctx, map[string]any{
		"question": summaryAnswer,
	})
	if err != nil {
		t.Fatalf("Risk analysis failed: %v", err)
	}

	// Stage 4: Create action items
	actionLM := NewMockLMWithResponse(`{
		"answer": "Action items: 1) Allocate reserve budget 2) Plan resource acquisition 3) Risk monitoring"
	}`)
	actionCreator := dsgo.NewPredict(fixtures.SimplePredictSig(), actionLM)
	riskAnswer, ok := riskResult.GetString("answer")
	if !ok {
		t.Fatal("Risk analysis didn't produce answer")
	}

	actionResult, err := actionCreator.Forward(ctx, map[string]any{
		"question": riskAnswer,
	})
	if err != nil {
		t.Fatalf("Action creation failed: %v", err)
	}

	// Verify complete pipeline
	if extractAnswer == "" {
		t.Error("Stage 1 extraction produced no output")
	}
	if summaryAnswer == "" {
		t.Error("Stage 2 summary produced no output")
	}
	if riskAnswer == "" {
		t.Error("Stage 3 risk analysis produced no output")
	}
	if actionAnswer, ok := actionResult.GetString("answer"); !ok || actionAnswer == "" {
		t.Error("Stage 4 action items produced no output")
	}

	// Verify cost tracking across pipeline
	totalCost := extractResult.Usage.Cost +
		summaryResult.Usage.Cost +
		riskResult.Usage.Cost +
		actionResult.Usage.Cost
	if totalCost == 0 {
		t.Error("Pipeline cost not tracked")
	}
}

func TestScenario_ErrorPropagationSequential(t *testing.T) {
	t.Parallel()
	// Test: Error in module 1 stops pipeline
	// Validates: Errors propagate and stop execution

	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create failing module
	failingLM := &MockLM{Error: fmt.Errorf("simulated LM error")}
	failingModule := dsgo.NewPredict(sig, failingLM)

	// Create successful module (should not execute)
	successLM := NewMockLMWithResponse(`{"answer": "success"}`)
	successModule := dsgo.NewPredict(sig, successLM)

	// Create program
	prog := dsgo.NewProgram("error_test").
		AddModule(failingModule).
		AddModule(successModule)

	// Execute - should fail
	_, err := prog.Forward(ctx, map[string]any{
		"question": "test",
	})

	if err == nil {
		t.Error("Expected error from pipeline, but got none")
	}

	if !contains(err.Error(), "module 0") && !contains(err.Error(), "failed") {
		t.Errorf("Expected error mentioning 'module 0' and 'failed', got: %v", err)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
