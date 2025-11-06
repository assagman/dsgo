package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/assagman/dsgo"
)

type Config struct {
	Concurrency  int
	Timeout      time.Duration
	ErrorDumpDir string
	OutputFormat string
	Verbose      bool
}

type ExecutionStats struct {
	StartTime  time.Time      `json:"start_time"`
	EndTime    time.Time      `json:"end_time"`
	Duration   time.Duration  `json:"duration_ms"`
	TokensUsed int            `json:"tokens_used"`
	CacheHits  int            `json:"cache_hits"`
	Retries    int            `json:"retries"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ExecutionResult struct {
	ExampleName string           `json:"example_name"`
	Stats       ExecutionStats   `json:"stats"`
	Prediction  *dsgo.Prediction `json:"-"`
}

type ExampleFunc func(ctx context.Context) (*dsgo.Prediction, *ExecutionStats, error)

type Harness struct {
	config Config
	mu     sync.Mutex
	stats  []ExecutionResult
}

func NewHarness(config Config) *Harness {
	if config.Concurrency <= 0 {
		config.Concurrency = 16
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.ErrorDumpDir == "" {
		config.ErrorDumpDir = "examples/errors"
	}
	if config.OutputFormat == "" {
		config.OutputFormat = "json"
	}

	os.MkdirAll(config.ErrorDumpDir, 0755)

	return &Harness{
		config: config,
		stats:  make([]ExecutionResult, 0),
	}
}

func (h *Harness) Run(ctx context.Context, exampleName string, fn ExampleFunc) error {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	start := time.Now()
	pred, stats, err := fn(ctx)

	if stats == nil {
		stats = &ExecutionStats{}
	}

	stats.StartTime = start
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	if err != nil {
		stats.Status = "FAIL"
		stats.Error = err.Error()
		h.dumpError(exampleName, pred, err)
	} else {
		stats.Status = "PASS"
	}

	result := ExecutionResult{
		ExampleName: exampleName,
		Stats:       *stats,
		Prediction:  pred,
	}

	h.mu.Lock()
	h.stats = append(h.stats, result)
	h.mu.Unlock()

	if h.config.Verbose {
		h.printResult(result)
	}

	return err
}

func (h *Harness) RunBatch(ctx context.Context, examples map[string]ExampleFunc) []ExecutionResult {
	semaphore := make(chan struct{}, h.config.Concurrency)
	var wg sync.WaitGroup

	for name, fn := range examples {
		wg.Add(1)
		go func(n string, f ExampleFunc) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := h.Run(ctx, n, f)
			if err != nil && h.config.Verbose {
				fmt.Fprintf(os.Stderr, "[ERROR] %s: %v\n", n, err)
			}
		}(name, fn)
	}

	wg.Wait()

	h.mu.Lock()
	results := append([]ExecutionResult{}, h.stats...)
	h.mu.Unlock()

	return results
}

func (h *Harness) dumpError(exampleName string, pred *dsgo.Prediction, err error) {
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(h.config.ErrorDumpDir, fmt.Sprintf("%s_%s.json", exampleName, timestamp))

	dump := map[string]any{
		"example":   exampleName,
		"timestamp": timestamp,
		"error":     err.Error(),
	}

	if pred != nil {
		dump["outputs"] = pred.Outputs
		dump["rationale"] = pred.Rationale
		dump["usage"] = pred.Usage
		dump["module_name"] = pred.ModuleName
		dump["inputs"] = pred.Inputs
		dump["adapter_used"] = pred.AdapterUsed
		dump["parse_diagnostics"] = pred.ParseDiagnostics
	}

	data, _ := json.MarshalIndent(dump, "", "  ")
	os.WriteFile(filename, data, 0644)
}

func (h *Harness) printResult(result ExecutionResult) {
	status := "✓"
	if result.Stats.Status == "FAIL" {
		status = "✗"
	}

	fmt.Printf("%s %s (%.2fms, %d tokens, %d cache hits, %d retries)\n",
		status,
		result.ExampleName,
		float64(result.Stats.Duration.Milliseconds()),
		result.Stats.TokensUsed,
		result.Stats.CacheHits,
		result.Stats.Retries,
	)

	if result.Stats.Error != "" {
		fmt.Printf("  Error: %s\n", result.Stats.Error)
	}
}

func (h *Harness) OutputResults() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.config.OutputFormat {
	case "json":
		return h.outputJSON()
	case "ndjson":
		return h.outputNDJSON()
	default:
		return h.outputSummary()
	}
}

func (h *Harness) outputJSON() error {
	data, err := json.MarshalIndent(h.stats, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func (h *Harness) outputNDJSON() error {
	for _, result := range h.stats {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}

func (h *Harness) outputSummary() error {
	passed := 0
	failed := 0
	totalDuration := time.Duration(0)
	totalTokens := 0
	totalCacheHits := 0
	totalRetries := 0

	for _, result := range h.stats {
		if result.Stats.Status == "PASS" {
			passed++
		} else {
			failed++
		}
		totalDuration += result.Stats.Duration
		totalTokens += result.Stats.TokensUsed
		totalCacheHits += result.Stats.CacheHits
		totalRetries += result.Stats.Retries
	}

	fmt.Println("\n=== Execution Summary ===")
	fmt.Printf("Total:        %d\n", len(h.stats))
	fmt.Printf("Passed:       %d\n", passed)
	fmt.Printf("Failed:       %d\n", failed)
	fmt.Printf("Duration:     %.2fs\n", totalDuration.Seconds())
	fmt.Printf("Tokens:       %d\n", totalTokens)
	fmt.Printf("Cache Hits:   %d\n", totalCacheHits)
	fmt.Printf("Retries:      %d\n", totalRetries)

	if failed > 0 {
		fmt.Println("\nFailed examples:")
		for _, result := range h.stats {
			if result.Stats.Status == "FAIL" {
				fmt.Printf("  ✗ %s: %s\n", result.ExampleName, result.Stats.Error)
			}
		}
	}

	return nil
}

func (h *Harness) GetStats() []ExecutionResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]ExecutionResult{}, h.stats...)
}
