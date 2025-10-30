package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MatrixResult struct {
	Example  string
	Model    string
	Success  bool
	Error    error
	Output   string
	Duration time.Duration
	ExitCode int
}

var models = []string{
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

var examples = []string{
	"examples/adapter_fallback",
	"examples/chat_cot",
	"examples/chat_predict",
	"examples/code_reviewer",
	"examples/composition",
	"examples/content_generator",
	"examples/customer_support",
	"examples/data_analyst",
	"examples/fewshot_conversation",
	"examples/interview",
	"examples/math_solver",
	"examples/react_agent",
	"examples/research_assistant",
	"examples/sentiment",
	"examples/streaming",
}

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	logDir := filepath.Join(projectRoot, "test_matrix_logs")
	if err := os.RemoveAll(logDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Failed to clean log directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	totalRuns := len(models) * len(examples)

	fmt.Println("=== Test Matrix Execution ===")
	fmt.Printf("Models: %d\n", len(models))
	fmt.Printf("Examples: %d\n", len(examples))
	fmt.Printf("Total executions: %d (CONCURRENT)\n", totalRuns)
	fmt.Printf("Log directory: %s\n\n", logDir)

	startTime := time.Now()

	// Pre-create all model directories
	for _, model := range models {
		modelDir := filepath.Join(logDir, sanitizeFilename(model))
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create model directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Launch all executions concurrently
	results := make(chan MatrixResult, totalRuns)
	var wg sync.WaitGroup

	fmt.Println("Launching all executions concurrently...")
	for _, model := range models {
		for _, example := range examples {
			wg.Add(1)
			go func(m, ex string) {
				defer wg.Done()
				result := runExampleWithModel(projectRoot, ex, m)
				results <- result

				// Save log immediately
				modelDir := filepath.Join(logDir, sanitizeFilename(m))
				logFile := filepath.Join(modelDir, sanitizeFilename(filepath.Base(ex))+".log")
				if err := saveLog(logFile, result); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to save log: %v\n", err)
				}
			}(model, example)
		}
	}

	// Close results channel when all done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and display results as they complete
	var allResults []MatrixResult
	completed := 0
	for result := range results {
		completed++
		allResults = append(allResults, result)

		status := "✅"
		if !result.Success {
			status = "❌"
		}
		fmt.Printf("[%d/%d] %s %s with %s (%.2fs, exit: %d)\n",
			completed, totalRuns, status, result.Example, result.Model,
			result.Duration.Seconds(), result.ExitCode)
	}

	totalDuration := time.Since(startTime)

	fmt.Println("\n=== Generating Report ===")
	reportPath := filepath.Join(logDir, "matrix_report.txt")
	if err := generateReport(reportPath, allResults, totalDuration); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Report saved to: %s\n", reportPath)
	fmt.Println("\n=== Evaluating Results ===")

	passed, total := countResults(allResults)
	successRate := float64(passed) / float64(total) * 100

	fmt.Printf("Overall: %d/%d passed (%.1f%%)\n", passed, total, successRate)

	modelStats := make(map[string]struct{ passed, total int })
	for _, result := range allResults {
		stats := modelStats[result.Model]
		stats.total++
		if result.Success {
			stats.passed++
		}
		modelStats[result.Model] = stats
	}

	type ModelScore struct {
		name   string
		passed int
		total  int
		rate   float64
	}

	var modelScores []ModelScore
	for _, model := range models {
		stats := modelStats[model]
		modelRate := float64(stats.passed) / float64(stats.total) * 100
		modelScores = append(modelScores, ModelScore{
			name:   model,
			passed: stats.passed,
			total:  stats.total,
			rate:   modelRate,
		})
	}

	// Sort by success rate (descending), then by model name for stability
	for i := 0; i < len(modelScores); i++ {
		for j := i + 1; j < len(modelScores); j++ {
			if modelScores[j].rate > modelScores[i].rate ||
				(modelScores[j].rate == modelScores[i].rate && modelScores[j].name < modelScores[i].name) {
				modelScores[i], modelScores[j] = modelScores[j], modelScores[i]
			}
		}
	}

	allModelsPassed := true
	for _, score := range modelScores {
		status := "✅"
		if score.rate <= 80.0 {
			status = "❌"
			allModelsPassed = false
		}
		fmt.Printf("%s %s: %d/%d (%.1f%%)\n", status, score.name, score.passed, score.total, score.rate)
	}

	fmt.Println("\n=== Top 3 Models ===")
	for i := 0; i < 3 && i < len(modelScores); i++ {
		rank := i + 1
		score := modelScores[i]
		medal := "🥇"
		if rank == 2 {
			medal = "🥈"
		} else if rank == 3 {
			medal = "🥉"
		}
		fmt.Printf("%s #%d: %s - %d/%d (%.1f%%)\n", medal, rank, score.name, score.passed, score.total, score.rate)
	}

	fmt.Println("\n=== Success Criteria ===")
	totalMinPassCountRequired := int(float64(total) * 0.95)
	overallPass := passed >= int(totalMinPassCountRequired)
	fmt.Printf("Overall success rate: %.1f%% (required: %d/%d = 95.0%%) - %s\n", successRate, totalMinPassCountRequired, total, boolToStatus(overallPass))
	fmt.Printf("All models > 80%%: %s\n", boolToStatus(allModelsPassed))

	if !overallPass || !allModelsPassed {
		fmt.Println("\n❌ Test matrix FAILED")
		os.Exit(1)
	}

	fmt.Println("\n✅ Test matrix PASSED")
}

func runExampleWithModel(projectRoot, examplePath, model string) MatrixResult {
	startTime := time.Now()

	result := MatrixResult{
		Example: filepath.Base(examplePath),
		Model:   model,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mainPath := filepath.Join(examplePath, "main.go")
	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "OPENROUTER_MODEL="+model)

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

func saveLog(logFile string, result MatrixResult) error {
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "Example: %s\n", result.Example)
	fmt.Fprintf(f, "Model: %s\n", result.Model)
	fmt.Fprintf(f, "Success: %v\n", result.Success)
	fmt.Fprintf(f, "Duration: %.2fs\n", result.Duration.Seconds())
	fmt.Fprintf(f, "Exit Code: %d\n", result.ExitCode)
	if result.Error != nil {
		fmt.Fprintf(f, "Error: %v\n", result.Error)
	}
	fmt.Fprintf(f, "\n--- Output ---\n%s\n", result.Output)

	return nil
}

func generateReport(reportPath string, results []MatrixResult, totalDuration time.Duration) error {
	f, err := os.Create(reportPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "=== Test Matrix Report ===\n")
	fmt.Fprintf(f, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "Total Wall-Clock Duration: %.2fs (concurrent execution)\n", totalDuration.Seconds())

	// Calculate duration statistics
	var totalExecTime float64
	var minDuration, maxDuration time.Duration
	for i, result := range results {
		totalExecTime += result.Duration.Seconds()
		if i == 0 || result.Duration < minDuration {
			minDuration = result.Duration
		}
		if i == 0 || result.Duration > maxDuration {
			maxDuration = result.Duration
		}
	}
	avgDuration := totalExecTime / float64(len(results))

	fmt.Fprintf(f, "Total Execution Time (sum): %.2fs\n", totalExecTime)
	fmt.Fprintf(f, "Average Example Duration: %.2fs\n", avgDuration)
	fmt.Fprintf(f, "Min Duration: %.2fs\n", minDuration.Seconds())
	fmt.Fprintf(f, "Max Duration: %.2fs\n\n", maxDuration.Seconds())

	exampleNames := make(map[string]bool)
	for _, result := range results {
		exampleNames[result.Example] = true
	}

	var sortedExamples []string
	for _, example := range examples {
		name := filepath.Base(example)
		if exampleNames[name] {
			sortedExamples = append(sortedExamples, name)
			delete(exampleNames, name)
		}
	}

	resultMap := make(map[string]map[string]bool)
	for _, result := range results {
		if resultMap[result.Model] == nil {
			resultMap[result.Model] = make(map[string]bool)
		}
		resultMap[result.Model][result.Example] = result.Success
	}

	fmt.Fprintf(f, "=== Matrix Table ===\n")
	fmt.Fprintf(f, "%-45s", "Model \\ Example")
	for _, example := range sortedExamples {
		fmt.Fprintf(f, " %-4s", truncate(example, 4))
	}
	fmt.Fprintf(f, " Total\n")
	fmt.Fprintf(f, "%s\n", strings.Repeat("-", 45+len(sortedExamples)*5+7))

	for _, model := range models {
		fmt.Fprintf(f, "%-45s", truncate(model, 45))
		passed := 0
		total := 0
		for _, example := range sortedExamples {
			if resultMap[model][example] {
				fmt.Fprintf(f, " ✅  ")
				passed++
			} else {
				fmt.Fprintf(f, " ❌  ")
			}
			total++
		}
		fmt.Fprintf(f, " %2d/%2d\n", passed, total)
	}

	fmt.Fprintf(f, "\n=== Statistics ===\n")
	passed, total := countResults(results)
	successRate := float64(passed) / float64(total) * 100
	fmt.Fprintf(f, "Overall: %d/%d (%.1f%%)\n\n", passed, total, successRate)

	type ModelScore struct {
		name   string
		passed int
		total  int
		rate   float64
	}

	var modelScores []ModelScore
	for _, model := range models {
		modelPassed := 0
		modelTotal := 0
		for _, result := range results {
			if result.Model == model {
				modelTotal++
				if result.Success {
					modelPassed++
				}
			}
		}
		modelRate := float64(modelPassed) / float64(modelTotal) * 100
		modelScores = append(modelScores, ModelScore{
			name:   model,
			passed: modelPassed,
			total:  modelTotal,
			rate:   modelRate,
		})
	}

	// Sort by success rate (descending)
	for i := 0; i < len(modelScores); i++ {
		for j := i + 1; j < len(modelScores); j++ {
			if modelScores[j].rate > modelScores[i].rate ||
				(modelScores[j].rate == modelScores[i].rate && modelScores[j].name < modelScores[i].name) {
				modelScores[i], modelScores[j] = modelScores[j], modelScores[i]
			}
		}
	}

	fmt.Fprintf(f, "Per-Model Results (sorted by success rate):\n")
	for _, score := range modelScores {
		status := "✅"
		if score.rate <= 80.0 {
			status = "❌"
		}
		fmt.Fprintf(f, "%s %-45s: %2d/%2d (%.1f%%)\n", status, score.name, score.passed, score.total, score.rate)
	}

	fmt.Fprintf(f, "\n=== Top 3 Models ===\n")
	for i := 0; i < 3 && i < len(modelScores); i++ {
		rank := i + 1
		score := modelScores[i]
		medal := "🥇"
		if rank == 2 {
			medal = "🥈"
		} else if rank == 3 {
			medal = "🥉"
		}
		fmt.Fprintf(f, "%s #%d: %s - %d/%d (%.1f%%)\n", medal, rank, score.name, score.passed, score.total, score.rate)
	}

	fmt.Fprintf(f, "\n=== Success Criteria ===\n")
	fmt.Fprintf(f, "Required overall: 152/160 (95.0%%)\n")
	fmt.Fprintf(f, "Actual overall: %d/%d (%.1f%%) - %s\n", passed, total, successRate, boolToStatus(passed >= 152))
	fmt.Fprintf(f, "All models > 80%%: %s\n", boolToStatus(checkAllModelsPass(results)))

	// Add per-example duration breakdown
	fmt.Fprintf(f, "\n=== Per-Example Duration Breakdown ===\n")
	exampleDurations := make(map[string][]time.Duration)
	for _, result := range results {
		exampleDurations[result.Example] = append(exampleDurations[result.Example], result.Duration)
	}

	for _, example := range sortedExamples {
		durations := exampleDurations[example]
		if len(durations) == 0 {
			continue
		}
		var total float64
		var min, max time.Duration
		for i, d := range durations {
			total += d.Seconds()
			if i == 0 || d < min {
				min = d
			}
			if i == 0 || d > max {
				max = d
			}
		}
		avg := total / float64(len(durations))
		fmt.Fprintf(f, "%-25s: avg=%.2fs, min=%.2fs, max=%.2fs, runs=%d\n",
			example, avg, min.Seconds(), max.Seconds(), len(durations))
	}

	// Add per-model duration breakdown
	fmt.Fprintf(f, "\n=== Per-Model Duration Breakdown ===\n")
	modelDurations := make(map[string][]time.Duration)
	for _, result := range results {
		modelDurations[result.Model] = append(modelDurations[result.Model], result.Duration)
	}

	for _, model := range models {
		durations := modelDurations[model]
		if len(durations) == 0 {
			continue
		}
		var total float64
		var min, max time.Duration
		for i, d := range durations {
			total += d.Seconds()
			if i == 0 || d < min {
				min = d
			}
			if i == 0 || d > max {
				max = d
			}
		}
		avg := total / float64(len(durations))
		fmt.Fprintf(f, "%-45s: avg=%.2fs, min=%.2fs, max=%.2fs, runs=%d\n",
			model, avg, min.Seconds(), max.Seconds(), len(durations))
	}

	return nil
}

func countResults(results []MatrixResult) (passed, total int) {
	for _, result := range results {
		total++
		if result.Success {
			passed++
		}
	}
	return
}

func checkAllModelsPass(results []MatrixResult) bool {
	modelStats := make(map[string]struct{ passed, total int })
	for _, result := range results {
		stats := modelStats[result.Model]
		stats.total++
		if result.Success {
			stats.passed++
		}
		modelStats[result.Model] = stats
	}

	for _, stats := range modelStats {
		rate := float64(stats.passed) / float64(stats.total) * 100
		if rate <= 80.0 {
			return false
		}
	}
	return true
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func boolToStatus(b bool) string {
	if b {
		return "✅ PASS"
	}
	return "❌ FAIL"
}
