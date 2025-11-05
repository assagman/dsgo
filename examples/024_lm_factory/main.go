package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "024_lm_factory", runExample)
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

	var totalTokens int
	var lastPred *dsgo.Prediction

	fmt.Println("=== LM Factory Pattern ===")
	fmt.Println("Demonstrates DSGo's LM factory for dynamic provider/model instantiation")
	fmt.Println()

	fmt.Println("--- Factory Features ---")
	fmt.Println("✓ Dynamic LM creation with dsgo.NewLM()")
	fmt.Println("✓ Provider switching at runtime")
	fmt.Println("✓ Automatic provider registration")
	fmt.Println("✓ Environment-based configuration")
	fmt.Println("✓ Graceful error handling")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Setup a collector to track LM usage
	memCollector := dsgo.NewMemoryCollector(100)

	// Example 1: Basic usage with factory
	fmt.Println("--- Example 1: Basic Factory Usage ---")
	fmt.Println("Create LM using factory with global configuration")
	fmt.Println()

	lm := shared.GetLM(shared.GetModel())
	fmt.Printf("✓ Created instrumented LM: %s\n", shared.GetModel())

	// Use the LM
	sig := dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")

	predictor := module.NewPredict(sig, lm)

	prediction, err := predictor.Forward(ctx, map[string]any{
		"text": "I love this library! It makes AI development so easy.",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("prediction failed: %w", err)
	}

	sentiment, _ := prediction.GetString("sentiment")
	fmt.Printf("\nSentiment: %s\n", sentiment)
	fmt.Printf("📊 Tokens used: %d\n", prediction.Usage.TotalTokens)

	totalTokens += prediction.Usage.TotalTokens
	lastPred = prediction

	// Print collected history
	fmt.Println("\nCollected History:")
	entries := memCollector.GetAll()
	if len(entries) > 0 {
		entry := entries[0]
		fmt.Printf("  ID: %s\n", entry.ID)
		fmt.Printf("  Model: %s\n", entry.Model)
		fmt.Printf("  Provider: %s\n", entry.Provider)
		fmt.Printf("  Usage: %d prompt tokens, %d completion tokens\n",
			entry.Usage.PromptTokens, entry.Usage.CompletionTokens)
		fmt.Printf("  Cost: $%.6f\n", entry.Usage.Cost)
		fmt.Printf("  Latency: %dms\n", entry.Usage.Latency)
		if len(entry.Response.Content) > 50 {
			fmt.Printf("  Response: %s...\n", entry.Response.Content[:50])
		} else {
			fmt.Printf("  Response: %s\n", entry.Response.Content)
		}

		stats.Metadata["history_entry_id"] = entry.ID
		stats.Metadata["history_model"] = entry.Model
	}

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 2: Switching providers dynamically
	fmt.Println("--- Example 2: Dynamic Provider Switching ---")
	fmt.Println("Switch between providers at runtime using dsgo.Configure()")
	fmt.Println()

	providers := []struct {
		name  string
		model string
	}{
		{"openrouter", "google/gemini-2.5-flash"},
		{"openai", "gpt-4"},
	}

	for _, p := range providers {
		dsgo.Configure(
			dsgo.WithProvider(p.name),
			dsgo.WithModel(p.model),
		)

		lmInstance, err := dsgo.NewLM(ctx)
		if err != nil {
			fmt.Printf("⨯ Failed to create LM for %s: %v\n", p.name, err)
			continue
		}
		fmt.Printf("✓ Created LM: %s (%s)\n", lmInstance.Name(), p.name)
	}

	stats.Metadata["providers_tested"] = len(providers)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 3: Error handling
	fmt.Println("--- Example 3: Error Handling ---")
	fmt.Println("Gracefully handle configuration errors")
	fmt.Println()

	// Missing provider
	dsgo.ResetConfig()
	dsgo.Configure(dsgo.WithModel("some-model"))
	_, err = dsgo.NewLM(ctx)
	if err != nil {
		fmt.Printf("✓ Expected error (no provider): %v\n", err)
	}

	// Missing model
	dsgo.ResetConfig()
	dsgo.Configure(dsgo.WithProvider("openai"))
	_, err = dsgo.NewLM(ctx)
	if err != nil {
		fmt.Printf("✓ Expected error (no model): %v\n", err)
	}

	// Unknown provider
	dsgo.ResetConfig()
	dsgo.Configure(
		dsgo.WithProvider("nonexistent"),
		dsgo.WithModel("some-model"),
	)
	_, err = dsgo.NewLM(ctx)
	if err != nil {
		fmt.Printf("✓ Expected error (unknown provider): %v\n", err)
	}

	stats.Metadata["error_handling_tests"] = 3

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 4: Environment variable configuration
	fmt.Println("--- Example 4: Environment Variables ---")
	fmt.Println("Configure LM factory using environment variables")
	fmt.Println()

	_ = os.Setenv("DSGO_PROVIDER", "openrouter")
	_ = os.Setenv("DSGO_MODEL", "anthropic/claude-3.5-sonnet")

	dsgo.ResetConfig()
	dsgo.Configure() // Loads from environment

	lmInstance, err := dsgo.NewLM(ctx)
	if err != nil {
		return lastPred, stats, fmt.Errorf("failed to create LM from env vars: %w", err)
	}
	fmt.Printf("✓ Created LM from env vars: %s\n", lmInstance.Name())

	stats.Metadata["env_configured_model"] = lmInstance.Name()

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 5: Export history to JSON
	fmt.Println("--- Example 5: Export History to JSON ---")
	fmt.Println("Serialize collected history for observability")
	fmt.Println()

	if len(entries) > 0 {
		jsonData, _ := json.MarshalIndent(entries[0], "", "  ")
		jsonStr := string(jsonData)
		if len(jsonStr) > 500 {
			fmt.Printf("Sample History Entry (JSON):\n%s...\n", jsonStr[:500])
		} else {
			fmt.Printf("Sample History Entry (JSON):\n%s\n", jsonStr)
		}
		stats.Metadata["history_export"] = true
	}

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- LM Factory Pattern Benefits ---")
	fmt.Println("✓ Centralized configuration management")
	fmt.Println("✓ Easy provider switching without code changes")
	fmt.Println("✓ Environment-based configuration for deployments")
	fmt.Println("✓ Type-safe LM instantiation")
	fmt.Println("✓ Automatic provider registration")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("LM Factory provides:")
	fmt.Println("  ✓ Dynamic provider/model selection")
	fmt.Println("  ✓ Global configuration with local overrides")
	fmt.Println("  ✓ Graceful error handling")
	fmt.Println("  ✓ Environment-based flexibility")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total examples: 5\n")
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["total_examples"] = 5

	return lastPred, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
