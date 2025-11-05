package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "021_best_of_n_parallel", runExample)
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

	fmt.Println("=== BestOfN Parallel Execution ===")
	fmt.Println("Demonstrates parallel candidate generation for improved performance")
	fmt.Println()

	fmt.Println("--- Parallel Execution Features ---")
	fmt.Println("✓ Generate N candidates concurrently")
	fmt.Println("✓ Significant performance improvement (2-3x faster)")
	fmt.Println("✓ Custom scoring functions for domain-specific selection")
	fmt.Println("✓ Early stopping with score thresholds")
	fmt.Println("✓ Analyze all candidates with WithReturnAll()")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 1: Creative Title Generation with Parallel Execution
	fmt.Println("--- Demo 1: Best Title Selection (Parallel) ---")
	fmt.Println("Generate 5 creative titles and select the best one")
	fmt.Println()

	sig1 := dsgo.NewSignature("Generate a creative and catchy title for a blog post").
		AddInput("topic", dsgo.FieldTypeString, "Blog post topic").
		AddOutput("title", dsgo.FieldTypeString, "Creative title").
		AddOutput("hook", dsgo.FieldTypeString, "One-sentence hook")

	basePred := module.NewPredict(sig1, lm)

	// Scoring function: prefer shorter, punchier titles
	titleScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		title, _ := pred.GetString("title")
		hook, _ := pred.GetString("hook")

		// Criteria:
		// - Shorter titles score higher (up to 50 chars is ideal)
		// - Titles with numbers score bonus (data-driven appeal)
		// - Hooks under 100 chars score higher

		score := 100.0

		// Length penalty for title
		titleLen := len(title)
		if titleLen > 50 {
			score -= float64(titleLen-50) * 0.5
		}

		// Bonus for numbers in title
		if strings.ContainsAny(title, "0123456789") {
			score += 15.0
		}

		// Hook conciseness
		hookLen := len(hook)
		if hookLen > 100 {
			score -= float64(hookLen-100) * 0.3
		}

		return score, nil
	}

	// Create BestOfN with parallel execution
	bestTitle := module.NewBestOfN(basePred, 5).
		WithScorer(titleScorer).
		WithParallel(true).  // Enable parallel execution
		WithReturnAll(true). // Return all candidates for comparison
		WithThreshold(110.0) // Early stop if score ≥ 110

	start := time.Now()
	result1, err := bestTitle.Forward(ctx, map[string]any{
		"topic": "Machine Learning for Beginners",
	})
	elapsed := time.Since(start)

	if err != nil {
		return nil, stats, fmt.Errorf("BestOfN failed: %w", err)
	}

	bestTitleStr, _ := result1.GetString("title")
	bestHook, _ := result1.GetString("hook")

	fmt.Printf("Topic: Machine Learning for Beginners\n")
	fmt.Printf("Generated %d candidates in parallel (%.2fs)\n\n", len(result1.Completions), elapsed.Seconds())

	// Show all candidates
	fmt.Println("All Candidates Generated:")
	for i, completion := range result1.Completions {
		title := fmt.Sprint(completion["title"])
		rank := ""
		if i == 0 {
			rank = " 👑 WINNER (highest score)"
		}
		fmt.Printf("%d. %s%s\n", i+1, title, rank)
	}

	fmt.Printf("\n🏆 Selected Best Title:\n")
	fmt.Printf("Title: %s\n", bestTitleStr)
	fmt.Printf("Hook: %s\n", bestHook)
	fmt.Printf("Score: %.1f\n", result1.Score)
	fmt.Printf("📊 Tokens used: %d\n", result1.Usage.TotalTokens)

	totalTokens += result1.Usage.TotalTokens

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: Performance Comparison (Sequential vs Parallel)
	fmt.Println("--- Demo 2: Performance Comparison ---")
	fmt.Println("Compare sequential vs parallel execution times")
	fmt.Println()

	sig2 := dsgo.NewSignature("Solve the math problem and show your work").
		AddInput("problem", dsgo.FieldTypeString, "Math problem").
		AddOutput("solution", dsgo.FieldTypeString, "Step-by-step solution").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")

	mathPred := module.NewPredict(sig2, lm)

	mathScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		solution, _ := pred.GetString("solution")
		answer, _ := pred.GetString("answer")

		score := 50.0

		// Prefer detailed solutions
		if len(solution) > 100 {
			score += 30.0
		}

		// Prefer numeric answers
		if strings.ContainsAny(answer, "0123456789") {
			score += 20.0
		}

		return score, nil
	}

	problem := "If a rectangle has a length of 12 cm and width of 8 cm, what is its area?"

	// Sequential execution
	fmt.Println("Sequential execution (N=3)...")
	seqBest := module.NewBestOfN(mathPred, 3).
		WithScorer(mathScorer).
		WithParallel(false)

	startSeq := time.Now()
	_, err = seqBest.Forward(ctx, map[string]any{
		"problem": problem,
	})
	seqTime := time.Since(startSeq)
	if err != nil {
		return nil, stats, fmt.Errorf("sequential BestOfN failed: %w", err)
	}
	fmt.Printf("Time: %.2fs\n\n", seqTime.Seconds())

	// Parallel execution
	fmt.Println("Parallel execution (N=3)...")
	parBest := module.NewBestOfN(mathPred, 3).
		WithScorer(mathScorer).
		WithParallel(true)

	startPar := time.Now()
	result2, err := parBest.Forward(ctx, map[string]any{
		"problem": problem,
	})
	parTime := time.Since(startPar)
	if err != nil {
		return nil, stats, fmt.Errorf("parallel BestOfN failed: %w", err)
	}
	fmt.Printf("Time: %.2fs\n", parTime.Seconds())

	speedup := float64(seqTime) / float64(parTime)
	fmt.Printf("⚡ Speedup: %.2fx faster\n\n", speedup)

	solution, _ := result2.GetString("solution")
	answer, _ := result2.GetString("answer")
	fmt.Printf("Best Solution:\n%s\n\n", solution)
	fmt.Printf("Answer: %s\n", answer)
	fmt.Printf("📊 Tokens used: %d\n", result2.Usage.TotalTokens)

	totalTokens += result2.Usage.TotalTokens

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Early Stopping with Threshold
	fmt.Println("--- Demo 3: Early Stopping with Threshold ---")
	fmt.Println("Save API calls by stopping when a good enough candidate is found")
	fmt.Println()

	sig3 := dsgo.NewSignature("Write a one-sentence product tagline").
		AddInput("product", dsgo.FieldTypeString, "Product description").
		AddOutput("tagline", dsgo.FieldTypeString, "Catchy tagline")

	taglinePred := module.NewPredict(sig3, lm)

	taglineScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		tagline, _ := pred.GetString("tagline")

		score := 100.0 - float64(len(tagline))

		// High score for very short, punchy taglines
		if len(tagline) < 40 {
			score += 50.0
		}

		return score, nil
	}

	earlyStop := module.NewBestOfN(taglinePred, 10).
		WithScorer(taglineScorer).
		WithParallel(true).
		WithThreshold(130.0). // Stop early if we find a great one
		WithReturnAll(true)

	result3, err := earlyStop.Forward(ctx, map[string]any{
		"product": "A smart water bottle that tracks hydration and glows when you need to drink",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("early stop BestOfN failed: %w", err)
	}

	fmt.Printf("Requested 10 candidates, generated %d (early stopped)\n", len(result3.Completions))
	tagline, _ := result3.GetString("tagline")
	fmt.Printf("Best tagline: \"%s\"\n", tagline)
	fmt.Printf("Score: %.1f (threshold: 130.0)\n", result3.Score)
	fmt.Printf("📊 Tokens used: %d\n", result3.Usage.TotalTokens)

	totalTokens += result3.Usage.TotalTokens

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 3
	stats.Metadata["candidates_demo1"] = len(result1.Completions)
	stats.Metadata["candidates_demo3"] = len(result3.Completions)
	stats.Metadata["speedup"] = speedup

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- When to Use Parallel Execution ---")
	fmt.Println("✅ Good for:")
	fmt.Println("  • Creative tasks (titles, taglines, summaries)")
	fmt.Println("  • Latency-sensitive applications")
	fmt.Println("  • High N values (N≥5)")
	fmt.Println("  • Stateless modules")
	fmt.Println()
	fmt.Println("❌ Avoid for:")
	fmt.Println("  • Modules with shared state (e.g., History)")
	fmt.Println("  • Low N values (N<3) - overhead not worth it")
	fmt.Println("  • API rate limits (use sequential)")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("Parallel execution benefits:")
	fmt.Println("  ✓ 2-3x faster execution with parallel=true")
	fmt.Println("  ✓ Custom scoring enables domain-specific selection")
	fmt.Println("  ✓ Early stopping saves API calls when threshold met")
	fmt.Println("  ✓ ReturnAll allows analysis of all candidates")
	fmt.Println()
	fmt.Printf("⚠️  Important: Ensure modules are stateless for parallel execution\n")
	fmt.Println("   Modules with History cause data races. Use separate instances.")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 3\n")
	fmt.Println()

	return result3, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
