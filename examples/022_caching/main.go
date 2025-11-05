package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
	"github.com/assagman/dsgo/providers/openai"
	"github.com/assagman/dsgo/providers/openrouter"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "022_caching", runExample)
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

	model := shared.GetModel()
	lm := shared.GetLM(model)

	cache := dsgo.NewLMCache(100)

	if strings.HasPrefix(model, "openrouter/") {
		or := lm.(*openrouter.OpenRouter)
		or.Cache = cache
	} else {
		oa := lm.(*openai.OpenAI)
		oa.Cache = cache
	}

	sig := dsgo.NewSignature("Translate the given text to the target language").
		AddInput("text", dsgo.FieldTypeString, "Text to translate").
		AddInput("target_language", dsgo.FieldTypeString, "Target language").
		AddOutput("translation", dsgo.FieldTypeString, "Translated text")

	predict := module.NewPredict(sig, lm)

	fmt.Println("--- Request 1: First Translation (Cache Miss) ---")
	result1, err := predict.Forward(ctx, map[string]any{
		"text":            "Hello, how are you?",
		"target_language": "Spanish",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("request 1 failed: %w", err)
	}
	translation1, _ := result1.GetString("translation")
	fmt.Printf("Translation: %s\n", translation1)
	fmt.Printf("Tokens: %d\n\n", result1.Usage.TotalTokens)

	fmt.Println("--- Request 2: Same Translation (Cache Hit) ---")
	result2, err := predict.Forward(ctx, map[string]any{
		"text":            "Hello, how are you?",
		"target_language": "Spanish",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("request 2 failed: %w", err)
	}
	translation2, _ := result2.GetString("translation")
	fmt.Printf("Translation: %s\n", translation2)
	fmt.Printf("Tokens: %d (cached)\n\n", result2.Usage.TotalTokens)

	var cacheStats dsgo.CacheStats
	if or, ok := lm.(*openrouter.OpenRouter); ok {
		cacheStats = or.Cache.Stats()
	} else if oa, ok := lm.(*openai.OpenAI); ok {
		cacheStats = oa.Cache.Stats()
	}

	stats.CacheHits = int(cacheStats.Hits)
	stats.TokensUsed = result1.Usage.TotalTokens
	stats.Metadata["cache_hit_rate"] = cacheStats.HitRate()
	stats.Metadata["cache_size"] = cacheStats.Size

	fmt.Println("--- Cache Statistics ---")
	fmt.Printf("Cache Hits: %d\n", cacheStats.Hits)
	fmt.Printf("Cache Misses: %d\n", cacheStats.Misses)
	fmt.Printf("Hit Rate: %.1f%%\n", cacheStats.HitRate())

	return result2, stats, nil
}
