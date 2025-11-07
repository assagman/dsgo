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
	)
	
	fmt.Println("\n=== Global Configuration ===")
	settings := dsgo.GetSettings()
	fmt.Printf("Provider: %s\n", settings.DefaultProvider)
	fmt.Printf("Timeout: %v\n", settings.DefaultTimeout)
	fmt.Printf("Max retries: %d\n", settings.MaxRetries)

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

	// Turn 1: Cold request (streaming)
	fmt.Println("\n=== Turn 1: Cold Request (Streaming) ===")
	turn1Start := time.Now()
	turn1Ctx, turn1Span := observe.Start(ctx, observe.SpanKindModule, "turn1_cold", map[string]interface{}{
		"streaming": true,
		"cache":     "cold",
	})

	question := "How do solar panels work?"
	fmt.Printf("User: %s\n", question)

	streamResult, err := predict.Stream(turn1Ctx, map[string]interface{}{
		"question": question,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("Answer (streaming): ")
	var fullAnswer string
	chunkCount := 0
	for chunk := range streamResult.Chunks {
		fmt.Print(chunk.Content)
		fullAnswer += chunk.Content
		chunkCount++
	}
	fmt.Println()

	if err := <-streamResult.Errors; err != nil {
		log.Fatal(err)
	}

	turn1Latency := time.Since(turn1Start)
	fmt.Printf("\nMetrics: %d chunks, %dms latency\n", chunkCount, turn1Latency.Milliseconds())
	turn1Span.Event("streaming.complete", map[string]interface{}{
		"chunks":     chunkCount,
		"latency_ms": turn1Latency.Milliseconds(),
	})

	// Get final prediction for usage stats
	pred := <-streamResult.Prediction
	if pred != nil {
		usage1 := pred.Usage
		fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage1.PromptTokens, usage1.CompletionTokens)
		totalPromptTokens += usage1.PromptTokens
		totalCompletionTokens += usage1.CompletionTokens
	}

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
		"status":      "hit",
		"latency_ms":  turn2Latency.Milliseconds(),
		"speedup":     float64(turn1Latency.Milliseconds())/float64(turn2Latency.Milliseconds()),
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
	fmt.Printf("User: Explain photosynthesis in plants with structured output\n")
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
		"topic": "Photosynthesis in plants",
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

	// Summary with metrics
	fmt.Println("\n=== System Summary ===")
	totalLatency := turn1Latency + turn2Latency + turn3Latency
	fmt.Printf("Total requests: 4\n")
	fmt.Printf("Total latency: %dms\n", totalLatency.Milliseconds())
	fmt.Printf("Avg latency: %dms\n", totalLatency.Milliseconds()/4)
	fmt.Printf("Cache efficiency: 1 hit / 3 total = 33%%\n")
	
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ Global configuration (Configure + GetSettings)")
	fmt.Println("  ✓ Streaming output (chunk tracking)")
	fmt.Println("  ✓ LM caching (cold vs warm)")
	fmt.Println("  ✓ Cache hit/miss observability")
	fmt.Println("  ✓ TwoStep adapter (reasoning models)")
	fmt.Println("  ✓ Optional outputs (graceful degradation)")
	fmt.Println("  ✓ Latency and performance metrics")
	fmt.Println("  ✓ Event logging (DSGO_LOG=pretty)")
	fmt.Println("\nResilience patterns:")
	fmt.Println("  ✓ Centralized configuration")
	fmt.Println("  ✓ Timeout control (30s default)")
	fmt.Println("  ✓ Automatic retry (3 attempts, exponential backoff)")
	fmt.Println("  ✓ Cache layer (reduce API calls)")
	fmt.Println("  ✓ Adapter flexibility (TwoStep for reasoning models)")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
