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

	fmt.Println("=== Chain of Thought with Multi-Turn Reasoning ===")
	fmt.Println("Demonstrates: ChainOfThought module with conversation context")
	fmt.Println()

	generateInterviewQuestion()
}

func generateInterviewQuestion() {
	ctx := context.Background()
	lm := shared.GetLM(shared.GetModel())

	history := dsgo.NewHistoryWithLimit(0)
	history.AddSystemMessage("You are a software engineer, have expertise on generating technical interview questions")

	sig := dsgo.NewSignature("Help generating technical interview question using step-by-step reasoning").
		AddInput("topic", dsgo.FieldTypeString, "The current question or problem").
		AddOutput("question", dsgo.FieldTypeString, "Clear technical coding question").
		AddOutput("solution", dsgo.FieldTypeString, "Code for solving the technical question without using builtin packages for search and sort")

	cot := module.NewChainOfThought(sig, lm).WithHistory(history)
	cot.Options.Temperature = 0.7

	topic := "algorithmic skills"
	history.AddUserMessage(topic)

	outputs, err := cot.Forward(ctx, map[string]any{
		"topic": topic,
	})
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	// Get reasoning from Rationale field (ChainOfThought stores it there)
	reasoning := outputs.Rationale

	question, ok2 := outputs.Outputs["question"].(string)
	solution, ok3 := outputs.Outputs["solution"].(string)

	if !ok2 || !ok3 {
		log.Printf("Error: Invalid output types - question=%v, solution=%v\n", outputs.Outputs["question"], outputs.Outputs["solution"])
		return
	}

	responseMsg := fmt.Sprintf("Reasoning: \n%s\n\nQuestion: \n%s\n\nSolution: \n%s", reasoning, question, solution)
	fmt.Println(responseMsg)
	history.AddAssistantMessage(responseMsg)
}
