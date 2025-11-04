package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/module"
	_ "github.com/assagman/dsgo/providers/openrouter"
)

func main() {
	// Example 1: Configure using functional options
	fmt.Println("=== Example 1: Functional Options ===")
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
	fmt.Printf("Tracing Enabled: %v\n\n", settings.EnableTracing)

	// Example 2: Use environment variables
	fmt.Println("=== Example 2: Environment Variables ===")
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
	fmt.Printf("Max Retries from env: %d\n\n", settings.MaxRetries)

	// Example 3: Override env vars with options
	fmt.Println("=== Example 3: Override Env with Options ===")
	dsgo.Configure(
		dsgo.WithModel("google/gemini-2.5-flash"),
		dsgo.WithTimeout(45*time.Second),
	)

	settings = dsgo.GetSettings()
	fmt.Printf("Provider (from env): %s\n", settings.DefaultProvider)
	fmt.Printf("Model (overridden): %s\n", settings.DefaultModel)
	fmt.Printf("Timeout (overridden): %v\n\n", settings.DefaultTimeout)

	// Example 4: Actual usage with configured LM
	fmt.Println("=== Example 4: Using Configured Settings ===")
	dsgo.ResetConfig()
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel("google/gemini-2.5-flash"),
	)

	// Create LM using the factory
	ctx := context.Background()
	lm, err := dsgo.NewLM(ctx)
	if err != nil {
		fmt.Printf("Error creating LM: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created LM: %s\n", lm.Name())

	// Use the LM
	sig := dsgo.NewSignature("Generate a greeting").
		AddInput("name", dsgo.FieldTypeString, "Person's name").
		AddOutput("greeting", dsgo.FieldTypeString, "A friendly greeting")

	predictor := module.NewPredict(sig, lm)

	prediction, err := predictor.Forward(ctx, map[string]any{
		"name": "Alice",
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	greeting, _ := prediction.GetString("greeting")
	fmt.Printf("\nGreeting: %s\n", greeting)
}
