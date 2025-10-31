package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	fmt.Println("=== ReAct Agent Example ===")
	reactAgent()
}

func reactAgent() {
	// Define tools
	searchTool := dsgo.NewTool(
		"search",
		"Search for information on the internet",
		func(ctx context.Context, args map[string]any) (any, error) {
			query := args["query"].(string)
			// Simulate search results
			return fmt.Sprintf("Search results for '%s': DSPy is a framework for programming language models, developed at Stanford.", query), nil
		},
	).AddParameter("query", "string", "The search query", true)

	calculatorTool := dsgo.NewTool(
		"calculator",
		"Perform mathematical calculations",
		func(ctx context.Context, args map[string]any) (any, error) {
			expression := args["expression"].(string)
			// Simple pattern matching for basic arithmetic
			// Supports: "X - Y" format
			parts := strings.Split(strings.ReplaceAll(expression, " ", ""), "-")
			if len(parts) == 2 {
				var num1, num2 int
				// Try to parse both as integers
				_, err1 := fmt.Sscanf(parts[0], "%d", &num1)
				_, err2 := fmt.Sscanf(parts[1], "%d", &num2)
				if err1 == nil && err2 == nil {
					return fmt.Sprintf("%d", num1-num2), nil
				}
			}
			return fmt.Sprintf("Unable to calculate: %s", expression), nil
		},
	).AddParameter("expression", "string", "The mathematical expression to evaluate", true)

	tools := []dsgo.Tool{*searchTool, *calculatorTool}

	// Create signature
	sig := dsgo.NewSignature("Answer the question using available tools").
		AddInput("question", dsgo.FieldTypeString, "The question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "The final answer").
		AddOutput("sources", dsgo.FieldTypeString, "Sources used to answer the question")

	// Create LM (auto-detects provider from environment)
	lm := shared.GetLM(shared.GetModel())

	// Create ReAct module
	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(5).
		WithVerbose(true)

	// Execute
	ctx := context.Background()
	inputs := map[string]any{
		"question": "What is DSPy and how many years has it been since 2020?",
	}

	outputs, err := react.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("\n=== Final Result ===\n")
	fmt.Printf("Question: %s\n", inputs["question"])
	fmt.Printf("Answer: %v\n", outputs.Outputs["answer"])
	fmt.Printf("Sources: %v\n", outputs.Outputs["sources"])
}
