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

	err := h.Run(context.Background(), "016_history", runExample)
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

	fmt.Println("=== History Management Demo ===")
	fmt.Println("Advanced history operations: limits, cloning, inspection, and restoration")
	fmt.Println()

	// Demo 1: Basic multi-turn conversation
	fmt.Println("--- Demo 1: Basic Multi-Turn Conversation with History ---")
	tokens1, _, err := basicConversation(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("basic conversation failed: %w", err)
	}
	totalTokens += tokens1

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: History with limit
	fmt.Println("--- Demo 2: Conversation with History Limit (Auto-Pruning) ---")
	tokens2, _, err := conversationWithLimit(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("limited conversation failed: %w", err)
	}
	totalTokens += tokens2

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Advanced history management
	fmt.Println("--- Demo 3: Advanced History Management Operations ---")
	tokens3, pred3, err := historyManagement(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("history management failed: %w", err)
	}
	totalTokens += tokens3

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 3
	stats.Metadata["features_demonstrated"] = []string{
		"WithHistory()",
		"NewHistoryWithLimit()",
		"Clone()",
		"GetLast()",
		"Clear()",
		"Manual message addition",
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("History management capabilities:\n")
	fmt.Printf("  ✓ Multi-turn conversation context\n")
	fmt.Printf("  ✓ Automatic message limit enforcement\n")
	fmt.Printf("  ✓ History cloning and restoration\n")
	fmt.Printf("  ✓ Message inspection and retrieval\n")
	fmt.Printf("  ✓ Manual history manipulation\n")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 3\n")
	fmt.Println()

	return pred3, stats, nil
}

func basicConversation(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("You are a helpful assistant. Answer the user's question based on conversation context.").
		AddInput("question", dsgo.FieldTypeString, "The user's question").
		AddOutput("answer", dsgo.FieldTypeString, "Your concise answer")

	// Create history with no limit
	history := dsgo.NewHistoryWithLimit(0)
	predict := module.NewPredict(sig, lm).WithHistory(history)

	var totalTokens int
	var lastPred *dsgo.Prediction

	questions := []struct {
		turn     int
		question string
	}{
		{1, "What is Go programming language?"},
		{2, "What are its main features?"},
		{3, "How does it compare to Python?"},
	}

	for _, q := range questions {
		fmt.Printf("\n[Turn %d]\n", q.turn)
		fmt.Printf("User: %s\n", q.question)

		result, err := predict.Forward(ctx, map[string]any{
			"question": q.question,
		})
		if err != nil {
			return totalTokens, lastPred, fmt.Errorf("turn %d failed: %w", q.turn, err)
		}

		answer, _ := result.GetString("answer")
		fmt.Printf("Assistant: %s\n", truncate(answer, 150))

		totalTokens += result.Usage.TotalTokens
		lastPred = result
	}

	fmt.Printf("\n📊 Total messages in history: %d\n", history.Len())
	fmt.Printf("💡 History stores all conversation context for multi-turn coherence\n")

	return totalTokens, lastPred, nil
}

func conversationWithLimit(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("You are a creative storyteller. Continue the story based on what the user says.").
		AddInput("input", dsgo.FieldTypeString, "User input to continue the story").
		AddOutput("continuation", dsgo.FieldTypeString, "Brief story continuation")

	// Create history with limit of 4 messages (2 turns)
	// This will automatically prune old messages
	history := dsgo.NewHistoryWithLimit(4)
	predict := module.NewPredict(sig, lm).WithHistory(history)

	var totalTokens int
	var lastPred *dsgo.Prediction

	turns := []string{
		"Once upon a time, there was a brave knight.",
		"The knight found a mysterious cave.",
		"Inside the cave was a sleeping dragon.",
		"The dragon woke up!",
	}

	for i, turn := range turns {
		fmt.Printf("\n[Turn %d] History size: %d (limit: 4)\n", i+1, history.Len())
		fmt.Printf("User: %s\n", turn)

		result, err := predict.Forward(ctx, map[string]any{
			"input": turn,
		})
		if err != nil {
			return totalTokens, lastPred, fmt.Errorf("turn %d failed: %w", i+1, err)
		}

		continuation, _ := result.GetString("continuation")
		fmt.Printf("Assistant: %s\n", truncate(continuation, 150))

		totalTokens += result.Usage.TotalTokens
		lastPred = result
	}

	fmt.Printf("\n📊 Final history size: %d (limited to 4)\n", history.Len())
	fmt.Printf("✅ Old messages automatically pruned to prevent context overflow!\n")

	return totalTokens, lastPred, nil
}

func historyManagement(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	sig := dsgo.NewSignature("Answer questions about math briefly.").
		AddInput("question", dsgo.FieldTypeString, "Math question").
		AddOutput("answer", dsgo.FieldTypeString, "Math answer")

	history := dsgo.NewHistoryWithLimit(0)
	predict := module.NewPredict(sig, lm).WithHistory(history)

	var totalTokens int
	var lastPred *dsgo.Prediction

	// 1. Build conversation
	fmt.Println("\n1️⃣ Building conversation...")
	questions := []string{
		"What is 5 + 3?",
		"What is 10 * 2?",
		"What is 15 / 3?",
	}

	for _, q := range questions {
		result, err := predict.Forward(ctx, map[string]any{
			"question": q,
		})
		if err != nil {
			return totalTokens, lastPred, fmt.Errorf("conversation build failed: %w", err)
		}
		totalTokens += result.Usage.TotalTokens
		lastPred = result
	}
	fmt.Printf("   History size: %d messages\n", history.Len())

	// 2. Clone history
	fmt.Println("\n2️⃣ Cloning history...")
	clonedHistory := history.Clone()
	fmt.Printf("   Original: %d messages, Clone: %d messages\n", history.Len(), clonedHistory.Len())
	fmt.Printf("   ✅ Clone is independent copy of conversation state\n")

	// 3. Get last N messages
	fmt.Println("\n3️⃣ Getting last 2 messages...")
	lastTwo := history.GetLast(2)
	fmt.Printf("   Retrieved %d messages:\n", len(lastTwo))
	for i, msg := range lastTwo {
		fmt.Printf("   [%d] Role: %-9s Content: %s\n", i+1, msg.Role, truncate(msg.Content, 50))
	}

	// 4. Manual message addition
	fmt.Println("\n4️⃣ Adding messages manually...")
	history.AddUserMessage("What is the meaning of life?")
	history.AddAssistantMessage("42")
	fmt.Printf("   History size after manual adds: %d messages\n", history.Len())
	fmt.Printf("   ✅ Can manually add messages without LM calls\n")

	// 5. Clear history
	fmt.Println("\n5️⃣ Clearing history...")
	history.Clear()
	fmt.Printf("   History size after clear: %d messages\n", history.Len())
	fmt.Printf("   ✅ History is empty!\n")

	// 6. Restore from clone
	fmt.Println("\n6️⃣ Restoring from clone...")
	history = clonedHistory
	fmt.Printf("   Restored history size: %d messages\n", history.Len())
	fmt.Printf("   ✅ History restored from clone!\n")

	return totalTokens, lastPred, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
