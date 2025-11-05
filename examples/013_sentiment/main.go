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

	err := h.Run(context.Background(), "013_sentiment", runExample)
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

	sig := dsgo.NewSignature("Solve the given math word problem").
		AddInput("problem", dsgo.FieldTypeString, "The math word problem to solve").
		AddOutput("answer", dsgo.FieldTypeFloat, "The numerical answer").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation")

	lm := shared.GetLM(shared.GetModel())
	cot := module.NewChainOfThought(sig, lm)

	inputs := map[string]any{
		"problem": "If John has 5 apples and gives 2 to Mary, then buys 3 more apples, how many apples does John have?",
	}

	result, err := cot.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("chain of thought failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	answer, _ := result.GetFloat("answer")
	explanation, _ := result.GetString("explanation")

	stats.Metadata["problem"] = inputs["problem"]
	stats.Metadata["reasoning"] = result.Rationale
	stats.Metadata["answer"] = answer
	stats.Metadata["explanation"] = explanation

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("Reasoning: %s\n", result.Rationale)
	fmt.Printf("Answer: %.0f\n", answer)
	fmt.Printf("Explanation: %s\n", explanation)

	return result, stats, nil
}
