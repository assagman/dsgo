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

	err := h.Run(context.Background(), "008_chat_predict", runExample)
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

	// Create conversation history with limit
	history := dsgo.NewHistoryWithLimit(20)
	history.AddSystemMessage("You are a helpful travel assistant. Keep responses concise and remember context from the conversation.")

	sig := dsgo.NewSignature("Answer travel-related questions using conversation context").
		AddInput("question", dsgo.FieldTypeString, "The current question").
		AddInput("context", dsgo.FieldTypeString, "Previous conversation context").
		AddOutput("answer", dsgo.FieldTypeString, "Answer that builds on previous context")

	predict := module.NewPredict(sig, lm)

	conversationTurns := []string{
		"I'm planning a trip to Japan in spring. What's the best time to visit?",
		"What cities should I visit there?",
		"How many days should I spend in each city you mentioned?",
		"What's the best way to travel between those cities?",
	}

	var lastResult *dsgo.Prediction
	var totalTokens int

	for i, question := range conversationTurns {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("User: %s\n", question)

		history.AddUserMessage(question)

		contextStr := formatHistoryContext(history.GetLast(6))

		result, err := predict.Forward(ctx, map[string]any{
			"question": question,
			"context":  contextStr,
		})
		if err != nil {
			return nil, stats, fmt.Errorf("turn %d failed: %w", i+1, err)
		}

		answer, _ := result.GetString("answer")
		fmt.Printf("Assistant: %s\n", answer)

		history.AddAssistantMessage(answer)

		totalTokens += result.Usage.TotalTokens
		lastResult = result
	}

	stats.TokensUsed = totalTokens
	stats.Metadata["total_turns"] = len(conversationTurns)
	stats.Metadata["total_messages"] = history.Len()
	stats.Metadata["system_messages"] = 1
	stats.Metadata["conversation_topic"] = "Japan travel planning"

	fmt.Printf("\n📊 Conversation Summary:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  System messages: 1\n")
	fmt.Printf("  User turns: %d\n", len(conversationTurns))
	fmt.Printf("  Total tokens used: %d\n", totalTokens)

	return lastResult, stats, nil
}

func formatHistoryContext(messages []dsgo.Message) string {
	if len(messages) == 0 {
		return "No previous context"
	}

	var context string
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		role := msg.Role
		switch role {
		case "user":
			role = "User"
		case "assistant":
			role = "Assistant"
		}
		context += fmt.Sprintf("%s: %s\n", role, truncate(msg.Content, 150))
	}
	return context
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
