package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
)

// Demonstrates: Cache configuration, TTL behavior, cache metrics
// Story: Simple Q&A system showing cache hits, misses, and TTL expiry

func main() {
	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "cache_demo", map[string]interface{}{
		"scenario": "cache_ttl_testing",
	})
	defer runSpan.End(nil)

	// Setup
	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}

	// Configure cache with 5-second TTL for demonstration
	dsgo.Configure(
		dsgo.WithCache(100),              // Cache capacity: 100 entries
		dsgo.WithCacheTTL(5*time.Second), // TTL: 5 seconds
	)

	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Display configuration
	settings := dsgo.GetSettings()
	fmt.Println("=== Cache Configuration ===")
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("Cache enabled: %v\n", settings.DefaultCache != nil)
	fmt.Printf("Cache capacity: %d entries\n", settings.DefaultCache.Capacity())
	fmt.Printf("Cache TTL: %v\n", settings.CacheTTL)
	fmt.Println()

	// Create signature and module
	sig := dsgo.NewSignature("You are a helpful assistant").
		AddInput("question", dsgo.FieldTypeString, "User question").
		AddOutput("answer", dsgo.FieldTypeString, "Concise answer")

	predict := module.NewPredict(sig, lm)

	// Test 1: Cache Miss (first call)
	fmt.Println("=== Test 1: Cache Miss ===")
	question := "What is the capital of Japan?"
	fmt.Printf("Question: %s\n", question)

	test1Start := time.Now()
	result1, err := predict.Forward(ctx, map[string]interface{}{
		"question": question,
	})
	if err != nil {
		log.Fatal(err)
	}
	test1Latency := time.Since(test1Start)

	answer1, _ := result1.GetString("answer")
	fmt.Printf("Answer: %s\n", truncate(answer1, 60))
	fmt.Printf("Latency: %dms (cache miss)\n", test1Latency.Milliseconds())
	fmt.Printf("Tokens: %d prompt, %d completion\n", result1.Usage.PromptTokens, result1.Usage.CompletionTokens)
	fmt.Println()

	// Test 2: Cache Hit (same question, within TTL)
	fmt.Println("=== Test 2: Cache Hit (within TTL) ===")
	fmt.Printf("Question: %s\n", question)

	test2Start := time.Now()
	result2, err := predict.Forward(ctx, map[string]interface{}{
		"question": question,
	})
	if err != nil {
		log.Fatal(err)
	}
	test2Latency := time.Since(test2Start)

	answer2, _ := result2.GetString("answer")
	fmt.Printf("Answer: %s\n", truncate(answer2, 60))
	fmt.Printf("Latency: %dms (cache hit, %.1fx faster)\n",
		test2Latency.Milliseconds(),
		float64(test1Latency.Milliseconds())/float64(test2Latency.Milliseconds()))
	fmt.Printf("Tokens: %d prompt, %d completion\n", result2.Usage.PromptTokens, result2.Usage.CompletionTokens)
	fmt.Println()

	// Test 3: Different Question (cache miss)
	fmt.Println("=== Test 3: Different Question (cache miss) ===")
	question3 := "What is the largest planet in our solar system?"
	fmt.Printf("Question: %s\n", question3)

	test3Start := time.Now()
	result3, err := predict.Forward(ctx, map[string]interface{}{
		"question": question3,
	})
	if err != nil {
		log.Fatal(err)
	}
	test3Latency := time.Since(test3Start)

	answer3, _ := result3.GetString("answer")
	fmt.Printf("Answer: %s\n", truncate(answer3, 60))
	fmt.Printf("Latency: %dms (cache miss)\n", test3Latency.Milliseconds())
	fmt.Printf("Tokens: %d prompt, %d completion\n", result3.Usage.PromptTokens, result3.Usage.CompletionTokens)
	fmt.Println()

	// Test 4: Wait for TTL expiry
	fmt.Println("=== Test 4: TTL Expiry ===")
	fmt.Printf("Waiting %v for TTL to expire...\n", settings.CacheTTL)
	time.Sleep(settings.CacheTTL + 100*time.Millisecond) // Wait a bit longer than TTL
	fmt.Println("TTL expired, making same request again...")
	fmt.Printf("Question: %s\n", question)

	test4Start := time.Now()
	result4, err := predict.Forward(ctx, map[string]interface{}{
		"question": question,
	})
	if err != nil {
		log.Fatal(err)
	}
	test4Latency := time.Since(test4Start)

	answer4, _ := result4.GetString("answer")
	fmt.Printf("Answer: %s\n", truncate(answer4, 60))
	fmt.Printf("Latency: %dms (cache expired, fresh call)\n", test4Latency.Milliseconds())
	fmt.Printf("Tokens: %d prompt, %d completion\n", result4.Usage.PromptTokens, result4.Usage.CompletionTokens)
	fmt.Println()

	// Test 5: Cache statistics
	fmt.Println("=== Cache Statistics ===")
	stats := settings.DefaultCache.Stats()
	fmt.Printf("Cache hits: %d\n", stats.Hits)
	fmt.Printf("Cache misses: %d\n", stats.Misses)
	fmt.Printf("Hit rate: %.1f%%\n", stats.HitRate()*100)
	fmt.Printf("Current size: %d/%d entries\n", settings.DefaultCache.Size(), settings.DefaultCache.Capacity())
	fmt.Println()

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Println("Cache behavior demonstrated:")
	fmt.Println("  ✓ Test 1: First call → cache miss (full LM call)")
	fmt.Println("  ✓ Test 2: Repeat call → cache hit (instant response)")
	fmt.Println("  ✓ Test 3: Different question → cache miss (new query)")
	fmt.Println("  ✓ Test 4: After TTL expiry → cache miss (fresh data)")
	fmt.Println()
	fmt.Println("Key benefits:")
	fmt.Println("  • 100-1000x faster responses for cached queries")
	fmt.Println("  • Reduced API costs (no tokens used on cache hits)")
	fmt.Println("  • TTL ensures data freshness")
	fmt.Println("  • Automatic cache management")
	fmt.Println()
	fmt.Println("Configuration options:")
	fmt.Println("  • Programmatic: dsgo.WithCache(capacity), dsgo.WithCacheTTL(duration)")
	fmt.Println("  • Environment: DSGO_CACHE_TTL=5m (e.g., 5m, 1h, 30s)")
	fmt.Println("  • TTL=0: No expiration (cache until capacity limit)")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
