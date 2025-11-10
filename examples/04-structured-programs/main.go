package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// Demonstrates: Program, ProgramOfThought, JSON adapter, Typed signatures
// Story: Code implementation and testing pipeline - generate, test, and refine code

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(2)
	}
	envFilePath := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "examples", ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// If not found in examples/, check cwd/.env.local
	if envFilePath == "" {
		candidate := filepath.Join(cwd, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
		}
	}
	if envFilePath == "" {
		fmt.Printf("Could not find .env.local file\n")
		os.Exit(3)
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(3)
	}

	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "itinerary_planner", map[string]interface{}{
		"scenario": "structured_pipeline",
	})
	defer runSpan.End(nil)

	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("failed to create LM: %v", err)
	}

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	// User request
	userRequest := "Write a Python function to implement binary search on a sorted array. Test it with [1, 3, 5, 7, 9] searching for 5 and 2."
	fmt.Printf("User: %s\n", userRequest)
	fmt.Println(strings.Repeat("=", 80))

	// Step 1: Program of Thought - Generate code implementation
	fmt.Println("\n=== Step 1: Generate Code Implementation (ProgramOfThought) ===")
	step1Ctx, step1Span := observe.Start(ctx, observe.SpanKindModule, "step1_code_generation", map[string]interface{}{
		"module": "program_of_thought",
	})

	potSig := dsgo.NewSignature("Generate Python code for binary search algorithm with test cases").
		AddInput("problem", dsgo.FieldTypeString, "Programming problem description").
		AddInput("test_cases", dsgo.FieldTypeJSON, "Test cases to include in the code").
		AddOutput("code", dsgo.FieldTypeString, "Complete Python function with test calls").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation of the algorithm")

	pot := module.NewProgramOfThought(potSig, lm, "python").
		WithAllowExecution(true). // Enable code execution
		WithExecutionTimeout(10)  // 10 second safety timeout

	// Increase max tokens for code generation (minimax-m2 needs more)
	pot.Options.MaxTokens = 20000

	planResult, err := pot.Forward(step1Ctx, map[string]interface{}{
		"problem":    "Write a function binary_search that takes a sorted array and a target value, and returns the index of the target if found, or -1 if not found.",
		"test_cases": [][]interface{}{{[]int{1, 3, 5, 7, 9}, 5}, {[]int{1, 3, 5, 7, 9}, 2}},
	})
	if err != nil {
		log.Fatal(err)
	}

	code, _ := planResult.GetString("code")
	explanation, _ := planResult.GetString("explanation")
	fmt.Printf("Generated code:\n%s\n\nExplanation: %s\n", code, explanation)

	// Show execution result if available
	if execResult, ok := planResult.GetString("execution_result"); ok && strings.TrimSpace(execResult) != "" {
		fmt.Printf("\n✓ Code executed successfully:\n%s\n", strings.TrimSpace(execResult))
	} else if execErr, ok := planResult.GetString("execution_error"); ok {
		fmt.Printf("\n✗ Execution failed: %s\n", execErr)
	} else if pot.AllowExecution {
		fmt.Printf("\n✓ Code executed successfully (no output)\n")
	}

	usage1 := planResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage1.PromptTokens, usage1.CompletionTokens)
	totalPromptTokens += usage1.PromptTokens
	totalCompletionTokens += usage1.CompletionTokens
	step1Span.End(nil)
	fmt.Println(strings.Repeat("-", 80))

	// Step 2: Extract test cases (Predict with JSON)
	fmt.Println("\n=== Step 2: Extract Test Cases (JSON Output) ===")
	step2Ctx, step2Span := observe.Start(ctx, observe.SpanKindModule, "step2_test_cases", map[string]interface{}{
		"module":  "predict",
		"adapter": "json",
	})

	testCasesSig := dsgo.NewSignature("Extract test cases from coding request").
		AddInput("request", dsgo.FieldTypeString, "User coding request").
		AddOutput("language", dsgo.FieldTypeString, "Programming language").
		AddOutput("test_inputs", dsgo.FieldTypeJSON, "List of test inputs").
		AddOutput("expected_outputs", dsgo.FieldTypeJSON, "List of expected outputs")

	extractPredict := module.NewPredict(testCasesSig, lm)

	constraintsResult, err := extractPredict.Forward(step2Ctx, map[string]interface{}{
		"request": userRequest,
	})
	if err != nil {
		log.Fatal(err)
	}

	language, _ := constraintsResult.GetString("language")
	testInputs, _ := constraintsResult.Get("test_inputs")
	expectedOutputs, _ := constraintsResult.Get("expected_outputs")

	fmt.Printf("Test Cases:\n")
	fmt.Printf("  Language: %s\n", language)

	// Pretty print test inputs
	if testInputsStr, ok := testInputs.(string); ok {
		var testInputsParsed interface{}
		if err := json.Unmarshal([]byte(testInputsStr), &testInputsParsed); err == nil {
			testInputsJSON, _ := json.MarshalIndent(testInputsParsed, "    ", "  ")
			fmt.Printf("  Test Inputs:\n%s\n", string(testInputsJSON))
		} else {
			fmt.Printf("  Test Inputs: %s\n", testInputsStr)
		}
	} else {
		testInputsJSON, _ := json.MarshalIndent(testInputs, "    ", "  ")
		fmt.Printf("  Test Inputs:\n%s\n", string(testInputsJSON))
	}

	// Pretty print expected outputs
	if expectedOutputsStr, ok := expectedOutputs.(string); ok {
		var expectedOutputsParsed interface{}
		if err := json.Unmarshal([]byte(expectedOutputsStr), &expectedOutputsParsed); err == nil {
			expectedOutputsJSON, _ := json.MarshalIndent(expectedOutputsParsed, "    ", "  ")
			fmt.Printf("  Expected Outputs:\n%s\n", string(expectedOutputsJSON))
		} else {
			fmt.Printf("  Expected Outputs: %s\n", expectedOutputsStr)
		}
	} else {
		expectedOutputsJSON, _ := json.MarshalIndent(expectedOutputs, "    ", "  ")
		fmt.Printf("  Expected Outputs:\n%s\n", string(expectedOutputsJSON))
	}
	usage2 := constraintsResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage2.PromptTokens, usage2.CompletionTokens)
	totalPromptTokens += usage2.PromptTokens
	totalCompletionTokens += usage2.CompletionTokens
	step2Span.End(nil)
	fmt.Println(strings.Repeat("-", 80))

	// Step 3: Build Program - Testing pipeline
	fmt.Println("\n=== Step 3: Execute Testing Pipeline (Program) ===")
	step3Ctx, step3Span := observe.Start(ctx, observe.SpanKindProgram, "step3_testing_pipeline", map[string]interface{}{
		"steps": 3,
	})

	// Sub-step 3a: Generate test runner code
	testRunnerSig := dsgo.NewSignature("Generate test runner code").
		AddInput("language", dsgo.FieldTypeString, "Programming language").
		AddInput("code", dsgo.FieldTypeString, "Function code to test").
		AddInput("test_inputs", dsgo.FieldTypeJSON, "Test input arrays").
		AddInput("expected_outputs", dsgo.FieldTypeJSON, "Expected outputs").
		AddOutput("test_code", dsgo.FieldTypeString, "Complete test runner code")

	testRunnerPredict := module.NewPredict(testRunnerSig, lm)

	// Sub-step 3b: Execute tests
	testExecutionSig := dsgo.NewSignature("Execute tests and collect results").
		AddInput("test_code", dsgo.FieldTypeString, "Test runner code to execute").
		AddOutput("test_results", dsgo.FieldTypeJSON, "Test execution results")

	testExecutionPredict := module.NewPredict(testExecutionSig, lm)

	// Sub-step 3c: Analyze results and create final solution
	finalSolutionSig := dsgo.NewSignature("Analyze test results and finalize solution").
		AddInput("code", dsgo.FieldTypeString, "Original generated code").
		AddInput("test_results", dsgo.FieldTypeJSON, "Test execution results").
		AddOutput("final_solution", dsgo.FieldTypeJSON, "Complete solution with results")

	finalSolutionPredict := module.NewPredict(finalSolutionSig, lm)

	// Create Program - testing pipeline
	program := module.NewProgram("code_testing_pipeline").
		AddModule(testRunnerPredict).
		AddModule(testExecutionPredict).
		AddModule(finalSolutionPredict)

	programInputs := map[string]interface{}{
		"language":         language,
		"code":             code,
		"test_inputs":      testInputs,
		"expected_outputs": expectedOutputs,
	}

	programResult, err := program.Forward(step3Ctx, programInputs)
	if err != nil {
		log.Fatal(err)
	}

	solutionData, _ := programResult.Get("final_solution")

	// Display final solution in original format
	if solutionStr, ok := solutionData.(string); ok {
		fmt.Printf("\nFinal Solution:\n%s\n", solutionStr)
	} else {
		// It's structured data, format as JSON
		solutionJSON, _ := json.MarshalIndent(solutionData, "", "  ")
		fmt.Printf("\nFinal Solution:\n%s\n", string(solutionJSON))
	}
	usage3 := programResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage3.PromptTokens, usage3.CompletionTokens)
	totalPromptTokens += usage3.PromptTokens
	totalCompletionTokens += usage3.CompletionTokens
	step3Span.End(nil)
	fmt.Println(strings.Repeat("-", 80))

	// Turn 2: Refine code
	fmt.Println("\n=== Turn 2: Refine Code (Fix Bugs and Optimize) ===")
	fmt.Printf("User: Fix any bugs found in the testing and provide an optimized version\n")
	turn2Ctx, turn2Span := observe.Start(ctx, observe.SpanKindModule, "turn2_refine", nil)

	refineSig := dsgo.NewSignature("Refine code based on test results").
		AddInput("current_solution", dsgo.FieldTypeJSON, "Current solution with test results").
		AddInput("refinement_request", dsgo.FieldTypeString, "Refinement request").
		AddOutput("refined_solution", dsgo.FieldTypeJSON, "Refined solution")

	refinePredict := module.NewPredict(refineSig, lm)

	modifyResult, err := refinePredict.Forward(turn2Ctx, map[string]interface{}{
		"current_solution":   solutionData,
		"refinement_request": "Fix any bugs found in testing and provide an optimized version",
	})
	if err != nil {
		log.Fatal(err)
	}

	refinedSolution, _ := modifyResult.Get("refined_solution")

	// Display refined solution in original format
	if refinedStr, ok := refinedSolution.(string); ok {
		fmt.Printf("\nRefined Solution:\n%s\n", refinedStr)
	} else {
		// It's structured data, format as JSON
		refinedJSON, _ := json.MarshalIndent(refinedSolution, "", "  ")
		fmt.Printf("\nRefined Solution:\n%s\n", string(refinedJSON))
	}
	usage4 := modifyResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage4.PromptTokens, usage4.CompletionTokens)
	totalPromptTokens += usage4.PromptTokens
	totalCompletionTokens += usage4.CompletionTokens
	turn2Span.End(nil)

	// Summary
	fmt.Println("\n=== Code Implementation Pipeline Summary ===")
	fmt.Println("Pipeline: PoT (code gen) → Extract (test cases) → Program (test runner → execute → analyze)")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ ProgramOfThought with code execution + timeout")
	fmt.Println("  ✓ JSON adapter (structured I/O)")
	fmt.Println("  ✓ Typed signatures (strong field types)")
	fmt.Println("  ✓ Program (multi-step module composition)")
	fmt.Println("  ✓ Multi-turn refinement")
	fmt.Println("  ✓ Event logging for each pipeline step")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
