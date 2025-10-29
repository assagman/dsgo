package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

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
			// Simple calculation simulation
			if strings.Contains(expression, "2024-2020") {
				return "4", nil
			}
			return "Result of calculation", nil
		},
	).AddParameter("expression", "string", "The mathematical expression to evaluate", true)

	tools := []dsgo.Tool{*searchTool, *calculatorTool}

	// Create signature
	sig := dsgo.NewSignature("Answer the question using available tools").
		AddInput("question", dsgo.FieldTypeString, "The question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "The final answer").
		AddOutput("sources", dsgo.FieldTypeString, "Sources used to answer the question")

	// Create LM (auto-detects provider from environment)
	lm := shared.GetLM("gpt-4")

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
	fmt.Printf("Answer: %v\n", outputs["answer"])
	fmt.Printf("Sources: %v\n", outputs["sources"])
}
