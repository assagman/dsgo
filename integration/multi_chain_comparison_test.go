package integration

import (
	"context"
	"testing"

	"github.com/assagman/dsgo"
)

func TestMultiChainComparisonEndToEnd(t *testing.T) {
	// 1. Setup
	ctx := context.Background()

	// Define signature
	sig := dsgo.NewSignature("Solve Problem").
		AddInput("problem", dsgo.FieldTypeString, "Problem statement").
		AddOutput("solution", dsgo.FieldTypeString, "Detailed solution")

	// Define mock responses
	// 3 candidate responses for BestOfN
	candidate1 := `{"reasoning": "thought1", "solution": "candidate 1"}`
	candidate2 := `{"reasoning": "thought2", "solution": "candidate 2"}`
	candidate3 := `{"reasoning": "thought3", "solution": "candidate 3"}`

	// 1 synthesis response for MCC
	// Note: MCC prepends 'rationale' to output fields
	synthesis := `{"rationale": "comparing 1, 2, and 3...", "solution": "synthesized solution"}`

	// Create MockLM with cycling responses
	lm := NewMockLMWithResponses([]string{candidate1, candidate2, candidate3, synthesis})

	// 2. Generate Candidates using BestOfN
	// We use ChainOfThought as the generator
	generator := dsgo.NewChainOfThought(sig, lm)

	// Use sequential BestOfN to ensure deterministic order of MockLM consumption
	bestOfN := dsgo.NewBestOfN(generator, 3).
		WithParallel(false). // Sequential ensures candidate1->2->3 order
		WithReturnAll(true).
		WithScorer(dsgo.DefaultScorer())

	inputs := map[string]any{"problem": "test problem"}

	candidates, err := bestOfN.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	if len(candidates.Completions) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(candidates.Completions))
	}

	// 3. Synthesize with MultiChainComparison
	mcc := dsgo.NewMultiChainComparison(sig, lm, 3)

	mccInputs := map[string]any{
		"problem":     "test problem",
		"completions": candidates.Completions,
	}

	result, err := mcc.Forward(ctx, mccInputs)
	if err != nil {
		t.Fatalf("MCC failed: %v", err)
	}

	// 4. Verify Results
	// Check synthesis output
	solution, _ := result.GetString("solution")
	if solution != "synthesized solution" {
		t.Errorf("Expected solution 'synthesized solution', got '%s'", solution)
	}

	rationale, _ := result.GetString("rationale")
	if rationale != "comparing 1, 2, and 3..." {
		t.Errorf("Expected rationale 'comparing 1, 2, and 3...', got '%s'", rationale)
	}

	// 5. Verify Prompt Construction (Implicitly via MockLM logs if we had them,
	// but here we trust the result correctness implies correct inputs were passed
	// because MockLM doesn't validate inputs unless we make a stricter Mock)
	// For now, E2E correctness of outputs is sufficient.
}

func TestMultiChainComparisonWithParallelGenerator(t *testing.T) {
	// Test to ensure parallel generation (common use case) works with MCC
	ctx := context.Background()

	sig := dsgo.NewSignature("Solve").
		AddInput("input", dsgo.FieldTypeString, "Input").
		AddOutput("output", dsgo.FieldTypeString, "Output")

	// Responses (order doesn't matter for correctness here)
	responses := []string{
		`{"output": "A"}`,
		`{"output": "B"}`,
		`{"output": "C"}`,
		`{"rationale": "final", "output": "FINAL"}`,
	}

	lm := NewMockLMWithResponses(responses)

	generator := dsgo.NewPredict(sig, lm)
	bestOfN := dsgo.NewBestOfN(generator, 3).
		WithParallel(true).
		WithReturnAll(true).
		WithScorer(dsgo.DefaultScorer())

	mcc := dsgo.NewMultiChainComparison(sig, lm, 3)

	inputs := map[string]any{"input": "test"}

	// Run pipeline
	candidates, err := bestOfN.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}

	mccInputs := map[string]any{
		"input":       "test",
		"completions": candidates.Completions,
	}

	result, err := mcc.Forward(ctx, mccInputs)
	if err != nil {
		t.Fatalf("Synthesis failed: %v", err)
	}

	if val, _ := result.GetString("output"); val != "FINAL" {
		t.Errorf("Expected 'FINAL', got '%s'", val)
	}
}
