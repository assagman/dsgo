package main

import (
	"context"
	"fmt"
	"log"

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

	fmt.Println("=== Customer Support Response Generator ===")
	fmt.Println("Demonstrates: Refine + BestOfN modules")
	fmt.Println()

	// Example 1: Generate response with Refine
	fmt.Println("--- Example 1: Refine customer response ---")
	refineResponseExample()

	fmt.Println("\n--- Example 2: Generate best response with BestOfN ---")
	bestOfNResponseExample()

	fmt.Println("\n--- Example 3: Combined workflow ---")
	combinedWorkflow()
}

func refineResponseExample() {
	ctx := context.Background()
	lm := shared.GetLM("gpt-4")

	sig := dsgo.NewSignature("Generate a professional customer support response").
		AddInput("customer_message", dsgo.FieldTypeString, "The customer's message").
		AddInput("issue_type", dsgo.FieldTypeString, "Type of issue").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback for refinement").
		AddOutput("response", dsgo.FieldTypeString, "Support response").
		AddOutput("tone", dsgo.FieldTypeString, "Tone of the response")

	refine := module.NewRefine(sig, lm).
		WithMaxIterations(2).
		WithRefinementField("feedback")

	inputs := map[string]any{
		"customer_message": "I ordered a laptop 3 weeks ago and it still hasn't arrived. This is unacceptable!",
		"issue_type":       "delayed_delivery",
		"feedback":         "make it more empathetic and offer specific compensation",
	}

	outputs, err := refine.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Customer Message: %s\n", inputs["customer_message"])
	fmt.Printf("\nRefined Response:\n%s\n", outputs.Outputs["response"])
	fmt.Printf("\nTone: %s\n", outputs.Outputs["tone"])
}

func bestOfNResponseExample() {
	ctx := context.Background()
	lm := shared.GetLM("gpt-4")

	sig := dsgo.NewSignature("Generate a customer support response").
		AddInput("customer_message", dsgo.FieldTypeString, "The customer's message").
		AddInput("issue_type", dsgo.FieldTypeString, "Type of issue").
		AddOutput("response", dsgo.FieldTypeString, "Support response").
		AddOutput("empathy_score", dsgo.FieldTypeFloat, "Empathy score 0-1").
		AddOutput("professionalism_score", dsgo.FieldTypeFloat, "Professionalism score 0-1")

	predict := module.NewPredict(sig, lm)

	// Custom scorer: combine empathy and professionalism
	scorer := func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error) {
		empathy, ok1 := prediction.Outputs["empathy_score"].(float64)
		professionalism, ok2 := prediction.Outputs["professionalism_score"].(float64)

		if !ok1 || !ok2 {
			return 0.5, nil
		}

		// Weighted score: 60% empathy, 40% professionalism
		score := (empathy * 0.6) + (professionalism * 0.4)
		return score, nil
	}

	bestOf := module.NewBestOfN(predict, 3).
		WithScorer(scorer).
		WithReturnAll(true)

	inputs := map[string]any{
		"customer_message": "Your product broke after just one week! I want a refund immediately!",
		"issue_type":       "product_defect",
	}

	outputs, err := bestOf.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Customer Message: %s\n", inputs["customer_message"])
	fmt.Printf("\nBest Response (Score: %.2f):\n%s\n", outputs.Score, outputs.Outputs["response"])
	fmt.Printf("\nEmpathy Score: %.2f\n", outputs.Outputs["empathy_score"])
	fmt.Printf("Professionalism Score: %.2f\n", outputs.Outputs["professionalism_score"])

	if scores, ok := outputs.Outputs["_best_of_n_all_scores"].([]float64); ok {
		fmt.Printf("\nAll Scores: %v\n", scores)
	}
}

func combinedWorkflow() {
	ctx := context.Background()
	lm := shared.GetLM("gpt-4")

	// Step 1: Classify the issue
	classifySig := dsgo.NewSignature("Classify the customer issue").
		AddInput("message", dsgo.FieldTypeString, "Customer message").
		AddClassOutput("category", []string{"billing", "technical", "delivery", "product_quality", "other"}, "Issue category").
		AddClassOutput("urgency", []string{"low", "medium", "high", "critical"}, "Urgency level").
		AddOutput("key_points", dsgo.FieldTypeString, "Key points from message")

	classify := module.NewPredict(classifySig, lm)

	// Step 2: Generate response
	responseSig := dsgo.NewSignature("Generate appropriate response").
		AddInput("category", dsgo.FieldTypeString, "Issue category").
		AddInput("urgency", dsgo.FieldTypeString, "Urgency level").
		AddInput("key_points", dsgo.FieldTypeString, "Key points").
		AddOutput("response", dsgo.FieldTypeString, "Support response").
		AddOutput("quality_score", dsgo.FieldTypeFloat, "Quality score 0-1")

	responseModule := module.NewPredict(responseSig, lm)

	// Use BestOfN for response generation
	bestResponse := module.NewBestOfN(responseModule, 2).
		WithScorer(module.ConfidenceScorer("quality_score"))

	// Create pipeline
	pipeline := module.NewProgram("Support Pipeline").
		AddModule(classify).
		AddModule(bestResponse)

	inputs := map[string]any{
		"message": "I've been trying to access my account for 2 days but keep getting error 500. I have an important meeting tomorrow and need this fixed ASAP!",
	}

	outputs, err := pipeline.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Customer Message: %s\n", inputs["message"])
	fmt.Printf("\nClassification:")
	fmt.Printf("\n  Category: %v", outputs.Outputs["category"])
	fmt.Printf("\n  Urgency: %v", outputs.Outputs["urgency"])
	fmt.Printf("\n  Key Points: %v\n", outputs.Outputs["key_points"])
	fmt.Printf("\nGenerated Response:\n%s\n", outputs.Outputs["response"])
	fmt.Printf("\nQuality Score: %.2f\n", outputs.Outputs["quality_score"])
}
