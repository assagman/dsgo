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
)

// Comprehensive observability example demonstrating:
// 1. Metadata extraction from provider responses
// 2. HistoryEntry tracking with MemoryCollector
// 3. Streaming with automatic observability collection

func main() {
	shared.LoadEnv()
	ctx := context.Background()

	fmt.Println("=== DSGo Observability Example ===")
	fmt.Println()

	// Demo 1: Metadata Extraction
	fmt.Println("📊 Demo 1: Provider Metadata Extraction")
	fmt.Println(strings.Repeat("─", 50))
	demoMetadataExtraction(ctx)

	fmt.Println()

	// Demo 2: History Tracking with Collector
	fmt.Println("📊 Demo 2: History Tracking & MemoryCollector")
	fmt.Println(strings.Repeat("─", 50))
	demoHistoryTracking(ctx)

	fmt.Println()

	// Demo 3: Streaming Observability
	fmt.Println("📊 Demo 3: Streaming with Observability")
	fmt.Println(strings.Repeat("─", 50))
	demoStreamingObservability(ctx)

	fmt.Println("\n=== All Demos Complete ===")
}

// Demo 1: Shows how to extract provider-specific metadata from responses
func demoMetadataExtraction(ctx context.Context) {
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
		log.Fatalf("Generation failed: %v", err)
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
}

// Demo 2: Shows HistoryEntry tracking with MemoryCollector
func demoHistoryTracking(ctx context.Context) {
	// Create a memory collector to track history
	collector := dsgo.NewMemoryCollector(100)

	// Configure global settings with collector
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel(shared.GetModel()),
		dsgo.WithCollector(collector),
	)

	// Create LM - automatically wrapped with observability
	lm := shared.GetLM(shared.GetModel())

	// Make a few calls
	fmt.Println("Making 2 API calls to track history...")

	for i := 1; i <= 2; i++ {
		messages := []dsgo.Message{
			{Role: "user", Content: fmt.Sprintf("Say 'Call %d' and nothing else.", i)},
		}

		options := dsgo.DefaultGenerateOptions()
		options.Temperature = 0.7
		options.MaxTokens = 20

		result, err := lm.Generate(ctx, messages, options)
		if err != nil {
			log.Fatalf("Call %d failed: %v", i, err)
		}
		fmt.Printf("  Call %d: %s\n", i, result.Content)
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
}

// Demo 3: Shows streaming with automatic observability tracking
func demoStreamingObservability(ctx context.Context) {
	model := shared.GetModel()
	lm := shared.GetLM(model)

	// Create collector for streaming
	collector := dsgo.NewMemoryCollector(10)

	// Wrap LM to enable observability
	wrappedLM := dsgo.NewLMWrapper(lm, collector)

	fmt.Printf("Using model: %s\n", model)

	// Demo 3a: Standard Generate call
	fmt.Println("\n🔹 Standard Generate Call:")
	messages := []dsgo.Message{
		{Role: "user", Content: "Write a haiku about Go programming."},
	}
	options := dsgo.DefaultGenerateOptions()
	options.Temperature = 0.7
	options.MaxTokens = 100

	result, err := wrappedLM.Generate(ctx, messages, options)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Response: %s\n", result.Content)
	fmt.Printf("Usage: %d tokens, $%.6f, %dms\n",
		result.Usage.TotalTokens, result.Usage.Cost, result.Usage.Latency)

	// Demo 3b: Streaming call
	fmt.Println("\n🔹 Streaming Call:")
	messages2 := []dsgo.Message{
		{Role: "user", Content: "Explain benefits of streaming in two sentences."},
	}
	options2 := dsgo.DefaultGenerateOptions()
	options2.Temperature = 0.7
	options2.MaxTokens = 100

	chunkChan, errChan := wrappedLM.Stream(ctx, messages2, options2)

	var fullContent string
	fmt.Print("Response: ")
	for chunk := range chunkChan {
		fullContent += chunk.Content
		fmt.Print(chunk.Content)
	}

	if err := <-errChan; err != nil {
		log.Printf("\nError: %v", err)
		return
	}

	fmt.Println()

	// Inspect observability data
	entries := collector.GetAll()
	if len(entries) > 0 {
		streamEntry := entries[len(entries)-1]
		fmt.Printf("Usage: %d tokens, $%.6f, %dms\n",
			streamEntry.Usage.TotalTokens, streamEntry.Usage.Cost, streamEntry.Usage.Latency)
	}

	// Compare Generate vs Stream
	if len(entries) >= 2 {
		fmt.Println("\n📊 Generate vs Stream Comparison:")
		genEntry := entries[len(entries)-2]
		streamEntry := entries[len(entries)-1]

		fmt.Printf("%-20s %-15s %-15s\n", "Metric", "Generate", "Stream")
		fmt.Println(strings.Repeat("─", 52))
		fmt.Printf("%-20s %-15d %-15d\n", "Tokens", genEntry.Usage.TotalTokens, streamEntry.Usage.TotalTokens)
		fmt.Printf("%-20s $%-14.6f $%-14.6f\n", "Cost", genEntry.Usage.Cost, streamEntry.Usage.Cost)
		fmt.Printf("%-20s %-14dms %-14dms\n", "Latency", genEntry.Usage.Latency, streamEntry.Usage.Latency)

		fmt.Println("\n✅ Both methods produce complete observability data!")
	}
}
