package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

// This example demonstrates ProgramOfThought, which solves problems by
// generating and optionally executing Python code for precise calculations.
//
// ProgramOfThought is ideal for:
// - Mathematical word problems
// - Data analysis tasks
// - Problems requiring exact calculations
// - Multi-step computational reasoning

func main() {
	shared.LoadEnv()

	fmt.Println("=== Program of Thought Example ===")
	fmt.Println("Solving problems with code generation and execution")

	lm := shared.GetLM(shared.GetModel())

	// Example 1: Basic Math Problem (Code Only)
	fmt.Println("--- Example 1: Math Problem (Code Generation) ---")

	sig1 := dsgo.NewSignature("Solve the math problem by writing Python code").
		AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code to solve the problem").
		AddOutput("explanation", dsgo.FieldTypeString, "Brief explanation of the approach")

	pot1 := module.NewProgramOfThought(sig1, lm, "python").
		WithAllowExecution(false) // Code generation only

	ctx := context.Background()

	problem := "[3,4,5,6,1,2] -> write a program to find a target value: 5, with optimum time complexity"
	fmt.Printf("Problem: %s\n", problem)
	result1, err := pot1.Forward(ctx, map[string]any{
		"problem": problem,
	})
	if err != nil {
		log.Fatalf("PoT failed: %v", err)
	}

	code1, _ := result1.GetString("code")
	explanation1, _ := result1.GetString("explanation")

	fmt.Println("\nGenerated Code:")
	fmt.Println("```python")
	fmt.Println(code1)
	fmt.Println("```")
	fmt.Printf("\nExplanation: %s\n\n", explanation1)

	// Example 2: Complex Calculation with Execution
	fmt.Println("--- Example 2: Compound Interest (With Execution) ---")

	sig2 := dsgo.NewSignature("Calculate compound interest using Python code").
		AddInput("principal", dsgo.FieldTypeString, "Initial principal amount").
		AddInput("rate", dsgo.FieldTypeString, "Annual interest rate").
		AddInput("time", dsgo.FieldTypeString, "Time period in years").
		AddInput("frequency", dsgo.FieldTypeString, "Compounding frequency per year").
		AddOutput("code", dsgo.FieldTypeString, "Python code").
		AddOutput("result", dsgo.FieldTypeString, "Calculated result").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	pot2 := module.NewProgramOfThought(sig2, lm, "python").
		WithAllowExecution(true). // Enable code execution
		WithExecutionTimeout(5)   // 5 second timeout

	result2, err := pot2.Forward(ctx, map[string]any{
		"principal": "$10,000",
		"rate":      "5% per year",
		"time":      "10 years",
		"frequency": "quarterly (4 times per year)",
	})
	if err != nil {
		log.Fatalf("PoT with execution failed: %v", err)
	}

	code2, _ := result2.GetString("code")
	result2Str, _ := result2.GetString("result")
	explanation2, _ := result2.GetString("explanation")

	fmt.Println("Problem: Calculate compound interest")
	fmt.Println("Principal: $10,000, Rate: 5%, Time: 10 years, Frequency: Quarterly")
	fmt.Println("\nGenerated & Executed Code:")
	fmt.Println("```python")
	fmt.Println(code2)
	fmt.Println("```")
	fmt.Printf("\nResult: %s\n", result2Str)
	fmt.Printf("Explanation: %s\n\n", explanation2)

	// Example 3: Data Analysis Problem
	fmt.Println("--- Example 3: Statistical Analysis ---")

	sig3 := dsgo.NewSignature("Analyze the data using Python code").
		AddInput("data", dsgo.FieldTypeString, "Data to analyze").
		AddInput("question", dsgo.FieldTypeString, "Question about the data").
		AddOutput("code", dsgo.FieldTypeString, "Python analysis code").
		AddOutput("answer", dsgo.FieldTypeString, "Answer to the question").
		AddOutput("insights", dsgo.FieldTypeString, "Key insights from the analysis")

	pot3 := module.NewProgramOfThought(sig3, lm, "python").
		WithAllowExecution(true)

	result3, err := pot3.Forward(ctx, map[string]any{
		"data":     "Test scores: [85, 92, 78, 95, 88, 91, 76, 89, 93, 87]",
		"question": "What is the mean, median, and standard deviation of the test scores?",
	})
	if err != nil {
		log.Fatalf("Statistical analysis failed: %v", err)
	}

	code3, _ := result3.GetString("code")
	answer3, _ := result3.GetString("answer")
	insights3, _ := result3.GetString("insights")

	fmt.Println("Data: [85, 92, 78, 95, 88, 91, 76, 89, 93, 87]")
	fmt.Println("Question: What is the mean, median, and standard deviation?")
	fmt.Println("\nGenerated Code:")
	fmt.Println("```python")
	fmt.Println(code3)
	fmt.Println("```")
	fmt.Printf("\nAnswer: %s\n", answer3)
	fmt.Printf("Insights: %s\n\n", insights3)

	// Example 4: Multi-Step Problem
	fmt.Println("--- Example 4: Multi-Step Word Problem ---")

	sig4 := dsgo.NewSignature("Solve the word problem with Python code, breaking it into steps").
		AddInput("problem", dsgo.FieldTypeString, "Word problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code with comments for each step").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer with units").
		AddOutput("steps", dsgo.FieldTypeString, "Description of solution steps")

	pot4 := module.NewProgramOfThought(sig4, lm, "python").
		WithAllowExecution(true)

	result4, err := pot4.Forward(ctx, map[string]any{
		"problem": "Sarah buys 3 notebooks for $2.50 each and 4 pens for $1.25 each. " +
			"She pays with a $20 bill. How much change does she receive?",
	})
	if err != nil {
		log.Fatalf("Multi-step problem failed: %v", err)
	}

	code4, _ := result4.GetString("code")
	answer4, _ := result4.GetString("answer")
	steps4, _ := result4.GetString("steps")

	fmt.Println("Problem: Sarah buys 3 notebooks for $2.50 each and 4 pens for $1.25 each.")
	fmt.Println("She pays with a $20 bill. How much change does she receive?")
	fmt.Println("\nGenerated Code:")
	fmt.Println("```python")
	fmt.Println(code4)
	fmt.Println("```")
	fmt.Printf("\nSteps:\n%s\n", steps4)
	fmt.Printf("\nFinal Answer: %s\n\n", answer4)

	fmt.Println("=== Key Takeaways ===")
	fmt.Println("✓ ProgramOfThought excels at precise mathematical reasoning")
	fmt.Println("✓ Code generation ensures accuracy for calculations")
	fmt.Println("✓ Execution validation catches errors in logic")
	fmt.Println("✓ Python support (Go support planned)")
	fmt.Println("✓ Timeout protection prevents infinite loops")
	fmt.Println("✓ Ideal for: math, statistics, data analysis, algorithms")

	fmt.Println("\n📝 Note: Code execution requires Python 3 in PATH")
	fmt.Println("   Set WithAllowExecution(false) to generate code only")
}
