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

	err := h.Run(context.Background(), "005_best_of_n", runExample)
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

	sig := dsgo.NewSignature("Generate a creative and catchy title for a blog post").
		AddInput("topic", dsgo.FieldTypeString, "Blog post topic").
		AddOutput("title", dsgo.FieldTypeString, "Creative title").
		AddOutput("hook", dsgo.FieldTypeString, "One-sentence hook")

	lm := shared.GetLM(shared.GetModel())
	basePred := module.NewPredict(sig, lm)

	titleScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		title, _ := pred.GetString("title")
		hook, _ := pred.GetString("hook")

		score := 100.0

		titleLen := len(title)
		if titleLen > 50 {
			score -= float64(titleLen-50) * 0.5
		}

		if strings.ContainsAny(title, "0123456789") {
			score += 15.0
		}

		hookLen := len(hook)
		if hookLen > 100 {
			score -= float64(hookLen-100) * 0.3
		}

		return score, nil
	}

	bestTitle := module.NewBestOfN(basePred, 5).
		WithScorer(titleScorer).
		WithParallel(true).
		WithReturnAll(true).
		WithThreshold(110.0)

	inputs := map[string]any{
		"topic": "Machine Learning for Beginners",
	}

	result, err := bestTitle.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("best of n failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	bestTitleStr, _ := result.GetString("title")
	bestHook, _ := result.GetString("hook")

	stats.Metadata["topic"] = inputs["topic"]
	stats.Metadata["candidates_generated"] = len(result.Completions)
	stats.Metadata["best_title"] = bestTitleStr
	stats.Metadata["best_hook"] = bestHook
	stats.Metadata["best_score"] = result.Score

	fmt.Printf("Topic: %s\n", inputs["topic"])
	fmt.Printf("Generated %d candidates\n\n", len(result.Completions))

	fmt.Println("All Candidates Generated:")
	for i, completion := range result.Completions {
		title := fmt.Sprint(completion["title"])
		rank := ""
		if i == 0 {
			rank = " 👑 WINNER"
		}
		fmt.Printf("%d. %s%s\n", i+1, title, rank)
	}

	fmt.Printf("\n🏆 Selected Best Title:\n")
	fmt.Printf("Title: %s\n", bestTitleStr)
	fmt.Printf("Hook: %s\n", bestHook)
	fmt.Printf("Score: %.1f\n", result.Score)

	return result, stats, nil
}
