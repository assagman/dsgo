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
	"sync/atomic"
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

// CircuitBreaker monitors test failures and triggers early termination
type CircuitBreaker struct {
	ctx               context.Context
	cancel            context.CancelFunc
	totalTests        int
	totalFailed       int64
	modelFailed       map[string]*int64
	modelTotal        map[string]int
	overallThreshold  float64 // 0.05 = 5% max failure (95% success)
	perModelThreshold float64 // 0.10 = 10% max failure (90% success)
	mu                sync.Mutex
	tripped           bool
	failedResults     []TestResult
}

func newCircuitBreaker(parentCtx context.Context, totalTests int, models []string, examplesPerModel int) *CircuitBreaker {
	ctx, cancel := context.WithCancel(parentCtx)
	cb := &CircuitBreaker{
		ctx:               ctx,
		cancel:            cancel,
		totalTests:        totalTests,
		modelFailed:       make(map[string]*int64),
		modelTotal:        make(map[string]int),
		overallThreshold:  0.05, // 95% success required
		perModelThreshold: 0.10, // 90% per-model success required
		failedResults:     make([]TestResult, 0),
	}

	for _, model := range models {
		var zero int64
		cb.modelFailed[model] = &zero
		cb.modelTotal[model] = examplesPerModel
	}

	return cb
}

func (cb *CircuitBreaker) recordResult(result TestResult) {
	if result.Success {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.tripped {
		return
	}

	// Record failure
	atomic.AddInt64(&cb.totalFailed, 1)
	if modelCounter, exists := cb.modelFailed[result.Model]; exists {
		atomic.AddInt64(modelCounter, 1)
	}
	cb.failedResults = append(cb.failedResults, result)

	// Check thresholds
	totalFailed := atomic.LoadInt64(&cb.totalFailed)
	maxTotalFailures := int64(float64(cb.totalTests) * cb.overallThreshold)

	if totalFailed > maxTotalFailures {
		cb.trip(fmt.Sprintf("Overall failure threshold exceeded: %d failures out of %d tests (%.1f%% failed, max allowed: %.1f%%)",
			totalFailed, cb.totalTests, float64(totalFailed)/float64(cb.totalTests)*100, cb.overallThreshold*100))
		return
	}

	// Check per-model thresholds
	for model, failed := range cb.modelFailed {
		modelFailures := atomic.LoadInt64(failed)
		modelTotal := cb.modelTotal[model]
		maxModelFailures := int64(float64(modelTotal) * cb.perModelThreshold)

		if modelFailures > maxModelFailures {
			cb.trip(fmt.Sprintf("Model failure threshold exceeded for %s: %d failures out of %d tests (%.1f%% failed, max allowed: %.1f%%)",
				model, modelFailures, modelTotal, float64(modelFailures)/float64(modelTotal)*100, cb.perModelThreshold*100))
			return
		}
	}
}

func (cb *CircuitBreaker) trip(reason string) {
	if cb.tripped {
		return
	}

	cb.tripped = true
	fmt.Fprintf(os.Stderr, "\n\n🚨 CIRCUIT BREAKER TRIPPED 🚨\n")
	fmt.Fprintf(os.Stderr, "Reason: %s\n\n", reason)
	fmt.Fprintf(os.Stderr, "Cancelling remaining tests...\n\n")

	// Dump failed test outputs
	if len(cb.failedResults) > 0 {
		fmt.Fprintf(os.Stderr, "=== Failed Test Outputs ===\n\n")
		for i, result := range cb.failedResults {
			fmt.Fprintf(os.Stderr, "[%d] %s [%s]\n", i+1, result.Example, result.Model)
			fmt.Fprintf(os.Stderr, "Exit Code: %d\n", result.ExitCode)
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
			}
			fmt.Fprintf(os.Stderr, "Output:\n%s\n", result.Output)
			fmt.Fprintf(os.Stderr, "---\n\n")
		}
	}

	cb.cancel()
}

func (cb *CircuitBreaker) isTripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped
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
	maxConcurrent := flag.Int("c", 10, "Maximum concurrent test executions (prevents resource exhaustion)")
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
	if *parallel {
		fmt.Printf("Max concurrent: %d\n", *maxConcurrent)
	}
	fmt.Printf("Timeout: %v\n\n", *timeout)

	startTime := time.Now()

	// Create circuit breaker
	totalTests := len(selectedModels) * len(allExamples)
	cb := newCircuitBreaker(context.Background(), totalTests, selectedModels, len(allExamples))

	// Run tests
	var results []TestResult
	if *parallel {
		results = runParallel(cb, projectRoot, selectedModels, allExamples, *timeout, *verbose, *maxConcurrent)
	} else {
		results = runSequential(cb, projectRoot, selectedModels, allExamples, *timeout, *verbose)
	}

	totalDuration := time.Since(startTime)

	// Check if circuit breaker tripped
	if cb.isTripped() {
		fmt.Println("\n❌ Test execution halted by circuit breaker")
		os.Exit(1)
	}

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
				status, model, rate, stats.passed, stats.total)
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

func runParallel(cb *CircuitBreaker, projectRoot string, models, examples []string, timeout time.Duration, verbose bool, maxConcurrent int) []TestResult {
	totalTests := len(models) * len(examples)
	results := make(chan TestResult, totalTests)
	semaphore := make(chan struct{}, maxConcurrent) // Limit concurrent executions
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

				// Check context before acquiring semaphore
				select {
				case <-cb.ctx.Done():
					return
				default:
				}

				// Acquire semaphore slot with context awareness
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-cb.ctx.Done():
					return
				}

				// Final check after acquiring semaphore
				select {
				case <-cb.ctx.Done():
					return
				default:
				}

				result := runTest(cb.ctx, projectRoot, ex, m, timeout)
				cb.recordResult(result)
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
				errMsg := ""
				if !result.Success {
					status = "❌"
					errMsg = fmt.Sprintf(" | %s", extractErrorFromResult(result))
				}
				modelInfo := ""
				if len(models) > 1 {
					modelInfo = fmt.Sprintf(" [%s]", m)
				}
				fmt.Printf("[%d/%d] %s %s%s (%.2fs, exit: %d)%s\n", currentCompleted, totalTests, status, filepath.Base(ex), modelInfo, result.Duration.Seconds(), result.ExitCode, errMsg)

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

func runSequential(cb *CircuitBreaker, projectRoot string, models, examples []string, timeout time.Duration, verbose bool) []TestResult {
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
			// Check if circuit breaker already tripped
			if cb.isTripped() {
				break
			}

			completed++
			result := runTest(cb.ctx, projectRoot, example, model, timeout)
			cb.recordResult(result)
			results = append(results, result)

			// Save individual log
			modelDir := filepath.Join(logDir, sanitizeFilename(model))
			logFile := filepath.Join(modelDir, sanitizeFilename(filepath.Base(example))+".log")
			if err := saveLog(logFile, result); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save log: %v\n", err)
			}

			status := "✅"
			errMsg := ""
			if !result.Success {
				status = "❌"
				errMsg = fmt.Sprintf(" | %s", extractErrorFromResult(result))
			}
			modelInfo := ""
			if len(models) > 1 {
				modelInfo = fmt.Sprintf(" [%s]", model)
			}
			fmt.Printf("[%d/%d] %s %s%s (%.2fs, exit: %d)%s\n",
				completed, total, status, filepath.Base(example), modelInfo, result.Duration.Seconds(), result.ExitCode, errMsg)

			if verbose && !result.Success {
				fmt.Printf("   Error: %v\n", result.Error)
			}
		}
	}

	return results
}

func runTest(ctx context.Context, projectRoot, examplePath, model string, timeout time.Duration) TestResult {
	startTime := time.Now()
	result := TestResult{
		Example: filepath.Base(examplePath),
		Model:   model,
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	mainPath := filepath.Join(examplePath, "main.go")
	cmd := exec.CommandContext(testCtx, "go", "run", mainPath)
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

	if testCtx.Err() == context.DeadlineExceeded {
		result.Success = false
		result.Error = fmt.Errorf("timeout exceeded")
		result.ExitCode = 124
		return result
	}

	if ctx.Err() == context.Canceled {
		result.Success = false
		result.Error = fmt.Errorf("cancelled by circuit breaker")
		result.ExitCode = 125
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

func extractErrorFromResult(result TestResult) string {
	// Special cases first
	if result.Error != nil {
		errStr := result.Error.Error()
		if errStr == "timeout exceeded" || errStr == "cancelled by circuit breaker" {
			return errStr
		}
	}

	// Parse output for actual error
	output := result.Output
	if output == "" {
		if result.Error != nil {
			return result.Error.Error()
		}
		return "unknown error"
	}

	// Look for common error patterns in output
	lines := strings.Split(output, "\n")

	// Search for error indicators (from bottom to top, as errors usually at end)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Look for panic messages
		if strings.HasPrefix(line, "panic:") {
			msg := strings.TrimPrefix(line, "panic:")
			return truncateMessage(strings.TrimSpace(msg))
		}

		// Look for "Error:" or "error:" lines
		if strings.Contains(strings.ToLower(line), "error:") {
			return truncateMessage(line)
		}

		// Look for "failed" messages
		if strings.Contains(strings.ToLower(line), "failed") {
			return truncateMessage(line)
		}

		// Look for API error responses
		if strings.Contains(line, "status") && (strings.Contains(line, "429") || strings.Contains(line, "500") || strings.Contains(line, "503")) {
			return truncateMessage(line)
		}

		// Stop searching after 20 lines from bottom
		if len(lines)-i > 20 {
			break
		}
	}

	// Fallback: return last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "exit status") {
			return truncateMessage(line)
		}
	}

	if result.Error != nil {
		return result.Error.Error()
	}

	return "unknown error"
}

func truncateMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	maxLen := 100
	if len(msg) > maxLen {
		return msg[:maxLen-3] + "..."
	}
	return msg
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
