package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
)

func main() {
	fmt.Println("=== DSGo Program Tracing Demo ===")
	fmt.Println("Pipeline: Task Understanding → Codebase Analysis → Implementation Plan")
	fmt.Println()

	// Check for API key
	if os.Getenv("OPENROUTER_API_KEY") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Println("⚠️  No API key found. Running in demo mode (showing configuration only).")
		demonstrateConfiguration()
		return
	}

	// Run the actual pipeline
	if err := runPipeline(); err != nil {
		fmt.Printf("❌ Pipeline failed: %v\n", err)
		os.Exit(1)
	}
}

func runPipeline() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Initialize LM
	modelName := getModelName()
	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		return fmt.Errorf("failed to create LM: %w", err)
	}
	fmt.Printf("✓ Using model: %s\n\n", modelName)

	// Get filesystem tools for codebase exploration
	fsTools := tools.GetAllFilesystemTools()
	fmt.Printf("✓ Loaded %d filesystem tools for codebase analysis\n", len(fsTools))

	// Create signatures for each step

	// Step 1: Task Understanding with filesystem access to explore the codebase
	taskSig := dsgo.NewSignature(`Understand a task and determine its scope by exploring the codebase.
Use the filesystem tools to:
1. List files at the project root to understand the structure
2. Read key files like README.md, go.mod to understand the project
3. Identify relevant directories and files for the task`).
		AddInput("Task", dsgo.FieldTypeString, "The task to analyze and plan for").
		AddOutput("Understanding", dsgo.FieldTypeString, "Clear understanding of what needs to be done based on codebase exploration").
		AddOutput("Scope", dsgo.FieldTypeString, "Scope and boundaries of the task with specific file/directory references")

	// Step 2: Codebase Analysis with filesystem access for deep exploration
	analysisSig := dsgo.NewSignature(`Analyze the codebase based on task understanding.
Use the filesystem tools to:
1. Navigate to relevant directories identified in the previous step
2. Read source files to understand implementation patterns
3. Search for related code using patterns
Provide concrete findings with file paths and code references.`).
		AddInput("Understanding", dsgo.FieldTypeString, "Understanding of the task from previous step").
		AddInput("Scope", dsgo.FieldTypeString, "Scope from previous step").
		AddOutput("KeyFindings", dsgo.FieldTypeString, "Key findings from analyzing the codebase with specific file references").
		AddOutput("Considerations", dsgo.FieldTypeString, "Important considerations for implementation based on existing code patterns")

	// Step 3: Implementation Plan (simple Predict - synthesizes findings without needing filesystem)
	planSig := dsgo.NewSignature("Create implementation plan based on analysis. No setup and git actions").
		AddInput("Understanding", dsgo.FieldTypeString, "Understanding from task step").
		AddInput("KeyFindings", dsgo.FieldTypeString, "Key findings from analysis step").
		AddInput("Considerations", dsgo.FieldTypeString, "Considerations from analysis step").
		AddOutput("Plan", dsgo.FieldTypeString, "Step-by-step implementation plan with specific file paths").
		AddOutput("Risks", dsgo.FieldTypeString, "Potential risks and mitigations").
		AddOutput("EstimatedTime", dsgo.FieldTypeString, "Estimated time to complete")

	// Create modules: ReAct for task & analysis (with tools), Predict for plan (synthesis only)
	taskModule := dsgo.NewReAct(taskSig, lm, fsTools).
		WithMaxIterations(8).
		WithVerbose(true)
	analysisModule := dsgo.NewReAct(analysisSig, lm, fsTools).
		WithMaxIterations(10).
		WithVerbose(true)
	planModule := dsgo.NewPredict(planSig, lm)

	// Build the program with tracing configuration
	program := dsgo.NewProgram("Task-Analysis-Plan Pipeline").
		WithVerbose(true).
		WithInputs([]string{"Task"}).
		WithExecutionRetention(5).
		AddModule(taskModule).
		AddModule(analysisModule).
		AddModule(planModule)

	fmt.Printf("✓ Created program: %s (modules: %d)\n", program.Name(), program.ModuleCount())

	// Execute with trace
	inputs := map[string]any{
		"Task": "improve parallel.go",
	}

	fmt.Println("\n--- Executing Pipeline ---")
	result, err := program.ForwardWithTrace(ctx, inputs)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	plan, exists := result.Prediction.Get("Plan")
	if !exists {
		return fmt.Errorf("no plan output")
	}
	fmt.Println("result.Prediction - Plan: \n", plan)

	// Display execution trace
	displayExecutionTrace(result)

	// Demonstrate helper APIs
	demonstrateHelperAPIs(program, result.ExecutionID)

	return nil
}

func displayExecutionTrace(result *dsgo.ProgramResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 EXECUTION TRACE")
	fmt.Println(strings.Repeat("=", 60))

	exec := result.Execution
	fmt.Printf("Execution ID: %s\n", exec.ID)
	fmt.Printf("Status: %s\n", exec.Status)
	fmt.Printf("Duration: %v\n", exec.Duration)
	fmt.Printf("Total Tokens: %d\n", exec.TotalUsage.TotalTokens)
	fmt.Printf("Total Cost: $%.6f\n", exec.TotalUsage.Cost)

	fmt.Println("\n--- Steps ---")
	for i, step := range exec.Steps {
		fmt.Printf("\nStep %d: %s\n", i+1, step.ModuleName)
		fmt.Printf("  Status: %s\n", step.Status)
		fmt.Printf("  Duration: %v\n", step.Duration)
		if step.Prediction != nil {
			fmt.Printf("  Output keys: %v\n", getKeys(step.Prediction.Outputs))
		}
		if step.Error != nil {
			fmt.Printf("  Error: %v\n", step.Error)
		}
	}

	// Display metrics
	metrics := exec.Metrics()
	fmt.Println("\n--- Metrics ---")
	fmt.Printf("Total Steps: %d\n", metrics.TotalSteps)
	fmt.Printf("Completed: %d\n", metrics.CompletedSteps)
	fmt.Printf("Slowest Step: %d (%.2fs)\n", metrics.SlowestStepIndex, metrics.SlowestStep.Seconds())
}

func demonstrateHelperAPIs(program *dsgo.Program, execID dsgo.ExecutionID) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🔧 HELPER API DEMONSTRATION")
	fmt.Println(strings.Repeat("=", 60))

	// GetLastExecutionID
	lastID := program.GetLastExecutionID()
	fmt.Printf("\nGetLastExecutionID(): %s\n", lastID)

	// GetAllExecutionIDs
	allIDs := program.GetAllExecutionIDs()
	fmt.Printf("GetAllExecutionIDs(): %d execution(s)\n", len(allIDs))

	// GetStepPrediction - get task understanding from step 0
	pred, err := program.GetStepPrediction(execID, 0)
	if err != nil {
		fmt.Printf("GetStepPrediction error: %v\n", err)
	} else {
		fmt.Printf("\nGetStepPrediction(execID, 0) - Task Understanding:\n")
		if understanding, ok := pred.Outputs["Understanding"]; ok {
			fmt.Printf("  Understanding: %s...\n", understanding)
		}
	}

	// GetStepOutput - get specific output
	plan, err := program.GetStepOutput(execID, 2, "Plan")
	if err != nil {
		fmt.Printf("GetStepOutput error: %v\n", err)
	} else {
		fmt.Printf("\nGetStepOutput(execID, 2, \"Plan\") - Implementation Plan:\n")
		fmt.Printf("  Plan: %s...\n", plan)
	}

	// GetLastStepOutput - convenience method
	risks, err := program.GetLastStepOutput(2, "Risks")
	if err != nil {
		fmt.Printf("GetLastStepOutput error: %v\n", err)
	} else {
		fmt.Printf("\nGetLastStepOutput(2, \"Risks\"):\n")
		fmt.Printf("  Risks: %s...\n", risks)
	}
}

func demonstrateConfiguration() {
	fmt.Println("\n--- Configuration Demo (no LM) ---")

	// Show how to configure the program
	program := dsgo.NewProgram("Demo Pipeline").
		WithVerbose(true).
		WithInputs([]string{"Task"}).
		WithExecutionRetention(5)

	fmt.Printf("✓ Program: %s\n", program.Name())
	fmt.Printf("✓ Module count: %d\n", program.ModuleCount())

	fmt.Println("\nAvailable APIs:")
	fmt.Println("  • ForwardWithTrace(ctx, inputs) - Execute and get trace")
	fmt.Println("  • GetExecution() - Get last execution trace")
	fmt.Println("  • GetExecutionByID(id) - Get execution by ID")
	fmt.Println("  • GetAllExecutionIDs() - List retained executions")
	fmt.Println("  • GetStepPrediction(id, index) - Get step prediction")
	fmt.Println("  • GetStepOutput(id, index, key) - Get specific output")
	fmt.Println("  • GetLastStepPrediction(index) - Convenience method")
	fmt.Println("  • GetLastStepOutput(index, key) - Convenience method")
}

func getModelName() string {
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	return "openrouter/google/gemini-2.0-flash-001"
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
