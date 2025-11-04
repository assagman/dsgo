package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	_ "github.com/assagman/dsgo/providers/openai"
	_ "github.com/assagman/dsgo/providers/openrouter"
)

func main() {
	shared.LoadEnv()

	ctx := context.Background()

	// Example 1: OpenAI with metadata extraction
	fmt.Println("=== OpenAI Observability Example ===")
	demonstrateOpenAI(ctx)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: OpenRouter with metadata extraction
	fmt.Println("=== OpenRouter Observability Example ===")
	demonstrateOpenRouter(ctx)
}

func demonstrateOpenAI(ctx context.Context) {
	// Create OpenAI LM using factory
	dsgo.Configure(
		dsgo.WithProvider("openai"),
		dsgo.WithModel(shared.GetModel()),
	)

	lm, err := dsgo.NewLM(ctx)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Generate a response
	messages := []dsgo.Message{
		{Role: "user", Content: "What are the three primary colors?"},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Temperature = 0.7
	options.MaxTokens = 100

	fmt.Println("Sending request to OpenAI...")
	result, err := lm.Generate(ctx, messages, options)
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	// Display response
	fmt.Printf("\nResponse: %s\n\n", result.Content)

	// Display usage metrics
	fmt.Println("📊 Usage Metrics:")
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
		fmt.Println("\n🔍 Provider Metadata (OpenAI):")

		if cacheStatus, ok := result.Metadata["cache_status"].(string); ok {
			fmt.Printf("  Cache Status:      %s\n", cacheStatus)
		}
		if cacheHit, ok := result.Metadata["cache_hit"].(bool); ok {
			fmt.Printf("  Cache Hit:         %v\n", cacheHit)
		}
		if reqID, ok := result.Metadata["request_id"].(string); ok {
			fmt.Printf("  Request ID:        %s\n", reqID)
		}
		if org, ok := result.Metadata["organization"].(string); ok {
			fmt.Printf("  Organization:      %s\n", org)
		}

		// Rate limit information
		if limit, ok := result.Metadata["rate_limit_requests"].(string); ok {
			fmt.Printf("  Rate Limit:        %s requests\n", limit)
		}
		if remaining, ok := result.Metadata["rate_limit_remaining_requests"].(string); ok {
			fmt.Printf("  Rate Remaining:    %s requests\n", remaining)
		}
		if tokenLimit, ok := result.Metadata["rate_limit_tokens"].(string); ok {
			fmt.Printf("  Token Limit:       %s tokens\n", tokenLimit)
		}
		if tokenRemaining, ok := result.Metadata["rate_limit_remaining_tokens"].(string); ok {
			fmt.Printf("  Tokens Remaining:  %s tokens\n", tokenRemaining)
		}
	}

	fmt.Printf("\n✅ Finish Reason: %s\n", result.FinishReason)
}

func demonstrateOpenRouter(ctx context.Context) {
	// Create OpenRouter LM using factory
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel("google/gemini-2.0-flash-exp:free"),
	)

	lm, err := dsgo.NewLM(ctx)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Generate a response
	messages := []dsgo.Message{
		{Role: "user", Content: "Name three programming languages known for concurrency."},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Temperature = 0.5
	options.MaxTokens = 150

	fmt.Println("Sending request to OpenRouter...")
	result, err := lm.Generate(ctx, messages, options)
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	// Display response
	fmt.Printf("\nResponse: %s\n\n", result.Content)

	// Display usage metrics
	fmt.Println("📊 Usage Metrics:")
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
		fmt.Println("\n🔍 Provider Metadata (OpenRouter):")

		if cacheStatus, ok := result.Metadata["cache_status"].(string); ok {
			fmt.Printf("  Cache Status:      %s\n", cacheStatus)
		}
		if cacheHit, ok := result.Metadata["cache_hit"].(bool); ok {
			fmt.Printf("  Cache Hit:         %v\n", cacheHit)
		}
		if genID, ok := result.Metadata["generation_id"].(string); ok {
			fmt.Printf("  Generation ID:     %s\n", genID)
		}

		// Rate limit information (OpenRouter format)
		if limit, ok := result.Metadata["rate_limit_limit"].(string); ok {
			fmt.Printf("  Rate Limit:        %s\n", limit)
		}
		if remaining, ok := result.Metadata["rate_limit_remaining"].(string); ok {
			fmt.Printf("  Rate Remaining:    %s\n", remaining)
		}
		if reset, ok := result.Metadata["rate_limit_reset"].(string); ok {
			fmt.Printf("  Rate Limit Reset:  %s\n", reset)
		}
	}

	fmt.Printf("\n✅ Finish Reason: %s\n", result.FinishReason)
}
