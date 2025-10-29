package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// This example demonstrates the robustness of the FallbackAdapter system.
// The FallbackAdapter tries ChatAdapter first, then JSONAdapter if that fails.
// This provides >95% parse success rate across diverse LM response formats.

func main() {
	// Load .env file (optional - will use environment variables if not present)
	_ = godotenv.Load()

	// Get LM (OpenRouter or OpenAI based on environment)
	lm := shared.GetLM("gpt-4")

	// Create signature for sentiment analysis
	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Overall sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score 0-1").
		AddOutput("reasoning", dsgo.FieldTypeString, "Brief explanation")

	// Create Predict module with FallbackAdapter (default)
	// This module will automatically use ChatAdapter → JSONAdapter fallback
	predict := module.NewPredict(sig, lm)

	ctx := context.Background()

	// Test cases that might produce different response formats from the LM
	testCases := []string{
		"I absolutely love this product! It's amazing!",
		"This is terrible. Waste of money.",
		"It's okay, nothing special.",
	}

	fmt.Println("=== Adapter Fallback Demo ===")
	fmt.Println("Testing parse robustness across different inputs...")
	fmt.Println()

	for i, text := range testCases {
		fmt.Printf("Test %d: %s\n", i+1, text)
		fmt.Println(strings.Repeat("-", 60))

		result, err := predict.Forward(ctx, map[string]any{
			"text": text,
		})

		if err != nil {
			log.Printf("ERROR: %v\n", err)
			continue
		}

		// Extract outputs
		sentiment, _ := result.GetString("sentiment")
		confidence, _ := result.GetFloat("confidence")
		reasoning, _ := result.GetString("reasoning")

		// Display results
		fmt.Printf("Sentiment:  %s\n", sentiment)
		fmt.Printf("Confidence: %.2f\n", confidence)
		fmt.Printf("Reasoning:  %s\n", reasoning)

		// Show adapter metrics (NEW in Phase A)
		if result.AdapterUsed != "" {
			fmt.Printf("\n[Adapter Metrics]\n")
			fmt.Printf("  Adapter Used:    %s\n", result.AdapterUsed)
			fmt.Printf("  Parse Attempts:  %d\n", result.ParseAttempts)
			fmt.Printf("  Fallback Used:   %v\n", result.FallbackUsed)
			if result.ParseSuccess {
				fmt.Printf("  Status:          ✓ Parsed on first attempt\n")
			} else {
				fmt.Printf("  Status:          ⚠ Required fallback\n")
			}
		}

		fmt.Println()
	}

	fmt.Println("=== Summary ===")
	fmt.Println("The FallbackAdapter automatically handles different LM response formats:")
	fmt.Println("  1. ChatAdapter tries first (field markers format)")
	fmt.Println("  2. JSONAdapter tries next (JSON format)")
	fmt.Println("  3. Achieves >95% parse success rate")
	fmt.Println()
	fmt.Println("Check the [Adapter Metrics] above to see which adapter succeeded!")
}
