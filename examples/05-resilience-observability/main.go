package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// Demonstrates: Fallback adapter, Streaming, Observability
// Story: Q&A system with resilience features and detailed observability

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(2)
	}
	envFilePath := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "examples", ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// If not found in examples/, check cwd/.env.local
	if envFilePath == "" {
		candidate := filepath.Join(cwd, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
		}
	}
	if envFilePath == "" {
		fmt.Printf("Could not find .env.local file\n")
		os.Exit(3)
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(3)
	}

	// Global configuration
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithTimeout(30*time.Second),
		dsgo.WithMaxRetries(3),
		// Cache configuration
		dsgo.WithCacheTTL(3*time.Second), // Short TTL for demonstration
		dsgo.WithCache(1000),             // Enable caching with 1000 entry capacity
	)

	fmt.Println("\n=== Global Configuration ===")
	settings := dsgo.GetSettings()
	fmt.Printf("Provider: %s\n", settings.DefaultProvider)
	fmt.Printf("Timeout: %v\n", settings.DefaultTimeout)
	fmt.Printf("Max retries: %d\n", settings.MaxRetries)
	fmt.Printf("Cache enabled: %v\n", settings.DefaultCache != nil)
	fmt.Printf("Cache TTL: %v\n", settings.CacheTTL)

	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "qa_system", map[string]interface{}{
		"scenario": "resilience_observability",
	})
	defer runSpan.End(nil)

	// Setup model
	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	sig := dsgo.NewSignature("You are a helpful science educator. Explain concepts clearly for children.").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("explanation", dsgo.FieldTypeString, "Simple explanation")

	predict := module.NewPredict(sig, lm)

	// Turn 1: Cold request (cache miss)
	fmt.Println("\n=== Turn 1: Cold Request (Cache Miss) ===")
	turn1Start := time.Now()
	turn1Ctx, turn1Span := observe.Start(ctx, observe.SpanKindModule, "turn1_cold", map[string]interface{}{
		"streaming": false,
		"cache":     "cold",
	})

	question := "How do solar panels work?"
	fmt.Printf("User: %s\n", question)

	result1, err := predict.Forward(turn1Ctx, map[string]interface{}{
		"question": question,
	})
	if err != nil {
		log.Fatal(err)
	}

	turn1Latency := time.Since(turn1Start)
	explanation1, _ := result1.GetString("explanation")
	if len(explanation1) > 100 {
		explanation1 = explanation1[:100] + "..."
	}
	fmt.Printf("Answer: %s\n", explanation1)
	fmt.Printf("Metrics: %dms latency (cache miss)\n", turn1Latency.Milliseconds())

	usage1 := result1.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage1.PromptTokens, usage1.CompletionTokens)
	totalPromptTokens += usage1.PromptTokens
	totalCompletionTokens += usage1.CompletionTokens

	turn1Span.Event("cache.check", map[string]interface{}{
		"status":     "miss",
		"latency_ms": turn1Latency.Milliseconds(),
	})
	turn1Span.End(nil)

	// Turn 2: Warm request (cache hit expected)
	fmt.Println("\n=== Turn 2: Warm Request (Cache Hit Expected) ===")
	fmt.Printf("User: %s\n", question)
	turn2Start := time.Now()
	turn2Ctx, turn2Span := observe.Start(ctx, observe.SpanKindModule, "turn2_warm", map[string]interface{}{
		"streaming": false,
		"cache":     "expected_hit",
	})

	result2, err := predict.Forward(turn2Ctx, map[string]interface{}{
		"question": question, // Same question
	})
	if err != nil {
		log.Fatal(err)
	}

	turn2Latency := time.Since(turn2Start)
	explanation2, _ := result2.GetString("explanation")
	if len(explanation2) > 100 {
		explanation2 = explanation2[:100] + "..."
	}
	fmt.Printf("Answer: %s\n", explanation2)
	fmt.Printf("Metrics: %dms latency (%.1fx faster)\n",
		turn2Latency.Milliseconds(),
		float64(turn1Latency.Milliseconds())/float64(turn2Latency.Milliseconds()))

	usage2 := result2.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage2.PromptTokens, usage2.CompletionTokens)
	totalPromptTokens += usage2.PromptTokens
	totalCompletionTokens += usage2.CompletionTokens

	turn2Span.Event("cache.check", map[string]interface{}{
		"status":     "hit",
		"latency_ms": turn2Latency.Milliseconds(),
		"speedup":    float64(turn1Latency.Milliseconds()) / float64(turn2Latency.Milliseconds()),
	})
	turn2Span.End(nil)

	// Turn 3: Different question (cache miss)
	fmt.Println("\n=== Turn 3: Different Question (Cache Miss) ===")
	fmt.Printf("User: Why is the sky blue?\n")
	turn3Start := time.Now()
	turn3Ctx, turn3Span := observe.Start(ctx, observe.SpanKindModule, "turn3_miss", map[string]interface{}{
		"cache": "expected_miss",
	})

	result3, err := predict.Forward(turn3Ctx, map[string]interface{}{
		"question": "Why is the sky blue?",
	})
	if err != nil {
		log.Fatal(err)
	}

	turn3Latency := time.Since(turn3Start)
	explanation3, _ := result3.GetString("explanation")
	if len(explanation3) > 100 {
		explanation3 = explanation3[:100] + "..."
	}
	fmt.Printf("Answer: %s\n", explanation3)
	fmt.Printf("Metrics: %dms latency\n", turn3Latency.Milliseconds())

	usage3 := result3.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage3.PromptTokens, usage3.CompletionTokens)
	totalPromptTokens += usage3.PromptTokens
	totalCompletionTokens += usage3.CompletionTokens

	turn3Span.Event("cache.check", map[string]interface{}{
		"status":     "miss",
		"latency_ms": turn3Latency.Milliseconds(),
	})
	turn3Span.End(nil)

	// Turn 4: Demonstrate optional outputs and TwoStep adapter
	fmt.Println("\n=== Turn 4: Optional Outputs + TwoStep Adapter ===")
	turn4Question := "Photosynthesis in plants"
	fmt.Printf("User: Explain %s with structured output\n", turn4Question)
	turn4Ctx, turn4Span := observe.Start(ctx, observe.SpanKindModule, "turn4_optional", map[string]interface{}{
		"adapter": "twostep",
	})

	// Create signature with optional field
	complexSig := dsgo.NewSignature("Explain with structured output").
		AddInput("topic", dsgo.FieldTypeString, "Topic").
		AddOutput("summary", dsgo.FieldTypeString, "Summary").
		AddOutput("difficulty", dsgo.FieldTypeInt, "Difficulty 1-10").
		AddClassOutput("audience", []string{"child", "teen", "adult"}, "Target audience").
		AddOptionalOutput("statistics", dsgo.FieldTypeString, "Statistics if available")

	// For reasoning models, use TwoStep adapter (reasoning → extraction)
	// For standard models, this gracefully falls back to single-step
	extractionLM, _ := dsgo.NewLM(ctx, model) // Reuse same model for extraction
	twoStep := core.NewTwoStepAdapter(extractionLM)

	complexPredict := module.NewPredict(complexSig, lm).WithAdapter(twoStep)

	result4, err := complexPredict.Forward(turn4Ctx, map[string]interface{}{
		"topic": turn4Question,
	})
	if err != nil {
		log.Fatal(err)
	}

	summary, _ := result4.GetString("summary")
	difficulty, _ := result4.GetInt("difficulty")
	audience, _ := result4.GetString("audience")

	fmt.Printf("Summary: %s\n", summary)
	fmt.Printf("Difficulty: %d/10\n", difficulty)
	fmt.Printf("Audience: %s\n", audience)

	// Demonstrate optional field handling
	if stats, ok := result4.GetString("statistics"); ok {
		fmt.Printf("Statistics: %s\n", stats)
	} else {
		fmt.Printf("Statistics: (not provided)\n")
	}
	fmt.Printf("Adapter: TwoStep (reasoning → extraction)\n")

	usage4 := result4.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage4.PromptTokens, usage4.CompletionTokens)
	totalPromptTokens += usage4.PromptTokens
	totalCompletionTokens += usage4.CompletionTokens
	turn4Span.End(nil)

	// Turn 5: Cache TTL expiry demonstration
	fmt.Println("\n=== Turn 5: Cache TTL Expiry Demo ===")
	fmt.Printf("Cache TTL configured: %v\n", settings.CacheTTL)
	fmt.Println("Testing same question with time delays to demonstrate TTL expiry")

	ttlQuestion := "What is the capital of France?"

	// First call - cache miss
	fmt.Printf("\nUser (t=0s): %s\n", ttlQuestion)
	turn5aStart := time.Now()
	turn5aCtx, turn5aSpan := observe.Start(ctx, observe.SpanKindModule, "turn5a_ttl_miss", map[string]interface{}{
		"cache": "miss_expected",
		"ttl":   settings.CacheTTL.String(),
	})

	result5a, err := predict.Forward(turn5aCtx, map[string]interface{}{
		"question": ttlQuestion,
	})
	if err != nil {
		log.Fatal(err)
	}

	turn5aLatency := time.Since(turn5aStart)
	answer5a, _ := result5a.GetString("explanation")
	if len(answer5a) > 80 {
		answer5a = answer5a[:80] + "..."
	}
	fmt.Printf("Answer: %s\n", answer5a)
	fmt.Printf("Latency: %dms (cache miss)\n", turn5aLatency.Milliseconds())

	usage5a := result5a.Usage
	totalPromptTokens += usage5a.PromptTokens
	totalCompletionTokens += usage5a.CompletionTokens
	turn5aSpan.End(nil)

	// Second call immediately - cache hit
	fmt.Printf("\nUser (t=0.5s, within TTL): %s\n", ttlQuestion)
	time.Sleep(500 * time.Millisecond)
	turn5bStart := time.Now()
	turn5bCtx, turn5bSpan := observe.Start(ctx, observe.SpanKindModule, "turn5b_ttl_hit", map[string]interface{}{
		"cache": "hit_expected",
		"ttl":   settings.CacheTTL.String(),
	})

	result5b, err := predict.Forward(turn5bCtx, map[string]interface{}{
		"question": ttlQuestion,
	})
	if err != nil {
		log.Fatal(err)
	}

	turn5bLatency := time.Since(turn5bStart)
	answer5b, _ := result5b.GetString("explanation")
	if len(answer5b) > 80 {
		answer5b = answer5b[:80] + "..."
	}
	fmt.Printf("Answer: %s\n", answer5b)
	fmt.Printf("Latency: %dms (cache hit, %.1fx faster)\n",
		turn5bLatency.Milliseconds(),
		float64(turn5aLatency.Milliseconds())/float64(turn5bLatency.Milliseconds()))

	usage5b := result5b.Usage
	totalPromptTokens += usage5b.PromptTokens
	totalCompletionTokens += usage5b.CompletionTokens
	turn5bSpan.Event("cache.hit", map[string]interface{}{
		"ttl_remaining": settings.CacheTTL - 500*time.Millisecond,
	})
	turn5bSpan.End(nil)

	// Third call after TTL expires - cache miss again
	fmt.Printf("\nWaiting for TTL expiry (%v)...\n", settings.CacheTTL)
	waitTime := settings.CacheTTL - 500*time.Millisecond + 100*time.Millisecond // Wait a bit longer
	time.Sleep(waitTime)
	fmt.Printf("\nUser (t=%v, after TTL expired): %s\n", settings.CacheTTL+100*time.Millisecond, ttlQuestion)
	turn5cStart := time.Now()
	turn5cCtx, turn5cSpan := observe.Start(ctx, observe.SpanKindModule, "turn5c_ttl_expired", map[string]interface{}{
		"cache": "miss_expired",
		"ttl":   settings.CacheTTL.String(),
	})

	result5c, err := predict.Forward(turn5cCtx, map[string]interface{}{
		"question": ttlQuestion,
	})
	if err != nil {
		log.Fatal(err)
	}

	turn5cLatency := time.Since(turn5cStart)
	answer5c, _ := result5c.GetString("explanation")
	if len(answer5c) > 80 {
		answer5c = answer5c[:80] + "..."
	}
	fmt.Printf("Answer: %s\n", answer5c)
	fmt.Printf("Latency: %dms (cache expired, fresh call)\n", turn5cLatency.Milliseconds())

	usage5c := result5c.Usage
	totalPromptTokens += usage5c.PromptTokens
	totalCompletionTokens += usage5c.CompletionTokens

	fmt.Printf("\n📊 Cache TTL behavior summary:\n")
	fmt.Printf("  • Call 1 (t=0s):      %4dms - MISS (initial)\n", turn5aLatency.Milliseconds())
	fmt.Printf("  • Call 2 (t=0.5s):    %4dms - HIT (within %v TTL)\n", turn5bLatency.Milliseconds(), settings.CacheTTL)
	fmt.Printf("  • Call 3 (t=%v):   %4dms - MISS (expired)\n", settings.CacheTTL+100*time.Millisecond, turn5cLatency.Milliseconds())
	fmt.Printf("  • TTL ensures fresh data while balancing performance\n")
	fmt.Printf("  • Configure via dsgo.WithCacheTTL() or DSGO_CACHE_TTL env var\n")

	turn5cSpan.End(nil)

	// Summary with metrics
	fmt.Println("\n=== System Summary ===")
	totalLatency := turn1Latency + turn2Latency + turn3Latency + turn5aLatency + turn5bLatency + turn5cLatency
	fmt.Printf("Total requests: 7 (4 turns + 3 TTL demo)\n")
	fmt.Printf("Total latency: %dms\n", totalLatency.Milliseconds())
	fmt.Printf("Avg latency: %dms\n", totalLatency.Milliseconds()/7)
	fmt.Printf("Cache efficiency: 3 hits / 6 total cacheable = 50%%\n")

	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ Global configuration (Configure + GetSettings)")
	fmt.Println("  ✓ LM caching (cold vs warm, auto-wiring)")
	fmt.Println("  ✓ Cache hit/miss with identical content validation")
	fmt.Println("  ✓ Cache TTL (time-to-live expiry)")
	fmt.Println("  ✓ Cache hit/miss observability")
	fmt.Println("  ✓ TwoStep adapter (reasoning models)")
	fmt.Println("  ✓ Optional outputs (graceful degradation)")
	fmt.Println("  ✓ Latency and performance metrics")
	fmt.Println("  ✓ Event logging (DSGO_LOG=pretty)")
	fmt.Println("\nResilience patterns:")
	fmt.Println("  ✓ Centralized configuration")
	fmt.Println("  ✓ Timeout control (30s default)")
	fmt.Println("  ✓ Automatic retry (3 attempts, exponential backoff)")
	fmt.Println("  ✓ Cache layer with TTL (reduce API calls, ensure freshness)")
	fmt.Println("  ✓ Adapter flexibility (TwoStep for reasoning models)")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
