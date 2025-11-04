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

	lm := shared.GetLM(shared.GetModel())

	fmt.Println("=== History Example: Multi-Turn Conversation ===")

	// Example 1: Basic multi-turn conversation with history
	basicConversation(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 2: History with limit (conversation pruning)
	conversationWithLimit(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 3: History cloning and management
	historyManagement(lm)
}

func basicConversation(lm dsgo.LM) {
	fmt.Println("Example 1: Basic Multi-Turn Conversation")
	fmt.Println("==========================================")

	sig := dsgo.NewSignature("You are a helpful assistant. Answer the user's question based on conversation context.").
		AddInput("question", dsgo.FieldTypeString, "The user's question").
		AddOutput("answer", dsgo.FieldTypeString, "Your detailed answer")

	// Create history with no limit
	history := dsgo.NewHistoryWithLimit(0)

	predict := module.NewPredict(sig, lm).WithHistory(history)

	ctx := context.Background()

	// Turn 1: Ask about Go
	fmt.Println("\n[Turn 1]")
	fmt.Println("User: What is Go programming language?")
	result1, err := predict.Forward(ctx, map[string]interface{}{
		"question": "What is Go programming language?",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Assistant: %s\n", result1.Outputs["answer"])

	// Turn 2: Follow-up question (requires context)
	fmt.Println("\n[Turn 2]")
	fmt.Println("User: What are its main features?")
	result2, err := predict.Forward(ctx, map[string]interface{}{
		"question": "What are its main features?",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Assistant: %s\n", result2.Outputs["answer"])

	// Turn 3: Another follow-up
	fmt.Println("\n[Turn 3]")
	fmt.Println("User: How does it compare to Python?")
	result3, err := predict.Forward(ctx, map[string]interface{}{
		"question": "How does it compare to Python?",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Assistant: %s\n", result3.Outputs["answer"])

	// Show history length
	fmt.Printf("\n📊 Total messages in history: %d\n", history.Len())
}

func conversationWithLimit(lm dsgo.LM) {
	fmt.Println("Example 2: Conversation with History Limit")
	fmt.Println("===========================================")

	sig := dsgo.NewSignature("You are a creative storyteller. Continue the story based on what the user says.").
		AddInput("input", dsgo.FieldTypeString, "User input to continue the story").
		AddOutput("continuation", dsgo.FieldTypeString, "Story continuation")

	// Create history with limit of 4 messages (2 turns)
	// This will automatically prune old messages
	history := dsgo.NewHistoryWithLimit(4)

	predict := module.NewPredict(sig, lm).WithHistory(history)

	ctx := context.Background()

	turns := []string{
		"Once upon a time, there was a brave knight.",
		"The knight found a mysterious cave.",
		"Inside the cave was a sleeping dragon.",
		"The dragon woke up!",
	}

	for i, turn := range turns {
		fmt.Printf("\n[Turn %d] History size: %d/%d\n", i+1, history.Len(), 4)
		fmt.Printf("User: %s\n", turn)

		result, err := predict.Forward(ctx, map[string]interface{}{
			"input": turn,
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Assistant: %s\n", result.Outputs["continuation"])
	}

	fmt.Printf("\n📊 Final history size (limited to 4): %d\n", history.Len())
	fmt.Println("✅ Old messages automatically pruned!")
}

func historyManagement(lm dsgo.LM) {
	fmt.Println("Example 3: History Management Operations")
	fmt.Println("=========================================")

	sig := dsgo.NewSignature("Answer questions about math.").
		AddInput("question", dsgo.FieldTypeString, "Math question").
		AddOutput("answer", dsgo.FieldTypeString, "Math answer")

	history := dsgo.NewHistoryWithLimit(0)
	predict := module.NewPredict(sig, lm).WithHistory(history)

	ctx := context.Background()

	// Add some conversation
	fmt.Println("\n1️⃣ Building conversation...")
	questions := []string{
		"What is 5 + 3?",
		"What is 10 * 2?",
		"What is 15 / 3?",
	}

	for _, q := range questions {
		_, err := predict.Forward(ctx, map[string]interface{}{
			"question": q,
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("   History size: %d messages\n", history.Len())

	// Clone history
	fmt.Println("\n2️⃣ Cloning history...")
	clonedHistory := history.Clone()
	fmt.Printf("   Original: %d messages, Clone: %d messages\n", history.Len(), clonedHistory.Len())

	// Get last N messages
	fmt.Println("\n3️⃣ Getting last 2 messages...")
	lastTwo := history.GetLast(2)
	fmt.Printf("   Retrieved %d messages\n", len(lastTwo))
	for i, msg := range lastTwo {
		fmt.Printf("   [%d] Role: %s, Content: %.50s...\n", i+1, msg.Role, msg.Content)
	}

	// Manual message addition
	fmt.Println("\n4️⃣ Adding messages manually...")
	history.AddUserMessage("What is the meaning of life?")
	history.AddAssistantMessage("42")
	fmt.Printf("   History size after manual adds: %d messages\n", history.Len())

	// Clear history
	fmt.Println("\n5️⃣ Clearing history...")
	history.Clear()
	fmt.Printf("   History size after clear: %d messages\n", history.Len())
	fmt.Println("   ✅ History is empty!")

	// Restore from clone
	fmt.Println("\n6️⃣ Restoring from clone...")
	history = clonedHistory
	fmt.Printf("   Restored history size: %d messages\n", history.Len())
	fmt.Println("   ✅ History restored!")
}
