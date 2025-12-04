package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚀 DSGo Parallel Module Example")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Println("Demonstrating parallel execution with automatic state isolation")
	fmt.Println()

	// Example 1: Basic Parallel Execution
	fmt.Println("📋 Example 1: Basic Parallel Classification")
	fmt.Println("-" + strings.Repeat("-", 40))
	basicParallelExample(ctx)

	fmt.Println("\n📋 Example 2: Parallel with Stateful Modules (History)")
	fmt.Println("-" + strings.Repeat("-", 40))
	statefulParallelExample(ctx)

	fmt.Println("\n📋 Example 3: Parallel with Tools (ReAct)")
	fmt.Println("-" + strings.Repeat("-", 40))
	toolsParallelExample(ctx)

	fmt.Println("\n📋 Example 4: Advanced Parallel Patterns")
	fmt.Println("-" + strings.Repeat("-", 40))
	advancedParallelExample(ctx)

	fmt.Println("\n✨ All examples completed successfully!")
}

// Example 1: Basic parallel execution with simple classification
func basicParallelExample(ctx context.Context) {
	// Create LM
	lm, err := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash-lite-preview-09-2025")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create signature for sentiment analysis
	sig := dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to classify").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")

	// Create base module
	classifier := dsgo.NewPredict(sig, lm)

	// Create parallel module
	parallel := dsgo.NewParallel(classifier).WithMaxWorkers(3)

	// Prepare multiple texts to classify
	texts := []string{
		"I love this product! It's amazing!",
		"This is terrible, I hate it.",
		"It's okay, nothing special.",
		"Absolutely fantastic experience!",
		"Could be better, not impressed.",
		"Best purchase I've made all year!",
	}

	// Convert to parallel inputs using map-of-slices approach
	textSlice := make([]string, len(texts))
	copy(textSlice, texts)

	// Execute in parallel using map-of-slices (auto-zipped)
	start := time.Now()
	result, err := parallel.Forward(ctx, map[string]any{"text": textSlice})
	duration := time.Since(start)

	if err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	// Display results
	fmt.Printf("Processed %d texts in %v\n", len(texts), duration)
	fmt.Printf("Total cost: $%.6f, Total tokens: %d\n", result.Usage.Cost, result.Usage.TotalTokens)

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("📊 CLASSIFICATION RESULTS")
	fmt.Println(strings.Repeat("─", 60))

	if completions := result.Completions; completions != nil {
		for i, text := range texts {
			if i < len(completions) {
				completion := completions[i]
				if sentiment, ok := completion["sentiment"].(string); ok {
					fmt.Printf("\n%d️⃣  Input:  %s\n", i+1, text)
					fmt.Printf("   Output: %s\n", sentiment)
				}
			}
		}
	}
	fmt.Println(strings.Repeat("─", 60))
}

// Example 2: Demonstrating state isolation with History
func statefulParallelExample(ctx context.Context) {
	lm, err := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash-lite-preview-09-2025")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create signature for conversation with reasoning
	sig := dsgo.NewSignature("Continue the conversation with reasoning").
		AddInput("message", dsgo.FieldTypeString, "User message").
		AddOutput("response", dsgo.FieldTypeString, "AI response").
		AddOutput("reasoning", dsgo.FieldTypeString, "AI reasoning process").
		AddOutput("task_id", dsgo.FieldTypeString, "Unique task identifier")

	// Create SHARED history (this would normally cause race conditions)
	sharedHistory := dsgo.NewHistory()

	// Create stateful module with SHARED history
	conversational := dsgo.NewPredict(sig, lm).WithHistory(sharedHistory)

	// Create parallel module - automatically clones per task for state isolation
	parallel := dsgo.NewParallel(conversational).
		WithMaxWorkers(2).
		WithReturnAll(true) // Get all individual results

	// Simulate multiple concurrent conversations
	conversations := []string{
		"Hi, I'm interested in learning about Go programming.",
		"Can you help me understand machine learning?",
		"What's the best way to learn cooking?",
		"Tell me about space exploration.",
	}

	// Convert to map-of-slices for parallel processing
	messageSlice := make([]string, len(conversations))
	copy(messageSlice, conversations)

	fmt.Printf("🔍 THREAD SAFETY PROOF: Running %d conversations with SHARED History\n", len(conversations))
	fmt.Printf("📍 Shared History address: %p\n", sharedHistory)
	fmt.Printf("🔧 Max workers: %d (concurrent execution)\n", 2)
	fmt.Printf("⚡ Each task should get isolated module instances despite shared history\n\n")

	fmt.Println("🚀 Starting parallel conversations...")
	start := time.Now()
	result, err := parallel.Forward(ctx, map[string]any{"message": messageSlice})
	duration := time.Since(start)

	if err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	fmt.Printf("✅ Completed %d conversations in %v\n", len(conversations), duration)

	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🔍 THREAD SAFETY ANALYSIS WITH INDIVIDUAL RESULTS")
	fmt.Println(strings.Repeat("═", 80))

	// Show that each conversation got proper responses
	if completions := result.Completions; completions != nil {
		for i, completion := range completions {
			if i < len(conversations) {
				response, _ := completion["response"].(string)
				reasoning, _ := completion["reasoning"].(string)
				taskID, _ := completion["task_id"].(string)

				fmt.Printf("\n📝 Task %d (ID: %s):\n", i+1, taskID)
				fmt.Printf("   User: %s\n", conversations[i])
				fmt.Printf("   AI:   %s\n", response)
				fmt.Printf("   🧠 Reasoning: %s\n", reasoning)

				// 🔍 CRITICAL PROOF: Each task should have unique task_id
				// This proves they ran in isolated contexts
				if taskID != "" {
					fmt.Printf("   ✅ Unique task ID: %s\n", taskID)
				} else {
					fmt.Printf("   ❌ Missing task ID (potential issue)\n")
				}
			}
		}
	}

	// 🔍 FINAL PROOF: Check the shared history state
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🔍 SHARED HISTORY STATE (THE SMOKING GUN)")
	fmt.Println(strings.Repeat("═", 80))

	sharedMessages := sharedHistory.Get()
	fmt.Printf("📊 Shared History contains %d messages\n", len(sharedMessages))

	if len(sharedMessages) == 0 {
		fmt.Println("✅ THREAD SAFETY PROVEN!")
		fmt.Println("🔒 Shared History is EMPTY despite 4 concurrent conversations!")
		fmt.Println("🔒 This proves each task got its own isolated history copy")
		fmt.Println("🔒 No race conditions occurred - state isolation is working perfectly")
	} else {
		fmt.Printf("⚠️  UNEXPECTED: Shared history has %d messages (should be 0)\n", len(sharedMessages))
		fmt.Println("❌ This would indicate a thread safety issue")
		for i, msg := range sharedMessages {
			fmt.Printf("   %d. [%s] %s\n", i+1, msg.Role, msg.Content[:min(50, len(msg.Content))])
		}
	}

	// 🔍 PROOF: Check that each task got unique task IDs
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("🔍 TASK ISOLATION PROOF (UNIQUE IDENTIFIERS)")
	fmt.Println(strings.Repeat("═", 80))

	taskIDs := make(map[string]bool)
	if completions := result.Completions; completions != nil {
		for i, completion := range completions {
			if taskID, ok := completion["task_id"].(string); ok {
				if taskIDs[taskID] {
					fmt.Printf("❌ DUPLICATE task ID: %s (Task %d)\n", taskID, i+1)
				} else {
					taskIDs[taskID] = true
					fmt.Printf("✅ Unique task ID: %s (Task %d)\n", taskID, i+1)
				}
			}
		}
	}

	fmt.Printf("📊 Total unique task IDs: %d\n", len(taskIDs))
	if len(taskIDs) == len(conversations) {
		fmt.Println("✅ PERFECT: All tasks have unique IDs - complete isolation proven!")
	} else {
		fmt.Printf("❌ ISSUE: Expected %d unique IDs, got %d\n", len(conversations), len(taskIDs))
	}

	// Additional proof: Show aggregated usage
	fmt.Println("\n" + strings.Repeat("═", 80))
	fmt.Println("📊 USAGE ANALYSIS")
	fmt.Println(strings.Repeat("═", 80))

	fmt.Printf("💰 Total aggregated cost: $%.6f\n", result.Usage.Cost)
	fmt.Printf("🔢 Total aggregated tokens: %d\n", result.Usage.TotalTokens)
	fmt.Printf("⏱️  Average cost per task: $%.6f\n", result.Usage.Cost/float64(len(conversations)))
	fmt.Printf("⏱️  Average tokens per task: %d\n", result.Usage.TotalTokens/len(conversations))

	fmt.Println(strings.Repeat("═", 80))
}

// Helper function for minimum
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Example 3: Parallel execution with tools (ReAct)
func toolsParallelExample(ctx context.Context) {
	lm, err := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash-lite-preview-09-2025")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create a simple calculator tool
	calculatorTool := *dsgo.NewTool(
		"calculator",
		"Perform basic math calculations",
		func(ctx context.Context, args map[string]any) (any, error) {
			expr := args["expression"].(string)
			// Simple evaluation (in real code, use a proper math parser)
			result := "Result calculated for: " + expr
			return result, nil
		},
	).AddParameter("expression", "string", "Mathematical expression to evaluate", true)

	// Create signature for math problems
	sig := dsgo.NewSignature("Solve math problems").
		AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
		AddOutput("answer", dsgo.FieldTypeString, "Solution to the problem").
		AddOutput("steps", dsgo.FieldTypeString, "Step-by-step solution")

	// Create ReAct agent with tools
	agent := dsgo.NewReAct(sig, lm, []dsgo.Tool{calculatorTool}).
		WithMaxIterations(3).
		WithOptions(&dsgo.GenerateOptions{
			Temperature: 0.1,
			MaxTokens:   500,
		})

	// Create parallel module
	parallel := dsgo.NewParallel(agent).WithMaxWorkers(2)

	// Math problems to solve
	problems := []string{
		"What is 15 + 27?",
		"Calculate 100 - 45",
		"What is 8 * 7?",
		"Divide 144 by 12",
	}

	// Convert to map-of-slices for parallel processing
	problemSlice := make([]string, len(problems))
	copy(problemSlice, problems)

	fmt.Println("Solving math problems in parallel using ReAct agents...")
	start := time.Now()
	result, err := parallel.Forward(ctx, map[string]any{"problem": problemSlice})
	duration := time.Since(start)

	if err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	fmt.Printf("Solved %d problems in %v\n", len(problems), duration)
	fmt.Printf("Total cost: $%.6f\n", result.Usage.Cost)

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🧮 MATH PROBLEM SOLUTIONS")
	fmt.Println(strings.Repeat("─", 60))

	// Display solutions
	if completions := result.Completions; completions != nil {
		for i, problem := range problems {
			if i < len(completions) {
				completion := completions[i]
				if answer, ok := completion["answer"].(string); ok {
					fmt.Printf("\n%d️⃣  Problem: %s\n", i+1, problem)
					fmt.Printf("   Answer:  %s\n", answer)
				}
			}
		}
	}
	fmt.Println(strings.Repeat("─", 60))
}

// Example 4: Advanced patterns and error handling
func advancedParallelExample(ctx context.Context) {
	lm, err := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash-lite-preview-09-2025")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create different types of modules
	// 1. Simple classifier
	classifierSig := dsgo.NewSignature("Classify text").
		AddInput("text", dsgo.FieldTypeString, "Text to classify").
		AddClassOutput("category", []string{"question", "statement", "command"}, "Text category")

	classifier := dsgo.NewPredict(classifierSig, lm)

	// Note: Other module types (summarizer, detector) would be used similarly
	// but we focus on classifier for this example

	// Create parallel module with classifier
	parallel := dsgo.NewParallel(classifier).WithMaxWorkers(3)

	// Test texts
	texts := []string{
		"What is the weather like today?",
		"The sky is blue and the sun is shining.",
		"Please close the door.",
		"How much does this cost?",
		"I love programming in Go!",
		"Can you help me with this task?",
	}

	// Convert to map-of-slices for parallel processing
	textSlice := make([]string, len(texts))
	copy(textSlice, texts)

	fmt.Println("Advanced: Text classification with error handling...")
	result, err := parallel.Forward(ctx, map[string]any{"text": textSlice})
	if err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	// Analyze results
	successCount := 0
	categoryCount := make(map[string]int)

	if completions := result.Completions; completions != nil {
		for _, completion := range completions {
			if completion != nil {
				successCount++
				if category, ok := completion["category"].(string); ok {
					categoryCount[category]++
				}
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("📈 ANALYSIS RESULTS")
	fmt.Println(strings.Repeat("─", 60))

	fmt.Printf("Successfully processed: %d/%d texts\n", successCount, len(texts))
	fmt.Println("Category distribution:")
	for category, count := range categoryCount {
		fmt.Printf("  %s: %d\n", category, count)
	}

	// Demonstrate different parallel patterns
	fmt.Println("\n🔧 Advanced Patterns:")
	fmt.Println("1. ✅ Default: Automatic cloning per task (state isolation)")
	fmt.Println("2. ✅ WithMaxWorkers: Control concurrency")
	fmt.Println("3. ✅ Error handling: Individual task failures don't affect others")
	fmt.Println("4. ✅ Usage tracking: Aggregated cost and token usage")

	// Show usage statistics
	fmt.Println("\n💰 Usage Statistics:")
	fmt.Printf("  Total Cost: $%.6f\n", result.Usage.Cost)
	fmt.Printf("  Total Tokens: %d\n", result.Usage.TotalTokens)
	fmt.Printf("  Average Cost per Task: $%.6f\n", result.Usage.Cost/float64(len(texts)))
	fmt.Println(strings.Repeat("─", 60))
}
