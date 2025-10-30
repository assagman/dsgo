package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

// This example demonstrates BestOfN with parallel execution for improved performance.
// BestOfN generates N candidate solutions and selects the best one based on a scoring function.
//
// Features demonstrated:
// - Parallel execution for speed
// - Custom scoring functions
// - Early stopping with thresholds
// - Returning all candidates for analysis

func main() {
	shared.LoadEnv()

	fmt.Println("=== BestOfN Parallel Execution Example ===")
	fmt.Println("Generate multiple solutions and select the best one")

	lm := shared.GetLM(shared.GetModel())

	// Example 1: Creative Title Generation
	fmt.Println("--- Example 1: Best Title Selection (Parallel) ---")
	
	sig1 := dsgo.NewSignature("Generate a creative and catchy title for a blog post").
		AddInput("topic", dsgo.FieldTypeString, "Blog post topic").
		AddOutput("title", dsgo.FieldTypeString, "Creative title").
		AddOutput("hook", dsgo.FieldTypeString, "One-sentence hook")

	// Create base module
	basePred := module.NewPredict(sig1, lm)

	// Scoring function: prefer shorter, punchier titles
	titleScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		title, _ := pred.GetString("title")
		hook, _ := pred.GetString("hook")
		
		// Criteria:
		// - Shorter titles score higher (up to 50 chars is ideal)
		// - Titles with numbers score bonus
		// - Hooks under 100 chars score higher
		
		score := 100.0
		
		// Length penalty for title
		titleLen := len(title)
		if titleLen > 50 {
			score -= float64(titleLen-50) * 0.5
		}
		
		// Bonus for numbers in title (data-driven appeal)
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
		WithParallel(true).     // Enable parallel execution
		WithReturnAll(true).    // Return all candidates for comparison
		WithThreshold(110.0)    // Early stop if score ≥ 110

	ctx := context.Background()

	start := time.Now()
	result1, err := bestTitle.Forward(ctx, map[string]any{
		"topic": "Machine Learning for Beginners",
	})
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("BestOfN failed: %v", err)
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
	fmt.Printf("Score: %.1f\n\n", result1.Score)

	// Example 2: Math Problem Solving (Sequential vs Parallel)
	fmt.Println("--- Example 2: Performance Comparison ---")
	
	sig2 := dsgo.NewSignature("Solve the math problem and show your work").
		AddInput("problem", dsgo.FieldTypeString, "Math problem").
		AddOutput("solution", dsgo.FieldTypeString, "Step-by-step solution").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")

	mathPred := module.NewPredict(sig2, lm)

	// Simple correctness scorer (would need validation in real use)
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

	// Sequential execution
	fmt.Println("Sequential execution (N=3)...")
	seqBest := module.NewBestOfN(mathPred, 3).
		WithScorer(mathScorer).
		WithParallel(false)

	startSeq := time.Now()
	_, err = seqBest.Forward(ctx, map[string]any{
		"problem": "If a rectangle has a length of 12 cm and width of 8 cm, what is its area?",
	})
	seqTime := time.Since(startSeq)
	if err != nil {
		log.Fatalf("Sequential BestOfN failed: %v", err)
	}
	fmt.Printf("Time: %.2fs\n\n", seqTime.Seconds())

	// Parallel execution
	fmt.Println("Parallel execution (N=3)...")
	parBest := module.NewBestOfN(mathPred, 3).
		WithScorer(mathScorer).
		WithParallel(true)

	startPar := time.Now()
	result3, err := parBest.Forward(ctx, map[string]any{
		"problem": "If a rectangle has a length of 12 cm and width of 8 cm, what is its area?",
	})
	parTime := time.Since(startPar)
	if err != nil {
		log.Fatalf("Parallel BestOfN failed: %v", err)
	}
	fmt.Printf("Time: %.2fs\n", parTime.Seconds())
	fmt.Printf("⚡ Speedup: %.2fx faster\n\n", float64(seqTime)/float64(parTime))

	solution, _ := result3.GetString("solution")
	answer, _ := result3.GetString("answer")
	fmt.Printf("Best Solution:\n%s\n\n", solution)
	fmt.Printf("Answer: %s\n\n", answer)

	// Example 3: Early Stopping with Threshold
	fmt.Println("--- Example 3: Early Stopping ---")
	
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

	result4, err := earlyStop.Forward(ctx, map[string]any{
		"product": "A smart water bottle that tracks hydration and glows when you need to drink",
	})
	if err != nil {
		log.Fatalf("Early stop BestOfN failed: %v", err)
	}

	fmt.Printf("Requested 10 candidates, generated %d (early stopped)\n", len(result4.Completions))
	tagline, _ := result4.GetString("tagline")
	fmt.Printf("Best tagline: \"%s\"\n", tagline)

	fmt.Println("\n=== Key Takeaways ===")
	fmt.Println("✓ Parallel execution speeds up BestOfN significantly")
	fmt.Println("✓ Custom scoring functions enable domain-specific selection")
	fmt.Println("✓ Early stopping saves API calls when threshold is met")
	fmt.Println("✓ ReturnAll allows analysis of all candidates")
	fmt.Println("✓ Perfect for creative tasks: titles, taglines, summaries")
	
	fmt.Println("\n⚠️  Note: For parallel execution, ensure modules are stateless")
	fmt.Println("   or use independent instances (see module/best_of_n.go docs)")
}
