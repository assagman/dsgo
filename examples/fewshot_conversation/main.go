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

	fmt.Println("=== DSGo Phase 1: Primitives Demo ===")
	fmt.Println("Demonstrates: History, Prediction, and Example (few-shot)")
	fmt.Println()

	// Demo 1: History for multi-turn conversations
	fmt.Println("--- Demo 1: Conversation History ---")
	conversationDemo()

	// Demo 2: Prediction with metadata
	fmt.Println("\n--- Demo 2: Rich Predictions ---")
	predictionDemo()

	// Demo 3: Few-shot learning with Examples
	fmt.Println("\n--- Demo 3: Few-Shot Learning ---")
	fewShotDemo()
}

func conversationDemo() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	// Create conversation history
	history := dsgo.NewHistoryWithLimit(10) // Keep last 10 messages
	
	history.AddSystemMessage("You are a helpful coding assistant. Keep responses concise.")
	
	// Simulate a multi-turn conversation
	questions := []string{
		"What is a closure in JavaScript?",
		"Can you show me a simple example?",
		"How is this different from a regular function?",
	}
	
	sig := dsgo.NewSignature("Answer coding questions").
		AddInput("question", dsgo.FieldTypeString, "The question").
		AddOutput("answer", dsgo.FieldTypeString, "Concise answer")
	
	predict := dsgo.NewPredict(sig, lm)
	
	for i, question := range questions {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("User: %s\n", question)
		
		// Add user message to history
		history.AddUserMessage(question)
		
		// Get response (in real usage, you'd incorporate history into the prompt)
		outputs, err := predict.Forward(ctx, map[string]interface{}{
			"question": question,
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}
		
		answer := outputs["answer"].(string)
		fmt.Printf("Assistant: %s\n", answer)
		
		// Add assistant response to history
		history.AddAssistantMessage(answer)
	}
	
	fmt.Printf("\n📊 Conversation Stats:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  Last 3 messages:\n")
	for i, msg := range history.GetLast(3) {
		fmt.Printf("    %d. [%s] %s\n", i+1, msg.Role, 
			truncate(msg.Content, 50))
	}
}

func predictionDemo() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")
	
	sig := dsgo.NewSignature("Classify sentiment with confidence").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score 0-1").
		AddOutput("reasoning", dsgo.FieldTypeString, "Brief explanation")
	
	predict := dsgo.NewPredict(sig, lm)
	
	text := "This product exceeded my expectations! Highly recommended."
	
	outputs, err := predict.Forward(ctx, map[string]interface{}{
		"text": text,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	// Create rich prediction with metadata
	prediction := dsgo.NewPrediction(outputs).
		WithRationale(outputs["reasoning"].(string)).
		WithScore(outputs["confidence"].(float64)).
		WithModuleName("SentimentClassifier").
		WithInputs(map[string]interface{}{"text": text})
	
	fmt.Printf("Input: %s\n\n", text)
	fmt.Printf("📦 Prediction Details:\n")
	fmt.Printf("  Sentiment: %v\n", prediction.Outputs["sentiment"])
	fmt.Printf("  Confidence: %.2f\n", prediction.Score)
	fmt.Printf("  Reasoning: %s\n", prediction.Rationale)
	fmt.Printf("  Module: %s\n", prediction.ModuleName)
	
	// Demonstrate type-safe getters
	if sentiment, ok := prediction.GetString("sentiment"); ok {
		fmt.Printf("\n✅ Type-safe access: sentiment = %s\n", sentiment)
	}
}

func fewShotDemo() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")
	
	// Create signature for movie genre classification
	sig := dsgo.NewSignature("Classify movie genre from plot description").
		AddInput("plot", dsgo.FieldTypeString, "Movie plot description").
		AddClassOutput("genre", []string{"action", "comedy", "drama", "horror", "sci-fi", "romance"}, "Primary genre").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Classification confidence")
	
	// Create few-shot examples
	examples_set := dsgo.NewExampleSet("movie-genres")
	
	examples_set.AddPair(
		map[string]interface{}{
			"plot": "A group of astronauts discovers an alien artifact on Mars that changes humanity's understanding of the universe.",
		},
		map[string]interface{}{
			"genre": "sci-fi",
			"confidence": 0.95,
		},
	).AddPair(
		map[string]interface{}{
			"plot": "Two rival chefs compete in a cooking competition while falling in love.",
		},
		map[string]interface{}{
			"genre": "romance",
			"confidence": 0.90,
		},
	).AddPair(
		map[string]interface{}{
			"plot": "A detective races against time to stop a bomb from destroying the city.",
		},
		map[string]interface{}{
			"genre": "action",
			"confidence": 0.92,
		},
	)
	
	// Format examples for the prompt
	examplesText, _ := examples_set.FormatExamples(sig)
	
	fmt.Printf("📚 Few-Shot Examples Loaded: %d examples\n", examples_set.Len())
	fmt.Printf("\n%s\n", examplesText)
	
	// Create extended signature with examples
	extendedSig := dsgo.NewSignature(
		sig.Description + "\n\n" + examplesText + "\nNow classify this new movie:",
	)
	extendedSig.InputFields = sig.InputFields
	extendedSig.OutputFields = sig.OutputFields
	
	predict := dsgo.NewPredict(extendedSig, lm)
	
	// Test with a new movie plot
	testPlot := "A young wizard attends a magical school and battles dark forces threatening the wizarding world."
	
	outputs, err := predict.Forward(ctx, map[string]interface{}{
		"plot": testPlot,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	fmt.Printf("🎬 New Movie Classification:\n")
	fmt.Printf("  Plot: %s\n", testPlot)
	fmt.Printf("  Predicted Genre: %v\n", outputs["genre"])
	fmt.Printf("  Confidence: %.2f\n", outputs["confidence"])
	
	fmt.Printf("\n💡 Few-shot learning helps the model understand the task better!\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
