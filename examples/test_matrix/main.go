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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Demonstrates: Comprehensive test matrix for all examples across multiple models
// Story: Automated testing with circuit breaker, parallel execution, and detailed error reporting

// ANSI colors
var (
	cReset  = "\033[0m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cGray   = "\033[90m"
	cBold   = "\033[1m"
)

type testResult struct {
	example   string
	model     string
	success   bool
	exitCode  int
	signal    string
	errorType string
	errorMsg  string
	httpCode  int
	duration  time.Duration
	cancelled bool
	artifact  string
	stdout    string
	stderr    string
}

type artifact struct {
	Example   string `json:"example"`
	Model     string `json:"model"`
	Success   bool   `json:"success"`
	ExitCode  int    `json:"exit_code"`
	Signal    string `json:"signal,omitempty"`
	ErrorType string `json:"error_type,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	HTTPCode  int    `json:"http_code,omitempty"`
	Duration  string `json:"duration"`
	Cancelled bool   `json:"cancelled"`
}

type modelStats struct {
	model  string
	total  int
	passed int
	failed int
}

type breaker struct {
	ctx     context.Context
	cancel  context.CancelFunc
	total   int
	failed  int64
	limit   float64
	mu      sync.Mutex
	tripped bool
	reason  string
}

func newBreaker(total int) *breaker {
	ctx, cancel := context.WithCancel(context.Background())
	return &breaker{ctx: ctx, cancel: cancel, total: total, limit: 0.15}
}

func (b *breaker) record(r testResult) {
	if r.success || r.cancelled {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped {
		return
	}
	atomic.AddInt64(&b.failed, 1)
	failed := atomic.LoadInt64(&b.failed)
	max := int64(float64(b.total) * b.limit)
	if failed > max {
		b.trip(fmt.Sprintf("%.1f%% failure rate exceeds %.1f%% (%d/%d)",
			float64(failed)/float64(b.total)*100, b.limit*100, failed, b.total))
	}
}

func (b *breaker) trip(reason string) {
	if b.tripped {
		return
	}
	b.tripped = true
	b.reason = reason
	fmt.Fprintf(os.Stderr, "\n%s🚨 CIRCUIT BREAKER%s\n%s%s%s\n\n", cRed+cBold, cReset, cRed, reason, cReset)
	b.cancel()
}

func (b *breaker) isTripped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tripped
}

// Available models
var allModels = []string{
	"openrouter/qwen/qwen3-235b-a22b-2507",
	"openrouter/z-ai/glm-4.6:exacto",
	"openrouter/minimax/minimax-m2",
	"openrouter/openai/gpt-oss-120b:exacto",
	"openrouter/deepseek/deepseek-v3.1-terminus:exacto",
	"openrouter/moonshotai/kimi-k2-0905:exacto",
	"openrouter/meta-llama/llama-3.3-70b-instruct",
	"openrouter/mistralai/mistral-large",
	"openrouter/anthropic/claude-3.5-sonnet",
}

// Test examples
var allExamples = []string{
	"examples/01-hello-chat",
	"examples/02-agent-tools-react",
	"examples/03-quality-refine-bestof",
	"examples/04-structured-programs",
	"examples/05-resilience-observability",
	"examples/06-parallel",
}

func main() {
	numModels := flag.Int("n", 1, "Number of models: 1=default, N=random N, 0=all")
	verbose := flag.Bool("v", false, "Verbose output")
	timeout := flag.Duration("timeout", 20*time.Minute, "Total timeout")
	maxConcurrent := flag.Int("c", 20, "Max concurrent tests")
	noColor := flag.Bool("no-color", false, "Disable colors")
	flag.Parse()

	if *noColor {
		cReset, cRed, cGreen, cYellow, cCyan, cGray, cBold = "", "", "", "", "", "", ""
	}

	root, err := os.Getwd()
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
		selectedModels = selectRandom(allModels, *numModels)
		printHeader(fmt.Sprintf("Testing %d Random Models", *numModels), *numModels)
	}

	totalTests := len(selectedModels) * len(allExamples)
	printInfo("Examples", len(allExamples))
	printInfo("Total Tests", totalTests)
	printInfo("Max Concurrent", *maxConcurrent)
	printInfo("Timeout", timeout.String())
	fmt.Println()

	// Print models
	fmt.Printf("%sModels:%s\n", cBold, cReset)
	for i, m := range selectedModels {
		fmt.Printf("  %d. %s\n", i+1, shortModel(m))
	}
	fmt.Println()

	start := time.Now()

	// Circuit breaker with timeout
	cb := newBreaker(totalTests)
	timeoutCtx, cancel := context.WithTimeout(cb.ctx, *timeout)
	defer cancel()

	go func() {
		<-timeoutCtx.Done()
		if timeoutCtx.Err() == context.DeadlineExceeded {
			cb.trip(fmt.Sprintf("Total timeout %v exceeded", *timeout))
		}
	}()

	// Run tests
	results := runTests(cb, root, selectedModels, allExamples, 10*time.Minute, *verbose, *maxConcurrent)

	// Print results
	printResults(results, selectedModels, time.Since(start), cb.isTripped())

	// Save error summary
	if failed := getFailed(results); len(failed) > 0 {
		if err := saveErrors(failed); err == nil {
			fmt.Printf("\n%s📊 Error summary: %stest_matrix_logs/ERROR_SUMMARY.md%s\n", cCyan, cBold, cReset)
		}
	}

	// Exit
	if cb.isTripped() {
		os.Exit(2)
	}
	if countPassed(results) < len(results)-countCancelled(results) {
		os.Exit(1)
	}
}

func runTests(cb *breaker, root string, models, examples []string, timeout time.Duration, verbose bool, maxC int) []testResult {
	var results []testResult
	var mu sync.Mutex

	type job struct {
		model, example string
	}

	jobs := make(chan job, len(models)*len(examples))
	resultsCh := make(chan testResult, maxC)
	total := int64(len(models) * len(examples))
	completed := int64(0)

	// Collector
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range resultsCh {
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			cb.record(r)
			current := atomic.AddInt64(&completed, 1)
			printTestResult(r, current, total)
		}
	}()

	// Enqueue
	for _, ex := range examples {
		for _, m := range models {
			jobs <- job{m, ex}
		}
	}
	close(jobs)

	// Workers
	var workerWg sync.WaitGroup
	for i := 0; i < maxC; i++ {
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
					resultsCh <- runTest(cb.ctx, root, j.example, j.model, timeout)
				}
			}
		}()
	}

	workerWg.Wait()
	close(resultsCh)
	wg.Wait()
	return results
}

func runTest(ctx context.Context, root, exPath, model string, timeout time.Duration) testResult {
	start := time.Now()
	exName := filepath.Base(exPath)

	// Create artifact dir
	ts := start.Format("20060102_150405")
	artifactDir := filepath.Join("test_matrix_logs", ts, sanitize(shortModel(model)), exName)
	_ = os.MkdirAll(artifactDir, 0755)

	// Test context
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	cmd := exec.CommandContext(testCtx, "go", "run", filepath.Join(exPath, "main.go"))
	cmd.Dir = root

	// Environment
	env := os.Environ()
	env = append(env,
		"EXAMPLES_DEFAULT_MODEL="+model,
		"DSGO_DEBUG_PARSE=1",
		"DSGO_SAVE_RAW_RESPONSES=true",
		"DSGO_ARTIFACT_DIR="+artifactDir,
	)
	cmd.Env = env

	setupProcessGroup(cmd)

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run
	result := testResult{
		example:  exName,
		model:    model,
		artifact: artifactDir,
		duration: 0,
	}

	err := cmd.Start()
	if err != nil {
		result.duration = time.Since(start)
		result.errorType = "START_FAILURE"
		result.errorMsg = err.Error()
		return result
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-testCtx.Done():
		killProcessGroup(cmd)
		<-done
		result.stdout = stdout.String()
		result.stderr = stderr.String()
		result.duration = time.Since(start)

		if ctx.Err() == context.Canceled {
			result.cancelled = true
			result.errorType = "CIRCUIT_BREAKER"
			result.errorMsg = "Cancelled by circuit breaker"
		} else {
			result.errorType = "TIMEOUT"
			result.errorMsg = fmt.Sprintf("Exceeded %v", timeout)
		}
		classifyError(&result)
		saveArtifact(&result)
		return result

	case err := <-done:
		result.stdout = stdout.String()
		result.stderr = stderr.String()
		result.duration = time.Since(start)

		if err == nil {
			result.success = true
		} else {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.exitCode = exitErr.ExitCode()
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					result.signal = status.Signal().String()
					result.errorType = "SIGNALED"
				}
			} else {
				result.exitCode = -1
			}
			classifyError(&result)
		}
		saveArtifact(&result)
		return result
	}
}

func setupProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	} else {
		_ = cmd.Process.Kill()
	}
}

func classifyError(r *testResult) {
	output := r.stdout + "\n" + r.stderr
	lowerOut := strings.ToLower(output)

	// Extract HTTP status (must be preceded by "http" or "status")
	httpRe := regexp.MustCompile(`(?i)(?:http|status)[:\s]+(4\d\d|5\d\d)`)
	if matches := httpRe.FindStringSubmatch(output); len(matches) > 1 {
		_, _ = fmt.Sscanf(matches[1], "%d", &r.httpCode)
	}

	// Classify error type
	if r.errorType == "" {
		switch {
		case strings.Contains(lowerOut, "panic"):
			r.errorType = "PANIC"
			r.errorMsg = extractPanic(output)
		case strings.Contains(lowerOut, "failed to parse output") || strings.Contains(lowerOut, "no json object found"):
			r.errorType = "PARSER_ERROR"
			r.errorMsg = extractParserError(output)
		case r.httpCode >= 400:
			r.errorType = fmt.Sprintf("HTTP_%d", r.httpCode)
			r.errorMsg = extractHTTPError(output)
		case strings.Contains(lowerOut, "timeout"):
			r.errorType = "TIMEOUT"
			r.errorMsg = extractError(output)
		case strings.Contains(lowerOut, "rate limit"):
			r.errorType = "RATE_LIMIT"
			r.errorMsg = extractError(output)
		case strings.Contains(lowerOut, "api key"):
			r.errorType = "API_KEY"
			r.errorMsg = extractError(output)
		case strings.Contains(lowerOut, "max_tokens") || strings.Contains(lowerOut, "finish_reason=length"):
			r.errorType = "MAX_TOKENS"
			r.errorMsg = extractError(output)
		default:
			r.errorType = "UNKNOWN"
			r.errorMsg = extractError(output)
		}
	}

	if r.errorMsg == "" {
		r.errorMsg = extractError(output)
	}
}

func extractParserError(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(lines)-i < 50; i-- {
		line := strings.TrimSpace(lines[i])
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "failed to parse") ||
			strings.Contains(lowerLine, "no json object") ||
			strings.Contains(lowerLine, "adapter") ||
			strings.Contains(lowerLine, "required field") {
			return line
		}
	}
	return "Output format parsing failed"
}

func extractHTTPError(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(lines)-i < 30; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(strings.ToLower(line), "status") ||
			regexp.MustCompile(`\b\d{3}\b`).MatchString(line) {
			return line
		}
	}
	return extractError(output)
}

func extractPanic(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "panic:") {
			return strings.TrimSpace(line)
		}
	}
	return "panic (message not found)"
}

func extractError(output string) string {
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
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "exit status") {
			return line
		}
	}
	return "unknown error"
}

func saveArtifact(r *testResult) {
	if r.artifact == "" {
		return
	}

	meta := artifact{
		Example:   r.example,
		Model:     r.model,
		Success:   r.success,
		ExitCode:  r.exitCode,
		Signal:    r.signal,
		ErrorType: r.errorType,
		ErrorMsg:  r.errorMsg,
		HTTPCode:  r.httpCode,
		Duration:  r.duration.String(),
		Cancelled: r.cancelled,
	}

	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(r.artifact, "metadata.json"), data, 0644)
	}
	if r.stdout != "" {
		_ = os.WriteFile(filepath.Join(r.artifact, "stdout.log"), []byte(r.stdout), 0644)
	}
	if r.stderr != "" {
		_ = os.WriteFile(filepath.Join(r.artifact, "stderr.log"), []byte(r.stderr), 0644)
	}
}

func printResults(results []testResult, models []string, duration time.Duration, tripped bool) {
	fmt.Println()
	printSep("=")
	fmt.Printf("%s%s TEST RESULTS %s%s\n", cBold, cCyan, cReset, cBold)
	printSep("=")
	fmt.Print(cReset)

	passed := countPassed(results)
	failed := countFailed(results)
	cancelled := countCancelled(results)
	total := len(results)

	fmt.Println()
	printStat("Total", total)
	printStat("✅ Passed", passed)
	if failed > 0 {
		fmt.Printf("  %s❌ Failed%s      %d (%.1f%%)\n", cRed, cReset, failed, float64(failed)/float64(total)*100)
	}
	if cancelled > 0 {
		printStat("🚫 Cancelled", cancelled)
	}
	printStat("⏱️  Duration", fmtDuration(duration))

	// Model scores
	fmt.Println()
	printSep("-")
	fmt.Printf("%s MODEL SCORES %s\n", cBold, cReset)
	printSep("-")

	stats := calcStats(results, models)
	sort.Slice(stats, func(i, j int) bool {
		return float64(stats[i].passed)/float64(stats[i].total) > float64(stats[j].passed)/float64(stats[j].total)
	})

	for _, s := range stats {
		rate := float64(s.passed) / float64(s.total) * 100
		icon, color := "✅", cGreen
		if s.failed > 0 {
			icon, color = "❌", cRed
		}
		fmt.Printf("  %s %s%-50s%s %s%3d/%d%s (%.1f%%)\n",
			icon, color, shortModel(s.model), cReset, cBold, s.passed, s.total, cReset, rate)
	}

	// Error breakdown
	if failed > 0 {
		fmt.Println()
		printSep("-")
		fmt.Printf("%s ERROR TYPES %s\n", cBold, cReset)
		printSep("-")

		errTypes := make(map[string]int)
		for _, r := range results {
			if !r.success && !r.cancelled {
				errTypes[r.errorType]++
			}
		}

		type ec struct {
			typ   string
			count int
		}
		var counts []ec
		for t, c := range errTypes {
			counts = append(counts, ec{t, c})
		}
		sort.Slice(counts, func(i, j int) bool { return counts[i].count > counts[j].count })

		for _, e := range counts {
			fmt.Printf("  %s%-40s%s %d\n", cYellow, e.typ, cReset, e.count)
		}
	}

	fmt.Println()
	printSep("=")
}

func calcStats(results []testResult, models []string) []modelStats {
	statsMap := make(map[string]*modelStats)
	for _, m := range models {
		statsMap[m] = &modelStats{model: m}
	}

	for _, r := range results {
		if s, ok := statsMap[r.model]; ok {
			s.total++
			if r.success {
				s.passed++
			} else if !r.cancelled {
				s.failed++
			}
		}
	}

	var list []modelStats
	for _, s := range statsMap {
		list = append(list, *s)
	}
	return list
}

func saveErrors(failed []testResult) error {
	path := "test_matrix_logs/ERROR_SUMMARY.md"
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(f, "# Test Matrix Error Summary\n\n")
	_, _ = fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(f, "Total Failures: %d\n\n", len(failed))

	// Error types
	_, _ = fmt.Fprintf(f, "## Error Types\n\n")
	errTypes := make(map[string]int)
	for _, r := range failed {
		errTypes[r.errorType]++
	}
	type ec struct {
		typ   string
		count int
	}
	var counts []ec
	for t, c := range errTypes {
		counts = append(counts, ec{t, c})
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i].count > counts[j].count })
	for _, e := range counts {
		_, _ = fmt.Fprintf(f, "- **%s**: %d\n", e.typ, e.count)
	}
	_, _ = fmt.Fprintf(f, "\n")

	// Details
	_, _ = fmt.Fprintf(f, "## Failed Tests\n\n")
	for i, r := range failed {
		_, _ = fmt.Fprintf(f, "### %d. %s [%s]\n\n", i+1, r.example, shortModel(r.model))
		_, _ = fmt.Fprintf(f, "- **Type**: %s\n", r.errorType)
		_, _ = fmt.Fprintf(f, "- **Error**: %s\n", r.errorMsg)
		if r.httpCode > 0 {
			_, _ = fmt.Fprintf(f, "- **HTTP**: %d\n", r.httpCode)
		}
		if r.signal != "" {
			_, _ = fmt.Fprintf(f, "- **Signal**: %s\n", r.signal)
		}
		_, _ = fmt.Fprintf(f, "- **Duration**: %s\n", r.duration)
		if r.artifact != "" {
			_, _ = fmt.Fprintf(f, "- **Artifacts**: `%s`\n", r.artifact)
		}
		_, _ = fmt.Fprintf(f, "\n---\n\n")
	}

	return nil
}

func selectRandom(models []string, n int) []string {
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
	if m := os.Getenv("EXAMPLES_DEFAULT_MODEL"); m != "" {
		return m
	}
	return "openrouter/qwen/qwen3-235b-a22b-2507"
}

func shortModel(model string) string {
	return strings.TrimPrefix(model, "openrouter/")
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func sanitize(s string) string {
	return strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(s)
}

func countPassed(results []testResult) int {
	count := 0
	for _, r := range results {
		if r.success {
			count++
		}
	}
	return count
}

func countFailed(results []testResult) int {
	count := 0
	for _, r := range results {
		if !r.success && !r.cancelled {
			count++
		}
	}
	return count
}

func countCancelled(results []testResult) int {
	count := 0
	for _, r := range results {
		if r.cancelled {
			count++
		}
	}
	return count
}

func getFailed(results []testResult) []testResult {
	var failed []testResult
	for _, r := range results {
		if !r.success && !r.cancelled {
			failed = append(failed, r)
		}
	}
	return failed
}

// Printing utilities
func printHeader(title string, count int) {
	fmt.Println()
	printSep("=")
	fmt.Printf("%s%s %s (%d) %s%s\n", cBold, cCyan, title, count, cReset, cBold)
	printSep("=")
	fmt.Print(cReset)
	fmt.Println()
}

func printInfo(label string, value interface{}) {
	fmt.Printf("  %s%-15s%s %v\n", cGray, label+":", cReset, value)
}

func printStat(label string, value interface{}) {
	fmt.Printf("  %s%-15s%s %v\n", cBold, label+":", cReset, value)
}

func printTestResult(r testResult, current, total int64) {
	icon, color, status := "✅", cGreen, "PASS"
	if r.cancelled {
		icon, color, status = "🚫", cGray, "CNCL"
	} else if !r.success {
		icon, color, status = "❌", cRed, "FAIL"
	}

	pct := float64(current) / float64(total) * 100
	fmt.Printf("[%s%4d/%d %.1f%%%s] %s %s%-4s%s %s%-20s%s %s%-40s%s %.2fs\n",
		cGray, current, total, pct, cReset,
		icon,
		color, status, cReset,
		cBold, r.example, cReset,
		cGray, shortModel(r.model), cReset,
		r.duration.Seconds())

	if !r.success && !r.cancelled {
		fmt.Printf("       %s↳ [%s] %s%s\n", cRed, r.errorType, r.errorMsg, cReset)
		if r.artifact != "" {
			fmt.Printf("       %s↳ %s%s\n", cGray, r.artifact, cReset)
		}
	}
}

func printSep(char string) {
	fmt.Println(strings.Repeat(char, 80))
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%sError: %s%s\n", cRed, fmt.Sprintf(format, args...), cReset)
	os.Exit(1)
}
