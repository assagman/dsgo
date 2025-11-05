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

	err := h.Run(context.Background(), "012_math_solver", runExample)
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

	var totalTokens int

	fmt.Println("=== Math Solver with Program of Thought ===")
	fmt.Println()

	// Example 1: Simple calculation
	fmt.Println("--- Example 1: Compound Interest Calculation ---")
	tokens1, _, err := simpleCalculation(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("simple calculation failed: %w", err)
	}
	totalTokens += tokens1

	// Example 2: Complex word problem
	fmt.Println("\n--- Example 2: Average Speed Word Problem ---")
	tokens2, _, err := complexProblem(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("complex problem failed: %w", err)
	}
	totalTokens += tokens2

	// Example 3: Statistical analysis
	fmt.Println("\n--- Example 3: Statistical Analysis ---")
	tokens3, pred3, err := statisticalAnalysis(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("statistical analysis failed: %w", err)
	}
	totalTokens += tokens3

	stats.TokensUsed = totalTokens
	stats.Metadata["problems_solved"] = 3
	stats.Metadata["problem_types"] = []string{"compound_interest", "average_speed", "statistics"}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Problems solved: 3\n")
	fmt.Printf("  Total tokens used: %d\n", totalTokens)
	fmt.Printf("  ✅ All mathematical problems solved successfully!\n")

	return pred3, stats, nil
}

func simpleCalculation(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("Solve the mathematical problem using Python code. Provide fields: code, explanation, answer").
		AddInput("problem", dsgo.FieldTypeString, "The problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code solution").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")

	pot := module.NewProgramOfThought(sig, lm, "python").
		WithAllowExecution(false) // Don't execute for safety

	inputs := map[string]any{
		"problem": "Calculate the compound interest on $1000 invested at 5% annually for 3 years",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, err
	}

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("\nGenerated Code:\n%s\n", outputs.Outputs["code"])
	fmt.Printf("\nExplanation: %s\n", outputs.Outputs["explanation"])
	if answer, ok := outputs.Outputs["answer"]; ok {
		fmt.Printf("Answer: %s\n", answer)
	}

	return outputs.Usage.TotalTokens, outputs, nil
}

func complexProblem(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("Solve complex math word problem with code. Provide fields: code, explanation, answer").
		AddInput("problem", dsgo.FieldTypeString, "The word problem").
		AddOutput("code", dsgo.FieldTypeString, "Python code").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation").
		AddOutput("answer", dsgo.FieldTypeString, "Final numerical answer")

	pot := module.NewProgramOfThought(sig, lm, "python")

	inputs := map[string]any{
		"problem": "A train travels 120 km in 2 hours, then 180 km in 3 hours. What is the average speed for the entire journey?",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, err
	}

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("\nPython Code:\n%s\n", outputs.Outputs["code"])
	fmt.Printf("\nExplanation:\n%s\n", outputs.Outputs["explanation"])
	fmt.Printf("\nAnswer: %s\n", outputs.Outputs["answer"])

	return outputs.Usage.TotalTokens, outputs, nil
}

func statisticalAnalysis(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("Perform statistical analysis using Python. Provide fields: code, explanation, interpretation").
		AddInput("data_description", dsgo.FieldTypeString, "Description of the data").
		AddInput("analysis_type", dsgo.FieldTypeString, "Type of analysis needed").
		AddOutput("code", dsgo.FieldTypeString, "Python code for analysis").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation of the code").
		AddOutput("interpretation", dsgo.FieldTypeString, "How to interpret results")

	pot := module.NewProgramOfThought(sig, lm, "python")

	inputs := map[string]any{
		"data_description": "Dataset of exam scores: [75, 82, 90, 68, 85, 92, 78, 88, 95, 72]",
		"analysis_type":    "mean, median, standard deviation, and identify outliers",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, err
	}

	fmt.Printf("Data: %s\n", inputs["data_description"])
	fmt.Printf("Analysis: %s\n", inputs["analysis_type"])
	fmt.Printf("\nGenerated Code:\n%s\n", outputs.Outputs["code"])
	fmt.Printf("\nExplanation:\n%s\n", outputs.Outputs["explanation"])
	fmt.Printf("\nInterpretation:\n%s\n", outputs.Outputs["interpretation"])

	return outputs.Usage.TotalTokens, outputs, nil
}
