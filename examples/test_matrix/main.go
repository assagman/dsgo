package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ANSI color codes for better output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

type TestResult struct {
	Example     string
	Model       string
	Success     bool
	Skipped     bool
	Error       error
	Output      string
	Duration    time.Duration
	ExitCode    int
	ErrorType   string // Categorized error type
	ErrorDetail string // Detailed error message
	StackTrace  string // Stack trace if available
}

type ModelStats struct {
	Model   string
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Results []TestResult
}

type CircuitBreaker struct {
	ctx               context.Context
	cancel            context.CancelFunc
	totalTests        int
	totalFailed       int64
	overallThreshold  float64
	mu                sync.Mutex
	tripped           bool
}

func newCircuitBreaker(parentCtx context.Context, totalTests int) *CircuitBreaker {
	ctx, cancel := context.WithCancel(parentCtx)
	return &CircuitBreaker{
		ctx:              ctx,
		cancel:           cancel,
		totalTests:       totalTests,
		overallThreshold: 0.15, // 85% success required
	}
}

func (cb *CircuitBreaker) recordResult(result TestResult) {
	if result.Success || result.Skipped {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.tripped {
		return
	}

	atomic.AddInt64(&cb.totalFailed, 1)
	totalFailed := atomic.LoadInt64(&cb.totalFailed)
	maxTotalFailures := int64(float64(cb.totalTests) * cb.overallThreshold)

	if totalFailed > maxTotalFailures {
		cb.trip(fmt.Sprintf("%.1f%% failure rate exceeds %.1f%% threshold (%d/%d failed)",
			float64(totalFailed)/float64(cb.totalTests)*100,
			cb.overallThreshold*100,
			totalFailed,
			cb.totalTests))
	}
}

func (cb *CircuitBreaker) trip(reason string) {
	if cb.tripped {
		return
	}

	cb.tripped = true
	fmt.Fprintf(os.Stderr, "\n%s🚨 CIRCUIT BREAKER TRIPPED 🚨%s\n", colorRed+colorBold, colorReset)
	fmt.Fprintf(os.Stderr, "%sReason: %s%s\n\n", colorRed, reason, colorReset)
	cb.cancel()
}

func (cb *CircuitBreaker) isTripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped
}

// All supported models for testing
var allModels = []string{
	"openrouter/google/gemini-2.5-flash",
	"openrouter/google/gemini-2.5-pro",
	"openrouter/anthropic/claude-haiku-4.5",
	"openrouter/qwen/qwen3-235b-a22b-2507",
	"openrouter/z-ai/glm-4.6:exacto",
	"openrouter/minimax/minimax-m2",
	"openrouter/openai/gpt-oss-120b:exacto",
	"openrouter/deepseek/deepseek-v3.1-terminus:exacto",
	"openrouter/moonshotai/kimi-k2-0905:exacto",
	"openrouter/qwen/qwen3-30b-a3b",
	"openrouter/google/gemini-2.0-flash-lite-001",
	"openrouter/meta-llama/llama-3.1-8b-instruct",
}

// All numbered examples
var allExamples = []string{
	"examples/001_predict",
	"examples/002_chain_of_thought",
	"examples/003_react",
	"examples/004_refine",
	"examples/005_best_of_n",
	"examples/006_program_of_thought",
	"examples/007_program_composition",
	"examples/008_chat_predict",
	"examples/009_chat_cot",
	"examples/010_typed_signatures",
	"examples/011_history_prediction",
	"examples/012_math_solver",
	"examples/013_sentiment",
	"examples/014_adapter_fallback",
	"examples/015_fewshot",
	"examples/016_history",
	"examples/017_tools",
	"examples/018_adapters",
	"examples/019_retry_resilience",
	"examples/020_streaming",
	"examples/021_best_of_n_parallel",
	"examples/022_caching",
	"examples/023_global_config",
	"examples/024_lm_factory",
	"examples/025_logging_tracing",
	"examples/026_observability",
	"examples/027_research_assistant",
	"examples/028_code_reviewer",
}

func main() {
	numModels := flag.Int("n", 1, "Number of models: 1=default, N=random N, 0=all")
	verbose := flag.Bool("v", false, "Verbose output (show each test)")
	timeout := flag.Duration("timeout", 10*time.Minute, "Timeout per example")
	parallel := flag.Bool("p", true, "Run tests in parallel")
	maxConcurrent := flag.Int("c", 20, "Max concurrent executions")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	flag.Parse()

	if *noColor {
		disableColors()
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fatal("Failed to get working directory: %v", err)
	}

	// Select models
	var selectedModels []string
	switch *numModels {
	case 0:
		selectedModels = allModels
		printHeader("Testing All Models", len(allModels))
	case 1:
		selectedModels = []string{getDefaultModel()}
		printHeader("Testing Default Model", 1)
	default:
		selectedModels = selectRandomModels(allModels, *numModels)
		printHeader(fmt.Sprintf("Testing %d Random Models", *numModels), *numModels)
	}

	printInfo("Models", formatModelList(selectedModels))
	printInfo("Examples", fmt.Sprintf("%d", len(allExamples)))
	printInfo("Total Tests", fmt.Sprintf("%d", len(selectedModels)*len(allExamples)))
	printInfo("Parallel", fmt.Sprintf("%v", *parallel))
	if *parallel {
		printInfo("Max Concurrent", fmt.Sprintf("%d", *maxConcurrent))
	}
	printInfo("Timeout", timeout.String())
	fmt.Println()

	startTime := time.Now()

	// Create circuit breaker
	totalTests := len(selectedModels) * len(allExamples)
	cb := newCircuitBreaker(context.Background(), totalTests)

	// Run tests
	var results []TestResult
	if *parallel {
		results = runParallel(cb, projectRoot, selectedModels, allExamples, *timeout, *verbose, *maxConcurrent)
	} else {
		results = runSequential(cb, projectRoot, selectedModels, allExamples, *timeout, *verbose)
	}

	duration := time.Since(startTime)

	// Print results
	printResults(results, selectedModels, duration, cb.isTripped())

	// Save detailed logs
	if err := saveLogs(results); err != nil {
		printWarning("Failed to save logs: %v", err)
	}

	// Generate error summary report for failures
	failedResults := getFailedResults(results)
	if len(failedResults) > 0 {
		if err := saveErrorSummary(failedResults); err != nil {
			printWarning("Failed to save error summary: %v", err)
		} else {
			fmt.Printf("\n%s📊 Error summary saved to: %stest_matrix_logs/ERROR_SUMMARY.md%s\n",
				colorCyan, colorBold, colorReset)
		}
	}

	// Exit with appropriate code
	if cb.isTripped() {
		os.Exit(2)
	}

	passed := countPassed(results)
	if passed < len(results) {
		os.Exit(1)
	}
}

func runParallel(cb *CircuitBreaker, projectRoot string, models, examples []string, timeout time.Duration, verbose bool, maxConcurrent int) []TestResult {
	var results []TestResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, maxConcurrent)
	totalTests := int64(len(models) * len(examples))
	completed := int64(0)

	startTime := time.Now()

	for _, model := range models {
		for _, example := range examples {
			if cb.isTripped() {
				break
			}

			wg.Add(1)
			go func(m, ex string) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				if cb.isTripped() {
					return
				}

				result := runTest(cb.ctx, projectRoot, ex, m, timeout)
				cb.recordResult(result)

				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				current := atomic.AddInt64(&completed, 1)

				if verbose || !result.Success {
					printTestResult(result, current, totalTests)
				} else if current%10 == 0 {
					printProgress(current, totalTests, time.Since(startTime))
				}
			}(model, example)
		}
	}

	wg.Wait()
	return results
}

func runSequential(cb *CircuitBreaker, projectRoot string, models, examples []string, timeout time.Duration, verbose bool) []TestResult {
	var results []TestResult
	totalTests := len(models) * len(examples)
	completed := 0

	startTime := time.Now()

	for _, model := range models {
		for _, example := range examples {
			if cb.isTripped() {
				break
			}

			result := runTest(cb.ctx, projectRoot, example, model, timeout)
			cb.recordResult(result)
			results = append(results, result)

			completed++
			if verbose || !result.Success {
				printTestResult(result, int64(completed), int64(totalTests))
			} else if completed%10 == 0 {
				printProgress(int64(completed), int64(totalTests), time.Since(startTime))
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

	env := os.Environ()
	env = append(env, "EXAMPLES_DEFAULT_MODEL="+model)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	result.Duration = time.Since(startTime)

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		result.ExitCode = -1
	}

	// Check for timeout
	if testCtx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Errorf("timeout after %v", timeout)
		return result
	}

	// Check for circuit breaker cancel
	if ctx.Err() == context.Canceled {
		result.Error = fmt.Errorf("cancelled by circuit breaker")
		result.Skipped = true
		return result
	}

	// Check for rate limit (treat as skip)
	if isRateLimitError(result.Output) {
		result.Success = true
		result.Skipped = true
		result.Error = fmt.Errorf("rate limit")
		return result
	}

	// Check for success
	if err == nil && result.ExitCode == 0 {
		result.Success = true
		return result
	}

	result.Error = err
	categorizeError(&result)
	return result
}

// categorizeError analyzes the error and categorizes it
func categorizeError(result *TestResult) {
	output := strings.ToLower(result.Output)
	
	// Extract detailed error and stack trace
	result.ErrorDetail = extractDetailedError(result.Output)
	result.StackTrace = extractStackTrace(result.Output)
	
	// Categorize by error type
	switch {
	case strings.Contains(output, "panic:"):
		result.ErrorType = "PANIC"
	case strings.Contains(output, "rate limit") || strings.Contains(output, "status 429"):
		result.ErrorType = "RATE_LIMIT"
	case strings.Contains(output, "status 403"):
		result.ErrorType = "FORBIDDEN"
	case strings.Contains(output, "status 500") || strings.Contains(output, "status 503"):
		result.ErrorType = "SERVER_ERROR"
	case strings.Contains(output, "timeout") || strings.Contains(output, "deadline exceeded"):
		result.ErrorType = "TIMEOUT"
	case strings.Contains(output, "connection refused") || strings.Contains(output, "connection reset"):
		result.ErrorType = "CONNECTION_ERROR"
	case strings.Contains(output, "json_schema"):
		result.ErrorType = "UNSUPPORTED_FEATURE"
	case strings.Contains(output, "invalid") && strings.Contains(output, "json"):
		result.ErrorType = "JSON_PARSE_ERROR"
	case strings.Contains(output, "validation") || strings.Contains(output, "invalid output"):
		result.ErrorType = "VALIDATION_ERROR"
	case strings.Contains(output, "api key") || strings.Contains(output, "unauthorized"):
		result.ErrorType = "AUTH_ERROR"
	case result.Error != nil && result.Error.Error() == "timeout after":
		result.ErrorType = "TIMEOUT"
	case result.Error != nil && result.Error.Error() == "cancelled by circuit breaker":
		result.ErrorType = "CANCELLED"
	default:
		result.ErrorType = "UNKNOWN"
	}
}

func isRateLimitError(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "status 429") ||
		strings.Contains(lower, "status 403") ||
		strings.Contains(lower, "key limit exceeded")
}

func printResults(results []TestResult, models []string, duration time.Duration, circuitTripped bool) {
	fmt.Println()
	printSeparator("=")
	fmt.Printf("%s%s TEST RESULTS %s%s\n", colorBold, colorCyan, colorReset, colorBold)
	printSeparator("=")
	fmt.Print(colorReset)

	passed := countPassed(results)
	failed := countFailed(results)
	skipped := countSkipped(results)
	total := len(results)

	fmt.Println()
	printStat("Total Tests", total)
	printStat("✅ Passed", passed)
	if failed > 0 {
		fmt.Printf("  %s❌ Failed%s      %d (%.1f%%)\n", colorRed, colorReset, failed, float64(failed)/float64(total)*100)
	}
	if skipped > 0 {
		printStat("⏭️  Skipped", skipped)
	}
	printStat("⏱️  Duration", formatDuration(duration))

	// Model scores
	fmt.Println()
	printSeparator("-")
	fmt.Printf("%s MODEL SCORES %s\n", colorBold, colorReset)
	printSeparator("-")

	modelStats := calculateModelStats(results, models)
	sort.Slice(modelStats, func(i, j int) bool {
		scoreI := float64(modelStats[i].Passed) / float64(modelStats[i].Total)
		scoreJ := float64(modelStats[j].Passed) / float64(modelStats[j].Total)
		return scoreI > scoreJ
	})

	for i, stats := range modelStats {
		successRate := float64(stats.Passed) / float64(stats.Total) * 100
		statusIcon := "✅"
		statusColor := colorGreen
		if stats.Failed > 0 {
			statusIcon = "⚠️ "
			statusColor = colorYellow
		}
		if successRate < 80 {
			statusIcon = "❌"
			statusColor = colorRed
		}

		modelName := formatModelName(stats.Model)
		fmt.Printf("  %s%2d. %-50s %s%3d/%d (%.1f%%)%s %s\n",
			colorGray, i+1, modelName, statusColor, stats.Passed, stats.Total, successRate, colorReset, statusIcon)
	}

	// Failed tests details
	if failed > 0 {
		printFailedTests(results)
	}

	// Circuit breaker warning
	if circuitTripped {
		fmt.Println()
		fmt.Printf("%s⚠️  Tests stopped early due to circuit breaker%s\n", colorYellow, colorReset)
	}

	printSeparator("=")
}

func printFailedTests(results []TestResult) {
	fmt.Println()
	printSeparator("-")
	fmt.Printf("%s FAILED TESTS %s\n", colorRed+colorBold, colorReset)
	printSeparator("-")

	failureNum := 1
	for _, result := range results {
		if result.Success || result.Skipped {
			continue
		}

		fmt.Printf("\n%s%d. %s %s[%s]%s\n", colorBold, failureNum, result.Example, colorGray, formatModelName(result.Model), colorReset)
		fmt.Printf("   %sType:%s %s\n", colorYellow, colorReset, result.ErrorType)
		fmt.Printf("   %sError:%s %v\n", colorRed, colorReset, extractError(result))
		fmt.Printf("   %sDuration:%s %.2fs\n", colorGray, colorReset, result.Duration.Seconds())
		fmt.Printf("   %sExit Code:%s %d\n", colorGray, colorReset, result.ExitCode)
		
		// Save log reference
		logFile := formatLogPath(result, false)
		fmt.Printf("   %sLog:%s %s\n", colorGray, colorReset, logFile)

		failureNum++
	}
}

func calculateModelStats(results []TestResult, models []string) []ModelStats {
	statsMap := make(map[string]*ModelStats)

	for _, model := range models {
		statsMap[model] = &ModelStats{
			Model:   model,
			Results: []TestResult{},
		}
	}

	for _, result := range results {
		stats := statsMap[result.Model]
		stats.Total++
		stats.Results = append(stats.Results, result)

		if result.Success && !result.Skipped {
			stats.Passed++
		} else if result.Skipped {
			stats.Skipped++
		} else {
			stats.Failed++
		}
	}

	var statsList []ModelStats
	for _, stats := range statsMap {
		statsList = append(statsList, *stats)
	}

	return statsList
}

func saveLogs(results []TestResult) error {
	passedDir := "test_matrix_logs/passed"
	failedDir := "test_matrix_logs/failed"

	if err := os.MkdirAll(passedDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(failedDir, 0755); err != nil {
		return err
	}

	for _, result := range results {
		if result.Skipped {
			continue
		}

		logPath := formatLogPath(result, result.Success)
		if err := writeLog(logPath, result); err != nil {
			return err
		}
	}

	return nil
}

func writeLog(path string, result TestResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Header
	if _, err := fmt.Fprintf(f, "================================================================================\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, " TEST RESULT: %s\n", result.Example); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "================================================================================\n\n"); err != nil {
		return err
	}

	// Basic Info
	if _, err := fmt.Fprintf(f, "Example:     %s\n", result.Example); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Model:       %s\n", result.Model); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Success:     %v\n", result.Success); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Duration:    %.2fs\n", result.Duration.Seconds()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Exit Code:   %d\n", result.ExitCode); err != nil {
		return err
	}
	
	// Error Details (for failed tests)
	if !result.Success && !result.Skipped {
		if _, err := fmt.Fprintf(f, "\n--- ERROR ANALYSIS ---\n\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "Error Type:  %s\n", result.ErrorType); err != nil {
			return err
		}
		if result.Error != nil {
			if _, err := fmt.Fprintf(f, "Error:       %v\n", result.Error); err != nil {
				return err
			}
		}
		if result.ErrorDetail != "" {
			if _, err := fmt.Fprintf(f, "Detail:      %s\n", result.ErrorDetail); err != nil {
				return err
			}
		}

		if result.StackTrace != "" {
			if _, err := fmt.Fprintf(f, "\n--- STACK TRACE ---\n\n"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(f, "%s\n", result.StackTrace); err != nil {
				return err
			}
		}
	}
	
	// Full Output
	if _, err := fmt.Fprintf(f, "\n--- FULL OUTPUT ---\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", result.Output); err != nil {
		return err
	}

	// Footer
	if _, err := fmt.Fprintf(f, "\n================================================================================\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, " END OF LOG\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "================================================================================\n"); err != nil {
		return err
	}

	return nil
}

func formatLogPath(result TestResult, success bool) string {
	dir := "test_matrix_logs/failed"
	if success {
		dir = "test_matrix_logs/passed"
	}

	modelName := sanitizeFilename(formatModelName(result.Model))
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.log", modelName, result.Example, timestamp)

	return filepath.Join(dir, filename)
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func extractError(result TestResult) string {
	// Use categorized error if available
	if result.ErrorDetail != "" {
		return result.ErrorDetail
	}
	
	if result.Error != nil {
		return result.Error.Error()
	}

	return extractDetailedError(result.Output)
}

// extractDetailedError extracts the most relevant error from output
func extractDetailedError(output string) string {
	if output == "" {
		return "unknown error"
	}

	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(lines)-i < 20; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "exit status") {
			continue
		}
		if strings.HasPrefix(line, "panic:") ||
			strings.Contains(strings.ToLower(line), "error:") ||
			strings.Contains(strings.ToLower(line), "failed") {
			return line
		}
	}

	// Return last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "exit status") {
			return line
		}
	}

	return "unknown error"
}

// extractStackTrace extracts stack trace from output
func extractStackTrace(output string) string {
	if output == "" {
		return ""
	}
	
	lines := strings.Split(output, "\n")
	var stackTrace []string
	inStack := false
	
	for _, line := range lines {
		// Detect start of stack trace
		if strings.Contains(line, "panic:") || strings.Contains(line, "goroutine") {
			inStack = true
		}
		
		if inStack {
			// Stack trace lines typically start with \t or contain file paths
			if strings.HasPrefix(line, "\t") || 
			   strings.Contains(line, ".go:") ||
			   strings.Contains(line, "goroutine") ||
			   strings.Contains(line, "panic:") {
				stackTrace = append(stackTrace, line)
			} else if len(stackTrace) > 0 && line == "" {
				// Empty line might end stack trace
				break
			}
		}
	}
	
	if len(stackTrace) > 0 {
		return strings.Join(stackTrace, "\n")
	}
	
	return ""
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
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	return "openrouter/google/gemini-2.5-flash"
}

func formatModelName(model string) string {
	return strings.TrimPrefix(model, "openrouter/")
}

func formatModelList(models []string) string {
	if len(models) == 1 {
		return formatModelName(models[0])
	}
	if len(models) <= 3 {
		names := make([]string, len(models))
		for i, m := range models {
			names[i] = formatModelName(m)
		}
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d models", len(models))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func countPassed(results []TestResult) int {
	count := 0
	for _, r := range results {
		if r.Success && !r.Skipped {
			count++
		}
	}
	return count
}

func countFailed(results []TestResult) int {
	count := 0
	for _, r := range results {
		if !r.Success && !r.Skipped {
			count++
		}
	}
	return count
}

func countSkipped(results []TestResult) int {
	count := 0
	for _, r := range results {
		if r.Skipped {
			count++
		}
	}
	return count
}

// Printing utilities

func printHeader(title string, count int) {
	fmt.Println()
	printSeparator("=")
	fmt.Printf("%s%s %s (%d) %s%s\n", colorBold, colorCyan, title, count, colorReset, colorBold)
	printSeparator("=")
	fmt.Print(colorReset)
	fmt.Println()
}

func printInfo(label, value string) {
	fmt.Printf("  %s%-15s%s %s\n", colorGray, label+":", colorReset, value)
}

func printStat(label string, value interface{}) {
	fmt.Printf("  %s%-15s%s %v\n", colorBold, label+":", colorReset, value)
}

func printWarning(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s⚠️  "+format+"%s\n", colorYellow, colorReset)
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func printTestResult(result TestResult, current, total int64) {
	icon := "✅"
	statusColor := colorGreen
	status := "PASS"

	if result.Skipped {
		icon = "⏭️ "
		statusColor = colorGray
		status = "SKIP"
	} else if !result.Success {
		icon = "❌"
		statusColor = colorRed
		status = "FAIL"
	}

	modelName := formatModelName(result.Model)
	fmt.Printf("[%s%4d/%d%s] %s %s%-4s%s %s%-20s%s %s%s%s %.2fs\n",
		colorGray, current, total, colorReset,
		icon,
		statusColor, status, colorReset,
		colorGray, result.Example, colorReset,
		colorGray, modelName, colorReset,
		result.Duration.Seconds())

	if !result.Success && !result.Skipped && result.Error != nil {
		fmt.Printf("       %s↳ %v%s\n", colorRed, result.Error, colorReset)
	}
}

func printProgress(current, total int64, elapsed time.Duration) {
	percent := float64(current) / float64(total) * 100
	fmt.Printf("\r%s[%4d/%d] %.1f%% complete - %s elapsed%s",
		colorGray, current, total, percent, formatDuration(elapsed), colorReset)
	if current == total {
		fmt.Println()
	}
}

func printSeparator(char string) {
	fmt.Println(strings.Repeat(char, 80))
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%sError: "+format+"%s\n", colorRed, colorReset)
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	os.Exit(1)
}

func disableColors() {
	// This would normally set all color constants to empty strings
	// For simplicity, we'll just use a global flag
	// In a real implementation, you'd reassign all color constants
}

func getFailedResults(results []TestResult) []TestResult {
	var failed []TestResult
	for _, r := range results {
		if !r.Success && !r.Skipped {
			failed = append(failed, r)
		}
	}
	return failed
}

func saveErrorSummary(failedResults []TestResult) error {
	summaryPath := "test_matrix_logs/ERROR_SUMMARY.md"
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(summaryPath), 0755); err != nil {
		return err
	}
	
	f, err := os.Create(summaryPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	
	// Header
	if _, err := fmt.Fprintf(f, "# Test Matrix Error Summary\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Total Failures: %d\n\n", len(failedResults)); err != nil {
		return err
	}
	
	// Error type breakdown
	if _, err := fmt.Fprintf(f, "## Error Type Breakdown\n\n"); err != nil {
		return err
	}
	errorTypes := make(map[string]int)
	for _, r := range failedResults {
		errorTypes[r.ErrorType]++
	}

	// Sort by count
	type errorCount struct {
		typ   string
		count int
	}
	var counts []errorCount
	for typ, count := range errorTypes {
		counts = append(counts, errorCount{typ, count})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	for _, ec := range counts {
		if _, err := fmt.Fprintf(f, "- **%s**: %d failures\n", ec.typ, ec.count); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(f, "\n"); err != nil {
		return err
	}
	
	// Model breakdown
	if _, err := fmt.Fprintf(f, "## Failures by Model\n\n"); err != nil {
		return err
	}
	modelFailures := make(map[string][]TestResult)
	for _, r := range failedResults {
		modelFailures[r.Model] = append(modelFailures[r.Model], r)
	}

	for model, failures := range modelFailures {
		if _, err := fmt.Fprintf(f, "### %s (%d failures)\n\n", formatModelName(model), len(failures)); err != nil {
			return err
		}
		for _, r := range failures {
			if _, err := fmt.Fprintf(f, "- **%s** (%s): %s\n", r.Example, r.ErrorType, r.ErrorDetail); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(f, "\n"); err != nil {
			return err
		}
	}
	
	// Detailed failures
	if _, err := fmt.Fprintf(f, "## Detailed Failure List\n\n"); err != nil {
		return err
	}
	for i, r := range failedResults {
		if _, err := fmt.Fprintf(f, "### %d. %s [%s]\n\n", i+1, r.Example, formatModelName(r.Model)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "- **Error Type**: %s\n", r.ErrorType); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "- **Error**: %s\n", r.ErrorDetail); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "- **Duration**: %.2fs\n", r.Duration.Seconds()); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "- **Exit Code**: %d\n", r.ExitCode); err != nil {
			return err
		}

		logFile := formatLogPath(r, false)
		if _, err := fmt.Fprintf(f, "- **Log**: `%s`\n", logFile); err != nil {
			return err
		}

		if r.StackTrace != "" {
			if _, err := fmt.Fprintf(f, "\n**Stack Trace:**\n```\n%s\n```\n", r.StackTrace); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(f, "\n---\n\n"); err != nil {
			return err
		}
	}
	
	return nil
}
