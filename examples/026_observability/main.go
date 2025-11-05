package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "026_observability", runExample)
	if err != nil {
		log.Fatal(err)
	}

	if err := h.OutputResults(); err != nil {
		log.Fatal(err)
	}
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
	stats := &harness.ExecutionStats{
		Metadata: make(map[string]any),
	}

	var totalTokens int
	var lastPred *dsgo.Prediction

	fmt.Println("=== Observability Example ===")
	fmt.Println("Demonstrates comprehensive observability with metadata, history tracking, and streaming")
	fmt.Println()

	fmt.Println("--- Observability Features ---")
	fmt.Println("✓ Provider metadata extraction (usage, cache, rate limits)")
	fmt.Println("✓ History tracking with MemoryCollector")
	fmt.Println("✓ Streaming observability with LMWrapper")
	fmt.Println("✓ Complete metrics for all interaction types")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 1: Metadata Extraction
	fmt.Println("--- Demo 1: Provider Metadata Extraction ---")
	fmt.Println("Extract rich metadata from provider responses")
	fmt.Println()

	lm := shared.GetLM(shared.GetModel())

	messages := []dsgo.Message{
		{Role: "user", Content: "What are the three primary colors?"},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Temperature = 0.7
	options.MaxTokens = 100

	fmt.Println("Making API call...")
	result, err := lm.Generate(ctx, messages, options)
	if err != nil {
		return nil, stats, fmt.Errorf("generation failed: %w", err)
	}

	fmt.Printf("\nResponse: %s\n", result.Content)

	// Display usage metrics
	fmt.Println("\n💰 Usage Metrics:")
	fmt.Printf("  Prompt Tokens:     %d\n", result.Usage.PromptTokens)
	fmt.Printf("  Completion Tokens: %d\n", result.Usage.CompletionTokens)
	fmt.Printf("  Total Tokens:      %d\n", result.Usage.TotalTokens)
	if result.Usage.Cost > 0 {
		fmt.Printf("  Estimated Cost:    $%.6f\n", result.Usage.Cost)
	}
	if result.Usage.Latency > 0 {
		fmt.Printf("  Latency:           %dms\n", result.Usage.Latency)
	}

	// Display provider-specific metadata
	if len(result.Metadata) > 0 {
		fmt.Println("\n🔍 Provider Metadata:")

		if cacheStatus, ok := result.Metadata["cache_status"].(string); ok {
			fmt.Printf("  Cache Status:      %s\n", cacheStatus)
		}
		if cacheHit, ok := result.Metadata["cache_hit"].(bool); ok {
			fmt.Printf("  Cache Hit:         %v\n", cacheHit)
		}
		if reqID, ok := result.Metadata["request_id"].(string); ok {
			fmt.Printf("  Request ID:        %s\n", reqID)
		}
		if genID, ok := result.Metadata["generation_id"].(string); ok {
			fmt.Printf("  Generation ID:     %s\n", genID)
		}

		// Rate limit information
		if limit, ok := result.Metadata["rate_limit_requests"].(string); ok {
			fmt.Printf("  Rate Limit:        %s requests\n", limit)
		}
		if remaining, ok := result.Metadata["rate_limit_remaining_requests"].(string); ok {
			fmt.Printf("  Rate Remaining:    %s requests\n", remaining)
		}
	}

	fmt.Printf("\n✅ Finish Reason: %s\n", result.FinishReason)

	totalTokens += result.Usage.TotalTokens
	stats.Metadata["demo1_metadata_keys"] = len(result.Metadata)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: History Tracking with Collector
	fmt.Println("--- Demo 2: History Tracking & MemoryCollector ---")
	fmt.Println("Automatic collection of HistoryEntry records")
	fmt.Println()

	// Create a memory collector to track history
	collector := dsgo.NewMemoryCollector(100)

	// Configure global settings with collector
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel(shared.GetModel()),
		dsgo.WithCollector(collector),
	)

	// Create LM - automatically wrapped with observability
	lm2 := shared.GetLM(shared.GetModel())

	// Make a few calls
	fmt.Println("Making 2 API calls to track history...")

	for i := 1; i <= 2; i++ {
		messages := []dsgo.Message{
			{Role: "user", Content: fmt.Sprintf("Say 'Call %d' and nothing else.", i)},
		}

		options := dsgo.DefaultGenerateOptions()
		options.Temperature = 0.7
		options.MaxTokens = 20

		result, err := lm2.Generate(ctx, messages, options)
		if err != nil {
			log.Printf("Call %d failed: %v", i, err)
			continue
		}
		fmt.Printf("  Call %d: %s\n", i, result.Content)
		totalTokens += result.Usage.TotalTokens
		lastPred = &dsgo.Prediction{
			Outputs: map[string]any{"response": result.Content},
			Usage:   result.Usage,
		}
	}

	// Inspect collected history
	fmt.Println("\n📋 Collected History Entries:")
	entries := collector.GetAll()
	fmt.Printf("  Total entries: %d\n", len(entries))

	for i, entry := range entries {
		fmt.Printf("\n  --- Entry %d ---\n", i+1)
		fmt.Printf("  ID:            %s\n", entry.ID[:8]+"...")
		fmt.Printf("  Session ID:    %s\n", entry.SessionID[:8]+"...")
		fmt.Printf("  Provider:      %s\n", entry.Provider)
		fmt.Printf("  Model:         %s\n", entry.Model)
		fmt.Printf("  Timestamp:     %s\n", entry.Timestamp.Format("15:04:05"))
		fmt.Printf("  Tokens:        %d prompt + %d completion = %d total\n",
			entry.Usage.PromptTokens, entry.Usage.CompletionTokens, entry.Usage.TotalTokens)
		fmt.Printf("  Cost:          $%.6f\n", entry.Usage.Cost)
		fmt.Printf("  Latency:       %dms\n", entry.Usage.Latency)
		fmt.Printf("  Cache Hit:     %v\n", entry.Cache.Hit)
		if entry.ProviderMeta != nil {
			fmt.Printf("  Metadata keys: %d\n", len(entry.ProviderMeta))
		}
	}

	// Save to JSON file
	if len(entries) > 0 {
		data, err := json.MarshalIndent(entries[0], "", "  ")
		if err == nil {
			filename := "observability_sample.json"
			if err := os.WriteFile(filename, data, 0644); err == nil {
				fmt.Printf("\n  ✓ Saved sample entry to %s\n", filename)
			}
		}
	}

	stats.Metadata["demo2_entries_collected"] = len(entries)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Streaming Observability
	fmt.Println("--- Demo 3: Streaming with Observability ---")
	fmt.Println("Complete tracking for streaming LM calls")
	fmt.Println()

	model := shared.GetModel()
	lm3 := shared.GetLM(model)

	// Create collector for streaming
	streamCollector := dsgo.NewMemoryCollector(10)

	// Wrap LM to enable observability
	wrappedLM := dsgo.NewLMWrapper(lm3, streamCollector)

	fmt.Printf("Using model: %s\n", model)

	// Demo 3a: Standard Generate call
	fmt.Println("\n🔹 Standard Generate Call:")
	messages3a := []dsgo.Message{
		{Role: "user", Content: "Write a haiku about Go programming."},
	}
	options3a := dsgo.DefaultGenerateOptions()
	options3a.Temperature = 0.7
	options3a.MaxTokens = 100

	result3a, err := wrappedLM.Generate(ctx, messages3a, options3a)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Response: %s\n", result3a.Content)
		fmt.Printf("Usage: %d tokens, $%.6f, %dms\n",
			result3a.Usage.TotalTokens, result3a.Usage.Cost, result3a.Usage.Latency)
		totalTokens += result3a.Usage.TotalTokens
		lastPred = &dsgo.Prediction{
			Outputs: map[string]any{"haiku": result3a.Content},
			Usage:   result3a.Usage,
		}
	}

	// Demo 3b: Streaming call
	fmt.Println("\n🔹 Streaming Call:")
	messages3b := []dsgo.Message{
		{Role: "user", Content: "Explain benefits of streaming in two sentences."},
	}
	options3b := dsgo.DefaultGenerateOptions()
	options3b.Temperature = 0.7
	options3b.MaxTokens = 100

	chunkChan, errChan := wrappedLM.Stream(ctx, messages3b, options3b)

	var fullContent string
	fmt.Print("Response: ")
	for chunk := range chunkChan {
		fullContent += chunk.Content
		fmt.Print(chunk.Content)
	}

	if err := <-errChan; err != nil {
		log.Printf("\nError: %v", err)
	} else {
		fmt.Println()
	}

	// Inspect observability data
	streamEntries := streamCollector.GetAll()
	if len(streamEntries) > 0 {
		streamEntry := streamEntries[len(streamEntries)-1]
		fmt.Printf("Usage: %d tokens, $%.6f, %dms\n",
			streamEntry.Usage.TotalTokens, streamEntry.Usage.Cost, streamEntry.Usage.Latency)
		totalTokens += streamEntry.Usage.TotalTokens
	}

	// Compare Generate vs Stream
	if len(streamEntries) >= 2 {
		fmt.Println("\n📊 Generate vs Stream Comparison:")
		genEntry := streamEntries[len(streamEntries)-2]
		streamEntry := streamEntries[len(streamEntries)-1]

		fmt.Printf("%-20s %-15s %-15s\n", "Metric", "Generate", "Stream")
		fmt.Println(strings.Repeat("─", 52))
		fmt.Printf("%-20s %-15d %-15d\n", "Tokens", genEntry.Usage.TotalTokens, streamEntry.Usage.TotalTokens)
		fmt.Printf("%-20s $%-14.6f $%-14.6f\n", "Cost", genEntry.Usage.Cost, streamEntry.Usage.Cost)
		fmt.Printf("%-20s %-14dms %-14dms\n", "Latency", genEntry.Usage.Latency, streamEntry.Usage.Latency)

		fmt.Println("\n✅ Both methods produce complete observability data!")
	}

	stats.Metadata["demo3_stream_entries"] = len(streamEntries)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- Observability Benefits ---")
	fmt.Println("✓ Automatic metadata collection (no manual instrumentation)")
	fmt.Println("✓ Complete metrics for all interaction types (Generate, Stream)")
	fmt.Println("✓ Flexible collectors (memory, file, composite)")
	fmt.Println("✓ Production-ready (best-effort, never fails calls)")
	fmt.Println("✓ Usage tracking (tokens, costs, latency)")
	fmt.Println("✓ Cache tracking (hits, misses, sources)")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("Observability provides:")
	fmt.Println("  ✓ Rich provider metadata extraction")
	fmt.Println("  ✓ Automatic history tracking with MemoryCollector")
	fmt.Println("  ✓ Complete streaming observability")
	fmt.Println("  ✓ Production-grade monitoring and debugging")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 3\n")
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 3

	return lastPred, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
