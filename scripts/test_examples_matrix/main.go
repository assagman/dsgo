package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TestResult struct {
	Example  string
	Model    string
	Success  bool
	Error    error
	Output   string
	Duration time.Duration
	ExitCode int
}

var allModels = []string{
	"openrouter/minimax/minimax-m2",
	"openrouter/openai/gpt-oss-120b:exacto",
	"openrouter/deepseek/deepseek-v3.1-terminus:exacto",
	"openrouter/z-ai/glm-4.6:exacto",
	"openrouter/moonshotai/kimi-k2-0905:exacto",
	"openrouter/openai/gpt-5-nano",
	"openrouter/anthropic/claude-haiku-4.5",
	"openrouter/google/gemini-2.5-flash",
	"openrouter/google/gemini-2.5-pro",
	"openrouter/qwen/qwen3-vl-32b-instruct",
}

var allExamples = []string{
	"examples/adapter_fallback",
	"examples/best_of_n_parallel",
	"examples/caching",
	"examples/chat_cot",
	"examples/chat_predict",
	"examples/code_reviewer",
	"examples/composition",
	"examples/content_generator",
	"examples/customer_support",
	"examples/data_analyst",
	"examples/fewshot_conversation",
	"examples/interview",
	"examples/logging_tracing",
	"examples/math_solver",
	"examples/program_of_thought",
	"examples/react_agent",
	"examples/research_assistant",
	"examples/retry_resilience",
	"examples/sentiment",
	"examples/streaming",
}

func main() {
	// Define flags
	numModels := flag.Int("n", 1, "Number of random models to test (1 = single model like test_examples, 0 = all models)")
	verbose := flag.Bool("v", false, "Verbose output")
	timeout := flag.Duration("timeout", 10*time.Minute, "Timeout per example")
	parallel := flag.Bool("p", true, "Run tests in parallel")
	flag.Parse()

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Select models
	var selectedModels []string
	switch *numModels {
	case 0:
		// All models
		selectedModels = allModels
		fmt.Printf("=== Testing All %d Models ===\n", len(allModels))
	case 1:
		// Single model (default behavior like test_examples)
		selectedModels = []string{getDefaultModel()}
		fmt.Printf("=== Testing Single Model ===\n")
	default:
		// Random N models
		selectedModels = selectRandomModels(allModels, *numModels)
		fmt.Printf("=== Testing %d Random Models ===\n", *numModels)
	}

	fmt.Printf("Models: %v\n", selectedModels)
	fmt.Printf("Examples: %d\n", len(allExamples))
	fmt.Printf("Total executions: %d\n", len(selectedModels)*len(allExamples))
	fmt.Printf("Parallel: %v\n", *parallel)
	fmt.Printf("Timeout: %v\n\n", *timeout)

	startTime := time.Now()

	// Run tests
	var results []TestResult
	if *parallel {
		results = runParallel(projectRoot, selectedModels, allExamples, *timeout, *verbose)
	} else {
		results = runSequential(projectRoot, selectedModels, allExamples, *timeout, *verbose)
	}

	totalDuration := time.Since(startTime)

	// Print summary
	fmt.Println("\n=== Summary ===")
	passed := 0
	failed := 0

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("Total: %d | Passed: %d | Failed: %d\n", len(results), passed, failed)
	fmt.Printf("Total execution time: %.2fs\n", totalDuration.Seconds())

	if failed > 0 {
		fmt.Println("\nFailed tests:")
		for _, result := range results {
			if !result.Success {
				modelInfo := ""
				if len(selectedModels) > 1 {
					modelInfo = fmt.Sprintf(" [%s]", result.Model)
				}
				fmt.Printf("  - %s%s (exit %d): %v\n", result.Example, modelInfo, result.ExitCode, result.Error)
			}
		}
	}

	// Exit code summary
	fmt.Println("\n=== Exit Codes ===")
	exitCodeStats := make(map[int]int)
	for _, result := range results {
		exitCodeStats[result.ExitCode]++
	}
	for code := 0; code <= 124; code++ {
		if count, exists := exitCodeStats[code]; exists {
			status := "✅"
			if code != 0 {
				status = "❌"
			}
			fmt.Printf("%s Exit %d: %d executions\n", status, code, count)
		}
	}
	// Handle special codes
	for code, count := range exitCodeStats {
		if code > 124 || code < 0 {
			fmt.Printf("❌ Exit %d: %d executions\n", code, count)
		}
	}

	// Quality criteria evaluation
	fmt.Println("\n=== Quality Criteria ===")

	// Overall success rate
	overallRate := float64(passed) / float64(len(results)) * 100
	overallPass := overallRate >= 95.0
	overallStatus := "✅"
	if !overallPass {
		overallStatus = "❌"
	}
	fmt.Printf("%s Overall success rate: %.1f%% (required: ≥95%%)\n", overallStatus, overallRate)

	// Per-model success rate (only if multiple models)
	allModelsPass := true
	if len(selectedModels) > 1 {
		modelStats := make(map[string]struct{ passed, total int })
		for _, result := range results {
			stats := modelStats[result.Model]
			stats.total++
			if result.Success {
				stats.passed++
			}
			modelStats[result.Model] = stats
		}

		fmt.Println("\nPer-model success rates:")
		for _, model := range selectedModels {
			stats := modelStats[model]
			rate := float64(stats.passed) / float64(stats.total) * 100
			modelPass := rate >= 90.0
			status := "✅"
			if !modelPass {
				status = "❌"
				allModelsPass = false
			}
			fmt.Printf("  %s %s: %.1f%% (%d/%d) (required: ≥90%%)\n",
				status, truncate(model, 40), rate, stats.passed, stats.total)
		}
	}

	// Final verdict
	fmt.Println()
	if overallPass && allModelsPass && failed == 0 {
		fmt.Println("✅ All tests passed! Quality criteria met.")
		os.Exit(0)
	} else if overallPass && allModelsPass {
		fmt.Println("⚠️  Quality criteria met but some tests failed.")
		fmt.Println("    Check test_examples_logs/ for detailed failure logs.")
		os.Exit(0) // Exit 0 but show warning
	} else {
		fmt.Println("❌  Quality criteria not met.")
		fmt.Println("    Check test_examples_logs/ for detailed failure logs.")
		os.Exit(1)
	}
}

func runParallel(projectRoot string, models, examples []string, timeout time.Duration, verbose bool) []TestResult {
	totalTests := len(models) * len(examples)
	results := make(chan TestResult, totalTests)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0

	// Create log directory
	logDir := filepath.Join(projectRoot, "test_examples_logs")
	if err := os.RemoveAll(logDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Failed to clean log directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Pre-create model directories
	for _, model := range models {
		modelDir := filepath.Join(logDir, sanitizeFilename(model))
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create model directory: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Log directory: %s\n", logDir)
	fmt.Println("Launching tests concurrently...")

	for _, model := range models {
		for _, example := range examples {
			wg.Add(1)
			go func(m, ex string) {
				defer wg.Done()
				result := runTest(projectRoot, ex, m, timeout)
				results <- result

				// Save individual log
				modelDir := filepath.Join(logDir, sanitizeFilename(m))
				logFile := filepath.Join(modelDir, sanitizeFilename(filepath.Base(ex))+".log")
				if err := saveLog(logFile, result); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to save log: %v\n", err)
				}

				mu.Lock()
				completed++
				currentCompleted := completed
				mu.Unlock()

				status := "✅"
				if !result.Success {
					status = "❌"
				}
				modelInfo := ""
				if len(models) > 1 {
					modelInfo = fmt.Sprintf(" [%s]", truncate(m, 30))
				}
				fmt.Printf("[%d/%d] %s %s%s (%.2fs, exit: %d)\n", currentCompleted, totalTests, status, filepath.Base(ex), modelInfo, result.Duration.Seconds(), result.ExitCode)

				if verbose && !result.Success {
					fmt.Printf("   Error: %v\n", result.Error)
				}
			}(model, example)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []TestResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

func runSequential(projectRoot string, models, examples []string, timeout time.Duration, verbose bool) []TestResult {
	var results []TestResult
	completed := 0
	total := len(models) * len(examples)

	// Create log directory
	logDir := filepath.Join(projectRoot, "test_examples_logs")
	if err := os.RemoveAll(logDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Failed to clean log directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Pre-create model directories
	for _, model := range models {
		modelDir := filepath.Join(logDir, sanitizeFilename(model))
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create model directory: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Log directory: %s\n", logDir)

	for _, model := range models {
		for _, example := range examples {
			completed++
			result := runTest(projectRoot, example, model, timeout)
			results = append(results, result)

			// Save individual log
			modelDir := filepath.Join(logDir, sanitizeFilename(model))
			logFile := filepath.Join(modelDir, sanitizeFilename(filepath.Base(example))+".log")
			if err := saveLog(logFile, result); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save log: %v\n", err)
			}

			status := "✅"
			if !result.Success {
				status = "❌"
			}
			modelInfo := ""
			if len(models) > 1 {
				modelInfo = fmt.Sprintf(" [%s]", model)
			}
			fmt.Printf("[%d/%d] %s %s%s (%.2fs, exit: %d)\n",
				completed, total, status, filepath.Base(example), modelInfo, result.Duration.Seconds(), result.ExitCode)

			if verbose && !result.Success {
				fmt.Printf("   Error: %v\n", result.Error)
			}
		}
	}

	return results
}

func runTest(projectRoot, examplePath, model string, timeout time.Duration) TestResult {
	startTime := time.Now()
	result := TestResult{
		Example: filepath.Base(examplePath),
		Model:   model,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mainPath := filepath.Join(examplePath, "main.go")
	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = projectRoot

	// Set model via environment variable
	env := os.Environ()
	if model != "" {
		env = append(env, "OPENROUTER_MODEL="+model)
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	result.Duration = time.Since(startTime)

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		result.ExitCode = -1
	} else {
		result.ExitCode = 0
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Error = fmt.Errorf("timeout exceeded")
		result.ExitCode = 124
		return result
	}

	if err != nil {
		result.Success = false
		result.Error = err
		return result
	}

	result.Success = true
	return result
}

func selectRandomModels(models []string, n int) []string {
	if n >= len(models) {
		return models
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	perm := r.Perm(len(models))
	selected := make([]string, n)
	for i := 0; i < n; i++ {
		selected[i] = models[perm[i]]
	}
	return selected
}

func getDefaultModel() string {
	// Check environment variable first
	if model := os.Getenv("OPENROUTER_MODEL"); model != "" {
		return model
	}
	if model := os.Getenv("MODEL"); model != "" {
		return model
	}

	// Default to a fast, reliable model
	return "openai/gpt-4o-mini"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func saveLog(logFile string, result TestResult) error {
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(f, "Example: %s\n", result.Example)
	_, _ = fmt.Fprintf(f, "Model: %s\n", result.Model)
	_, _ = fmt.Fprintf(f, "Success: %v\n", result.Success)
	_, _ = fmt.Fprintf(f, "Duration: %.2fs\n", result.Duration.Seconds())
	_, _ = fmt.Fprintf(f, "Exit Code: %d\n", result.ExitCode)
	if result.Error != nil {
		_, _ = fmt.Fprintf(f, "Error: %v\n", result.Error)
	}
	_, _ = fmt.Fprintf(f, "\n--- Output ---\n%s\n", result.Output)

	return nil
}
