package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
)

const DefaultModel = "openrouter/google/gemini-2.5-flash-lite-preview-09-2025"

func getModel() string {
	if model := os.Getenv("DSGO_MODEL"); model != "" {
		return model
	}
	return DefaultModel
}

func main() {
	ctx := context.Background()

	model := getModel()
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("failed to init LM: %v", err)
	}

	// Get filesystem tools for agents that need codebase access
	fsTools := tools.GetAllFilesystemTools()

	// Stage 1: IdeaToObjective (no filesystem needed)
	ideaSig := dsgo.NewSignature("Convert a raw idea into clear objectives").
		AddInput("idea", dsgo.FieldTypeString, "Raw product or feature idea").
		AddOutput("objectives", dsgo.FieldTypeString, "Clear, measurable objectives")
	ideaModule := dsgo.NewPredict(ideaSig, lm)

	// Stage 2: ObjectiveToPlan (ReAct with filesystem - explores codebase to ground the plan)
	planSig := dsgo.NewSignature(`You MUST follow these steps IN ORDER before outputting a plan:
Step 1: Use list_files to explore the project structure (start at root, then drill into relevant directories)
Step 2: Use read_file to examine key files that are relevant to the objectives
Step 3: Use search_files if needed to find specific file types or patterns
Step 4: Only AFTER completing steps 1-3, synthesize your findings into a concrete implementation plan`).
		AddInput("objectives", dsgo.FieldTypeString, "Clear objectives").
		AddOutput("plan", dsgo.FieldTypeString, "Implementation plan with SPECIFIC file paths, function names, and code patterns you discovered. Must reference actual files you read - no generic steps like 'explore' or 'analyze'")
	planModule := dsgo.NewReAct(planSig, lm, fsTools).WithMaxIterations(8)

	// Stage 3: PlanToImpl (ReAct with filesystem - reads existing code)
	implSig := dsgo.NewSignature("Generate implementation from plan. Use filesystem tools to explore the codebase structure and existing patterns before implementing.").
		AddInput("plan", dsgo.FieldTypeString, "Implementation plan").
		AddOutput("implementation", dsgo.FieldTypeString, "Code implementation following existing patterns")
	implModule := dsgo.NewReAct(implSig, lm, fsTools).WithMaxIterations(5)

	// Stage 4: TestImpl (ReAct with filesystem - reads implementation)
	testSig := dsgo.NewSignature("Generate tests for implementation. Use filesystem tools to read the implementation code and understand existing test patterns.").
		AddInput("implementation", dsgo.FieldTypeString, "Code implementation").
		AddOutput("tests", dsgo.FieldTypeString, "Test cases for the implementation")
	testModule := dsgo.NewReAct(testSig, lm, fsTools).WithMaxIterations(5)

	// Stage 5: ReviewImpl (ReAct with filesystem - reads implementation and tests)
	reviewSig := dsgo.NewSignature("Review implementation and tests. Use filesystem tools to check code against existing conventions and patterns in the codebase.").
		AddInput("implementation", dsgo.FieldTypeString, "Code implementation").
		AddInput("tests", dsgo.FieldTypeString, "Test cases").
		AddOutput("review", dsgo.FieldTypeString, "Code review with suggestions")
	reviewModule := dsgo.NewReAct(reviewSig, lm, fsTools).WithMaxIterations(5)

	// Connect all stages serially
	pipeline := dsgo.NewProgram("SDLC Pipeline").
		AddModule(ideaModule).
		AddModule(planModule).
		AddModule(implModule).
		AddModule(testModule).
		AddModule(reviewModule)

	// Get user input
	fmt.Print("Enter your idea: ")
	reader := bufio.NewReader(os.Stdin)
	idea, _ := reader.ReadString('\n')
	idea = strings.TrimSpace(idea)

	if idea == "" {
		log.Fatal("idea cannot be empty")
	}

	log.Printf("[START] Processing idea: %s", idea)

	// Run pipeline
	result, err := pipeline.Forward(ctx, map[string]any{"idea": idea})
	if err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}

	// Extract outputs
	objectives, _ := result.GetString("objectives")
	plan, _ := result.GetString("plan")
	implementation, _ := result.GetString("implementation")
	tests, _ := result.GetString("tests")
	review, _ := result.GetString("review")

	// Log critical outputs
	log.Printf("[STAGE 1] Objectives: %s", truncate(objectives, 100))
	log.Printf("[STAGE 2] Plan: %s", truncate(plan, 100))
	log.Printf("[STAGE 3] Implementation generated")
	log.Printf("[STAGE 4] Tests generated")
	log.Printf("[STAGE 5] Review complete")

	// Print final results as structured markdown
	fmt.Println("\n# SDLC Pipeline Results")
	fmt.Printf("\n> **Idea:** %s\n", idea)

	fmt.Println("\n---\n")
	fmt.Println("## 📋 Objectives")
	fmt.Println()
	fmt.Println(objectives)

	fmt.Println("\n---\n")
	fmt.Println("## 📝 Implementation Plan")
	fmt.Println()
	fmt.Println(plan)

	fmt.Println("\n---\n")
	fmt.Println("## 💻 Implementation")
	fmt.Println()
	fmt.Println("```go")
	fmt.Println(implementation)
	fmt.Println("```")

	fmt.Println("\n---\n")
	fmt.Println("## 🧪 Tests")
	fmt.Println()
	fmt.Println("```go")
	fmt.Println(tests)
	fmt.Println("```")

	fmt.Println("\n---\n")
	fmt.Println("## ✅ Code Review")
	fmt.Println()
	fmt.Println(review)

	fmt.Println("\n---\n")
	fmt.Println("## 📊 Metrics")
	fmt.Println()
	fmt.Printf("| Metric | Value |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Total Tokens | %d |\n", result.Usage.TotalTokens)
	fmt.Printf("| Cost | $%.4f |\n", result.Usage.Cost)

	log.Printf("[DONE] Total tokens: %d, Cost: $%.4f", result.Usage.TotalTokens, result.Usage.Cost)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
