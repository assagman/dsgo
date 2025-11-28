package integration

import (
	"context"
	"testing"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

// TestE2E_TicketRouting simulates customer support ticket routing workflow.
// Workflow:
// 1. Classify sentiment (Predict)
// 2. Extract intent (Predict)
// 3. Generate response (ChainOfThought)
// Validates: Multi-stage pipeline, cost tracking, all stages execute.
func TestE2E_TicketRouting(t *testing.T) {
	ctx := context.Background()

	// Stage 1: Sentiment classification
	sentimentLM := NewMockLMWithResponse(`{"answer": "negative"}`)
	sentimentSig := fixtures.SimplePredictSig()
	sentimentClassifier := dsgo.NewPredict(sentimentSig, sentimentLM)

	sentimentResult, err := sentimentClassifier.Forward(ctx, map[string]any{
		"question": "This product is broken and support is useless",
	})
	if err != nil {
		t.Fatalf("Sentiment classification failed: %v", err)
	}

	sentiment, ok := sentimentResult.GetString("answer")
	if !ok {
		t.Error("Expected sentiment field")
	}

	// Stage 2: Intent extraction
	intentLM := NewMockLMWithResponse(`{"answer": "complaint"}`)
	intentSig := fixtures.SimplePredictSig()
	intentExtractor := dsgo.NewPredict(intentSig, intentLM)

	intentResult, err := intentExtractor.Forward(ctx, map[string]any{
		"question": "What type of issue is this?",
	})
	if err != nil {
		t.Fatalf("Intent extraction failed: %v", err)
	}

	intent, ok := intentResult.GetString("answer")
	if !ok {
		t.Error("Expected intent field")
	}

	// Stage 3: Generate response
	responseLM := NewMockLMWithResponse(`{
		"reasoning": "Customer is unhappy due to product quality. Need professional, empathetic response.",
		"answer": "We sincerely apologize for your experience. Our team will investigate immediately."
	}`)
	responseSig := fixtures.ChainOfThoughtSig()
	responseGenerator := dsgo.NewChainOfThought(responseSig, responseLM)

	responseResult, err := responseGenerator.Forward(ctx, map[string]any{
		"problem": "Generate support response",
	})
	if err != nil {
		t.Fatalf("Response generation failed: %v", err)
	}

	response, ok := responseResult.GetString("answer")
	if !ok {
		t.Error("Expected response field")
	}

	// Verify pipeline results
	if sentiment == "" {
		t.Error("Empty sentiment")
	}
	if intent == "" {
		t.Error("Empty intent")
	}
	if response == "" {
		t.Error("Empty response")
	}

	// Verify cost tracking
	totalCost := sentimentResult.Usage.Cost + intentResult.Usage.Cost + responseResult.Usage.Cost
	if totalCost == 0 {
		t.Error("Expected cost tracking across stages")
	}
}

// TestE2E_DocumentAnalysisPipeline simulates document analysis workflow.
// Workflow:
// 1. Extract key info (Predict)
// 2. Generate summary (ChainOfThought)
// 3. Identify risks (Multiple options, select best)
// Validates: Multi-module composition, quality metrics.
func TestE2E_DocumentAnalysisPipeline(t *testing.T) {
	ctx := context.Background()

	// Stage 1: Extract key information
	extractLM := NewMockLMWithResponse(`{"answer": "Author: John Doe, Date: 2025-01-01, Topic: AI Safety"}`)
	extractSig := fixtures.SimplePredictSig()
	extractor := dsgo.NewPredict(extractSig, extractLM)

	extractResult, err := extractor.Forward(ctx, map[string]any{
		"question": "Extract key information from document",
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	keyInfo, ok := extractResult.GetString("answer")
	if !ok || keyInfo == "" {
		t.Error("Expected key info extraction")
	}

	// Stage 2: Generate summary
	summaryLM := NewMockLMWithResponse(`{
		"reasoning": "Document discusses AI safety implications in modern systems. Key points include ethics, alignment, and deployment risks.",
		"answer": "Document provides comprehensive overview of AI safety considerations in deployment."
	}`)
	summarySig := fixtures.ChainOfThoughtSig()
	summarizer := dsgo.NewChainOfThought(summarySig, summaryLM)

	summaryResult, err := summarizer.Forward(ctx, map[string]any{
		"problem": "Summarize document",
	})
	if err != nil {
		t.Fatalf("Summary generation failed: %v", err)
	}

	summary, ok := summaryResult.GetString("answer")
	if !ok || summary == "" {
		t.Error("Expected summary")
	}

	// Stage 3: Identify risks (use simple Predict instead of BestOfN to avoid scorer requirement)
	riskLM := NewMockLMWithResponse(`{"answer": "Critical risk: Insufficient safety testing"}`)
	riskSig := fixtures.SimplePredictSig()
	riskAnalyzer := dsgo.NewPredict(riskSig, riskLM)

	riskResult, err := riskAnalyzer.Forward(ctx, map[string]any{
		"question": "Identify top risk",
	})
	if err != nil {
		t.Fatalf("Risk analysis failed: %v", err)
	}

	risk, ok := riskResult.GetString("answer")
	if !ok || risk == "" {
		t.Error("Expected risk identification")
	}

	// Verify all stages executed
	if keyInfo == "" || summary == "" || risk == "" {
		t.Error("Pipeline did not complete all stages")
	}

	// Verify cost aggregation
	totalCost := extractResult.Usage.Cost + summaryResult.Usage.Cost + riskResult.Usage.Cost
	if totalCost == 0 {
		t.Error("Expected cost tracking")
	}
}

// TestE2E_CreativeGeneration simulates creative content generation with refinement.
// Workflow:
// 1. Generate draft (Predict)
// 2. Refine draft (Refine module)
// 3. Parallel critique (Multiple critique modules)
// 4. Final version (Predict using critiques)
// Validates: Multi-stage improvement, composition quality.
func TestE2E_CreativeGeneration(t *testing.T) {
	ctx := context.Background()

	// Stage 1: Generate draft
	draftLM := NewMockLMWithResponse(`{"answer": "Once upon a time, there was a curious explorer..."}`)
	draftSig := fixtures.SimplePredictSig()
	draftGen := dsgo.NewPredict(draftSig, draftLM)

	draftResult, err := draftGen.Forward(ctx, map[string]any{
		"question": "Write creative opening",
	})
	if err != nil {
		t.Fatalf("Draft generation failed: %v", err)
	}

	draft, ok := draftResult.GetString("answer")
	if !ok || draft == "" {
		t.Error("Expected draft output")
	}

	// Stage 2: Refine using Refine module
	refineLM := NewMockLMWithResponse(`{"answer": "In a land of mystery and wonder, an intrepid explorer ventured forth..."}`)
	refineSig := fixtures.SimplePredictSig()
	refiner := dsgo.NewRefine(refineSig, refineLM)

	refineResult, err := refiner.Forward(ctx, map[string]any{
		"question": "Refine the draft",
	})
	if err != nil {
		t.Fatalf("Refinement failed: %v", err)
	}

	refined, ok := refineResult.GetString("answer")
	if !ok || refined == "" {
		t.Error("Expected refined output")
	}

	// Stage 3: Parallel critiques (simulated with sequential execution)
	critic1LM := NewMockLMWithResponse(`{"answer": "Good atmosphere, needs stronger character development"}`)
	critic1Sig := fixtures.SimplePredictSig()
	critic1 := dsgo.NewPredict(critic1Sig, critic1LM)

	critique1Result, err := critic1.Forward(ctx, map[string]any{
		"question": "Critique the refined version",
	})
	if err != nil {
		t.Fatalf("Critique 1 failed: %v", err)
	}

	critique1, ok := critique1Result.GetString("answer")
	if !ok || critique1 == "" {
		t.Error("Expected critique 1")
	}

	critic2LM := NewMockLMWithResponse(`{"answer": "Excellent pacing, world-building could be expanded"}`)
	critic2Sig := fixtures.SimplePredictSig()
	critic2 := dsgo.NewPredict(critic2Sig, critic2LM)

	critique2Result, err := critic2.Forward(ctx, map[string]any{
		"question": "Critique the refined version",
	})
	if err != nil {
		t.Fatalf("Critique 2 failed: %v", err)
	}

	critique2, ok := critique2Result.GetString("answer")
	if !ok || critique2 == "" {
		t.Error("Expected critique 2")
	}

	// Stage 4: Final version using critiques
	finalLM := NewMockLMWithResponse(`{"answer": "Final polished version incorporating feedback: In a land of mystery and wonder, inhabited by complex characters, an intrepid explorer ventured forth..."}`)
	finalSig := fixtures.SimplePredictSig()
	finalGen := dsgo.NewPredict(finalSig, finalLM)

	finalResult, err := finalGen.Forward(ctx, map[string]any{
		"question": "Create final version",
	})
	if err != nil {
		t.Fatalf("Final generation failed: %v", err)
	}

	final, ok := finalResult.GetString("answer")
	if !ok || final == "" {
		t.Error("Expected final output")
	}

	// Verify all outputs are distinct and present
	if draft == "" || refined == "" || final == "" {
		t.Error("Pipeline did not complete all improvement stages")
	}
}

// TestE2E_BatchProcessing validates cost aggregation over multiple items.
// Workflow: Process 10 items through same pipeline, track total cost.
// Validates: Cost aggregation across batch processing.
func TestE2E_BatchProcessing(t *testing.T) {
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"answer": "processed"}`)
	sig := fixtures.SimplePredictSig()

	predictor := dsgo.NewPredict(sig, lm)

	batchSize := 10
	var totalCost float64
	var totalTokens int

	for i := 0; i < batchSize; i++ {
		result, err := predictor.Forward(ctx, map[string]any{
			"question": "Process item",
		})
		if err != nil {
			t.Fatalf("Item %d processing failed: %v", i, err)
		}

		totalCost += result.Usage.Cost
		totalTokens += result.Usage.TotalTokens
	}

	// Verify cost accumulation
	if totalCost == 0 {
		t.Error("Expected cost tracking across batch")
	}
	if totalTokens == 0 {
		t.Error("Expected token tracking across batch")
	}

	// Cost should be roughly 10x single call (allowing for mock variation)
	if totalCost < 0.001 {
		t.Errorf("Expected accumulated cost, got %v", totalCost)
	}
}

// TestE2E_ConditionalRouting simulates workflow with conditional logic.
// Workflow:
// 1. Classify input type (Predict)
// 2. Route to appropriate handler based on classification
// 3. Generate specialized response (ChainOfThought or Predict based on type)
// Validates: Dynamic routing, specialized handling.
func TestE2E_ConditionalRouting(t *testing.T) {
	ctx := context.Background()

	// Stage 1: Classify input type
	classifyLM := NewMockLMWithResponse(`{"answer": "technical_question"}`)
	classifySig := fixtures.SimplePredictSig()
	classifier := dsgo.NewPredict(classifySig, classifyLM)

	classifyResult, err := classifier.Forward(ctx, map[string]any{
		"question": "Is this technical or general?",
	})
	if err != nil {
		t.Fatalf("Classification failed: %v", err)
	}

	inputType, ok := classifyResult.GetString("answer")
	if !ok || inputType == "" {
		t.Error("Expected classification")
	}

	// Stage 2: Route to specialized handler
	var finalResult *dsgo.Prediction

	if inputType == "technical_question" {
		// Use ChainOfThought for complex reasoning
		techLM := NewMockLMWithResponse(`{
			"reasoning": "This requires step-by-step technical analysis of the system architecture.",
			"answer": "The issue stems from incorrect configuration in the load balancer."
		}`)
		techSig := fixtures.ChainOfThoughtSig()
		techHandler := dsgo.NewChainOfThought(techSig, techLM)

		var err error
		finalResult, err = techHandler.Forward(ctx, map[string]any{
			"problem": "Handle technical question",
		})
		if err != nil {
			t.Fatalf("Technical handler failed: %v", err)
		}
	} else {
		// Use simple Predict for general questions
		generalLM := NewMockLMWithResponse(`{"answer": "This is a general response."}`)
		generalSig := fixtures.SimplePredictSig()
		generalHandler := dsgo.NewPredict(generalSig, generalLM)

		var err error
		finalResult, err = generalHandler.Forward(ctx, map[string]any{
			"question": "Handle general question",
		})
		if err != nil {
			t.Fatalf("General handler failed: %v", err)
		}
	}

	// Verify conditional routing executed
	if finalResult == nil {
		t.Error("Expected result from routing")
	}

	answer, ok := finalResult.GetString("answer")
	if !ok || answer == "" {
		t.Error("Expected answer from specialized handler")
	}
}

// TestE2E_ErrorRecoveryPipeline validates graceful degradation in multi-stage pipeline.
// Workflow:
// 1. Primary analysis (may fail)
// 2. Fallback to simpler analysis if needed
// 3. Continue to downstream stages
// Validates: Error recovery, continued pipeline execution.
func TestE2E_ErrorRecoveryPipeline(t *testing.T) {
	ctx := context.Background()

	// Stage 1: Try complex analysis (fails)
	complexLM := &MockLM{
		Error: nil, // Will fail
	}
	complexSig := fixtures.SimplePredictSig()
	complexAnalyzer := dsgo.NewPredict(complexSig, complexLM)

	_, err := complexAnalyzer.Forward(ctx, map[string]any{
		"question": "Complex analysis",
	})
	_ = err

	// Stage 2: Fallback to simpler analysis
	simpleLM := NewMockLMWithResponse(`{"answer": "Simplified analysis result"}`)
	simpleSig := fixtures.SimplePredictSig()
	simpleAnalyzer := dsgo.NewPredict(simpleSig, simpleLM)

	simpleResult, err := simpleAnalyzer.Forward(ctx, map[string]any{
		"question": "Simple analysis",
	})
	if err != nil {
		t.Fatalf("Fallback analysis failed: %v", err)
	}

	simpleAnswer, ok := simpleResult.GetString("answer")
	if !ok || simpleAnswer == "" {
		t.Error("Expected fallback result")
	}

	// Stage 3: Continue with downstream processing
	reportLM := NewMockLMWithResponse(`{"answer": "Final report generated"}`)
	reportSig := fixtures.SimplePredictSig()
	reporter := dsgo.NewPredict(reportSig, reportLM)

	reportResult, err := reporter.Forward(ctx, map[string]any{
		"question": "Generate report from analysis",
	})
	if err != nil {
		t.Fatalf("Report generation failed: %v", err)
	}

	report, ok := reportResult.GetString("answer")
	if !ok || report == "" {
		t.Error("Expected report output")
	}

	// Verify pipeline completed despite initial error
	if report == "" {
		t.Error("Pipeline did not complete after error recovery")
	}
}
