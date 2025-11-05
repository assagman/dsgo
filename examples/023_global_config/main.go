package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "023_global_config", runExample)
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

	fmt.Println("=== Global Configuration System ===")
	fmt.Println("Demonstrates DSGo's global configuration via functional options and environment variables")
	fmt.Println()

	fmt.Println("--- Configuration Features ---")
	fmt.Println("✓ Functional options (WithProvider, WithModel, etc.)")
	fmt.Println("✓ Environment variable fallbacks (DSGO_PROVIDER, DSGO_MODEL, etc.)")
	fmt.Println("✓ Options override environment variables")
	fmt.Println("✓ Thread-safe global settings")
	fmt.Println("✓ Runtime reconfiguration with ResetConfig()")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 1: Configure using functional options
	fmt.Println("--- Example 1: Functional Options ---")
	fmt.Println("Configure DSGo using functional options for explicit control")
	fmt.Println()

	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel("google/gemini-2.5-flash"),
		dsgo.WithTimeout(30*time.Second),
		dsgo.WithMaxRetries(3),
		dsgo.WithTracing(true),
	)

	settings := dsgo.GetSettings()
	fmt.Printf("Configured Provider: %s\n", settings.DefaultProvider)
	fmt.Printf("Configured Model: %s\n", settings.DefaultModel)
	fmt.Printf("Timeout: %v\n", settings.DefaultTimeout)
	fmt.Printf("Max Retries: %d\n", settings.MaxRetries)
	fmt.Printf("Tracing Enabled: %v\n", settings.EnableTracing)

	stats.Metadata["example1_provider"] = settings.DefaultProvider
	stats.Metadata["example1_model"] = settings.DefaultModel

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 2: Use environment variables
	fmt.Println("--- Example 2: Environment Variables ---")
	fmt.Println("Configure DSGo using environment variables (fallback mechanism)")
	fmt.Println()

	// Set env vars
	_ = os.Setenv("DSGO_PROVIDER", "openai")
	_ = os.Setenv("DSGO_MODEL", "gpt-4")
	_ = os.Setenv("DSGO_TIMEOUT", "60")
	_ = os.Setenv("DSGO_MAX_RETRIES", "5")

	dsgo.ResetConfig()
	dsgo.Configure()

	settings = dsgo.GetSettings()
	fmt.Printf("Provider from env: %s\n", settings.DefaultProvider)
	fmt.Printf("Model from env: %s\n", settings.DefaultModel)
	fmt.Printf("Timeout from env: %v\n", settings.DefaultTimeout)
	fmt.Printf("Max Retries from env: %d\n", settings.MaxRetries)

	stats.Metadata["example2_provider"] = settings.DefaultProvider
	stats.Metadata["example2_model"] = settings.DefaultModel

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 3: Override env vars with options
	fmt.Println("--- Example 3: Override Env with Options ---")
	fmt.Println("Functional options take precedence over environment variables")
	fmt.Println()

	dsgo.Configure(
		dsgo.WithModel("google/gemini-2.5-flash"),
		dsgo.WithTimeout(45*time.Second),
	)

	settings = dsgo.GetSettings()
	fmt.Printf("Provider (from env): %s\n", settings.DefaultProvider)
	fmt.Printf("Model (overridden): %s\n", settings.DefaultModel)
	fmt.Printf("Timeout (overridden): %v\n", settings.DefaultTimeout)

	stats.Metadata["example3_provider"] = settings.DefaultProvider
	stats.Metadata["example3_model"] = settings.DefaultModel

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 4: Actual usage with configured LM
	fmt.Println("--- Example 4: Using Configured Settings ---")
	fmt.Println("Create LM instance and run prediction with configured settings")
	fmt.Println()

	lm := shared.GetLM(shared.GetModel())
	fmt.Printf("Created LM: %s\n", shared.GetModel())

	// Use the LM
	sig := dsgo.NewSignature("Generate a greeting").
		AddInput("name", dsgo.FieldTypeString, "Person's name").
		AddOutput("greeting", dsgo.FieldTypeString, "A friendly greeting")

	predictor := module.NewPredict(sig, lm)

	prediction, err := predictor.Forward(ctx, map[string]any{
		"name": "Alice",
	})

	if err != nil {
		return nil, stats, fmt.Errorf("prediction failed: %w", err)
	}

	greeting, _ := prediction.GetString("greeting")
	fmt.Printf("\nGreeting: %s\n", greeting)
	fmt.Printf("📊 Tokens used: %d\n", prediction.Usage.TotalTokens)

	totalTokens += prediction.Usage.TotalTokens
	lastPred = prediction

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- Configuration Precedence ---")
	fmt.Println("1. Functional options (highest priority)")
	fmt.Println("2. Environment variables")
	fmt.Println("3. Built-in defaults (lowest priority)")
	fmt.Println()

	fmt.Println("--- Available Configuration Options ---")
	fmt.Println("Functional Options:")
	fmt.Println("  • dsgo.WithProvider(name)     - Set default provider")
	fmt.Println("  • dsgo.WithModel(model)        - Set default model")
	fmt.Println("  • dsgo.WithTimeout(duration)   - Set default timeout")
	fmt.Println("  • dsgo.WithMaxRetries(n)       - Set max retry attempts")
	fmt.Println("  • dsgo.WithTracing(bool)       - Enable/disable tracing")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  • DSGO_PROVIDER     - Default provider name")
	fmt.Println("  • DSGO_MODEL        - Default model name")
	fmt.Println("  • DSGO_TIMEOUT      - Default timeout (seconds)")
	fmt.Println("  • DSGO_MAX_RETRIES  - Max retry attempts")
	fmt.Println("  • DSGO_ENABLE_TRACE - Enable tracing (true/false)")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("Global configuration benefits:")
	fmt.Println("  ✓ Centralized settings management")
	fmt.Println("  ✓ Environment-based configuration (dev/staging/prod)")
	fmt.Println("  ✓ Explicit override capability")
	fmt.Println("  ✓ Thread-safe access")
	fmt.Println("  ✓ Runtime reconfiguration support")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total examples: 4\n")
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["total_examples"] = 4

	return lastPred, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
