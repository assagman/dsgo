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

	err := h.Run(context.Background(), "006_program_of_thought", runExample)
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

	sig := dsgo.NewSignature("Solve the math problem by writing Python code. Provide the code in field 'code' and explanation in field 'explanation'.").
		AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code to solve the problem").
		AddOutput("explanation", dsgo.FieldTypeString, "Brief explanation of the approach")

	lm := shared.GetLM(shared.GetModel())
	
	pot := module.NewProgramOfThought(sig, lm, "python").
		WithAllowExecution(false)

	problem := "[3,4,5,6,1,2] -> write a program to find a target value: 5, with optimum time complexity"

	inputs := map[string]any{
		"problem": problem,
	}

	result, err := pot.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("program of thought failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	code, _ := result.GetString("code")
	explanation, _ := result.GetString("explanation")

	stats.Metadata["problem"] = problem
	stats.Metadata["code"] = code
	stats.Metadata["explanation"] = explanation

	fmt.Printf("Problem: %s\n", problem)
	fmt.Println("\nGenerated Code:")
	fmt.Println("```python")
	fmt.Println(code)
	fmt.Println("```")
	fmt.Printf("\nExplanation: %s\n", explanation)

	return result, stats, nil
}
