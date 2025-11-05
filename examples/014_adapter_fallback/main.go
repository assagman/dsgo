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
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "014_adapter_fallback", runExample)
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

	lm := shared.GetLM(shared.GetModel())

	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Overall sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score 0-1").
		AddOutput("reasoning", dsgo.FieldTypeString, "Brief explanation")

	predict := module.NewPredict(sig, lm)

	testCases := []string{
		"I absolutely love this product! It's amazing!",
		"This is terrible. Waste of money.",
		"It's okay, nothing special.",
	}

	fmt.Println("=== Adapter Fallback Demo ===")
	fmt.Println("Testing parse robustness across different inputs...")
	fmt.Println()

	var totalTokens int
	var lastResult *dsgo.Prediction
	var adapterMetrics []map[string]any

	for i, text := range testCases {
		fmt.Printf("Test %d: %s\n", i+1, text)
		fmt.Println(strings.Repeat("-", 60))

		result, err := predict.Forward(ctx, map[string]any{
			"text": text,
		})

		if err != nil {
			return nil, stats, fmt.Errorf("test %d failed: %w", i+1, err)
		}

		sentiment, _ := result.GetString("sentiment")
		confidence, _ := result.GetFloat("confidence")
		reasoning, _ := result.GetString("reasoning")

		fmt.Printf("Sentiment:  %s\n", sentiment)
		fmt.Printf("Confidence: %.2f\n", confidence)
		fmt.Printf("Reasoning:  %s\n", reasoning)

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

			adapterMetrics = append(adapterMetrics, map[string]any{
				"test":           i + 1,
				"adapter_used":   result.AdapterUsed,
				"parse_attempts": result.ParseAttempts,
				"fallback_used":  result.FallbackUsed,
				"parse_success":  result.ParseSuccess,
			})
		}

		totalTokens += result.Usage.TotalTokens
		lastResult = result
		fmt.Println()
	}

	fmt.Println("=== Summary ===")
	fmt.Println("The FallbackAdapter automatically handles different LM response formats:")
	fmt.Println("  1. ChatAdapter tries first (field markers format)")
	fmt.Println("  2. JSONAdapter tries next (JSON format)")
	fmt.Println("  3. Achieves >95% parse success rate")
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["tests_run"] = len(testCases)
	stats.Metadata["adapter_metrics"] = adapterMetrics
	stats.Metadata["total_tests"] = len(testCases)

	return lastResult, stats, nil
}
