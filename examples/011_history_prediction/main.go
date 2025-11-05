package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "011_history_prediction", runExample)
	if err != nil {
		log.Fatal(err)
	}

	if err := h.OutputResults(); err != nil {
		log.Fatal(err)
	}
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
	stats := &harness.ExecutionStats{
		Metadata: make(map[string]any),
	}

	lm := shared.GetLM(shared.GetModel())

	var totalTokens int

	fmt.Println("=== DSGo Primitives Demo: History and Prediction ===")
	fmt.Println("Learn about conversation history and rich prediction metadata")
	fmt.Println()

	// Demo 1: History for multi-turn conversations
	fmt.Println("--- Demo 1: Conversation History ---")
	tokens1, _, err := conversationDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("conversation demo failed: %w", err)
	}
	totalTokens += tokens1

	// Demo 2: Prediction with metadata
	fmt.Println("\n--- Demo 2: Rich Predictions with Metadata ---")
	tokens2, pred2, err := predictionDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("prediction demo failed: %w", err)
	}
	totalTokens += tokens2

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 2
	stats.Metadata["conversation_turns"] = 3

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total demos executed: 2\n")
	fmt.Printf("  Total tokens used: %d\n", totalTokens)
	fmt.Printf("  ✅ All primitive examples completed successfully!\n")
	fmt.Println()
	fmt.Println("💡 Next: See 015_fewshot for few-shot learning with example demonstrations")

	return pred2, stats, nil
}

func conversationDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
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

	predict := module.NewPredict(sig, lm).WithHistory(history)

	var totalTokens int
	var lastPred *dsgo.Prediction

	for i, question := range questions {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("User: %s\n", question)

		// // Add user message to history
		// history.AddUserMessage(question)

		// Get response (in real usage, you'd incorporate history into the prompt)
		outputs, err := predict.Forward(ctx, map[string]any{
			"question": question,
		})
		if err != nil {
			return totalTokens, lastPred, fmt.Errorf("predict failed on turn %d: %w", i+1, err)
		}

		answer := outputs.Outputs["answer"].(string)
		fmt.Printf("Assistant: %s\n", answer)

		// // Add assistant response to history
		// history.AddAssistantMessage(answer)

		totalTokens += outputs.Usage.TotalTokens
		lastPred = outputs
	}

	fmt.Printf("\n📊 Conversation Stats:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  Last 3 messages:\n")
	for i, msg := range history.GetLast(3) {
		fmt.Printf("    %d. [%s] %s\n", i+1, msg.Role,
			truncate(msg.Content, 50))
	}

	return totalTokens, lastPred, nil
}

func predictionDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("Classify sentiment with confidence").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score 0-1").
		AddOutput("reasoning", dsgo.FieldTypeString, "Brief explanation")

	predict := module.NewPredict(sig, lm)

	text := "This product exceeded my expectations! Highly recommended."

	outputs, err := predict.Forward(ctx, map[string]any{
		"text": text,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("prediction failed: %w", err)
	}

	// Create rich prediction with metadata
	prediction := outputs.
		WithRationale(outputs.Outputs["reasoning"].(string)).
		WithScore(outputs.Outputs["confidence"].(float64)).
		WithModuleName("SentimentClassifier").
		WithInputs(map[string]any{"text": text})

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

	return outputs.Usage.TotalTokens, outputs, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
