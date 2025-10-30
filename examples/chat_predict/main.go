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

	fmt.Println("=== Chat with Predict and Conversation History ===")
	fmt.Println("Demonstrates: Predict module with conversation context")
	fmt.Println()

	predictChatDemo()
}

func predictChatDemo() {
	ctx := context.Background()
	lm := shared.GetLM(shared.GetModel())

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

	for i, question := range conversationTurns {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("User: %s\n", question)

		history.AddUserMessage(question)

		contextStr := formatHistoryContext(history.GetLast(6))

		outputs, err := predict.Forward(ctx, map[string]any{
			"question": question,
			"context":  contextStr,
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		answer := outputs.Outputs["answer"].(string)
		fmt.Printf("Assistant: %s\n", answer)

		history.AddAssistantMessage(answer)
	}

	fmt.Printf("\n📊 Conversation Summary:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  System messages: 1\n")
	fmt.Printf("  User turns: %d\n", len(conversationTurns))
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
