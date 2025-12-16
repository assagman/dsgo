package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
	"github.com/assagman/dsgo/internal/core"
)

const MaxIterations = 10

func main() {
	ctx := context.Background()

	models := []string{
		"openrouter/google/gemini-2.5-flash-lite-preview-09-2025",
		"openrouter/openai/gpt-4o-mini",
		"openrouter/openai/gpt-5-mini-2025-08-07",
		"openrouter/openai/gpt-5-nano-2025-08-07",
		"openrouter/openai/gpt-4.1-2025-04-14",
		"openrouter/google/gemini-2.5-flash",
		"openrouter/x-ai/grok-code-fast-1",
		"openrouter/deepseek/deepseek-v3.2",
		"openrouter/qwen/qwen3-next-80b-a3b-instruct",
		"openrouter/z-ai/glm-4.6:exacto",
		"openrouter/moonshotai/kimi-k2-0905:exacto",
		"openrouter/openai/gpt-oss-120b:exacto",
		"openrouter/qwen/qwen3-coder:exacto",
	}

	fmt.Println("🧪 ReAct Experiment: Cost Package Analysis")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("Models: %s\n", strings.Join(models, ", "))
	fmt.Printf("Max Iterations: %d\n", MaxIterations)
	fmt.Println()

	var wg sync.WaitGroup
	results := make(map[string]*ExperimentResult)
	var mu sync.Mutex

	for _, modelName := range models {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()
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
	})

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	inputs := map[string]any{
		"task": "Analyze the internal/core package. Read the source files to understand: 1) The overall architecture and design, 2) Key interfaces like LM, Module, and Provider, 3) The pipeline and execution flow, 4) Error handling and logging, 5) Test coverage and examples. Use the filesystem tools to read actual source code.",
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
