package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	fmt.Println("=== DSGo Module Composition Examples ===")

	// Example 1: Program (Pipeline)
	fmt.Println("--- Example 1: Program Pipeline ---")
	programExample()

	// Example 2: Refine
	fmt.Println("\n--- Example 2: Refine Module ---")
	refineExample()

	// Example 3: BestOfN
	fmt.Println("\n--- Example 3: BestOfN Module ---")
	bestOfNExample()

	// Example 4: Program + BestOfN
	fmt.Println("\n--- Example 4: Combined Pipeline ---")
	combinedExample()
}

func programExample() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	// Step 1: Extract key information
	extractSig := dsgo.NewSignature("Extract key information from the text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddOutput("main_topic", dsgo.FieldTypeString, "The main topic").
		AddOutput("key_points", dsgo.FieldTypeString, "Key points (comma-separated)")

	extractModule := dsgo.NewPredict(extractSig, lm)

	// Step 2: Analyze sentiment using extracted information
	sentimentSig := dsgo.NewSignature("Analyze sentiment of the main topic and key points").
		AddInput("main_topic", dsgo.FieldTypeString, "The main topic").
		AddInput("key_points", dsgo.FieldTypeString, "Key points").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Overall sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

	sentimentModule := dsgo.NewPredict(sentimentSig, lm)

	// Create a program that chains these modules
	program := dsgo.NewProgram("Extract and Analyze").
		AddModule(extractModule).
		AddModule(sentimentModule)

	// Execute the pipeline
	inputs := map[string]interface{}{
		"text": "This product is amazing! The quality is outstanding and customer service is excellent. Highly recommended!",
	}

	outputs, err := program.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Main Topic: %v\n", outputs["main_topic"])
	fmt.Printf("Key Points: %v\n", outputs["key_points"])
	fmt.Printf("Sentiment: %v\n", outputs["sentiment"])
	fmt.Printf("Confidence: %v\n", outputs["confidence"])
}

func refineExample() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	sig := dsgo.NewSignature("Write a professional email").
		AddInput("recipient", dsgo.FieldTypeString, "Who the email is for").
		AddInput("purpose", dsgo.FieldTypeString, "Purpose of the email").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback for refinement").
		AddOutput("email", dsgo.FieldTypeString, "The email content").
		AddOutput("subject", dsgo.FieldTypeString, "Email subject line")

	refine := dsgo.NewRefine(sig, lm).WithMaxIterations(2)

	inputs := map[string]interface{}{
		"recipient": "hiring manager",
		"purpose":   "follow up after job interview",
		"feedback":  "make it more concise and add a specific timeline",
	}

	outputs, err := refine.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Subject: %v\n", outputs["subject"])
	fmt.Printf("Email:\n%v\n", outputs["email"])
}

func bestOfNExample() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	sig := dsgo.NewSignature("Generate a creative tagline").
		AddInput("product", dsgo.FieldTypeString, "The product name").
		AddInput("description", dsgo.FieldTypeString, "Product description").
		AddOutput("tagline", dsgo.FieldTypeString, "Creative tagline").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

	predict := dsgo.NewPredict(sig, lm)

	// Use BestOfN to generate 3 taglines and pick the best based on confidence
	bestOf3 := dsgo.NewBestOfN(predict, 3).
		WithScorer(dsgo.ConfidenceScorer("confidence")).
		WithReturnAll(true)

	inputs := map[string]interface{}{
		"product":     "SmartWatch Pro",
		"description": "A fitness tracker with advanced health monitoring and AI coaching",
	}

	outputs, err := bestOf3.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Best Tagline: %v\n", outputs["tagline"])
	fmt.Printf("Confidence: %v\n", outputs["confidence"])
	fmt.Printf("Best Score: %v\n", outputs["_best_of_n_score"])
	if scores, ok := outputs["_best_of_n_all_scores"].([]float64); ok {
		fmt.Printf("All Scores: %v\n", scores)
	}
}

func combinedExample() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	// Step 1: Analyze the problem
	analyzeSig := dsgo.NewSignature("Analyze the problem and break it down").
		AddInput("problem", dsgo.FieldTypeString, "The problem statement").
		AddOutput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddOutput("approach", dsgo.FieldTypeString, "Recommended approach")

	analyzeModule := dsgo.NewChainOfThought(analyzeSig, lm)

	// Step 2: Generate solution
	solveSig := dsgo.NewSignature("Generate a solution based on the analysis").
		AddInput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddInput("approach", dsgo.FieldTypeString, "Recommended approach").
		AddOutput("solution", dsgo.FieldTypeString, "The solution").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence in solution")

	solveModule := dsgo.NewPredict(solveSig, lm)

	// Use BestOfN on the solve module
	bestSolve := dsgo.NewBestOfN(solveModule, 2).
		WithScorer(dsgo.ConfidenceScorer("confidence"))

	// Create a program that chains analysis with best-of-n solving
	program := dsgo.NewProgram("Analyze and Solve").
		AddModule(analyzeModule).
		AddModule(bestSolve)

	inputs := map[string]interface{}{
		"problem": "How can I reduce the latency of my web application's database queries?",
	}

	outputs, err := program.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Analysis: %v\n", outputs["analysis"])
	fmt.Printf("Approach: %v\n", outputs["approach"])
	fmt.Printf("\nBest Solution:\n%v\n", outputs["solution"])
	fmt.Printf("Confidence: %v\n", outputs["confidence"])
}

// Custom scorer based on solution length and detail
func solutionQualityScorer(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
	solution, ok := outputs["solution"].(string)
	if !ok {
		return 0, fmt.Errorf("solution field not found or not a string")
	}

	confidence, ok := outputs["confidence"].(float64)
	if !ok {
		confidence = 0.5 // Default if not provided
	}

	// Score based on length (prefer detailed solutions) and confidence
	wordCount := float64(len(strings.Fields(solution)))
	lengthScore := wordCount / 10.0 // Normalize

	// Combined score: 60% confidence, 40% length
	score := (confidence * 0.6) + (lengthScore * 0.4)

	return score, nil
}
