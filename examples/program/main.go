package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	lm := shared.GetLM(shared.GetModel())

	fmt.Println("=== Program Module Example: Module Composition ===")

	// Example 1: Simple 2-stage pipeline
	simplePipeline(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 2: Multi-stage pipeline with different module types
	complexPipeline(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 3: Conditional pipeline with branching
	conditionalPipeline(lm)
}

func simplePipeline(lm dsgo.LM) {
	fmt.Println("Example 1: Simple 2-Stage Pipeline")
	fmt.Println("===================================")

	// Stage 1: Extract key information
	extractSig := dsgo.NewSignature("Extract key information from text.").
		AddInput("text", dsgo.FieldTypeString, "Input text").
		AddOutput("topic", dsgo.FieldTypeString, "Main topic").
		AddOutput("keywords", dsgo.FieldTypeString, "Comma-separated keywords")

	extractModule := module.NewPredict(extractSig, lm)

	// Stage 2: Analyze sentiment
	sentimentSig := dsgo.NewSignature("Analyze the sentiment of the topic.").
		AddInput("topic", dsgo.FieldTypeString, "Topic to analyze").
		AddOutput("sentiment", dsgo.FieldTypeString, "Sentiment (positive/negative/neutral)").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

	sentimentModule := module.NewPredict(sentimentSig, lm)

	// Create pipeline
	pipeline := module.NewProgram("Extract and Analyze").
		AddModule(extractModule).
		AddModule(sentimentModule)

	ctx := context.Background()

	text := `I absolutely love the new features in this software update! 
The performance improvements are incredible, and the UI looks fantastic. 
The development team really outdid themselves this time.`

	fmt.Printf("📝 Input text:\n%s\n\n", text)

	result, err := pipeline.Forward(ctx, map[string]interface{}{
		"text": text,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("🔍 Stage 1 - Extraction Results:")
	fmt.Printf("   Topic: %s\n", result.Outputs["topic"])
	fmt.Printf("   Keywords: %s\n", result.Outputs["keywords"])

	fmt.Println("\n📊 Stage 2 - Sentiment Analysis:")
	fmt.Printf("   Sentiment: %s\n", result.Outputs["sentiment"])
	fmt.Printf("   Confidence: %.2f\n", result.Outputs["confidence"])
}

func complexPipeline(lm dsgo.LM) {
	fmt.Println("Example 2: Multi-Stage Pipeline with Different Modules")
	fmt.Println("=======================================================")

	// Stage 1: Analyze the problem (ChainOfThought)
	analyzeSig := dsgo.NewSignature("Analyze the problem and break it down.").
		AddInput("problem", dsgo.FieldTypeString, "Problem description").
		AddOutput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddOutput("approach", dsgo.FieldTypeString, "Recommended approach")

	analyzeModule := module.NewChainOfThought(analyzeSig, lm)

	// Stage 2: Generate solution (Predict)
	solveSig := dsgo.NewSignature("Generate a solution based on the analysis.").
		AddInput("analysis", dsgo.FieldTypeString, "Problem analysis").
		AddInput("approach", dsgo.FieldTypeString, "Recommended approach").
		AddOutput("solution", dsgo.FieldTypeString, "Detailed solution").
		AddOutput("steps", dsgo.FieldTypeString, "Implementation steps")

	solveModule := module.NewPredict(solveSig, lm)

	// Stage 3: Evaluate solution quality (Predict)
	evaluateSig := dsgo.NewSignature("Evaluate the quality of the solution.").
		AddInput("problem", dsgo.FieldTypeString, "Original problem").
		AddInput("solution", dsgo.FieldTypeString, "Proposed solution").
		AddOutput("quality_score", dsgo.FieldTypeFloat, "Quality score (0-1)").
		AddOutput("strengths", dsgo.FieldTypeString, "Solution strengths").
		AddOutput("improvements", dsgo.FieldTypeString, "Potential improvements")

	evaluateModule := module.NewPredict(evaluateSig, lm)

	// Create pipeline
	pipeline := module.NewProgram("Analyze-Solve-Evaluate").
		AddModule(analyzeModule).
		AddModule(solveModule).
		AddModule(evaluateModule)

	ctx := context.Background()

	problem := `Our web application is experiencing slow load times during peak hours. 
Users are complaining about pages taking 5-10 seconds to load.`

	fmt.Printf("🎯 Problem:\n%s\n\n", problem)

	result, err := pipeline.Forward(ctx, map[string]interface{}{
		"problem": problem,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("📋 Stage 1 - Analysis (ChainOfThought):")
	fmt.Printf("%s\n", result.Outputs["analysis"])
	fmt.Printf("\n   Recommended Approach: %s\n", result.Outputs["approach"])

	fmt.Println("\n💡 Stage 2 - Solution:")
	fmt.Printf("%s\n", result.Outputs["solution"])
	fmt.Printf("\n   Steps:\n%s\n", result.Outputs["steps"])

	fmt.Println("\n⭐ Stage 3 - Evaluation:")
	fmt.Printf("   Quality Score: %.2f/1.0\n", result.Outputs["quality_score"])
	fmt.Printf("   Strengths: %s\n", result.Outputs["strengths"])
	fmt.Printf("   Improvements: %s\n", result.Outputs["improvements"])
}

func conditionalPipeline(lm dsgo.LM) {
	fmt.Println("Example 3: Conditional Pipeline (Manual Branching)")
	fmt.Println("==================================================")

	// Stage 1: Classify the input
	classifySig := dsgo.NewSignature("Classify the type of customer inquiry.").
		AddInput("message", dsgo.FieldTypeString, "Customer message").
		AddClassOutput("category", []string{"technical", "billing", "general"}, "Inquiry category").
		AddOutput("urgency", dsgo.FieldTypeString, "Urgency level (low/medium/high)")

	classifyModule := module.NewPredict(classifySig, lm)

	// Stage 2a: Technical response
	techSig := dsgo.NewSignature("Generate a technical support response.").
		AddInput("message", dsgo.FieldTypeString, "Customer message").
		AddOutput("response", dsgo.FieldTypeString, "Technical support response").
		AddClassOutput("escalate", []string{"true", "false"}, "Should escalate to engineer")

	techModule := module.NewPredict(techSig, lm)

	// Stage 2b: Billing response
	billingSig := dsgo.NewSignature("Generate a billing support response.").
		AddInput("message", dsgo.FieldTypeString, "Customer message").
		AddOutput("response", dsgo.FieldTypeString, "Billing support response").
		AddClassOutput("refund_eligible", []string{"true", "false"}, "Is customer eligible for refund")

	billingModule := module.NewPredict(billingSig, lm)

	// Stage 2c: General response
	generalSig := dsgo.NewSignature("Generate a general support response.").
		AddInput("message", dsgo.FieldTypeString, "Customer message").
		AddOutput("response", dsgo.FieldTypeString, "General support response")

	generalModule := module.NewPredict(generalSig, lm)

	ctx := context.Background()

	testMessages := []string{
		"My account keeps logging me out every 5 minutes. I've cleared cache but it's still happening.",
		"I was charged twice for my subscription this month. Can you help?",
		"What are your business hours and do you have a phone number?",
	}

	for i, message := range testMessages {
		fmt.Printf("\n[Message %d]\n", i+1)
		fmt.Printf("Customer: %s\n\n", message)

		// Step 1: Classify
		classifyResult, err := classifyModule.Forward(ctx, map[string]interface{}{
			"message": message,
		})
		if err != nil {
			log.Fatal(err)
		}

		category := classifyResult.Outputs["category"].(string)
		urgency := classifyResult.Outputs["urgency"].(string)

		fmt.Printf("🏷️  Category: %s | Urgency: %s\n\n", category, urgency)

		// Step 2: Route to appropriate handler
		var responseResult *dsgo.Prediction

		switch category {
		case "technical":
			responseResult, err = techModule.Forward(ctx, map[string]interface{}{
				"message": message,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("💻 Technical Response:\n%s\n", responseResult.Outputs["response"])
			if responseResult.Outputs["escalate"].(string) == "true" {
				fmt.Println("⚠️  ESCALATE TO ENGINEERING TEAM")
			}

		case "billing":
			responseResult, err = billingModule.Forward(ctx, map[string]interface{}{
				"message": message,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("💰 Billing Response:\n%s\n", responseResult.Outputs["response"])
			if responseResult.Outputs["refund_eligible"].(string) == "true" {
				fmt.Println("✅ REFUND ELIGIBLE")
			}

		case "general":
			responseResult, err = generalModule.Forward(ctx, map[string]interface{}{
				"message": message,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("📧 General Response:\n%s\n", responseResult.Outputs["response"])
		}

		if i < len(testMessages)-1 {
			fmt.Println("\n" + string(make([]rune, 60)))
		}
	}
}
