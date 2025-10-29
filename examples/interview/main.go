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

	generateInterviewQuestion()
}

func generateInterviewQuestion() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	history := dsgo.NewHistoryWithLimit(0)
	history.AddSystemMessage("You are a software engineer, have expertise on generating technical interview questions")

	sig := dsgo.NewSignature("Help generating technical interview question using step-by-step reasoning").
		AddInput("topic", dsgo.FieldTypeString, "The current question or problem").
		AddInput("history", dsgo.FieldTypeString, "Conversation history").
		AddOutput("question", dsgo.FieldTypeString, "Clear technical coding question").
		AddOutput("solution", dsgo.FieldTypeString, "Code for solving the technical question without using builtin packages for search and sort")

	cot := dsgo.NewChainOfThought(sig, lm)
	cot.Options.Temperature = 0.7

	topic := "algorithmic skills"
	history.AddUserMessage(topic)

	outputs, err := cot.Forward(ctx, map[string]interface{}{
		"topic":   topic,
		"history": history,
	})
	if err != nil {
		log.Printf("Error: %v\n", err)
	}

	reasoning := outputs["reasoning"].(string)
	question := outputs["question"].(string)
	solution := outputs["solution"].(string)

	responseMsg := fmt.Sprintf("Reasoning: \n%s\n\nQuestion: \n%s\n\nSolution: \n%s", reasoning, question, solution)
	fmt.Println(responseMsg)
	history.AddAssistantMessage(responseMsg)
}
