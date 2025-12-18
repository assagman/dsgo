package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
	"github.com/assagman/dsgo/internal/core"
)

const (
	MaxIterations = 10
	// Limit concurrency to reduce rate limit errors.
	DefaultMaxConcurrent = 3
)

func main() {
	ctx := context.Background()

	// This example is intended for experimentation; allow running models that aren't in the catalog.
	dsgo.Configure()

	models := selectModels()
	if len(models) == 0 {
		fmt.Println("No models selected.\n" +
			"Set OPENROUTER_API_KEY and/or OPENAI_API_KEY, or update examples/react_experiment/main.go.")
		return
	}

	maxConcurrent := DefaultMaxConcurrent
	if v := os.Getenv("DSGO_EXPERIMENT_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrent = n
		}
	}

	fmt.Println("🧪 ReAct Experiment: Core Package Analysis")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("Models: %s\n", strings.Join(models, ", "))
	fmt.Printf("Max Iterations: %d\n", MaxIterations)
	fmt.Printf("Max Concurrent: %d\n", maxConcurrent)
	fmt.Println()

	var wg sync.WaitGroup
	results := make(map[string]*ExperimentResult)
	var mu sync.Mutex

	sem := make(chan struct{}, maxConcurrent)
	for _, modelName := range models {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := runExperiment(ctx, model)
			mu.Lock()
			results[model] = result
			mu.Unlock()
		}(modelName)
	}

	wg.Wait()

	fmt.Println("\n📊 Comparison Results")
	fmt.Println("=" + strings.Repeat("=", 60))
	displayComparison(results)
}

func selectModels() []string {
	hasOpenRouter := os.Getenv("OPENROUTER_API_KEY") != ""
	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""

	// Curated selection:
	// - max 10 models total
	// - 2 models each from: moonshotai, google, openai, z-ai, qwen
	// - avoid ":free" variants (often unstable)
	maxTotal := 10
	perOrg := 2

	type candidate struct {
		id     string
		cost   float64   // PromptPrice + CompletionPrice (USD / 1M tokens)
		newest time.Time // parsed from LastUpdated/ReleaseDate
	}

	parseDate := func(s string) time.Time {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}
		}
		// Allow YYYY-MM-DD, YYYY-MM, or YYYY.
		for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
		return time.Time{}
	}

	pickForPrefix := func(provider, prefix string, max int) []string {
		models := dsgo.ListModelsByProvider(provider)
		cands := make([]candidate, 0, len(models))
		for _, m := range models {
			if !strings.HasPrefix(m.ID, prefix) {
				continue
			}
			// Avoid free-tier models; they tend to be unstable / rate-limited.
			if strings.Contains(m.ID, ":free") {
				continue
			}
			// ReAct relies on tool calling (native when supported).
			if !m.Capabilities.ToolCall {
				continue
			}
			// Some catalog entries use 0 output tokens as "unknown"; avoid those.
			if m.Limits.OutputTokens <= 0 {
				continue
			}

			newest := parseDate(m.Metadata.LastUpdated)
			if newest.IsZero() {
				newest = parseDate(m.Metadata.ReleaseDate)
			}

			cands = append(cands, candidate{
				id:     m.ID,
				cost:   m.Pricing.PromptPrice + m.Pricing.CompletionPrice,
				newest: newest,
			})
		}

		if len(cands) == 0 {
			return nil
		}

		// Pick 1 cheapest and 1 newest (if distinct). This matches "newest and cheapest".
		cheapestIdx := 0
		for i := 1; i < len(cands); i++ {
			if cands[i].cost < cands[cheapestIdx].cost || (cands[i].cost == cands[cheapestIdx].cost && cands[i].id < cands[cheapestIdx].id) {
				cheapestIdx = i
			}
		}

		newestIdx := 0
		for i := 1; i < len(cands); i++ {
			if cands[i].newest.After(cands[newestIdx].newest) {
				newestIdx = i
				continue
			}
			if cands[i].newest.Equal(cands[newestIdx].newest) {
				// tie-breaker: cheaper first
				if cands[i].cost < cands[newestIdx].cost || (cands[i].cost == cands[newestIdx].cost && cands[i].id < cands[newestIdx].id) {
					newestIdx = i
				}
			}
		}

		picked := make([]string, 0, max)
		picked = append(picked, cands[cheapestIdx].id)
		if len(picked) >= max {
			return picked
		}
		if newestIdx != cheapestIdx {
			picked = append(picked, cands[newestIdx].id)
		}

		// If we still need more (e.g. cheapest==newest), fill by (newest desc, cost asc).
		if len(picked) < max {
			sort.Slice(cands, func(i, j int) bool {
				if cands[i].newest.Equal(cands[j].newest) {
					if cands[i].cost == cands[j].cost {
						return cands[i].id < cands[j].id
					}
					return cands[i].cost < cands[j].cost
				}
				return cands[i].newest.After(cands[j].newest)
			})
			for i := 0; i < len(cands) && len(picked) < max; i++ {
				already := false
				for _, id := range picked {
					if id == cands[i].id {
						already = true
						break
					}
				}
				if already {
					continue
				}
				picked = append(picked, cands[i].id)
			}
		}

		return picked
	}

	out := make([]string, 0, maxTotal)

	if hasOpenRouter {
		orgPrefixes := []string{
			"openrouter/moonshotai/",
			"openrouter/google/",
			"openrouter/openai/",
			"openrouter/z-ai/",
			"openrouter/qwen/",
		}
		for _, prefix := range orgPrefixes {
			out = append(out, pickForPrefix("openrouter", prefix, perOrg)...)
		}
		// Exactly 10 (5 orgs * 2 models).
		return out
	}

	// Fallback when OpenRouter isn't configured: only OpenAI direct models can be selected.
	if hasOpenAI {
		return pickForPrefix("openai", "openai/", perOrg)
	}

	return nil
}

// SpyLM wraps an LM to capture interactions
type SpyLM struct {
	dsgo.LM
	Interactions []Interaction
	mu           sync.Mutex
}

type Interaction struct {
	Messages []core.Message
	Response *core.GenerateResult
}

func NewSpyLM(lm dsgo.LM) *SpyLM {
	return &SpyLM{LM: lm}
}

func (s *SpyLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	// Capture the call
	result, err := s.LM.Generate(ctx, messages, options)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err == nil {
		// Deep copy messages to preserve state at this point
		msgs := make([]core.Message, len(messages))
		copy(msgs, messages)

		s.Interactions = append(s.Interactions, Interaction{
			Messages: msgs,
			Response: result,
		})
	}

	return result, err
}

type ExperimentResult struct {
	Model          string
	Success        bool
	Error          error
	Iterations     int
	TotalTokens    int
	Cost           float64
	Latency        time.Duration
	FinalOutput    string
	ToolCalls      []ToolCallInfo
	ThoughtProcess []string
}

type ToolCallInfo struct {
	Step      int
	Tool      string
	Arguments string
	Result    string
}

func runExperiment(ctx context.Context, modelName string) *ExperimentResult {
	fmt.Printf("🔍 Running experiment with %s...\n", modelName)

	baseLM, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		return &ExperimentResult{
			Model:   modelName,
			Success: false,
			Error:   fmt.Errorf("failed to create LM: %w", err),
		}
	}

	// Wrap with SpyLM
	spyLM := NewSpyLM(baseLM)

	filesystemTools := tools.GetAllFilesystemTools()

	// Correctly create Signature using constructor and builder methods
	sig := dsgo.NewSignature("Analysis of core package")
	sig.AddInput("task", dsgo.FieldTypeString, "Analyze the internal/core package to understand its architecture, functionality, and implementation")
	sig.AddOutput("analysis", dsgo.FieldTypeString, "Comprehensive analysis of the core package including architecture, key functions, and design decisions")

	react := dsgo.NewReAct(sig, spyLM, filesystemTools)

	react = react.WithMaxIterations(MaxIterations)
	react = react.WithOptions(&dsgo.GenerateOptions{
		Temperature: 0.3,
		MaxTokens:   2000,
		RetryConfig: &dsgo.RetryConfig{MaxRetries: 2},
	})

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	inputs := map[string]any{
		"task": "Analyze the internal/core package. First, list internal/core and only read files that exist in that listing (do not guess file names). Focus on: overall architecture, LM + Module interfaces, signatures/validation, adapters/parsing flow, prediction/usage metadata, tools and tool calling surfaces, caching, history, settings/config, and collectors.",
	}

	prediction, err := react.Forward(ctx, inputs)

	latency := time.Since(startTime)

	result := &ExperimentResult{
		Model:      modelName,
		Success:    err == nil,
		Error:      err,
		Latency:    latency,
		Iterations: len(spyLM.Interactions), // Approximate iterations based on LM calls
	}

	if prediction != nil {
		result.TotalTokens = prediction.Usage.TotalTokens
		result.Cost = prediction.Usage.Cost
		if analysis, ok := prediction.GetString("analysis"); ok {
			result.FinalOutput = analysis
		}

		// Prefer module-reported iteration count (includes termination and extractor info).
		if it, ok := prediction.Metadata["react_iterations_used"]; ok {
			switch v := it.(type) {
			case int:
				result.Iterations = v
			case int64:
				result.Iterations = int(v)
			case float64:
				result.Iterations = int(v)
			}
		}

		// Process captured interactions to extract thoughts and tool calls
		processInteractions(spyLM.Interactions, result)
	}

	status := "✅"
	if !result.Success {
		status = "❌"
	}
	fmt.Printf("%s %s completed in %v (iterations: %d, cost: $%.6f)\n",
		status, modelName, latency, result.Iterations, result.Cost)

	return result
}

func processInteractions(interactions []Interaction, result *ExperimentResult) {
	for i, interaction := range interactions {
		step := i + 1

		// Extract thought from content (stripping markers)
		content := interaction.Response.Content
		thought := core.StripMarkers(content)
		if thought != "" {
			result.ThoughtProcess = append(result.ThoughtProcess, thought)
		}

		// Extract tool calls
		for _, tc := range interaction.Response.ToolCalls {
			args := fmt.Sprintf("%v", tc.Arguments)
			result.ToolCalls = append(result.ToolCalls, ToolCallInfo{
				Step:      step,
				Tool:      tc.Name,
				Arguments: args,
			})
		}
	}
}

func displayComparison(results map[string]*ExperimentResult) {
	// Sort results by latency for ranking
	type rankedResult struct {
		model  string
		result *ExperimentResult
	}
	var ranked []rankedResult
	for model, result := range results {
		ranked = append(ranked, rankedResult{model, result})
	}
	// Sort by success first, then by latency
	for i := 0; i < len(ranked)-1; i++ {
		for j := i + 1; j < len(ranked); j++ {
			// Successful results come first
			if ranked[j].result.Success && !ranked[i].result.Success {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			} else if ranked[i].result.Success == ranked[j].result.Success {
				// Among same status, sort by latency
				if ranked[j].result.Latency < ranked[i].result.Latency {
					ranked[i], ranked[j] = ranked[j], ranked[i]
				}
			}
		}
	}

	for i, r := range ranked {
		result := r.result
		model := r.model

		fmt.Printf("\n%d. %s\n", i+1, model)
		fmt.Println(strings.Repeat("-", 60))

		if !result.Success {
			fmt.Printf("Status: FAILED\n")
			fmt.Printf("Error: %v\n", result.Error)
			continue
		}

		fmt.Printf("Status: SUCCESS\n")
		fmt.Printf("Iterations Used: %d/%d\n", result.Iterations, MaxIterations)
		fmt.Printf("Total Tokens: %d\n", result.TotalTokens)
		fmt.Printf("Cost: $%.6f\n", result.Cost)
		fmt.Printf("Latency: %v\n", result.Latency)
		fmt.Printf("Tool Calls: %d\n", len(result.ToolCalls))

		if len(result.ToolCalls) > 0 {
			fmt.Println("\nTool Usage (first 5):")
			for j, call := range result.ToolCalls {
				if j >= 5 {
					fmt.Printf("  ... and %d more\n", len(result.ToolCalls)-5)
					break
				}
				fmt.Printf("  Step %d: %s(%s)\n", call.Step, call.Tool, call.Arguments)
			}
		}

		if len(result.ThoughtProcess) > 0 {
			fmt.Println("\nThought Process (first 2 thoughts):")
			for j, thought := range result.ThoughtProcess {
				if j >= 2 {
					fmt.Printf("  ... and %d more\n", len(result.ThoughtProcess)-2)
					break
				}
				lines := strings.Split(thought, "\n")
				firstLine := lines[0]
				if len(firstLine) > 80 {
					firstLine = firstLine[:80] + "..."
				}
				fmt.Printf("  %d. %s\n", j+1, firstLine)
			}
		}

		if result.FinalOutput != "" {
			fmt.Println("\nAnalysis Summary:")
			lines := strings.Split(result.FinalOutput, "\n")
			count := 0
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if count >= 3 {
					fmt.Printf("  ...\n")
					break
				}
				if len(line) > 80 {
					line = line[:80] + "..."
				}
				fmt.Printf("  %s\n", line)
				count++
			}
		}
	}

	// Summary statistics
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📈 Summary Statistics")
	fmt.Println(strings.Repeat("=", 60))

	var successful, failed int
	var totalTokens int
	var totalCost float64
	var fastestLatency time.Duration
	var fastestModel string

	for model, result := range results {
		if result.Success {
			successful++
			totalTokens += result.TotalTokens
			totalCost += result.Cost
			if fastestLatency == 0 || result.Latency < fastestLatency {
				fastestLatency = result.Latency
				fastestModel = model
			}
		} else {
			failed++
		}
	}

	fmt.Printf("\nSuccess Rate: %d/%d (%.0f%%)\n", successful, len(results), float64(successful)/float64(len(results))*100)
	fmt.Printf("Total Tokens Used: %d\n", totalTokens)
	fmt.Printf("Total Cost: $%.6f\n", totalCost)
	if fastestModel != "" {
		fmt.Printf("Fastest Model: %s (%v)\n", fastestModel, fastestLatency)
	}

	// Find model with most tool calls
	var mostToolCalls int
	var mostToolCallsModel string
	for model, result := range results {
		if result.Success && len(result.ToolCalls) > mostToolCalls {
			mostToolCalls = len(result.ToolCalls)
			mostToolCallsModel = model
		}
	}
	if mostToolCallsModel != "" {
		fmt.Printf("Most Tool Calls: %s (%d calls)\n", mostToolCallsModel, mostToolCalls)
	}

	// Find model with fewest iterations (most efficient)
	var fewestIterations = MaxIterations + 1
	var mostEfficientModel string
	for model, result := range results {
		if result.Success && result.Iterations < fewestIterations {
			fewestIterations = result.Iterations
			mostEfficientModel = model
		}
	}
	if mostEfficientModel != "" {
		fmt.Printf("Most Efficient (fewest iterations): %s (%d iterations)\n", mostEfficientModel, fewestIterations)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✨ Experiment Complete")
	fmt.Println("=" + strings.Repeat("=", 60))
}
