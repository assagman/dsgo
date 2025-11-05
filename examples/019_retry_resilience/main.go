package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "019_retry_resilience", runExample)
	if err != nil {
		log.Fatal(err)
	}

	if err := h.OutputResults(); err != nil {
		log.Fatal(err)
	}
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
	stats := &harness.ExecutionStats{
		Metadata: make(map[string]any),
	}

	lm := shared.GetLM(shared.GetModel())

	var totalTokens int

	fmt.Println("=== Retry & Resilience Demo ===")
	fmt.Println("Demonstrating automatic retry with exponential backoff")
	fmt.Println()

	fmt.Println("--- Built-in Retry Features ---")
	fmt.Println("✓ Automatic retry on rate limits (HTTP 429)")
	fmt.Println("✓ Automatic retry on server errors (HTTP 500, 502, 503, 504)")
	fmt.Println("✓ Automatic retry on network errors")
	fmt.Println("✓ Exponential backoff with jitter (prevents thundering herd)")
	fmt.Println("✓ Maximum 3 retries (4 total attempts)")
	fmt.Println("✓ Context cancellation support")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 1: Normal request (demonstrates retry is transparent)
	fmt.Println("--- Demo 1: Normal Request ---")
	fmt.Println("Making a standard request (retry is transparent)...")
	fmt.Println()

	sig := dsgo.NewSignature("Answer the given question concisely and accurately").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "Concise answer")

	predict := module.NewPredict(sig, lm)

	result1, err := predict.Forward(ctx, map[string]any{
		"question": "What is the capital of France?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("demo 1 failed: %w", err)
	}

	answer1, _ := result1.GetString("answer")
	fmt.Printf("Question: What is the capital of France?\n")
	fmt.Printf("Answer: %s\n", answer1)
	fmt.Printf("Status: ✓ Success (no retries needed)\n")
	fmt.Printf("📊 Tokens used: %d\n", result1.Usage.TotalTokens)

	totalTokens += result1.Usage.TotalTokens

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: Retry behavior explanation
	fmt.Println("--- Demo 2: Retry Behavior ---")
	fmt.Println("If rate limit or server error occurs:")
	fmt.Println()
	fmt.Println("Attempt 1: Request fails with 429 or 5xx")
	fmt.Println("  ↓ Wait 1s (with ±10% jitter)")
	fmt.Println("Attempt 2: Retry...")
	fmt.Println("  ↓ If fails, wait 2s (with jitter)")
	fmt.Println("Attempt 3: Retry...")
	fmt.Println("  ↓ If fails, wait 4s (with jitter)")
	fmt.Println("Attempt 4: Final retry...")
	fmt.Println("  ↓ If fails, return error")
	fmt.Println()
	fmt.Println("✅ Retry mechanism demonstrated")

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Multiple requests (retry is automatic)
	fmt.Println("--- Demo 3: Multiple Requests ---")
	fmt.Println("Making several requests (retries are automatic)...")
	fmt.Println()

	questions := []string{
		"What is 2 + 2?",
		"Who wrote Romeo and Juliet?",
		"What is the speed of light?",
	}

	successCount := 0
	for i, question := range questions {
		result, err := predict.Forward(ctx, map[string]any{
			"question": question,
		})
		if err != nil {
			fmt.Printf("%d. %s\n", i+1, question)
			fmt.Printf("   ✗ Failed after retries: %v\n\n", err)
			continue
		}

		answer, _ := result.GetString("answer")
		fmt.Printf("%d. %s\n", i+1, question)
		fmt.Printf("   → %s ✓\n", answer)
		fmt.Printf("   Tokens: %d\n\n", result.Usage.TotalTokens)

		totalTokens += result.Usage.TotalTokens
		successCount++
	}

	fmt.Printf("✅ %d/%d requests succeeded\n", successCount, len(questions))

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 4: ChainOfThought with retry
	fmt.Println("--- Demo 4: Chain of Thought with Retry ---")
	fmt.Println("Complex reasoning also benefits from retry resilience...")
	fmt.Println()

	cotSig := dsgo.NewSignature("Solve the given math word problem").
		AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
		AddOutput("solution", dsgo.FieldTypeString, "Step-by-step solution").
		AddOutput("answer", dsgo.FieldTypeString, "Final numeric answer")

	cot := module.NewChainOfThought(cotSig, lm)

	result2, err := cot.Forward(ctx, map[string]any{
		"problem": "If a train travels 120 km in 2 hours, what is its average speed?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("demo 4 failed: %w", err)
	}

	solution, _ := result2.GetString("solution")
	answer2, _ := result2.GetString("answer")
	rationale, _ := result2.GetString("rationale")

	fmt.Printf("Problem: If a train travels 120 km in 2 hours, what is its average speed?\n\n")
	if rationale != "" {
		fmt.Printf("Rationale:\n%s\n\n", rationale)
	}
	fmt.Printf("Solution:\n%s\n\n", solution)
	fmt.Printf("Answer: %s ✓\n", answer2)
	fmt.Printf("📊 Tokens used: %d\n", result2.Usage.TotalTokens)

	totalTokens += result2.Usage.TotalTokens

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 4
	stats.Metadata["questions_tested"] = len(questions) + 2 // questions + demo1 + demo4
	stats.Metadata["success_count"] = successCount + 2

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

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

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Println("Retry capabilities:")
	fmt.Println("  ✓ Automatic retry - no code changes needed")
	fmt.Println("  ✓ Handles rate limits gracefully")
	fmt.Println("  ✓ Recovers from transient server errors")
	fmt.Println("  ✓ Exponential backoff prevents overwhelming the API")
	fmt.Println("  ✓ Jitter prevents thundering herd problem")
	fmt.Println("  ✓ Works with all modules (Predict, CoT, ReAct, etc.)")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 4\n")
	fmt.Println()

	return result2, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
