package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()
	
	fmt.Println("=== LM Factory Pattern Demo ===")

	// Setup a collector to track LM usage
	memCollector := dsgo.NewMemoryCollector(100)

	// Example 1: Basic usage with OpenRouter + Observability
	fmt.Println("Example 1: Basic Factory Usage with Observability")
	fmt.Println("--------------------------------------------------")
	
	lm := shared.GetLM(shared.GetModel())
	fmt.Printf("✓ Created instrumented LM: %s\n\n", shared.GetModel())

	// Use the LM
	sig := dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")

	predictor := module.NewPredict(sig, lm)

	ctx := context.Background()
	prediction, err := predictor.Forward(ctx, map[string]any{
		"text": "I love this library! It makes AI development so easy.",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	sentiment, _ := prediction.GetString("sentiment")
	fmt.Printf("Sentiment: %s\n\n", sentiment)

	// Print collected history
	fmt.Println("Collected History:")
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
		fmt.Printf("  Response: %s\n", entry.Response.Content[:min(50, len(entry.Response.Content))]+"...")
	}
	fmt.Println()

	// Example 2: Switching providers dynamically
	fmt.Println("Example 2: Dynamic Provider Switching")
	fmt.Println("--------------------------------------")

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

		lm, err := dsgo.NewLM(ctx)
		if err != nil {
			fmt.Printf("⨯ Failed to create LM for %s: %v\n", p.name, err)
			continue
		}
		fmt.Printf("✓ Created LM: %s (%s)\n", lm.Name(), p.name)
	}
	fmt.Println()

	// Example 3: Error handling
	fmt.Println("Example 3: Error Handling")
	fmt.Println("-------------------------")

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
	fmt.Println()

	// Example 4: Environment variable configuration
	fmt.Println("Example 4: Environment Variables")
	fmt.Println("---------------------------------")
	_ = os.Setenv("DSGO_PROVIDER", "openrouter")
	_ = os.Setenv("DSGO_MODEL", "anthropic/claude-3.5-sonnet")

	dsgo.ResetConfig()
	dsgo.Configure() // Loads from environment

	lm, err = dsgo.NewLM(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created LM from env vars: %s\n", lm.Name())

	// Example 5: Export history to JSON
	fmt.Println("\nExample 5: Export History to JSON")
	fmt.Println("----------------------------------")
	if len(entries) > 0 {
		jsonData, _ := json.MarshalIndent(entries[0], "", "  ")
		fmt.Printf("Sample History Entry (JSON):\n%s\n", string(jsonData)[:500]+"...")
	}

	fmt.Println("\n=== Demo Complete ===")
}
