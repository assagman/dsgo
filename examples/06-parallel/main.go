package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
)

// Demonstrates: Parallel module for batch processing
// Story: Process multiple customer reviews in parallel for sentiment analysis

func main() {
	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "parallel_sentiment", map[string]interface{}{
		"scenario": "batch_processing",
	})
	defer runSpan.End(nil)

	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Customer reviews to analyze
	reviews := []string{
		"The product quality is amazing! Fast shipping too. Highly recommend.",
		"Disappointed with the customer service. Product is okay but nothing special.",
		"Absolutely love it! Best purchase I've made this year.",
		"Received a damaged item. Waiting for replacement. Not happy.",
		"Good value for money. Does what it promises.",
	}

	fmt.Println("=== Parallel Sentiment Analysis ===")
	fmt.Printf("Processing %d reviews in parallel...\n\n", len(reviews))

	// Define signature for sentiment analysis
	sig := core.NewSignature("Analyze customer review sentiment").
		AddInput("review", core.FieldTypeString, "Customer review text").
		AddClassOutput("sentiment", []string{"positive", "neutral", "negative"}, "Overall sentiment").
		AddOutput("reason", core.FieldTypeString, "Brief explanation")

	// Create base predictor
	predictor := module.NewPredict(sig, lm)

	// Create parallel module with configuration
	parallel := module.NewParallel(predictor).
		WithMaxWorkers(3).       // Process up to 3 reviews concurrently
		WithMaxFailures(1).      // Allow 1 failure without stopping
		WithReturnAll(true).     // Return all results
		WithOnlySuccessful(true) // Only include successful results

	// Prepare batch inputs using map-of-slices pattern
	batchInputs := map[string]any{
		"review": make([]any, len(reviews)),
	}
	for i, review := range reviews {
		batchInputs["review"].([]any)[i] = review
	}

	// Execute parallel processing
	result, err := parallel.Forward(ctx, batchInputs)
	if err != nil {
		log.Fatalf("Parallel processing failed: %v", err)
	}

	// Display results
	fmt.Printf("Successfully processed %d/%d reviews\n\n", len(result.Completions), len(reviews))

	sentimentCounts := map[string]int{"positive": 0, "neutral": 0, "negative": 0}

	for i, completion := range result.Completions {
		sentiment, _ := completion["sentiment"].(string)
		reason, _ := completion["reason"].(string)

		fmt.Printf("Review %d:\n", i+1)
		fmt.Printf("  Text: %s\n", reviews[i])
		fmt.Printf("  Sentiment: %s\n", sentiment)
		fmt.Printf("  Reason: %s\n\n", reason)

		sentimentCounts[sentiment]++
	}

	// Display summary
	fmt.Println("=== Summary ===")
	fmt.Printf("Positive: %d\n", sentimentCounts["positive"])
	fmt.Printf("Neutral: %d\n", sentimentCounts["neutral"])
	fmt.Printf("Negative: %d\n\n", sentimentCounts["negative"])

	// Display usage metrics
	fmt.Println("=== Performance Metrics ===")
	fmt.Printf("Total tokens: %d (prompt: %d, completion: %d)\n",
		result.Usage.TotalTokens,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens)
	fmt.Printf("Total cost: $%.6f\n", result.Usage.Cost)
	fmt.Printf("Latency: %dms\n", result.Usage.Latency)

	// Extract parallel metrics
	if metrics, ok := result.Outputs["__parallel_metrics"].(module.ParallelMetrics); ok {
		fmt.Printf("\nParallel execution:\n")
		fmt.Printf("  Total tasks: %d\n", metrics.Total)
		fmt.Printf("  Successes: %d\n", metrics.Successes)
		fmt.Printf("  Failures: %d\n", metrics.Failures)
		fmt.Printf("  Latency: min=%dms, avg=%dms, max=%dms, p50=%dms\n",
			metrics.Latency.MinMs,
			metrics.Latency.AvgMs,
			metrics.Latency.MaxMs,
			metrics.Latency.P50Ms)
	}

	fmt.Println("\n=== Alternative: Factory Pattern for Stateful Modules ===")
	fmt.Println("For modules with state (e.g., History), use factory pattern:")
	fmt.Println()
	fmt.Println("  factory := func(i int) core.Module {")
	fmt.Println("    history := core.NewHistory()")
	fmt.Println("    return module.NewPredict(sig, lm).WithHistory(history)")
	fmt.Println("  }")
	fmt.Println("  parallel := module.NewParallelWithFactory(factory)")
	fmt.Println()
	fmt.Println("This ensures each parallel task has its own isolated state.")
}
