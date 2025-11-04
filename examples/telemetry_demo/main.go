package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
)

// Example demonstrating Phase 4.1: Metadata Persistence
//
// This example shows how DSGo now captures and persists:
// 1. Provider-specific metadata (request IDs, rate limits, cache headers)
// 2. Cache hit tracking from provider responses
// 3. Provider name resolution from global settings
//
// The metadata is automatically collected in HistoryEntry and can be
// accessed for debugging, cost tracking, and observability.

func main() {
	ctx := context.Background()

	shared.LoadEnv()

	// Create a memory collector to capture telemetry
	collector := dsgo.NewMemoryCollector(100)

	// Configure global settings with provider name and collector
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel("google/gemini-2.5-flash"),
		dsgo.WithCollector(collector),
	)

	// Create LM - automatically wrapped with telemetry collection
	lm, err := dsgo.NewLM(ctx)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	fmt.Println("=== DSGo Telemetry Demo: Metadata Persistence ===")

	// Example 1: Basic call with metadata collection
	fmt.Println("1. Making API call with metadata collection...")
	result1, err := makeSimpleCall(ctx, lm)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}
	fmt.Printf("   Response: %s\n\n", result1.Content)

	// Example 2: Make a second call (might be cached)
	fmt.Println("2. Making second call (check for cache hit)...")
	result2, err := makeSimpleCall(ctx, lm)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}
	fmt.Printf("   Response: %s\n\n", result2.Content)

	// Example 3: Inspect collected telemetry
	fmt.Println("3. Inspecting collected telemetry...")
	displayTelemetry(collector)

	fmt.Println("\n=== Demo Complete ===")
}

func makeSimpleCall(ctx context.Context, lm dsgo.LM) (*dsgo.GenerateResult, error) {
	messages := []dsgo.Message{
		{Role: "user", Content: "Say 'Hello from DSGo!' and nothing else."},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Temperature = 0.7
	options.MaxTokens = 50

	return lm.Generate(ctx, messages, options)
}

func displayTelemetry(collector *dsgo.MemoryCollector) {
	entries := collector.GetAll()
	fmt.Printf("   Collected %d history entries\n\n", len(entries))

	for i, entry := range entries {
		fmt.Printf("   --- Entry %d ---\n", i+1)
		fmt.Printf("   ID:          %s\n", entry.ID)
		fmt.Printf("   Timestamp:   %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("   SessionID:   %s\n", entry.SessionID)
		fmt.Printf("   Provider:    %s (from global settings)\n", entry.Provider)
		fmt.Printf("   Model:       %s\n", entry.Model)
		fmt.Println()

		// Usage and cost
		fmt.Printf("   Usage:\n")
		fmt.Printf("     Prompt tokens:     %d\n", entry.Usage.PromptTokens)
		fmt.Printf("     Completion tokens: %d\n", entry.Usage.CompletionTokens)
		fmt.Printf("     Total tokens:      %d\n", entry.Usage.TotalTokens)
		fmt.Printf("     Cost (USD):        $%.6f\n", entry.Usage.Cost)
		fmt.Printf("     Latency (ms):      %d\n", entry.Usage.Latency)
		fmt.Println()

		// Cache information
		fmt.Printf("   Cache:\n")
		fmt.Printf("     Hit:    %t\n", entry.Cache.Hit)
		if entry.Cache.Hit {
			fmt.Printf("     Source: %s\n", entry.Cache.Source)
		}
		fmt.Println()

		// Provider-specific metadata (NEW in Phase 4.1)
		if len(entry.ProviderMeta) > 0 {
			fmt.Printf("   Provider Metadata (NEW):\n")
			for key, value := range entry.ProviderMeta {
				fmt.Printf("     %s: %v\n", key, value)
			}
			fmt.Println()
		}

		// Response summary
		fmt.Printf("   Response:\n")
		contentPreview := entry.Response.Content
		if len(contentPreview) > 60 {
			contentPreview = contentPreview[:60] + "..."
		}
		fmt.Printf("     Content:      %s\n", contentPreview)
		fmt.Printf("     Finish:       %s\n", entry.Response.FinishReason)
		fmt.Printf("     Length:       %d chars\n", entry.Response.ResponseLength)
		fmt.Println()
	}

	// Export to JSON for inspection
	if len(entries) > 0 {
		fmt.Println("   Exporting first entry as JSON...")
		exportEntryJSON(entries[0])
	}
}

func exportEntryJSON(entry *dsgo.HistoryEntry) {
	data, err := json.MarshalIndent(entry, "   ", "  ")
	if err != nil {
		fmt.Printf("   Error marshaling JSON: %v\n", err)
		return
	}

	filename := "telemetry_sample.json"
	if err := os.WriteFile(filename, data, 0644); err != nil {
		fmt.Printf("   Error writing file: %v\n", err)
		return
	}

	fmt.Printf("   ✓ Saved to %s\n", filename)
	fmt.Printf("\n   Sample JSON structure (first 15 lines):\n")

	// Print a preview of the first 15 lines
	content := string(data)
	lineStart := 0
	lineCount := 0

	for i := 0; i < len(content); i++ {
		if content[i] == '\n' || i == len(content)-1 {
			if lineCount >= 15 {
				fmt.Println("   ...")
				break
			}
			
			// Print the line
			var lineEnd int
			if content[i] == '\n' {
				lineEnd = i
			} else {
				lineEnd = i + 1
			}
			
			fmt.Printf("   %s\n", content[lineStart:lineEnd])
			lineStart = i + 1
			lineCount++
		}
	}
}
