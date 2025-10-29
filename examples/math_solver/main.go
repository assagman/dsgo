package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	fmt.Println("=== Math Solver with Program of Thought ===")
	fmt.Println("Demonstrates: ProgramOfThought module for mathematical reasoning")
	fmt.Println()

	// Example 1: Simple calculation
	fmt.Println("--- Example 1: Simple calculation ---")
	simpleCalculation()

	// Example 2: Complex problem
	fmt.Println("\n--- Example 2: Complex word problem ---")
	complexProblem()

	// Example 3: Statistical analysis
	fmt.Println("\n--- Example 3: Statistical analysis ---")
	statisticalAnalysis()
}

func simpleCalculation() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4")

	sig := dsgo.NewSignature("Solve the mathematical problem using Python code").
		AddInput("problem", dsgo.FieldTypeString, "The problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code solution").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")

	pot := dsgo.NewProgramOfThought(sig, lm, "python").
		WithAllowExecution(false) // Don't execute for safety

	inputs := map[string]interface{}{
		"problem": "Calculate the compound interest on $1000 invested at 5% annually for 3 years",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("\nGenerated Code:\n%s\n", outputs["code"])
	fmt.Printf("\nExplanation: %s\n", outputs["explanation"])
	if answer, ok := outputs["answer"]; ok {
		fmt.Printf("Answer: %s\n", answer)
	}
}

func complexProblem() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	sig := dsgo.NewSignature("Solve complex math word problem with code").
		AddInput("problem", dsgo.FieldTypeString, "The word problem").
		AddOutput("code", dsgo.FieldTypeString, "Python code").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation").
		AddOutput("answer", dsgo.FieldTypeString, "Final numerical answer")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	inputs := map[string]interface{}{
		"problem": "A train travels 120 km in 2 hours, then 180 km in 3 hours. What is the average speed for the entire journey?",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("\nPython Code:\n%s\n", outputs["code"])
	fmt.Printf("\nExplanation:\n%s\n", outputs["explanation"])
	fmt.Printf("\nAnswer: %s\n", outputs["answer"])
}

func statisticalAnalysis() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	sig := dsgo.NewSignature("Perform statistical analysis using Python").
		AddInput("data_description", dsgo.FieldTypeString, "Description of the data").
		AddInput("analysis_type", dsgo.FieldTypeString, "Type of analysis needed").
		AddOutput("code", dsgo.FieldTypeString, "Python code for analysis").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation of the code").
		AddOutput("interpretation", dsgo.FieldTypeString, "How to interpret results")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	inputs := map[string]interface{}{
		"data_description": "Dataset of exam scores: [75, 82, 90, 68, 85, 92, 78, 88, 95, 72]",
		"analysis_type":    "mean, median, standard deviation, and identify outliers",
	}

	outputs, err := pot.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Data: %s\n", inputs["data_description"])
	fmt.Printf("Analysis: %s\n", inputs["analysis_type"])
	fmt.Printf("\nGenerated Code:\n%s\n", outputs["code"])
	fmt.Printf("\nExplanation:\n%s\n", outputs["explanation"])
	fmt.Printf("\nInterpretation:\n%s\n", outputs["interpretation"])
}
