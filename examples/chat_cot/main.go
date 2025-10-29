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

	fmt.Println("=== Chain of Thought with Multi-Turn Reasoning ===")
	fmt.Println("Demonstrates: ChainOfThought module with conversation context")
	fmt.Println()

	cotChatDemo()
}

func cotChatDemo() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	history := dsgo.NewHistoryWithLimit(20)
	history.AddSystemMessage("You are a math tutor helping a student solve a complex problem step by step. Use previous conversation context.")

	sig := dsgo.NewSignature("Help solve math problems using step-by-step reasoning and conversation context").
		AddInput("question", dsgo.FieldTypeString, "The current question or problem").
		AddInput("context", dsgo.FieldTypeString, "Previous conversation and work done").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation of the reasoning").
		AddOutput("answer", dsgo.FieldTypeString, "Answer or next step")

	cot := dsgo.NewChainOfThought(sig, lm)

	problemTurns := []string{
		"I need to calculate the total cost of buying 3 shirts at $25 each and 2 pairs of pants. Can you help me set up the problem?",
		"Great! Now, if each pair of pants costs $40, what's the cost of the pants?",
		"Perfect! Now what's the total cost for everything, and if I have a 15% discount coupon, what's my final price?",
	}

	for i, question := range problemTurns {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("Student: %s\n", question)

		history.AddUserMessage(question)

		contextStr := formatHistoryContext(history.GetLast(6))

		outputs, err := cot.Forward(ctx, map[string]interface{}{
			"question": question,
			"context":  contextStr,
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		explanation := outputs["explanation"].(string)
		answer := outputs["answer"].(string)

		fmt.Printf("Tutor Reasoning: %s\n", explanation)
		fmt.Printf("Tutor Answer: %s\n", answer)

		responseMsg := fmt.Sprintf("Explanation: %s\nAnswer: %s", explanation, answer)
		history.AddAssistantMessage(responseMsg)
	}

	fmt.Printf("\n📊 Problem-Solving Session:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  Problem-solving turns: %d\n", len(problemTurns))
	fmt.Println("  ✅ Successfully solved multi-step problem across conversation!")
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
		if role == "user" {
			role = "User"
		} else if role == "assistant" {
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
