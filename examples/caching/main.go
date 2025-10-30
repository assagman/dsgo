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
	"github.com/assagman/dsgo/providers/openai"
	"github.com/assagman/dsgo/providers/openrouter"
)

// This example demonstrates the LMCache feature for improving performance
// and reducing API costs by caching identical requests.

func main() {
	shared.LoadEnv()

	fmt.Println("=== LM Caching Example ===")
	fmt.Println("Demonstrating cache hits, misses, and performance benefits\n")

	// Create LM with caching enabled
	model := shared.GetModel()
	var lm dsgo.LM
	var cache dsgo.Cache

	// Create cache
	cache = dsgo.NewLMCache(100) // Cache up to 100 requests

	// Create LM based on model prefix
	if strings.HasPrefix(model, "openrouter/") {
		actualModel := strings.TrimPrefix(model, "openrouter/")
		or := openrouter.NewOpenRouter(actualModel)
		or.Cache = cache
		lm = or
	} else {
		actualModel := strings.TrimPrefix(model, "openai/")
		oa := openai.NewOpenAI(actualModel)
		oa.Cache = cache
		lm = oa
	}

	// Create signature for translation
	sig := dsgo.NewSignature("Translate the given text to the target language").
		AddInput("text", dsgo.FieldTypeString, "Text to translate").
		AddInput("target_language", dsgo.FieldTypeString, "Target language").
		AddOutput("translation", dsgo.FieldTypeString, "Translated text")

	predict := module.NewPredict(sig, lm)
	ctx := context.Background()

	// Example 1: First request (cache miss)
	fmt.Println("--- Request 1: First Translation (Cache Miss) ---")
	start := time.Now()
	result1, err := predict.Forward(ctx, map[string]any{
		"text":            "Hello, how are you?",
		"target_language": "Spanish",
	})
	elapsed1 := time.Since(start)
	if err != nil {
		log.Fatalf("Request 1 failed: %v", err)
	}
	translation1, _ := result1.GetString("translation")
	fmt.Printf("Translation: %s\n", translation1)
	fmt.Printf("Time: %v\n", elapsed1)
	fmt.Printf("Tokens: %d (prompt) + %d (completion) = %d total\n\n",
		result1.Usage.PromptTokens, result1.Usage.CompletionTokens, result1.Usage.TotalTokens)

	// Example 2: Identical request (cache hit)
	fmt.Println("--- Request 2: Same Translation (Cache Hit) ---")
	start = time.Now()
	result2, err := predict.Forward(ctx, map[string]any{
		"text":            "Hello, how are you?",
		"target_language": "Spanish",
	})
	elapsed2 := time.Since(start)
	if err != nil {
		log.Fatalf("Request 2 failed: %v", err)
	}
	translation2, _ := result2.GetString("translation")
	fmt.Printf("Translation: %s\n", translation2)
	fmt.Printf("Time: %v\n", elapsed2)
	fmt.Printf("Tokens: %d (cached)\n", result2.Usage.TotalTokens)
	fmt.Printf("⚡ Speedup: %.2fx faster\n\n", float64(elapsed1)/float64(elapsed2))

	// Example 3: Different request (cache miss)
	fmt.Println("--- Request 3: Different Text (Cache Miss) ---")
	start = time.Now()
	result3, err := predict.Forward(ctx, map[string]any{
		"text":            "Good morning!",
		"target_language": "Spanish",
	})
	elapsed3 := time.Since(start)
	if err != nil {
		log.Fatalf("Request 3 failed: %v", err)
	}
	translation3, _ := result3.GetString("translation")
	fmt.Printf("Translation: %s\n", translation3)
	fmt.Printf("Time: %v\n", elapsed3)
	fmt.Printf("Tokens: %d total\n\n", result3.Usage.TotalTokens)

	// Example 4: First request again (cache hit)
	fmt.Println("--- Request 4: First Translation Again (Cache Hit) ---")
	start = time.Now()
	result4, err := predict.Forward(ctx, map[string]any{
		"text":            "Hello, how are you?",
		"target_language": "Spanish",
	})
	elapsed4 := time.Since(start)
	if err != nil {
		log.Fatalf("Request 4 failed: %v", err)
	}
	translation4, _ := result4.GetString("translation")
	fmt.Printf("Translation: %s\n", translation4)
	fmt.Printf("Time: %v\n", elapsed4)
	fmt.Printf("⚡ Speedup: %.2fx faster\n\n", float64(elapsed1)/float64(elapsed4))

	// Get cache statistics
	fmt.Println("--- Cache Statistics ---")
	var stats dsgo.CacheStats
	if or, ok := lm.(*openrouter.OpenRouter); ok {
		stats = or.Cache.Stats()
	} else if oa, ok := lm.(*openai.OpenAI); ok {
		stats = oa.Cache.Stats()
	}
	
	fmt.Printf("Cache Hits: %d\n", stats.Hits)
	fmt.Printf("Cache Misses: %d\n", stats.Misses)
	fmt.Printf("Hit Rate: %.1f%%\n", stats.HitRate())
	fmt.Printf("Cache Size: %d entries\n", stats.Size)

	// Calculate savings
	totalTokensWithoutCache := result1.Usage.TotalTokens + result2.Usage.TotalTokens +
		result3.Usage.TotalTokens + result4.Usage.TotalTokens
	totalTokensWithCache := result1.Usage.TotalTokens + result3.Usage.TotalTokens
	savedTokens := totalTokensWithoutCache - totalTokensWithCache

	fmt.Println("\n--- Cost Savings ---")
	fmt.Printf("Tokens without cache: %d\n", totalTokensWithoutCache)
	fmt.Printf("Tokens with cache: %d\n", totalTokensWithCache)
	fmt.Printf("Tokens saved: %d (%.1f%% reduction)\n",
		savedTokens, float64(savedTokens)/float64(totalTokensWithoutCache)*100)

	// Estimate cost savings (using GPT-4 pricing as example)
	costPer1MTokens := 30.0 // $30 per 1M tokens (example)
	costSaved := float64(savedTokens) / 1_000_000 * costPer1MTokens
	fmt.Printf("Estimated cost saved: $%.4f (at $%.0f/1M tokens)\n", costSaved, costPer1MTokens)

	fmt.Println("\n=== Key Takeaways ===")
	fmt.Println("✓ Cache automatically speeds up identical requests")
	fmt.Println("✓ Cache key includes model, messages, temperature, and all parameters")
	fmt.Println("✓ LRU eviction ensures memory efficiency")
	fmt.Println("✓ Thread-safe for concurrent use")
	fmt.Println("✓ Significant cost savings for repeated queries")
}
