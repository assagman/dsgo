package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "004_refine", runExample)
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

	sig := dsgo.NewSignature("Improve the given text based on feedback.").
		AddInput("text", dsgo.FieldTypeString, "Text to improve").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback for improvement").
		AddOutput("improved_text", dsgo.FieldTypeString, "Improved version of the text").
		AddOutput("changes_made", dsgo.FieldTypeString, "Summary of improvements")

	lm := shared.GetLM(shared.GetModel())
	refine := module.NewRefine(sig, lm).WithMaxIterations(3)

	originalText := `The product is good. It works fine. The price is okay.`

	inputs := map[string]any{
		"text":     originalText,
		"feedback": "Make it more engaging, professional, and detailed. Add specific benefits.",
	}

	result, err := refine.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("refine failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	improvedText, _ := result.GetString("improved_text")
	changesMade, _ := result.GetString("changes_made")

	stats.Metadata["original_text"] = originalText
	stats.Metadata["feedback"] = inputs["feedback"]
	stats.Metadata["improved_text"] = improvedText
	stats.Metadata["changes_made"] = changesMade

	fmt.Printf("📝 Original text:\n%s\n\n", originalText)
	fmt.Printf("✨ Refined text:\n%s\n\n", improvedText)
	fmt.Printf("📊 Changes made:\n%s\n", changesMade)

	return result, stats, nil
}
