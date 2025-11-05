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

	err := h.Run(context.Background(), "007_program_composition", runExample)
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

	// Create a program that chains analysis with solving
	// Step 1: Analyze the problem using ChainOfThought
	analyzeSig := dsgo.NewSignature("Analyze the problem and break it down").
		AddInput("problem", dsgo.FieldTypeString, "The problem statement").
		AddOutput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddOutput("approach", dsgo.FieldTypeString, "Recommended approach")

	analyzeModule := module.NewChainOfThought(analyzeSig, lm)

	// Step 2: Generate solution using Predict
	solveSig := dsgo.NewSignature("Generate a solution based on the analysis").
		AddInput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddInput("approach", dsgo.FieldTypeString, "Recommended approach").
		AddOutput("solution", dsgo.FieldTypeString, "The solution").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score from 0.0 to 1.0")

	solveModule := module.NewPredict(solveSig, lm)

	customScorer := func(inputs map[string]any, pred *dsgo.Prediction) (float64, error) {
		if conf, ok := pred.Outputs["confidence"].(float64); ok {
			return conf, nil
		}
		return 0.5, nil
	}

	// Use BestOfN on the solve module to get best of 3 solutions
	bestSolve := module.NewBestOfN(solveModule, 3).
		WithScorer(customScorer).
		WithReturnAll(true)

	// Create a program that chains analysis with best-of-n solving
	program := module.NewProgram("Analyze and Solve").
		AddModule(analyzeModule).
		AddModule(bestSolve)

	inputs := map[string]any{
		"problem": "How can I reduce the latency of my web application's database queries?",
	}

	result, err := program.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("program failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	analysis, _ := result.GetString("analysis")
	approach, _ := result.GetString("approach")
	solution, _ := result.GetString("solution")
	confidence, _ := result.GetFloat("confidence")

	stats.Metadata["problem"] = inputs["problem"]
	stats.Metadata["analysis"] = analysis
	stats.Metadata["approach"] = approach
	stats.Metadata["solution"] = solution
	stats.Metadata["confidence"] = confidence
	stats.Metadata["best_score"] = result.Score

	if allScores, ok := result.Outputs["_best_of_n_all_scores"].([]float64); ok {
		stats.Metadata["all_scores"] = allScores
	}

	fmt.Printf("Problem: %s\n\n", inputs["problem"])
	fmt.Printf("Analysis: %s\n\n", analysis)
	fmt.Printf("Approach: %s\n\n", approach)
	fmt.Printf("Best Solution:\n%s\n\n", solution)
	fmt.Printf("Confidence: %.2f\n", confidence)
	fmt.Printf("Best Score: %.3f\n", result.Score)

	if allScores, ok := result.Outputs["_best_of_n_all_scores"].([]float64); ok {
		fmt.Printf("All Scores: %v\n", allScores)
	}

	return result, stats, nil
}
