package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

// This example demonstrates streaming support for real-time output.
// The Stream method allows you to process LM responses as they arrive,
// chunk by chunk, providing a better user experience for long-running tasks.

func main() {
	shared.LoadEnv()

	// Get LM (OpenRouter or OpenAI based on environment)
	lm := shared.GetLM(shared.GetModel())

	// Create signature for story generation
	sig := dsgo.NewSignature("Generate a creative short story based on the given prompt").
		AddInput("prompt", dsgo.FieldTypeString, "Story prompt or theme").
		AddOutput("story", dsgo.FieldTypeString, "The generated story").
		AddOutput("title", dsgo.FieldTypeString, "A catchy title for the story").
		AddOutput("genre", dsgo.FieldTypeString, "The story genre")

	// Create Predict module
	predict := module.NewPredict(sig, lm)

	ctx := context.Background()

	fmt.Println("=== Streaming Story Generation ===")
	fmt.Println("Watch the story being written in real-time...")
	fmt.Println()

	// Start streaming
	result, err := predict.Stream(ctx, map[string]any{
		"prompt": "A lone astronaut discovers an ancient alien artifact on Mars",
	})
	if err != nil {
		log.Fatalf("Failed to start streaming: %v", err)
	}

	fmt.Println("--- Streaming Output ---")

	// Process chunks in real-time
	for chunk := range result.Chunks {
		// Print each chunk as it arrives (simulating real-time typing effect)
		fmt.Print(chunk.Content)

		// Check if stream ended
		if chunk.FinishReason != "" {
			fmt.Printf("\n\n[Stream finished: %s]\n", chunk.FinishReason)
		}
	}

	// Check for errors during streaming
	select {
	case err := <-result.Errors:
		if err != nil {
			log.Fatalf("Streaming error: %v", err)
		}
	default:
	}

	// Wait for final parsed prediction
	prediction := <-result.Prediction

	fmt.Println("\n--- Parsed Structured Output ---")
	title, _ := prediction.GetString("title")
	genre, _ := prediction.GetString("genre")
	story, _ := prediction.GetString("story")

	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Genre: %s\n", genre)
	fmt.Printf("Story Length: %d characters\n", len(story))

	// Show usage statistics
	fmt.Printf("\n--- Token Usage ---\n")
	fmt.Printf("Prompt Tokens: %d\n", prediction.Usage.PromptTokens)
	fmt.Printf("Completion Tokens: %d\n", prediction.Usage.CompletionTokens)
	fmt.Printf("Total Tokens: %d\n", prediction.Usage.TotalTokens)

	// Show adapter metrics
	if prediction.AdapterUsed != "" {
		fmt.Printf("\n--- Adapter Metrics ---\n")
		fmt.Printf("Adapter Used: %s\n", prediction.AdapterUsed)
		fmt.Printf("Parse Success: %v\n", prediction.ParseSuccess)
	}

	fmt.Println("\n=== Streaming Complete ===")
}
