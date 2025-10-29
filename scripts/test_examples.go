package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type ExampleResult struct {
	Name     string
	Path     string
	Success  bool
	Error    error
	Output   string
	Duration time.Duration
}

func main() {
	examples := []string{
		"examples/sentiment",
		"examples/fewshot_conversation",
		"examples/chat_predict",
		"examples/chat_cot",
		"examples/content_generator",
		"examples/composition",
		"examples/code_reviewer",
		"examples/customer_support",
		"examples/data_analyst",
		"examples/interview",
		"examples/math_solver",
		"examples/react_agent",
		"examples/research_assistant",
		"examples/adapter_fallback",
		"examples/streaming",
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Println("=== Running All Examples ===")
	fmt.Printf("Project root: %s\n", projectRoot)
	fmt.Printf("Total examples: %d\n", len(examples))
	fmt.Printf("Timeout: 3 minutes\n\n")

	results := make(chan ExampleResult, len(examples))
	var wg sync.WaitGroup

	startTime := time.Now()

	for _, examplePath := range examples {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			results <- runExample(ctx, projectRoot, path)
		}(examplePath)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []ExampleResult
	for result := range results {
		allResults = append(allResults, result)
		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
		}
		fmt.Printf("%s %s (%.2fs)\n", status, result.Name, result.Duration.Seconds())
		if !result.Success && result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
		}
	}

	totalDuration := time.Since(startTime)

	fmt.Println("\n=== Summary ===")
	passed := 0
	failed := 0

	for _, result := range allResults {
		if result.Success {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", len(allResults), passed, failed)
	fmt.Printf("Total execution time: %.2fs\n", totalDuration.Seconds())

	if failed > 0 {
		fmt.Println("\nFailed examples:")
		for _, result := range allResults {
			if !result.Success {
				fmt.Printf("  - %s: %v\n", result.Name, result.Error)
			}
		}
		os.Exit(1)
	}

	fmt.Println("\n✅ All examples passed!")
}

func runExample(ctx context.Context, projectRoot, examplePath string) ExampleResult {
	name := filepath.Base(examplePath)
	result := ExampleResult{
		Name: name,
		Path: examplePath,
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	mainPath := filepath.Join(examplePath, "main.go")
	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Error = fmt.Errorf("timeout exceeded")
		return result
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("%w: %s", err, string(output))
		return result
	}

	result.Success = true
	return result
}
