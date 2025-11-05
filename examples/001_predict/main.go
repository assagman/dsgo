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

	err := h.Run(context.Background(), "001_predict", runExample)
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

	sig := dsgo.NewSignature("Classify the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "The sentiment classification").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score between 0 and 1")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	inputs := map[string]any{
		"text": "I absolutely love this product! It exceeded all my expectations.",
	}

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("prediction failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	sentiment, _ := result.GetString("sentiment")
	confidence, _ := result.GetFloat("confidence")

	stats.Metadata["input"] = inputs["text"]
	stats.Metadata["sentiment"] = sentiment
	stats.Metadata["confidence"] = confidence

	fmt.Printf("Input: %s\n", inputs["text"])
	fmt.Printf("Sentiment: %s\n", sentiment)
	fmt.Printf("Confidence: %.2f\n", confidence)

	return result, stats, nil
}
