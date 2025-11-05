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

	err := h.Run(context.Background(), "015_fewshot", runExample)
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

	fmt.Println("=== Few-Shot Learning Demo ===")
	fmt.Println("Demonstrating how to improve predictions with example demonstrations")
	fmt.Println()

	// Create signature for movie genre classification
	sig := dsgo.NewSignature("Classify movie genre from plot description").
		AddInput("plot", dsgo.FieldTypeString, "Movie plot description").
		AddClassOutput("genre", []string{"action", "comedy", "drama", "horror", "sci-fi", "romance", "fantasy"}, "Primary genre").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Classification confidence 0-1")

	// Test plot for comparison
	testPlot := "A young wizard attends a magical school and battles dark forces threatening the wizarding world."

	// First: Prediction WITHOUT few-shot examples
	fmt.Println("--- Part 1: Zero-Shot Prediction (No Examples) ---")
	fmt.Println()

	predictZeroShot := module.NewPredict(sig, lm)

	resultZeroShot, err := predictZeroShot.Forward(ctx, map[string]any{
		"plot": testPlot,
	})
	if err != nil {
		return nil, stats, fmt.Errorf("zero-shot prediction failed: %w", err)
	}

	genreZeroShot, _ := resultZeroShot.GetString("genre")
	confidenceZeroShot, _ := resultZeroShot.GetFloat("confidence")

	fmt.Printf("📝 Test Plot: %s\n\n", testPlot)
	fmt.Printf("Results (Zero-Shot):\n")
	fmt.Printf("  Predicted Genre: %s\n", genreZeroShot)
	fmt.Printf("  Confidence:      %.2f\n", confidenceZeroShot)
	fmt.Printf("  Tokens Used:     %d\n", resultZeroShot.Usage.TotalTokens)
	fmt.Println()

	// Second: Prediction WITH few-shot examples
	fmt.Println("--- Part 2: Few-Shot Prediction (With 5 Examples) ---")
	fmt.Println()

	// Create few-shot examples using dsgo.NewExample
	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{
				"plot": "A group of astronauts discovers an alien artifact on Mars that changes humanity's understanding of the universe.",
			},
			map[string]any{
				"genre":      "sci-fi",
				"confidence": 0.95,
			},
		),
		*dsgo.NewExample(
			map[string]any{
				"plot": "Two rival chefs compete in a cooking competition while falling in love.",
			},
			map[string]any{
				"genre":      "romance",
				"confidence": 0.90,
			},
		),
		*dsgo.NewExample(
			map[string]any{
				"plot": "A detective races against time to stop a bomb from destroying the city.",
			},
			map[string]any{
				"genre":      "action",
				"confidence": 0.92,
			},
		),
		*dsgo.NewExample(
			map[string]any{
				"plot": "A group of friends get stranded in a remote cabin where they're hunted by a supernatural entity.",
			},
			map[string]any{
				"genre":      "horror",
				"confidence": 0.88,
			},
		),
		*dsgo.NewExample(
			map[string]any{
				"plot": "An unlikely hero embarks on a quest to destroy a powerful ancient ring before evil forces claim it.",
			},
			map[string]any{
				"genre":      "fantasy",
				"confidence": 0.94,
			},
		),
	}

	fmt.Printf("📚 Loaded %d few-shot examples:\n", len(demos))
	for i, demo := range demos {
		plot := demo.Inputs["plot"].(string)
		genre := demo.Outputs["genre"].(string)
		fmt.Printf("  %d. [%s] %s\n", i+1, genre, truncate(plot, 60))
	}
	fmt.Println()

	// Create Predict module with few-shot examples
	predictFewShot := module.NewPredict(sig, lm).WithDemos(demos)

	resultFewShot, err := predictFewShot.Forward(ctx, map[string]any{
		"plot": testPlot,
	})
	if err != nil {
		return nil, stats, fmt.Errorf("few-shot prediction failed: %w", err)
	}

	genreFewShot, _ := resultFewShot.GetString("genre")
	confidenceFewShot, _ := resultFewShot.GetFloat("confidence")

	fmt.Printf("Results (Few-Shot):\n")
	fmt.Printf("  Predicted Genre: %s\n", genreFewShot)
	fmt.Printf("  Confidence:      %.2f\n", confidenceFewShot)
	fmt.Printf("  Tokens Used:     %d\n", resultFewShot.Usage.TotalTokens)
	fmt.Println()

	// Comparison
	fmt.Println("--- Comparison ---")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("%-20s | %-15s | %-10s | %-10s\n", "Approach", "Genre", "Confidence", "Tokens")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-20s | %-15s | %-10.2f | %-10d\n", "Zero-Shot", genreZeroShot, confidenceZeroShot, resultZeroShot.Usage.TotalTokens)
	fmt.Printf("%-20s | %-15s | %-10.2f | %-10d\n", "Few-Shot (5 demos)", genreFewShot, confidenceFewShot, resultFewShot.Usage.TotalTokens)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Demonstrate with multiple test cases
	fmt.Println("--- Part 3: Testing Few-Shot on Multiple Inputs ---")
	fmt.Println()

	testCases := []struct {
		plot     string
		expected string
	}{
		{
			plot:     "After a global pandemic, survivors fight for resources in a post-apocalyptic wasteland.",
			expected: "action/sci-fi",
		},
		{
			plot:     "A clumsy office worker accidentally becomes CEO and tries to hide their incompetence.",
			expected: "comedy",
		},
		{
			plot:     "A mother struggles to reconnect with her estranged daughter after years apart.",
			expected: "drama",
		},
	}

	var totalTokens int
	var lastResult *dsgo.Prediction

	for i, tc := range testCases {
		result, err := predictFewShot.Forward(ctx, map[string]any{
			"plot": tc.plot,
		})
		if err != nil {
			return nil, stats, fmt.Errorf("test case %d failed: %w", i+1, err)
		}

		genre, _ := result.GetString("genre")
		confidence, _ := result.GetFloat("confidence")

		fmt.Printf("Test %d:\n", i+1)
		fmt.Printf("  Plot:       %s\n", truncate(tc.plot, 70))
		fmt.Printf("  Predicted:  %s (%.0f%% confidence)\n", genre, confidence*100)
		fmt.Printf("  Expected:   %s\n", tc.expected)
		fmt.Println()

		totalTokens += result.Usage.TotalTokens
		lastResult = result
	}

	totalTokens += resultZeroShot.Usage.TotalTokens + resultFewShot.Usage.TotalTokens

	fmt.Println("=== Summary ===")
	fmt.Println("Few-shot learning provides:")
	fmt.Println("  ✓ Better accuracy through example demonstrations")
	fmt.Println("  ✓ Clearer task understanding for the LM")
	fmt.Println("  ✓ More consistent output formatting")
	fmt.Println("  ✓ Higher confidence scores")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("📚 Total examples provided: %d\n", len(demos))
	fmt.Printf("🧪 Total test cases: %d\n", len(testCases)+2) // +2 for zero-shot and few-shot comparison
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["zero_shot_genre"] = genreZeroShot
	stats.Metadata["few_shot_genre"] = genreFewShot
	stats.Metadata["demos_count"] = len(demos)
	stats.Metadata["test_cases_run"] = len(testCases) + 2

	return lastResult, stats, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
