package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

// This example demonstrates the automatic retry mechanism with exponential backoff
// that handles transient errors like rate limits (429) and server errors (5xx).
//
// The retry logic is built into the OpenAI and OpenRouter providers and includes:
// - Automatic retry on 429, 500, 502, 503, 504 status codes
// - Exponential backoff: 1s → 2s → 4s (with jitter)
// - Maximum 3 retries before giving up
// - Context-aware (respects cancellation and timeouts)

func main() {
	shared.LoadEnv()

	fmt.Println("=== Retry & Resilience Example ===")
	fmt.Println("Demonstrating automatic retry with exponential backoff\n")

	// Get LM (retry logic is built-in)
	lm := shared.GetLM(shared.GetModel())

	// Create a signature for question answering
	sig := dsgo.NewSignature("Answer the given question concisely and accurately").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "Concise answer")

	predict := module.NewPredict(sig, lm)
	ctx := context.Background()

	fmt.Println("--- Built-in Retry Features ---")
	fmt.Println("✓ Automatic retry on rate limits (HTTP 429)")
	fmt.Println("✓ Automatic retry on server errors (HTTP 500, 502, 503, 504)")
	fmt.Println("✓ Automatic retry on network errors")
	fmt.Println("✓ Exponential backoff with jitter (prevents thundering herd)")
	fmt.Println("✓ Maximum 3 retries (4 total attempts)")
	fmt.Println("✓ Context cancellation support\n")

	// Example 1: Normal request (demonstrates retry is transparent)
	fmt.Println("--- Example 1: Normal Request ---")
	fmt.Println("Making a standard request...")
	
	result1, err := predict.Forward(ctx, map[string]any{
		"question": "What is the capital of France?",
	})
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	
	answer1, _ := result1.GetString("answer")
	fmt.Printf("Question: What is the capital of France?\n")
	fmt.Printf("Answer: %s\n", answer1)
	fmt.Printf("Status: ✓ Success (no retries needed)\n\n")

	// Example 2: Demonstrate retry behavior description
	fmt.Println("--- Example 2: Retry Behavior ---")
	fmt.Println("If rate limit or server error occurs:")
	fmt.Println()
	fmt.Println("Attempt 1: Request fails with 429 or 5xx")
	fmt.Println("  ↓ Wait 1s (with ±10% jitter)")
	fmt.Println("Attempt 2: Retry...")
	fmt.Println("  ↓ If fails, wait 2s (with jitter)")
	fmt.Println("Attempt 3: Retry...")
	fmt.Println("  ↓ If fails, wait 4s (with jitter)")
	fmt.Println("Attempt 4: Final retry...")
	fmt.Println("  ↓ If fails, return error\n")

	// Example 3: Multiple requests (retry is automatic)
	fmt.Println("--- Example 3: Multiple Requests ---")
	fmt.Println("Making several requests (retries are automatic)...\n")

	questions := []string{
		"What is 2 + 2?",
		"Who wrote Romeo and Juliet?",
		"What is the speed of light?",
	}

	for i, question := range questions {
		result, err := predict.Forward(ctx, map[string]any{
			"question": question,
		})
		if err != nil {
			// Error after all retries exhausted
			fmt.Printf("%d. %s\n", i+1, question)
			fmt.Printf("   ✗ Failed after retries: %v\n\n", err)
			continue
		}

		answer, _ := result.GetString("answer")
		fmt.Printf("%d. %s\n", i+1, question)
		fmt.Printf("   → %s ✓\n\n", answer)
	}

	// Example 4: ChainOfThought with retry
	fmt.Println("--- Example 4: Chain of Thought with Retry ---")
	fmt.Println("Complex reasoning also benefits from retry resilience...\n")

	cotSig := dsgo.NewSignature("Solve the given math word problem").
		AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
		AddOutput("solution", dsgo.FieldTypeString, "Step-by-step solution").
		AddOutput("answer", dsgo.FieldTypeString, "Final numeric answer")

	cot := module.NewChainOfThought(cotSig, lm)

	result2, err := cot.Forward(ctx, map[string]any{
		"problem": "If a train travels 120 km in 2 hours, what is its average speed?",
	})
	if err != nil {
		log.Fatalf("CoT request failed: %v", err)
	}

	solution, _ := result2.GetString("solution")
	answer2, _ := result2.GetString("answer")
	fmt.Printf("Problem: If a train travels 120 km in 2 hours, what is its average speed?\n\n")
	fmt.Printf("Reasoning:\n%s\n\n", solution)
	fmt.Printf("Answer: %s ✓\n\n", answer2)

	fmt.Println("--- Retry Configuration ---")
	fmt.Println("The retry mechanism is configured in internal/retry/retry.go:")
	fmt.Println("  • MaxRetries = 3")
	fmt.Println("  • InitialBackoff = 1 second")
	fmt.Println("  • MaxBackoff = 30 seconds")
	fmt.Println("  • JitterFactor = 0.1 (±10% randomness)")
	fmt.Println()
	fmt.Println("Retryable status codes:")
	fmt.Println("  • 429 - Too Many Requests (rate limit)")
	fmt.Println("  • 500 - Internal Server Error")
	fmt.Println("  • 502 - Bad Gateway")
	fmt.Println("  • 503 - Service Unavailable")
	fmt.Println("  • 504 - Gateway Timeout")

	fmt.Println("\n=== Key Takeaways ===")
	fmt.Println("✓ Retry is automatic - no code changes needed")
	fmt.Println("✓ Handles rate limits gracefully")
	fmt.Println("✓ Recovers from transient server errors")
	fmt.Println("✓ Exponential backoff prevents overwhelming the API")
	fmt.Println("✓ Jitter prevents thundering herd problem")
	fmt.Println("✓ Works with all modules (Predict, CoT, ReAct, etc.)")
}
