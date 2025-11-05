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

	err := h.Run(context.Background(), "009_chat_cot", runExample)
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
	history.AddSystemMessage("You are a math tutor helping a student solve a complex problem step by step. Use previous conversation context.")

	sig := dsgo.NewSignature("Help solve math problems using step-by-step reasoning and conversation context").
		AddInput("question", dsgo.FieldTypeString, "The current question or problem").
		AddInput("context", dsgo.FieldTypeString, "Previous conversation and work done").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation of the reasoning").
		AddOutput("answer", dsgo.FieldTypeString, "Answer or next step")

	cot := module.NewChainOfThought(sig, lm)

	problemTurns := []string{
		"I need to calculate the total cost of buying 3 shirts at $25 each and 2 pairs of pants. Can you help me set up the problem?",
		"Great! Now, if each pair of pants costs $40, what's the cost of the pants?",
		"Perfect! Now what's the total cost for everything, and if I have a 15% discount coupon, what's my final price?",
	}

	var lastResult *dsgo.Prediction
	var totalTokens int

	for i, question := range problemTurns {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("Student: %s\n", question)

		history.AddUserMessage(question)

		contextStr := formatHistoryContext(history.GetLast(6))

		result, err := cot.Forward(ctx, map[string]any{
			"question": question,
			"context":  contextStr,
		})
		if err != nil {
			return nil, stats, fmt.Errorf("turn %d failed: %w", i+1, err)
		}

		explanation, _ := result.GetString("explanation")
		answer, _ := result.GetString("answer")

		fmt.Printf("Tutor Reasoning: %s\n", explanation)
		fmt.Printf("Tutor Answer: %s\n", answer)

		responseMsg := fmt.Sprintf("Explanation: %s\nAnswer: %s", explanation, answer)
		history.AddAssistantMessage(responseMsg)

		totalTokens += result.Usage.TotalTokens
		lastResult = result
	}

	stats.TokensUsed = totalTokens
	stats.Metadata["total_turns"] = len(problemTurns)
	stats.Metadata["total_messages"] = history.Len()
	stats.Metadata["system_messages"] = 1
	stats.Metadata["problem_type"] = "multi-step math problem"

	fmt.Printf("\n📊 Problem-Solving Session:\n")
	fmt.Printf("  Total messages: %d\n", history.Len())
	fmt.Printf("  System messages: 1\n")
	fmt.Printf("  Problem-solving turns: %d\n", len(problemTurns))
	fmt.Printf("  Total tokens used: %d\n", totalTokens)
	fmt.Println("  ✅ Successfully solved multi-step problem across conversation!")

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
