package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// Demonstrates: Predict, Chat adapter, Streaming, History
// Story: Personal assistant that remembers context and streams responses

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(2)
	}
	envFilePath := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "examples", ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// If not found in examples/, check cwd/.env.local
	if envFilePath == "" {
		candidate := filepath.Join(cwd, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
		}
	}
	if envFilePath == "" {
		fmt.Printf("Could not find .env.local file\n")
		os.Exit(3)
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(3)
	}

	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "chat_assistant", map[string]interface{}{
		"scenario": "personal_assistant",
	})
	defer runSpan.End(nil)

	// Setup - NewLM auto-detects provider from model name
	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Configuration
	fmt.Println("\n=== Configuration ===")
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("Temperature: 0.9 (creative responses)\n")
	fmt.Printf("Max tokens: 5000\n")

	sig := dsgo.NewSignature("You are a helpful personal assistant").
		AddInput("message", dsgo.FieldTypeString, "User message").
		AddOutput("response", dsgo.FieldTypeString, "Assistant response")

	history := dsgo.NewHistoryWithLimit(10)
	predict := module.NewPredict(sig, lm).
		WithHistory(history).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.9,   // Creative, varied responses
			MaxTokens:   5000,  // Generous token limit for verbose models
		})

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	// Turn 1: Introduction with streaming
	fmt.Println("\n=== Turn 1: Introduction (Streaming) ===")
	turn1Ctx, turn1Span := observe.Start(ctx, observe.SpanKindModule, "turn1", map[string]interface{}{
		"streaming": true,
	})

	userMessage1 := "Hi! My name is Alex and I love hiking. What outdoor activities would you recommend?"
	fmt.Printf("User: %s\n", userMessage1)

	streamResult, err := predict.Stream(turn1Ctx, map[string]interface{}{
		"message": userMessage1,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("Assistant: ")
	var fullResponse string
	for chunk := range streamResult.Chunks {
		fmt.Print(chunk.Content)
		fullResponse += chunk.Content
	}
	fmt.Println()

	if err := <-streamResult.Errors; err != nil {
		log.Fatal(err)
	}

	// Get final prediction for usage stats
	pred := <-streamResult.Prediction
	if pred != nil {
		usage := pred.Usage
		fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage.PromptTokens, usage.CompletionTokens)
		totalPromptTokens += usage.PromptTokens
		totalCompletionTokens += usage.CompletionTokens
	}

	turn1Span.End(nil)

	// Turn 2: Follow-up using history
	fmt.Println("\n=== Turn 2: Follow-up (Using History) ===")
	turn2Ctx, turn2Span := observe.Start(ctx, observe.SpanKindModule, "turn2", map[string]interface{}{
		"history_entries": history.Len(),
	})

	userMessage2 := "Which of those would be best for a beginner?"
	fmt.Printf("User: %s\n", userMessage2)

	result2, err := predict.Forward(turn2Ctx, map[string]interface{}{
		"message": userMessage2,
	})
	if err != nil {
		log.Fatal(err)
	}

	response2, _ := result2.GetString("response")
	fmt.Printf("Assistant: %s\n", response2)
	usage2 := result2.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage2.PromptTokens, usage2.CompletionTokens)
	totalPromptTokens += usage2.PromptTokens
	totalCompletionTokens += usage2.CompletionTokens
	turn2Span.End(nil)

	// Turn 3: Another follow-up question
	fmt.Println("\n=== Turn 3: Equipment Question ===")
	turn3Ctx, turn3Span := observe.Start(ctx, observe.SpanKindModule, "turn3", nil)

	userMessage3 := "What gear do I need for hiking?"
	fmt.Printf("User: %s\n", userMessage3)

	result3, err := predict.Forward(turn3Ctx, map[string]interface{}{
		"message": userMessage3,
	})
	if err != nil {
		log.Fatal(err)
	}

	response3, _ := result3.GetString("response")
	fmt.Printf("Assistant: %s\n", response3)
	usage3 := result3.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage3.PromptTokens, usage3.CompletionTokens)
	totalPromptTokens += usage3.PromptTokens
	totalCompletionTokens += usage3.CompletionTokens
	turn3Span.End(nil)

	// Summary
	fmt.Println("\n=== Conversation Summary ===")
	fmt.Printf("Total turns: 3\n")
	fmt.Printf("History entries: %d\n", history.Len())
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ Predict module with chat adapter")
	fmt.Println("  ✓ Streaming responses")
	fmt.Println("  ✓ History management (context retention)")
	fmt.Println("  ✓ Multi-turn conversations")
	fmt.Println("  ✓ Generation options (temperature, max_tokens)")
	fmt.Println("  ✓ Event logging (set DSGO_LOG=pretty)")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
