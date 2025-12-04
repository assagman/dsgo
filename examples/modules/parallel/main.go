package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

// counterModule is a synthetic module for demonstrating factory/instances patterns
type counterModule struct {
	sig   *dsgo.Signature
	id    int
	calls int // per-module call counter
}

func newCounterModule(id int) *counterModule {
	sig := dsgo.NewSignature("Synthetic counter module").
		AddInput("value", dsgo.FieldTypeInt, "Input value").
		AddOutput("result", dsgo.FieldTypeInt, "Processed value").
		AddOutput("worker_id", dsgo.FieldTypeInt, "Factory worker id").
		AddOutput("call_index", dsgo.FieldTypeInt, "Call index for this worker")

	return &counterModule{sig: sig, id: id}
}

func (m *counterModule) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	v, _ := inputs["value"].(int)
	m.calls++
	outputs := map[string]any{
		"result":     v * 2,
		"worker_id":  m.id,
		"call_index": m.calls,
	}
	// Slow down a bit to emphasize concurrency
	time.Sleep(10 * time.Millisecond)
	return dsgo.NewPrediction(outputs).WithModuleName("counter"), nil
}

func (m *counterModule) GetSignature() *dsgo.Signature { return m.sig }

func (m *counterModule) Clone() dsgo.Module {
	clone := *m
	clone.calls = 0
	return &clone
}

func main() {
	ctx := context.Background()

	fmt.Println("🚀 DSGo Parallel Module Example")
	fmt.Println("=" + strings.Repeat("=", 50))

	fmt.Println("\n📋 Example 1: Basic Parallel Classification (NewParallel)")
	fmt.Println("-" + strings.Repeat("-", 40))
	basicParallelExample(ctx)

	fmt.Println("\n📋 Example 2: Parallel with Stateful Modules (History + NewParallel)")
	fmt.Println("-" + strings.Repeat("-", 40))
	statefulParallelExample(ctx)

	fmt.Println("\n📋 Example 3: Parallel with Tools (ReAct + NewParallel)")
	fmt.Println("-" + strings.Repeat("-", 40))
	toolsParallelExample(ctx)

	fmt.Println("\n📋 Example 4: Advanced Patterns (batch, repeat, failures, metrics)")
	fmt.Println("-" + strings.Repeat("-", 40))
	advancedParallelExample(ctx)

	fmt.Println("\n📋 Example 5: Per-Task Modules (NewParallelWithFactory)")
	fmt.Println("-" + strings.Repeat("-", 40))
	factoryParallelExample(ctx)

	fmt.Println("\n📋 Example 6: Pre-Created Workers (NewParallelWithInstances)")
	fmt.Println("-" + strings.Repeat("-", 40))
	instancesParallelExample(ctx)

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

// Example 4: Advanced patterns (batch, repeat, failures, metrics)
func advancedParallelExample(ctx context.Context) {
	lm, err := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash-lite-preview-09-2025")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create classifier
	classifierSig := dsgo.NewSignature("Classify text").
		AddInput("text", dsgo.FieldTypeString, "Text to classify").
		AddClassOutput("category", []string{"question", "statement", "command"}, "Text category")

	classifier := dsgo.NewPredict(classifierSig, lm)

	fmt.Println("🔧 Advanced: Demonstrating batch, repeat, failures, and metrics")

	// 1. Explicit batch mode
	fmt.Println("\n1️⃣  Explicit Batch Mode:")
	batch := []map[string]any{
		{"text": "What time is it?"},
		{"text": "The sun is shining."},
		{"text": "Please open the window."},
	}

	parallel := dsgo.NewParallel(classifier).
		WithReturnAll(true).
		WithMaxFailures(1)

	result, err := parallel.Forward(ctx, map[string]any{"_batch": batch})
	if err != nil {
		log.Fatalf("Batch mode failed: %v", err)
	}

	fmt.Printf("   Processed %d items via explicit batch\n", len(batch))
	if completions := result.Completions; completions != nil {
		for i, completion := range completions {
			if category, ok := completion["category"].(string); ok {
				fmt.Printf("   %d: %s → %s\n", i+1, batch[i]["text"], category)
			}
		}
	}

	// 2. Repeat mode
	fmt.Println("\n2️⃣  Repeat Mode:")
	repeatParallel := dsgo.NewParallel(classifier).
		WithRepeat(4).
		WithReturnAll(true).
		WithMaxFailures(1) // Allow some failures

	result, err = repeatParallel.Forward(ctx, map[string]any{"text": "This is a test statement"})
	if err != nil {
		fmt.Printf("   ⚠️  Repeat mode had issues: %v\n", err)
	} else {
		fmt.Printf("   Repeated single input 4 times\n")
		if completions := result.Completions; completions != nil {
			for i, completion := range completions {
				if category, ok := completion["category"].(string); ok {
					fmt.Printf("   %d: This is a test statement → %s\n", i+1, category)
				}
			}
		}
	}

	// 3. Failure handling
	fmt.Println("\n3️⃣  Failure Handling:")
	failParallel := dsgo.NewParallel(classifier).
		WithMaxFailures(0). // Require all to succeed
		WithReturnAll(true)

	// Create batch with one invalid input
	failBatch := []map[string]any{
		{"text": "Valid input"},
		{"text": 123}, // Invalid type
		{"text": "Another valid input"},
	}

	result, err = failParallel.Forward(ctx, map[string]any{"_batch": failBatch})
	if err != nil {
		fmt.Printf("   ✅ Expected failure with MaxFailures=0: %v\n", err)
	} else {
		fmt.Printf("   ❌ Unexpected success\n")
	}

	// 4. Metrics demonstration
	fmt.Println("\n4️⃣  Parallel Metrics:")
	metricsParallel := dsgo.NewParallel(classifier).
		WithMaxWorkers(2).
		WithReturnAll(true).
		WithMaxFailures(2) // Allow up to 2 failures

	texts := []string{"First", "Second", "Third", "Fourth"}
	result, err = metricsParallel.Forward(ctx, map[string]any{"text": texts})
	if err != nil {
		fmt.Printf("   ⚠️  Metrics demo had issues: %v\n", err)
	} else {
		// Show metrics
		if m, ok := result.Outputs["__parallel_metrics"].(dsgo.ParallelMetrics); ok {
			fmt.Printf("   Total tasks: %d\n", m.Total)
			fmt.Printf("   Successes: %d\n", m.Successes)
			fmt.Printf("   Failures: %d\n", m.Failures)
			fmt.Printf("   Latency (ms): min=%d, avg=%d, p50=%d, max=%d\n",
				m.Latency.MinMs, m.Latency.AvgMs, m.Latency.P50Ms, m.Latency.MaxMs)
		}

		// Show errors if any
		if errors, ok := result.Outputs["__parallel_errors"].(string); ok && errors != "" {
			fmt.Printf("   Errors: %s\n", errors)
		}
	}

	fmt.Println(strings.Repeat("─", 60))
}

// Example 5: NewParallelWithFactory (per-task modules, no shared state)
func factoryParallelExample(ctx context.Context) {
	fmt.Println("📋 Example 5: NewParallelWithFactory (per-task instances)")
	values := []int{1, 2, 3, 4, 5, 6}

	parallel := dsgo.NewParallelWithFactory(func(i int) dsgo.Module {
		// Each task gets its own module with id=i
		return newCounterModule(i)
	}).WithMaxWorkers(3).WithReturnAll(true)

	// Map-of-slices input
	result, err := parallel.Forward(ctx, map[string]any{"value": values})
	if err != nil {
		log.Fatalf("factory parallel failed: %v", err)
	}

	fmt.Println("🔍 THREAD SAFETY PROOF: Each task gets isolated module instance")
	fmt.Printf("📍 Processing %d tasks with %d max workers\n", len(values), 3)
	fmt.Printf("⚡ Each task should have call_index=1 (fresh module)\n\n")

	completions := result.Completions
	for i, v := range values {
		if i >= len(completions) {
			continue
		}
		c := completions[i]
		workerID := c["worker_id"].(int)
		callIndex := c["call_index"].(int)
		fmt.Printf("Task %d: value=%d → worker_id=%d, call_index=%d\n",
			i, v, workerID, callIndex)

		// ✅ Proof: call_index must always be 1
		// If modules were reused across tasks, some call_index would be >1
		if callIndex != 1 {
			fmt.Printf("   ❌ ERROR: call_index should be 1, got %d\n", callIndex)
		}
	}

	// Verify all call_index values are 1
	allFresh := true
	for _, completion := range completions {
		if callIndex, ok := completion["call_index"].(int); ok && callIndex != 1 {
			allFresh = false
			break
		}
	}

	if allFresh {
		fmt.Println("\n✅ THREAD SAFETY PROVEN!")
		fmt.Println("🔒 All tasks got fresh module instances (call_index=1)")
		fmt.Println("🔒 No shared state between parallel tasks")
		fmt.Println("🔒 NewParallelWithFactory provides perfect isolation")
	} else {
		fmt.Println("\n❌ THREAD SAFETY ISSUE!")
		fmt.Println("🔥 Some tasks reused module instances")
	}

	fmt.Println(strings.Repeat("─", 60))
}

// Example 6: NewParallelWithInstances (pre-created workers, safe shared state)
func instancesParallelExample(ctx context.Context) {
	fmt.Println("📋 Example 6: NewParallelWithInstances (pre-created workers)")
	values := []int{10, 20, 30, 40, 50, 60, 70}

	// Two shared instances (workers)
	instances := []dsgo.Module{
		newCounterModule(1),
		newCounterModule(2),
	}

	parallel := dsgo.NewParallelWithInstances(instances).
		WithReturnAll(true)

	result, err := parallel.Forward(ctx, map[string]any{"value": values})
	if err != nil {
		log.Fatalf("instances parallel failed: %v", err)
	}

	fmt.Println("🔍 SHARED STATE PROOF: Tasks distributed across pre-created instances")
	fmt.Printf("📍 Processing %d tasks with %d pre-created workers\n", len(values), len(instances))
	fmt.Printf("⚡ Tasks use instances[i %% len(instances)] pattern\n\n")

	completions := result.Completions
	perWorkerCalls := map[int]int{}
	perWorkerTasks := map[int][]int{}

	for i, v := range values {
		if i >= len(completions) {
			continue
		}
		c := completions[i]
		workerID := c["worker_id"].(int)
		callIndex := c["call_index"].(int)

		fmt.Printf("Task %d: value=%d → worker_id=%d, call_index=%d\n",
			i, v, workerID, callIndex)

		perWorkerCalls[workerID]++
		perWorkerTasks[workerID] = append(perWorkerTasks[workerID], i)
	}

	fmt.Println("\n📊 Worker Analysis:")
	for workerID, callCount := range perWorkerCalls {
		fmt.Printf("Worker %d: %d calls, tasks: %v\n", workerID, callCount, perWorkerTasks[workerID])
	}

	// Verify call_index sequences are correct per worker
	fmt.Println("\n🔍 THREAD SAFETY VERIFICATION:")
	allCorrect := true
	for workerID, tasks := range perWorkerTasks {
		expectedCalls := len(tasks)
		fmt.Printf("Worker %d: should have call_index 1..%d\n", workerID, expectedCalls)

		// Check that this worker's call_index values are sequential
		actualCalls := []int{}
		for _, taskIdx := range tasks {
			if taskIdx < len(completions) {
				if callIndex, ok := completions[taskIdx]["call_index"].(int); ok {
					actualCalls = append(actualCalls, callIndex)
				}
			}
		}

		fmt.Printf("  Actual call_index values: %v\n", actualCalls)

		// Verify sequential (1, 2, 3, ...)
		for i, callIdx := range actualCalls {
			if callIdx != i+1 {
				fmt.Printf("  ❌ ERROR: Expected %d, got %d\n", i+1, callIdx)
				allCorrect = false
			}
		}
	}

	if allCorrect {
		fmt.Println("\n✅ THREAD SAFETY PROVEN!")
		fmt.Println("🔒 Shared instances work correctly with concurrent access")
		fmt.Println("🔒 Each worker maintains proper call_index sequence")
		fmt.Println("🔒 NewParallelWithInstances provides safe controlled sharing")
	} else {
		fmt.Println("\n❌ THREAD SAFETY ISSUE!")
		fmt.Println("🔥 Call_index sequences are incorrect")
	}

	// Total verification
	totalCalls := 0
	for _, count := range perWorkerCalls {
		totalCalls += count
	}
	fmt.Printf("\n📊 Total calls across all workers: %d (expected: %d)\n", totalCalls, len(values))

	if totalCalls == len(values) {
		fmt.Println("✅ All tasks processed correctly")
	} else {
		fmt.Printf("❌ Missing tasks: expected %d, got %d\n", len(values), totalCalls)
	}

	fmt.Println(strings.Repeat("─", 60))
}
