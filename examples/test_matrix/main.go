package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ANSI color codes
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
	Error       error
	Stdout      string
	Stderr      string
	Combined    string
	Duration    time.Duration
	ExitCode    int
	Signal      string
	ErrorType   string
	ErrorDetail string
	HTTPStatus  int
	StackTrace  string
	StartTime   time.Time
	EndTime     time.Time
	ArtifactDir string
	Cancelled   bool
}

type TestArtifact struct {
	Example     string    `json:"example"`
	Model       string    `json:"model"`
	Success     bool      `json:"success"`
	ExitCode    int       `json:"exit_code"`
	Signal      string    `json:"signal,omitempty"`
	ErrorType   string    `json:"error_type,omitempty"`
	ErrorDetail string    `json:"error_detail,omitempty"`
	HTTPStatus  int       `json:"http_status,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    string    `json:"duration"`
	Cancelled   bool      `json:"cancelled"`
}

type ModelStats struct {
	Model   string
	Total   int
	Passed  int
	Failed  int
	Results []TestResult
}

type CircuitBreaker struct {
	ctx              context.Context
	cancel           context.CancelFunc
	totalTests       int
	totalFailed      int64
	overallThreshold float64
	mu               sync.Mutex
	tripped          bool
	tripReason       string
}

func newCircuitBreaker(totalTests int) *CircuitBreaker {
	ctx, cancel := context.WithCancel(context.Background())
	return &CircuitBreaker{
		ctx:              ctx,
		cancel:           cancel,
		totalTests:       totalTests,
		overallThreshold: 0.15, // 15% max failure rate
	}
}

func (cb *CircuitBreaker) recordResult(result TestResult) {
	if result.Success || result.Cancelled {
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
		reason := fmt.Sprintf("%.1f%% failure rate exceeds %.1f%% threshold (%d/%d failed)",
			float64(totalFailed)/float64(cb.totalTests)*100,
			cb.overallThreshold*100,
			totalFailed,
			cb.totalTests)
		cb.trip(reason)
	}
}

func (cb *CircuitBreaker) trip(reason string) {
	if cb.tripped {
		return
	}

	cb.tripped = true
	cb.tripReason = reason
	fmt.Fprintf(os.Stderr, "\n%s🚨 CIRCUIT BREAKER TRIPPED 🚨%s\n", colorRed+colorBold, colorReset)
	fmt.Fprintf(os.Stderr, "%sReason: %s%s\n\n", colorRed, reason, colorReset)

	// Immediately cancel all running tests
	cb.cancel()
}

func (cb *CircuitBreaker) isTripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.tripped
}

// All supported models
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
	"openrouter/google/gemini-2.0-flash-lite-001",
	"openrouter/meta-llama/llama-3.3-70b-instruct",
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
	timeout := flag.Duration("timeout", 20*time.Minute, "Total timeout for all tests (hard stop)")
	maxConcurrent := flag.Int("c", 20, "Max concurrent executions (use 1 for sequential)")
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

	printInfo("Examples", fmt.Sprintf("%d", len(allExamples)))
	printInfo("Total Tests", fmt.Sprintf("%d", len(selectedModels)*len(allExamples)))
	printInfo("Max Concurrent", fmt.Sprintf("%d", *maxConcurrent))
	printInfo("Timeout", timeout.String()+" (total, hard stop)")
	fmt.Println()

	// Print all selected models
	fmt.Printf("%sModels:%s\n", colorBold, colorReset)
	for i, model := range selectedModels {
		fmt.Printf("  %d. %s\n", i+1, formatModelName(model))
	}
	fmt.Println()

	startTime := time.Now()

	// Create circuit breaker with total timeout
	totalTests := len(selectedModels) * len(allExamples)
	cb := newCircuitBreaker(totalTests)

	// Add total timeout wrapper
	totalTimeoutCtx, totalTimeoutCancel := context.WithTimeout(cb.ctx, *timeout)
	defer totalTimeoutCancel()

	// Monitor total timeout
	go func() {
		<-totalTimeoutCtx.Done()
		if totalTimeoutCtx.Err() == context.DeadlineExceeded {
			cb.trip(fmt.Sprintf("Total timeout of %v exceeded - hard stop all tests", *timeout))
		}
	}()

	// Run tests (individual test timeout is 2 minutes)
	perTestTimeout := 5 * time.Minute
	results := runParallel(cb, projectRoot, selectedModels, allExamples, perTestTimeout, *verbose, *maxConcurrent)

	duration := time.Since(startTime)

	// Print results
	printResults(results, selectedModels, duration, cb.isTripped())

	// Generate error summary
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
	if passed < len(results)-countCancelled(results) {
		os.Exit(1)
	}
}

func runParallel(cb *CircuitBreaker, projectRoot string, models, examples []string, timeout time.Duration, verbose bool, maxConcurrent int) []TestResult {
	var results []TestResult
	var mu sync.Mutex

	type job struct {
		model   string
		example string
	}

	jobs := make(chan job, len(models)*len(examples))
	resultsCh := make(chan TestResult, maxConcurrent)

	totalTests := int64(len(models) * len(examples))
	completed := int64(0)
	running := int64(0)

	// Collector
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for result := range resultsCh {
			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			cb.recordResult(result)
			current := atomic.AddInt64(&completed, 1)
			atomic.AddInt64(&running, -1)

			printTestResult(result, current, totalTests)
		}
	}()

	// Enqueue jobs - iterate examples first, then models
	for _, example := range examples {
		for _, model := range models {
			jobs <- job{model: model, example: example}
		}
	}
	close(jobs)

	// Worker pool
	var workerWg sync.WaitGroup
	for i := 0; i < maxConcurrent; i++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for {
				select {
				case <-cb.ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok {
						return
					}
					atomic.AddInt64(&running, 1)
					result := runTest(cb.ctx, projectRoot, j.example, j.model, timeout)
					resultsCh <- result
				}
			}
		}()
	}

	workerWg.Wait()
	close(resultsCh)
	collectorWg.Wait()

	return results
}

func runTest(ctx context.Context, projectRoot, examplePath, model string, timeout time.Duration) TestResult {
	startTime := time.Now()
	result := TestResult{
		Example:   filepath.Base(examplePath),
		Model:     model,
		StartTime: startTime,
	}

	// Create test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	mainPath := filepath.Join(examplePath, "main.go")
	cmd := exec.CommandContext(testCtx, "go", "run", mainPath)
	cmd.Dir = projectRoot

	// Set environment
	env := os.Environ()
	env = append(env, "EXAMPLES_DEFAULT_MODEL="+model)
	// Always enable debug mode for maximum verbosity in artifacts
	env = append(env, "DSGO_DEBUG_PARSE=1")
	cmd.Env = env

	// Setup process group for clean killing
	setupProcessGroup(cmd)

	// Capture output
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start command
	err := cmd.Start()
	if err != nil {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Error = fmt.Errorf("failed to start: %w", err)
		result.ErrorType = "START_FAILURE"
		result.ErrorDetail = err.Error()
		return result
	}

	// Monitor for cancellation to kill process group immediately
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-testCtx.Done():
		// Kill process group immediately
		killProcessGroup(cmd)
		<-done // Wait for Wait() to finish

		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
		result.Combined = result.Stdout + "\n" + result.Stderr

		// Check if parent context was cancelled (circuit breaker)
		if ctx.Err() == context.Canceled {
			result.Cancelled = true
			result.ErrorType = "CIRCUIT_BREAKER_CANCEL"
			result.ErrorDetail = "Test cancelled by circuit breaker"
		} else {
			result.ErrorType = "TIMEOUT"
			result.ErrorDetail = fmt.Sprintf("Test exceeded %v timeout", timeout)
		}

		classifyError(&result)
		saveArtifact(&result)
		return result

	case err := <-done:
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
		result.Combined = result.Stdout + "\n" + result.Stderr

		// Extract exit code and signal
		if err == nil {
			result.Success = true
			result.ExitCode = 0
		} else {
			result.Error = err
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						result.Signal = status.Signal().String()
						result.ErrorType = "SIGNALED"
					}
				}
			} else {
				result.ExitCode = -1
			}
		}

		// Classify error if not successful
		if !result.Success {
			classifyError(&result)
		}

		saveArtifact(&result)
		return result
	}
}

// setupProcessGroup sets up process group for clean killing
func setupProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
}

// killProcessGroup kills the entire process group
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		// Windows: just kill the process (limited)
		_ = cmd.Process.Kill()
	} else {
		// Unix: kill process group
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// classifyError performs comprehensive error classification
func classifyError(result *TestResult) {
	output := strings.ToLower(result.Combined)

	// Extract stack trace
	result.StackTrace = extractStackTrace(result.Combined)

	// Already classified as cancelled or timeout
	if result.ErrorType != "" && (result.ErrorType == "CIRCUIT_BREAKER_CANCEL" || result.ErrorType == "TIMEOUT") {
		return
	}

	// Check for signals
	if result.Signal != "" {
		result.ErrorType = "SIGNAL_" + strings.ToUpper(result.Signal)
		result.ErrorDetail = fmt.Sprintf("Process killed by signal %s", result.Signal)
		return
	}

	// Extract HTTP status code
	httpStatus := extractHTTPStatus(result.Combined)
	if httpStatus > 0 {
		result.HTTPStatus = httpStatus
	}

	// Categorize by error patterns (order matters)
	switch {
	case strings.Contains(output, "panic:"):
		result.ErrorType = "PANIC"
		result.ErrorDetail = extractPanicMessage(result.Combined)

	case httpStatus == 400:
		result.ErrorType = "HTTP_400_BAD_REQUEST"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 401:
		result.ErrorType = "HTTP_401_UNAUTHORIZED"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 403:
		result.ErrorType = "HTTP_403_FORBIDDEN"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 404:
		result.ErrorType = "HTTP_404_NOT_FOUND"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 408:
		result.ErrorType = "HTTP_408_REQUEST_TIMEOUT"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 409:
		result.ErrorType = "HTTP_409_CONFLICT"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 422:
		result.ErrorType = "HTTP_422_UNPROCESSABLE"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 429:
		result.ErrorType = "HTTP_429_RATE_LIMITED"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 500:
		result.ErrorType = "HTTP_500_INTERNAL_ERROR"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 502:
		result.ErrorType = "HTTP_502_BAD_GATEWAY"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 503:
		result.ErrorType = "HTTP_503_UNAVAILABLE"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus == 504:
		result.ErrorType = "HTTP_504_GATEWAY_TIMEOUT"
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus >= 400 && httpStatus < 500:
		result.ErrorType = fmt.Sprintf("HTTP_%d_CLIENT_ERROR", httpStatus)
		result.ErrorDetail = extractHTTPError(result.Combined)

	case httpStatus >= 500 && httpStatus < 600:
		result.ErrorType = fmt.Sprintf("HTTP_%d_SERVER_ERROR", httpStatus)
		result.ErrorDetail = extractHTTPError(result.Combined)

	case strings.Contains(output, "no such host") || strings.Contains(output, "nxdomain"):
		result.ErrorType = "DNS_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "x509:") || strings.Contains(output, "tls") || strings.Contains(output, "handshake"):
		result.ErrorType = "TLS_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "connection refused"):
		result.ErrorType = "CONNECTION_REFUSED"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "connection reset") || strings.Contains(output, "econnreset"):
		result.ErrorType = "CONNECTION_RESET"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "unexpected eof") || strings.Contains(output, "eof while reading"):
		result.ErrorType = "UNEXPECTED_EOF"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "stream error") || strings.Contains(output, "protocol_error"):
		result.ErrorType = "HTTP2_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "timeout") || strings.Contains(output, "deadline exceeded"):
		result.ErrorType = "TIMEOUT"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "context canceled"):
		result.ErrorType = "CONTEXT_CANCELED"
		result.ErrorDetail = "Context was cancelled"

	case strings.Contains(output, "json_schema") || strings.Contains(output, "not supported"):
		result.ErrorType = "UNSUPPORTED_FEATURE"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "failed to parse") ||
		strings.Contains(output, "all adapters failed") ||
		(strings.Contains(output, "invalid") && strings.Contains(output, "json")) ||
		strings.Contains(output, "unexpected end of json"):
		result.ErrorType = "PARSE_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "missing required") ||
		strings.Contains(output, "validation") ||
		strings.Contains(output, "invalid output") ||
		strings.Contains(output, "invalid class value"):
		result.ErrorType = "VALIDATION_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case strings.Contains(output, "api key") || strings.Contains(output, "unauthorized"):
		result.ErrorType = "AUTH_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)

	case result.ExitCode == 2:
		result.ErrorType = "EXIT_CODE_2"
		result.ErrorDetail = fmt.Sprintf("Process exited with code 2: %s", extractErrorLine(result.Combined))

	case result.ExitCode == 130:
		result.ErrorType = "SIGINT"
		result.ErrorDetail = "Process interrupted (SIGINT)"

	case result.ExitCode == 137:
		result.ErrorType = "SIGKILL_OR_OOM"
		result.ErrorDetail = "Process killed (SIGKILL or OOM)"

	case result.ExitCode == 143:
		result.ErrorType = "SIGTERM"
		result.ErrorDetail = "Process terminated (SIGTERM)"

	case result.ExitCode > 0:
		result.ErrorType = fmt.Sprintf("EXIT_CODE_%d", result.ExitCode)
		result.ErrorDetail = extractErrorLine(result.Combined)

	default:
		result.ErrorType = "UNKNOWN_ERROR"
		result.ErrorDetail = extractErrorLine(result.Combined)
	}

	// Ensure we have an error detail
	if result.ErrorDetail == "" {
		result.ErrorDetail = extractErrorLine(result.Combined)
	}
}

// extractHTTPStatus extracts HTTP status code from output
func extractHTTPStatus(output string) int {
	patterns := []string{
		`HTTP/\d\.\d\s+(\d{3})`,
		`status(?:\s*code)?:\s*(\d{3})`,
		`"status":\s*(\d{3})`,
		`status\s+(\d{3})`,
		`\b(\d{3})\s+[A-Z][a-z]+\s+[A-Z][a-z]+`, // "429 Too Many"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			if code, err := strconv.Atoi(matches[1]); err == nil && code >= 100 && code < 600 {
				return code
			}
		}
	}
	return 0
}

// extractHTTPError extracts HTTP error message
func extractHTTPError(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(lines)-i < 30; i-- {
		line := strings.TrimSpace(lines[i])
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "status") ||
			strings.Contains(lowerLine, "failed") || regexp.MustCompile(`\b\d{3}\b`).MatchString(line) {
			return line
		}
	}
	return extractErrorLine(output)
}

// extractPanicMessage extracts panic message
func extractPanicMessage(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "panic:") {
			return strings.TrimSpace(line)
		}
	}
	return "panic (message not found)"
}

// extractErrorLine extracts the most relevant error line
func extractErrorLine(output string) string {
	if output == "" {
		return "unknown error"
	}

	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(lines)-i < 30; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "exit status") {
			continue
		}
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(line, "panic:") ||
			strings.Contains(lowerLine, "error:") ||
			strings.Contains(lowerLine, "failed") ||
			strings.Contains(lowerLine, "fatal") {
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
		if strings.Contains(line, "panic:") || strings.Contains(line, "goroutine") {
			inStack = true
		}

		if inStack {
			if strings.HasPrefix(line, "\t") ||
				strings.Contains(line, ".go:") ||
				strings.Contains(line, "goroutine") ||
				strings.Contains(line, "panic:") {
				stackTrace = append(stackTrace, line)
			} else if len(stackTrace) > 0 && line == "" {
				break
			}
		}
	}

	if len(stackTrace) > 0 {
		return strings.Join(stackTrace, "\n")
	}

	return ""
}

// saveArtifact saves test artifacts to disk
func saveArtifact(result *TestResult) {
	// Create artifact directory
	timestamp := result.StartTime.Format("20060102_150405")
	modelName := sanitizeFilename(formatModelName(result.Model))
	artifactDir := filepath.Join("test_matrix_logs", modelName, result.Example, timestamp)

	result.ArtifactDir = artifactDir

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create artifact dir: %v\n", err)
		return
	}

	// Save metadata
	metadata := TestArtifact{
		Example:     result.Example,
		Model:       result.Model,
		Success:     result.Success,
		ExitCode:    result.ExitCode,
		Signal:      result.Signal,
		ErrorType:   result.ErrorType,
		ErrorDetail: result.ErrorDetail,
		HTTPStatus:  result.HTTPStatus,
		StartTime:   result.StartTime,
		EndTime:     result.EndTime,
		Duration:    result.Duration.String(),
		Cancelled:   result.Cancelled,
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(artifactDir, "metadata.json"), metadataJSON, 0644)
	}

	// Save stdout
	if result.Stdout != "" {
		_ = os.WriteFile(filepath.Join(artifactDir, "stdout.log"), []byte(result.Stdout), 0644)
	}

	// Save stderr
	if result.Stderr != "" {
		_ = os.WriteFile(filepath.Join(artifactDir, "stderr.log"), []byte(result.Stderr), 0644)
	}

	// Save combined
	if result.Combined != "" {
		_ = os.WriteFile(filepath.Join(artifactDir, "combined.log"), []byte(result.Combined), 0644)
	}

	// Save stack trace if present
	if result.StackTrace != "" {
		_ = os.WriteFile(filepath.Join(artifactDir, "stacktrace.log"), []byte(result.StackTrace), 0644)
	}
}

func printResults(results []TestResult, models []string, duration time.Duration, circuitTripped bool) {
	fmt.Println()
	printSeparator("=")
	fmt.Printf("%s%s TEST RESULTS %s%s\n", colorBold, colorCyan, colorReset, colorBold)
	printSeparator("=")
	fmt.Print(colorReset)

	passed := countPassed(results)
	failed := countFailed(results)
	cancelled := countCancelled(results)
	total := len(results)

	fmt.Println()
	printStat("Total Tests", total)
	printStat("✅ Passed", passed)
	if failed > 0 {
		fmt.Printf("  %s❌ Failed%s      %d (%.1f%%)\n", colorRed, colorReset, failed, float64(failed)/float64(total)*100)
	}
	if cancelled > 0 {
		printStat("🚫 Cancelled", cancelled)
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

	for _, stats := range modelStats {
		successRate := float64(stats.Passed) / float64(stats.Total) * 100
		statusIcon := "✅"
		statusColor := colorGreen
		if stats.Failed > 0 {
			statusIcon = "❌"
			statusColor = colorRed
		}

		fmt.Printf("  %s %s%-50s%s %s%3d/%d%s (%.1f%%)\n",
			statusIcon,
			statusColor, formatModelName(stats.Model), colorReset,
			colorBold, stats.Passed, stats.Total, colorReset,
			successRate)
	}

	// Error type breakdown
	if failed > 0 {
		fmt.Println()
		printSeparator("-")
		fmt.Printf("%s ERROR TYPE BREAKDOWN %s\n", colorBold, colorReset)
		printSeparator("-")

		errorTypes := make(map[string]int)
		for _, r := range results {
			if !r.Success && !r.Cancelled {
				errorTypes[r.ErrorType]++
			}
		}

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
			fmt.Printf("  %s%-40s%s %d\n", colorYellow, ec.typ, colorReset, ec.count)
		}
	}

	fmt.Println()
	printSeparator("=")
}

func calculateModelStats(results []TestResult, models []string) []ModelStats {
	statsMap := make(map[string]*ModelStats)
	for _, model := range models {
		statsMap[model] = &ModelStats{
			Model:   model,
			Results: []TestResult{},
		}
	}

	for _, r := range results {
		if stats, ok := statsMap[r.Model]; ok {
			stats.Total++
			if r.Success {
				stats.Passed++
			} else if !r.Cancelled {
				stats.Failed++
			}
			stats.Results = append(stats.Results, r)
		}
	}

	var statsList []ModelStats
	for _, stats := range statsMap {
		statsList = append(statsList, *stats)
	}
	return statsList
}

func saveErrorSummary(failedResults []TestResult) error {
	summaryPath := "test_matrix_logs/ERROR_SUMMARY.md"

	if err := os.MkdirAll(filepath.Dir(summaryPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(summaryPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Header
	_, _ = fmt.Fprintf(f, "# Test Matrix Error Summary\n\n")
	_, _ = fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(f, "Total Failures: %d\n\n", len(failedResults))

	// Error type breakdown
	_, _ = fmt.Fprintf(f, "## Error Type Breakdown\n\n")
	errorTypes := make(map[string]int)
	for _, r := range failedResults {
		errorTypes[r.ErrorType]++
	}

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
		_, _ = fmt.Fprintf(f, "- **%s**: %d failures\n", ec.typ, ec.count)
	}
	_, _ = fmt.Fprintf(f, "\n")

	// Model breakdown
	_, _ = fmt.Fprintf(f, "## Failures by Model\n\n")
	modelFailures := make(map[string][]TestResult)
	for _, r := range failedResults {
		modelFailures[r.Model] = append(modelFailures[r.Model], r)
	}

	for model, failures := range modelFailures {
		_, _ = fmt.Fprintf(f, "### %s (%d failures)\n\n", formatModelName(model), len(failures))
		for _, r := range failures {
			_, _ = fmt.Fprintf(f, "- **%s** (%s): %s\n", r.Example, r.ErrorType, r.ErrorDetail)
			if r.ArtifactDir != "" {
				_, _ = fmt.Fprintf(f, "  - Artifacts: `%s`\n", r.ArtifactDir)
			}
		}
		_, _ = fmt.Fprintf(f, "\n")
	}

	// Detailed failures
	_, _ = fmt.Fprintf(f, "## Detailed Failure List\n\n")
	for i, r := range failedResults {
		_, _ = fmt.Fprintf(f, "### %d. %s [%s]\n\n", i+1, r.Example, formatModelName(r.Model))
		_, _ = fmt.Fprintf(f, "- **Error Type**: %s\n", r.ErrorType)
		_, _ = fmt.Fprintf(f, "- **Error**: %s\n", r.ErrorDetail)
		if r.HTTPStatus > 0 {
			_, _ = fmt.Fprintf(f, "- **HTTP Status**: %d\n", r.HTTPStatus)
		}
		if r.Signal != "" {
			_, _ = fmt.Fprintf(f, "- **Signal**: %s\n", r.Signal)
		}
		_, _ = fmt.Fprintf(f, "- **Duration**: %s\n", r.Duration)
		_, _ = fmt.Fprintf(f, "- **Exit Code**: %d\n", r.ExitCode)
		if r.ArtifactDir != "" {
			_, _ = fmt.Fprintf(f, "- **Artifacts**: `%s`\n", r.ArtifactDir)
		}

		if r.StackTrace != "" {
			_, _ = fmt.Fprintf(f, "\n**Stack Trace:**\n```\n%s\n```\n", r.StackTrace)
		}

		_, _ = fmt.Fprintf(f, "\n---\n\n")
	}

	return nil
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
		if r.Success {
			count++
		}
	}
	return count
}

func countFailed(results []TestResult) int {
	count := 0
	for _, r := range results {
		if !r.Success && !r.Cancelled {
			count++
		}
	}
	return count
}

func countCancelled(results []TestResult) int {
	count := 0
	for _, r := range results {
		if r.Cancelled {
			count++
		}
	}
	return count
}

func getFailedResults(results []TestResult) []TestResult {
	var failed []TestResult
	for _, r := range results {
		if !r.Success && !r.Cancelled {
			failed = append(failed, r)
		}
	}
	return failed
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
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
	fmt.Fprintf(os.Stderr, "%s⚠️  %s%s\n", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

func printTestResult(result TestResult, current, total int64) {
	icon := "✅"
	statusColor := colorGreen
	status := "PASS"

	if result.Cancelled {
		icon = "🚫"
		statusColor = colorGray
		status = "CNCL"
	} else if !result.Success {
		icon = "❌"
		statusColor = colorRed
		status = "FAIL"
	}

	percent := float64(current) / float64(total) * 100
	modelName := formatModelName(result.Model)

	fmt.Printf("[%s%4d/%d %.1f%%%s] %s %s%-4s%s %s%-20s%s %s%-40s%s %.2fs\n",
		colorGray, current, total, percent, colorReset,
		icon,
		statusColor, status, colorReset,
		colorBold, result.Example, colorReset,
		colorGray, modelName, colorReset,
		result.Duration.Seconds())

	if !result.Success && !result.Cancelled {
		fmt.Printf("       %s↳ [%s] %s%s\n", colorRed, result.ErrorType, result.ErrorDetail, colorReset)
		if result.ArtifactDir != "" {
			fmt.Printf("       %s↳ Artifacts: %s%s\n", colorGray, result.ArtifactDir, colorReset)
		}
	}
}

func printSeparator(char string) {
	fmt.Println(strings.Repeat(char, 80))
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%sError: %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
	os.Exit(1)
}

func disableColors() {
	// Implementation would set all color constants to empty strings
	// For simplicity, using global flag approach
}
