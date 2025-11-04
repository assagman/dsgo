# LM Factory Pattern Example

This example demonstrates the LM factory pattern for dynamic language model instantiation based on global configuration.

## Features Demonstrated

1. **Basic Factory Usage**: Create an LM using `dsgo.NewLM(ctx)` with configured provider and model
2. **Dynamic Provider Switching**: Switch between different providers at runtime
3. **Error Handling**: Gracefully handle missing configuration and unknown providers
4. **Environment Variables**: Configure provider and model via `DSGO_PROVIDER` and `DSGO_MODEL`

## Key Concepts

### LM Factory Registration

Providers automatically register themselves via `init()` functions:

```go
import (
    _ "github.com/assagman/dsgo/providers/openai"
    _ "github.com/assagman/dsgo/providers/openrouter"
)
```

### Creating an LM

```go
// Configure globally
dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.5-flash"),
)

// Create LM from configuration
ctx := context.Background()
lm, err := dsgo.NewLM(ctx)
if err != nil {
    log.Fatal(err)
}
```

### Benefits

- **Centralized Configuration**: Set provider/model once, use everywhere
- **Easy Provider Switching**: Change providers without code changes
- **Environment-Based Config**: Use env vars for deployment flexibility
- **Type Safety**: Factory pattern ensures correct LM types

## Running the Example

```bash
cd examples/lm_factory
go run main.go
```

## Environment Variables

- `DSGO_PROVIDER` - Default provider name (e.g., "openai", "openrouter")
- `DSGO_MODEL` - Default model identifier
- `DSGO_OPENAI_API_KEY` or `OPENAI_API_KEY` - OpenAI API key
- `DSGO_OPENROUTER_API_KEY` or `OPENROUTER_API_KEY` - OpenRouter API key

## Expected Output

```
=== LM Factory Pattern Demo ===

Example 1: Basic Factory Usage
-------------------------------
✓ Created LM: google/gemini-2.5-flash

Sentiment: positive

Example 2: Dynamic Provider Switching
--------------------------------------
✓ Created LM: google/gemini-2.5-flash (openrouter)
✓ Created LM: gpt-4 (openai)

Example 3: Error Handling
-------------------------
✓ Expected error (no provider): no default provider configured (use dsgo.Configure with dsgo.WithProvider)
✓ Expected error (no model): no default model configured for provider 'openai' (use dsgo.Configure with dsgo.WithModel)
✓ Expected error (unknown provider): provider 'nonexistent' not registered (available: [openai openrouter])

Example 4: Environment Variables
---------------------------------
✓ Created LM from env vars: anthropic/claude-3.5-sonnet

=== Demo Complete ===
```
