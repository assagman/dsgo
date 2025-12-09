package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/assagman/dsgo"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		// Use a mock provider if no key is provided, so example can compile/run in CI
		fmt.Println("No OPENROUTER_API_KEY provided, example would fail in real execution.")
		fmt.Println("Please set OPENROUTER_API_KEY to run with real models.")
		return
	}

	// Create LM
	lm, err := dsgo.NewLM(context.Background(), "openrouter/google/gemini-2.5-flash")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Define signature for Q&A
	sig := dsgo.NewSignature("Answer question with detail").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "Detailed answer").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

	// 1. Create a Generator (BestOfN) to produce candidates
	// Use ChainOfThought to produce diverse reasoning
	generator := dsgo.NewChainOfThought(sig, lm)

	// Generate 3 candidates in parallel
	// We use a dummy scorer because MCC will be the final judge
	bestOfN := dsgo.NewBestOfN(generator, 3).
		WithParallel(true).
		WithReturnAll(true).
		WithScorer(dsgo.DefaultScorer())

	// 2. Create MultiChainComparison to synthesize the best answer
	// It takes the 3 candidates and produces a final, better answer
	mcc := dsgo.NewMultiChainComparison(sig, lm, 3)

	// Test question
	question := "What are the main benefits of Go's concurrency model?"
	fmt.Println("=== MultiChainComparison Example ===")
	fmt.Printf("Question: %s\n\n", question)

	ctx := context.Background()
	inputs := map[string]any{"question": question}

	// Step 1: Generate candidates
	fmt.Println("1. Generating 3 diverse candidates...")
	candidates, err := bestOfN.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Printf("   Generated %d candidates\n", len(candidates.Completions))

	// Step 2: Synthesize with MCC
	// Pass the candidates to MCC via the "completions" input
	fmt.Println("2. Synthesizing best answer with MultiChainComparison...")

	mccInputs := map[string]any{
		"question":    question,
		"completions": candidates.Completions,
	}

	result, err := mcc.Forward(ctx, mccInputs)
	if err != nil {
		log.Fatalf("Synthesis failed: %v", err)
	}

	// Get results
	answer, _ := result.GetString("answer")
	rationale, _ := result.GetString("rationale") // MCC adds a rationale field explaining the choice

	fmt.Println("\n=== Final Result ===")
	fmt.Printf("Rationale: %s\n\n", rationale)
	fmt.Printf("Answer: %s\n", answer)

	// Show usage
	totalCost := candidates.Usage.Cost + result.Usage.Cost
	fmt.Printf("\nTotal Cost: $%.6f\n", totalCost)
}
