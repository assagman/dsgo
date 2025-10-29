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

	// Example 1: Basic Predict
	fmt.Println("=== Example 1: Basic Predict ===")
	basicPredict()

	fmt.Println("\n=== Example 2: Chain of Thought ===")
	chainOfThought()
}

func basicPredict() {
	// Create signature for sentiment analysis
	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "The sentiment classification").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score between 0 and 1")

	// Create LM (auto-detects provider based on API keys)
	lm := examples.GetLM("gpt-4o-mini")

	// Create Predict module
	predict := dsgo.NewPredict(sig, lm)

	// Execute
	ctx := context.Background()
	inputs := map[string]interface{}{
		"text": "I absolutely love this product! It exceeded all my expectations.",
	}

	outputs, err := predict.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Input: %s\n", inputs["text"])
	fmt.Printf("Sentiment: %v\n", outputs["sentiment"])
	fmt.Printf("Confidence: %v\n", outputs["confidence"])
}

func chainOfThought() {
	// Create signature for complex reasoning
	sig := dsgo.NewSignature("Solve the given math word problem").
		AddInput("problem", dsgo.FieldTypeString, "The math word problem to solve").
		AddOutput("answer", dsgo.FieldTypeFloat, "The numerical answer").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation")

	// Create LM
	lm := examples.GetLM("gpt-4o-mini")

	// Create ChainOfThought module
	cot := dsgo.NewChainOfThought(sig, lm)

	// Execute
	ctx := context.Background()
	inputs := map[string]interface{}{
		"problem": "If John has 5 apples and gives 2 to Mary, then buys 3 more apples, how many apples does John have?",
	}

	outputs, err := cot.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Problem: %s\n", inputs["problem"])
	fmt.Printf("Reasoning: %v\n", outputs["reasoning"])
	fmt.Printf("Answer: %v\n", outputs["answer"])
	fmt.Printf("Explanation: %v\n", outputs["explanation"])
}
