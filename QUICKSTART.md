# DSGo Quick Start Guide

Learn DSGo through practical tutorials, from basics to production patterns.

## Table of Contents

- [1. Installation & Setup](#1-installation--setup)
- [2. Your First Prediction](#2-your-first-prediction)
- [3. Understanding Signatures](#3-understanding-signatures)
- [4. Core Modules](#4-core-modules)
- [5. Working with Tools](#5-working-with-tools)
- [6. Module Composition](#6-module-composition)
- [7. Production Patterns](#7-production-patterns)
- [8. Advanced Features](#8-advanced-features)

---

## 1. Installation & Setup

### Installation

```bash
go get github.com/assagman/dsgo
```

### Configuration

Set up your API keys:

```bash
# OpenAI (for GPT models)
export OPENAI_API_KEY=sk-...

# OpenRouter (access to 100+ models)
export OPENROUTER_API_KEY=sk-or-v1-...
```

Or configure programmatically:

```go
package main

import (
    "context"
    "github.com/assagman/dsgo"
)

func main() {
    dsgo.Configure(
        dsgo.WithAPIKey("openai", "sk-..."),
        dsgo.WithAPIKey("openrouter", "sk-or-v1-..."),
    )
}
```

### Choosing a Model

DSGo uses the `provider/model` format:

```go
// OpenAI models
lm, _ := dsgo.NewLM(ctx, "openai/gpt-4o-mini")
lm, _ := dsgo.NewLM(ctx, "openai/gpt-4o")

// OpenRouter models (access to 100+ models)
lm, _ := dsgo.NewLM(ctx, "openrouter/google/gemini-2.5-flash")
lm, _ := dsgo.NewLM(ctx, "openrouter/meta-llama/llama-3.1-8b-instruct")
```

---

## 2. Your First Prediction

### Basic Text Generation

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
)

func main() {
    // Create LM instance
    lm, err := dsgo.NewLM(context.Background(), "openrouter/google/gemini-2.5-flash")
    if err != nil {
        log.Fatal(err)
    }
    
    // Define signature: inputs and outputs
    sig := dsgo.NewSignature("Answer a question").
        AddInput("question", dsgo.FieldTypeString, "The question to answer").
        AddOutput("answer", dsgo.FieldTypeString, "A helpful answer")
    
    // Create predictor module
    predictor := dsgo.NewPredict(sig, lm)
    
    // Execute
    result, err := predictor.Forward(context.Background(), map[string]any{
        "question": "What is the capital of France?",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Get result
    fmt.Println("Answer:", result.GetString("answer"))
    
    // Track usage
    fmt.Printf("Cost: $%.6f, Tokens: %d\n", 
        result.Usage.Cost, result.Usage.TotalTokens)
}
```

**Key Points:**
- `Signature` defines the contract between inputs and outputs
- `Predict` module handles basic LLM calls
- `Forward()` executes the module with given inputs
- `Usage` tracks cost and tokens automatically

---

## 3. Understanding Signatures

### Field Types

DSGo supports multiple field types for structured I/O:

```go
sig := dsgo.NewSignature("Process data").
    // Basic types
    AddInput("text", dsgo.FieldTypeString, "Text input").
    AddInput("count", dsgo.FieldTypeInt, "Integer input").
    AddInput("score", dsgo.FieldTypeFloat, "Float score 0-1").
    AddInput("active", dsgo.FieldTypeBool, "Boolean flag").
    
    // Structured types
    AddInput("metadata", dsgo.FieldTypeJSON, "JSON data").
    AddInput("timestamp", dsgo.FieldTypeDatetime, "Timestamp").
    
    // Classification (enums)
    AddClassInput("priority", []string{"low", "medium", "high"}, "Task priority").
    AddClassOutput("category", []string{"bug", "feature", "docs"}, "Issue category").
    
    // Outputs
    AddOutput("result", dsgo.FieldTypeString, "Processing result").
    AddOptionalOutput("error_info", dsgo.FieldTypeString, "Error details if any")
```

### Optional Fields

```go
sig := dsgo.NewSignature("Summarize text").
    AddInput("text", dsgo.FieldTypeString, "Text to summarize").
    AddOutput("summary", dsgo.FieldTypeString, "Summary").
    AddOptionalOutput("keywords", dsgo.FieldTypeJSON, "Extracted keywords")

// Optional fields can be missing
result, _ := dsgo.NewPredict(sig, lm).Forward(ctx, inputs)
summary := result.GetString("answer")           // Always present
keywords, hasKeywords := result.GetJSON("keywords") // May be nil
```

### Classification with Aliases

```go
// Flexible matching for classification tasks
sig := dsgo.NewSignature("Classify sentiment").
    AddClassOutput("sentiment", 
        []string{"positive", "negative", "neutral"},
        "Sentiment classification").
    WithAlias("sentiment", "pos", "positive").
    WithAlias("sentiment", "neg", "negative")

// LM can say "pos" → automatically normalized to "positive"
```

---

## 4. Core Modules

### Predict - Basic Generation

For simple input→output tasks:

```go
sig := dsgo.NewSignature("Translate text").
    AddInput("text", dsgo.FieldTypeString, "Text to translate").
    AddInput("target_language", dsgo.FieldTypeString, "Target language").
    AddOutput("translation", dsgo.FieldTypeString, "Translated text")

translator := dsgo.NewPredict(sig, lm)

result, _ := translator.Forward(ctx, map[string]any{
    "text": "Hello world",
    "target_language": "Spanish",
})
fmt.Println(result.GetString("translation")) // "Hola mundo"
```

### ChainOfThought - Step-by-Step Reasoning

For complex tasks requiring reasoning:

```go
sig := dsgo.NewSignature("Solve math problem").
    AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
    AddOutput("reasoning", dsgo.FieldTypeString, "Step-by-step reasoning").
    AddOutput("answer", dsgo.FieldTypeString, "Final numerical answer")

solver := dsgo.NewChainOfThought(sig, lm)

result, _ := solver.Forward(ctx, map[string]any{
    "problem": "If Alice has 5 apples and Bob has 3, how many do they have together?",
})

fmt.Println("Reasoning:", result.GetString("reasoning"))
fmt.Println("Answer:", result.GetString("answer"))
```

### ReAct - Tool-Using Agents

For tasks requiring external tools:

```go
// Define a tool
func searchWeb(ctx context.Context, args map[string]interface{}) (string, error) {
    query := args["query"].(string)
    return fmt.Sprintf("Search results for '%s': Wikipedia, Google, etc.", query), nil
}

searchTool := dsgo.NewTool(
    "search",
    "Search the web for information",
    searchWeb,
).AddParameter("query", dsgo.FieldTypeString, "Search query", true)

sig := dsgo.NewSignature("Answer questions using tools").
    AddInput("question", dsgo.FieldTypeString, "Question to answer").
    AddOutput("answer", dsgo.FieldTypeString, "Final answer based on tool results")

agent := dsgo.NewReAct(sig, lm, []dsgo.Tool{*searchTool})

result, _ := agent.Forward(ctx, map[string]any{
    "question": "Who is the current president of France?",
})
fmt.Println(result.GetString("answer"))
```

### Refine - Iterative Improvement

For improving outputs through iteration:

```go
sig := dsgo.NewSignature("Write professional email").
    AddInput("topic", dsgo.FieldTypeString, "Email topic").
    AddInput("tone", dsgo.FieldTypeString, "Desired tone (formal/casual)").
    AddOutput("email", dsgo.FieldTypeString, "Final email")

// Refine takes optional feedback via inputs["feedback"].
// If no feedback is provided, it returns the initial prediction.
refiner := dsgo.NewRefine(sig, lm).WithMaxIterations(2)

// Initial draft (no refinement)
result, _ := refiner.Forward(ctx, map[string]any{
    "topic": "Project status update",
    "tone": "formal",
})
fmt.Println("Draft:", result.GetString("email"))

// Refined draft (feedback triggers refinement loop)
refined, _ := refiner.Forward(ctx, map[string]any{
    "topic": "Project status update",
    "tone": "formal",
    "feedback": "Make the email more professional and clear",
})
fmt.Println("Refined:", refined.GetString("email"))
```

### BestOfN - Generate Multiple Candidates

For creative tasks where you want the best output:

```go
sig := dsgo.NewSignature("Generate marketing slogan").
    AddInput("product", dsgo.FieldTypeString, "Product name").
    AddOutput("slogan", dsgo.FieldTypeString, "Marketing slogan")

// Wrap a base module, then run it N times.
base := dsgo.NewPredict(sig, lm)

// Generate 3 candidates and pick the best (requires a scorer).
bestof := dsgo.NewBestOfN(base, 3).
    WithScorer(dsgo.DefaultScorer()).
    WithParallel(true)   // optional

result, _ := bestof.Forward(ctx, map[string]any{
    "product": "Eco-friendly water bottle",
})
fmt.Println(result.GetString("slogan"))
```

### ChainOfThought - Step-by-Step Reasoning

For complex tasks requiring reasoning:

```go
sig := dsgo.NewSignature("Solve math problem").
    AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
    AddOutput("reasoning", dsgo.FieldTypeString, "Step-by-step reasoning").
    AddOutput("answer", dsgo.FieldTypeString, "Final numerical answer")

solver := module.NewChainOfThought(sig, lm)

result, _ := solver.Forward(ctx, map[string]any{
    "problem": "If Alice has 5 apples and Bob has 3, how many do they have together?",
})

fmt.Println("Reasoning:", result.GetString("reasoning"))
fmt.Println("Answer:", result.GetString("answer"))
```

### ReAct - Tool-Using Agents

For tasks requiring external tools:

```go
// Define a tool
func searchWeb(ctx context.Context, args map[string]interface{}) (string, error) {
    query := args["query"].(string)
    return fmt.Sprintf("Search results for '%s': Wikipedia, Google, etc.", query), nil
}

searchTool := dsgo.NewTool(
    "search",
    "Search the web for information",
    searchWeb,
).AddParameter("query", dsgo.FieldTypeString, "Search query", true)

sig := dsgo.NewSignature("Answer questions using tools").
    AddInput("question", dsgo.FieldTypeString, "Question to answer").
    AddOutput("answer", dsgo.FieldTypeString, "Final answer based on tool results")

agent := module.NewReAct(sig, lm, []dsgo.Tool{*searchTool})

result, _ := agent.Forward(ctx, map[string]any{
    "question": "Who is the current president of France?",
})
fmt.Println(result.GetString("answer"))
```

### Refine - Iterative Improvement

For improving outputs through iteration:

```go
sig := dsgo.NewSignature("Write professional email").
    AddInput("topic", dsgo.FieldTypeString, "Email topic").
    AddInput("tone", dsgo.FieldTypeString, "Desired tone (formal/casual)").
    AddOutput("email", dsgo.FieldTypeString, "Final email")

// Refine takes optional feedback via inputs["feedback"].
// If no feedback is provided, it returns the initial prediction.
refiner := module.NewRefine(sig, lm).WithMaxIterations(2)

// Initial draft (no refinement)
result, _ := refiner.Forward(ctx, map[string]any{
    "topic": "Project status update",
    "tone": "formal",
})
fmt.Println("Draft:", result.GetString("email"))

// Refined draft (feedback triggers refinement loop)
refined, _ := refiner.Forward(ctx, map[string]any{
    "topic": "Project status update",
    "tone": "formal",
    "feedback": "Make the email more professional and clear",
})
fmt.Println("Refined:", refined.GetString("email"))
```

### BestOfN - Generate Multiple Candidates

For creative tasks where you want the best output:

```go
sig := dsgo.NewSignature("Generate marketing slogan").
    AddInput("product", dsgo.FieldTypeString, "Product name").
    AddOutput("slogan", dsgo.FieldTypeString, "Marketing slogan")

// Wrap a base module, then run it N times.
base := module.NewPredict(sig, lm)

// Generate 3 candidates and pick the best (requires a scorer).
bestof := module.NewBestOfN(base, 3).
    WithScorer(module.DefaultScorer()).
    WithParallel(true)   // optional

result, _ := bestof.Forward(ctx, map[string]any{
    "product": "Eco-friendly water bottle",
})
fmt.Println(result.GetString("slogan"))
```

---

## 5. Working with Tools

### Tool Definition

Tools are functions that LLMs can call:

```go
func calculate(ctx context.Context, args map[string]interface{}) (string, error) {
    operation := args["operation"].(string)
    a := args["a"].(float64)
    b := args["b"].(float64)
    
    var result float64
    switch operation {
    case "add":
        result = a + b
    case "multiply":
        result = a * b
    case "divide":
        if b == 0 {
            return "Error: Division by zero", nil
        }
        result = a / b
    default:
        return fmt.Sprintf("Unknown operation: %s", operation), nil
    }
    
    return fmt.Sprintf("%.2f", result), nil
}

calcTool := dsgo.NewTool("calculate", "Perform mathematical operations", calculate).
    AddParameter("operation", dsgo.FieldTypeString, "Operation (add/multiply/divide)", true).
    AddParameter("a", dsgo.FieldTypeFloat, "First number", true).
    AddParameter("b", dsgo.FieldTypeFloat, "Second number", true).
    AddParameter("precision", dsgo.FieldTypeInt, "Decimal places", false) // Optional
```

### Multi-Tool Agents

```go
weatherTool := dsgo.NewTool("get_weather", "Get current weather", getWeatherFunc).
    AddParameter("location", dsgo.FieldTypeString, "City name", true)

tools := []dsgo.Tool{*calcTool, *weatherTool}

sig := dsgo.NewSignature("Helpful assistant").
    AddInput("request", dsgo.FieldTypeString, "User request").
    AddOutput("response", dsgo.FieldTypeString, "Helpful response")

agent := dsgo.NewReAct(sig, lm, tools)
```

### Using MCP Tools

MCP (Model Context Protocol) allows your agents to access external tools:

```go
// Initialize Exa MCP for web search
exaClient, err := dsgo.NewMCPExaClient(os.Getenv("EXA_API_KEY"))
if err != nil {
    log.Fatal(err)
}
if err := exaClient.Initialize(ctx); err != nil {
    log.Fatal(err)
}

// Create ReAct agent with MCP tools
tools := exaClient.GetTools()
agent := dsgo.NewReAct(sig, lm, tools)

// The agent can now search the web, read URLs, etc.
result, err := agent.Forward(ctx, map[string]any{
    "query": "Find recent news about AI breakthroughs",
})
```

---

## 6. Module Composition

### Sequential Execution

Chain modules for complex workflows:

```go
// Stage 1: Generate outline
outlineSig := dsgo.NewSignature("Generate article outline").
    AddInput("topic", dsgo.FieldTypeString, "Article topic").
    AddOutput("outline", dsgo.FieldTypeString, "Numbered outline")

outliner := dsgo.NewPredict(outlineSig, lm)

// Stage 2: Expand outline
expandSig := dsgo.NewSignature("Expand outline to full article").
    AddInput("outline", dsgo.FieldTypeString, "Article outline").
    AddOutput("article", dsgo.FieldTypeString, "Full article text")

expander := dsgo.NewPredict(expandSig, lm)

// Execute pipeline
outlineResult, _ := outliner.Forward(ctx, map[string]any{
    "topic": "Machine Learning Basics",
})

articleResult, _ := expander.Forward(ctx, map[string]any{
    "outline": outlineResult.GetString("outline"),
})

fmt.Println(articleResult.GetString("article"))
```

### Program Module

For more complex composition with data flow:

```go
program := dsgo.NewProgram("ArticleGenerator").
    AddModule(outliner).
    AddModule(expander)

result, _ := program.Forward(ctx, map[string]any{
    "topic": "Machine Learning Basics",
})

fmt.Println(result.GetString("article"))
```

---

## 7. Production Patterns

### Cost Tracking

Monitor usage and costs:

```go
predictor := dsgo.NewPredict(sig, lm)
result, _ := predictor.Forward(ctx, inputs)

fmt.Printf("Usage Summary:\n")
fmt.Printf("  Prompt tokens:     %d\n", result.Usage.PromptTokens)
fmt.Printf("  Completion tokens: %d\n", result.Usage.CompletionTokens)
fmt.Printf("  Total tokens:      %d\n", result.Usage.TotalTokens)
fmt.Printf("  Latency:          %dms\n", result.Usage.Latency)
fmt.Printf("  Cost:             $%.6f\n", result.Usage.Cost)

// Accumulate across batch
totalCost += result.Usage.Cost
totalTokens += result.Usage.TotalTokens
```

Supported models are validated via the built-in model catalog:

```go
fmt.Println(dsgo.IsValidModel("openai/gpt-4o-mini"))
for _, m := range dsgo.ListModelsByProvider("openai") {
    pricing, ok := dsgo.DefaultCostCalculator.GetPricing(m.ID)
    if ok {
        fmt.Println(m.ID, pricing)
    } else {
        fmt.Println(m.ID)
    }
}
```

If you register a custom provider via `dsgo.RegisterLM`, you must also register its supported models:

```go
_ = dsgo.RegisterModel(dsgo.Model{ID: "myprovider/my-model"})
```

### Caching

DSGo includes automatic LRU caching:

```go
// Configure cache
dsgo.Configure(
    dsgo.WithCache(1000),              // 1000 entry capacity
    dsgo.WithCacheTTL(5 * time.Minute), // 5 minute TTL
)

predictor := dsgo.NewPredict(sig, lm)

// First call - cache miss
result1, _ := predictor.Forward(ctx, map[string]any{"text": "Hello"})
fmt.Printf("Call 1: %v (cache hit)\n", result1.CacheHit)

// Second call - cache hit (instant, no tokens charged)
result2, _ := predictor.Forward(ctx, map[string]any{"text": "Hello"})
fmt.Printf("Call 2: %v (cache hit)\n", result2.CacheHit)
```

### Error Handling

Robust error handling and validation:

```go
result, err := predictor.Forward(ctx, inputs)
if err != nil {
    // Handle hard errors (API failures, timeouts, etc.)
    log.Printf("Prediction failed: %v", err)
    return
}

// Check for soft failures (parsing issues)
if result.ParseDiagnostics != nil {
    if len(result.ParseDiagnostics.MissingFields) > 0 {
        log.Printf("Warning: missing fields: %v", 
            result.ParseDiagnostics.MissingFields)
    }
    if len(result.ParseDiagnostics.TypeErrors) > 0 {
        log.Printf("Warning: type errors: %v", 
            result.ParseDiagnostics.TypeErrors)
    }
}

// Safe getters return zero values if missing
answer := result.GetString("answer")        // "" if missing
confidence := result.GetFloat("confidence") // 0.0 if missing
```

### Streaming

For long responses and better UX:

```go
predictor := dsgo.NewPredict(sig, lm)
streamResult, _ := predictor.Stream(ctx, inputs)

// Process chunks as they arrive
for chunk := range streamResult.Chunks {
    fmt.Print(chunk.Content) // Clean content (no internal markers)
}

// Get final result
result := <-streamResult.Prediction
err := <-streamResult.Errors

if err != nil {
    log.Printf("Streaming error: %v", err)
}

fmt.Printf("\nFinal: %s\n", result.GetString("output"))
```

---

## 8. Advanced Features

### Few-Shot Learning

Provide examples to guide the model:

```go
predictor := dsgo.NewPredict(sig, lm).
    WithExamples([]dsgo.Example{
        {
            Inputs: map[string]interface{}{"text": "How much does the premium plan cost?"},
            Outputs: map[string]interface{}{"category": "billing"},
        },
        {
            Inputs: map[string]interface{}{"text": "I have a bug in the dashboard"},
            Outputs: map[string]interface{}{"category": "support"},
        },
        {
            Inputs: map[string]interface{}{"text": "Interested in enterprise pricing"},
            Outputs: map[string]interface{}{"category": "sales"},
        },
    })

result, _ := predictor.Forward(ctx, map[string]any{
    "text": "We'd like to discuss bulk licensing options",
})
fmt.Println(result.GetString("category")) // "sales"
```

### Custom Adapters

Control how prompts are formatted and responses are parsed:

```go
// Use JSON adapter for structured data
jsonPredictor := dsgo.NewPredict(sig, lm).WithAdapter(dsgo.NewJSONAdapter())

// Use fallback adapter for robustness (default behavior)
fallbackPredictor := dsgo.NewPredict(sig, lm).WithAdapter(
    dsgo.NewFallbackAdapterWithChain([]dsgo.Adapter{
        dsgo.NewChatAdapter(),    // Try first
        dsgo.NewJSONAdapter(),    // Fallback
    })
)
```

> **OpenAI Compatibility**: DSGo automatically detects OpenAI providers and uses OpenAI-compliant JSON schemas. No manual configuration needed - structured outputs work seamlessly with GPT models.

*Note: The JSON adapter and other adapter types are implemented in `internal/core/`, but the public API remains accessible through the main `dsgo` package via re-exports.*

### Observability

Track all LLM interactions automatically when a collector is configured:

```go
type ProductionCollector struct{}

func (c *ProductionCollector) Collect(entry dsgo.HistoryEntry) {
    log.Printf("LLM Call: provider=%s model=%s tokens=%d cost=$%.6f latency=%dms",
        entry.Provider, entry.Model, entry.Usage.TotalTokens,
        entry.Usage.Cost, entry.Usage.Latency)
}

// Observability is automatically enabled when a collector is configured
dsgo.Configure(dsgo.WithCollector(&ProductionCollector{}))
```

*Note: The HistoryEntry and observability infrastructure are implemented in `internal/core/` and `internal/logging/`, but the public API remains accessible through the main `dsgo` package via re-exports.*

### Parallel Execution

Run multiple modules concurrently:

```go
// Create multiple modules
translator := dsgo.NewPredict(translateSig, lm)
// Create a module to run in parallel
classifier := dsgo.NewPredict(classifySig, lm)

// Execute in parallel with automatic state isolation
parallel := dsgo.NewParallel(classifier).WithMaxWorkers(3)

// Process multiple inputs in parallel
result, _ := parallel.Forward(ctx, map[string]any{
    "text": []string{
        "I love this product!",
        "This is terrible.",
        "It's okay, nothing special.",
    },
})

// Access results from parallel execution
if completions := result.Completions; completions != nil {
    for i, completion := range completions {
        if sentiment, ok := completion["sentiment"].(string); ok {
            fmt.Printf("Text %d: %s\n", i+1, sentiment)
        }
    }
}
```

### Structured Outputs (Automatic Validation & Retry)

DSGo enables **structured output enforcement by default**. This means:

- **Automatic validation**: Outputs are validated against your signature
- **Automatic retry**: If validation fails, the LM is asked to fix the output (up to 3 attempts by default)
- **Lenient completion**: If all retries exhaust, partial outputs are returned with diagnostics
- **Schema-first formatting**: When your LM supports JSON mode, DSGo uses JSON schemas for better reliability

#### Disable Structured Outputs

To disable structured output enforcement (use legacy behavior):

```go
// Via environment variable
os.Setenv("DSGO_STRUCTURED_OUTPUTS", "false")

// Or programmatically
dsgo.Configure(
    dsgo.WithStructuredOutputEnabled(false),
)
```

#### Tune Structured Outputs

```go
// Increase retry attempts
dsgo.Configure(
    dsgo.WithStructuredOutputMaxAttempts(5),
)

// Use slightly higher temperature for more variation during retries
dsgo.Configure(
    dsgo.WithStructuredOutputTemperature(0.1),
)

// Or all together
dsgo.Configure(
    dsgo.WithStructuredOutputEnabled(true),
    dsgo.WithStructuredOutputMaxAttempts(3),
    dsgo.WithStructuredOutputTemperature(0.0),
)
```

#### Understanding Diagnostics

When a module returns a partial output (validation failed after retries), check `Prediction.ParseDiagnostics`:

```go
result, err := predictor.Forward(ctx, inputs)
if err != nil {
    // Hard error - parsing completely failed
    return err
}

// Check if output is partial (missing required fields)
if result.ParseDiagnostics != nil && result.ParseDiagnostics.HasErrors() {
    fmt.Printf("Partial output with %d missing fields\n", len(result.ParseDiagnostics.MissingFields))
    for _, field := range result.ParseDiagnostics.MissingFields {
        fmt.Printf("  - %s: nil\n", field)
    }
    
    // You can still use the output, but some fields are nil
    // The module tried its best and gave up gracefully
}
```

#### Metadata

When structured output enforcement is active, outputs include `__structured_meta` with:

```go
// Access metadata (for debugging)
if meta, ok := result.Outputs["__structured_meta"].(map[string]any); ok {
    fmt.Printf("Attempts: %v\n", meta["attempts"])
    fmt.Printf("Converged: %v\n", meta["converged"])
    fmt.Printf("Last error: %v\n", meta["last_error"])
}
```

---

## Next Steps

- **See [examples/](examples/)** — Working code for all patterns
- **Read [REFERENCE.md](REFERENCE.md)** — Tables / quick reference
- **Read [README.md](README.md)** — Index + diagrams + tiny example
- **Check [AGENTS.md](AGENTS.md)** — Development guide
- **Review [ROADMAP.md](ROADMAP.md)** — Feature status

### Quick Reference

| Task | Module | Example |
|------|--------|---------|
| Simple I/O | Predict | `dsgo.NewPredict(sig, lm)` |
| Reasoning | ChainOfThought | `dsgo.NewChainOfThought(sig, lm)` |
| Tools | ReAct | `dsgo.NewReAct(sig, lm, tools)` |
| Improvement | Refine | `dsgo.NewRefine(sig, lm).WithMaxIterations(n)` *(uses `inputs["feedback"]`)* |
| Quality | BestOfN | `dsgo.NewBestOfN(base, n).WithScorer(dsgo.DefaultScorer())` |
| Composition | Program | `dsgo.NewProgram("name").AddModule(...)` |
| Parallel | Parallel | `dsgo.NewParallel(module).WithMaxWorkers(n)` |
| Reasoning (code) | ProgramOfThought | `dsgo.NewProgramOfThought(sig, lm, "python")` |
| Synthesis | MultiChainComparison | `dsgo.NewMultiChainComparison(sig, lm, m)` *(expects `inputs["completions"]`)* |

### Common Patterns

```go
// Always handle errors
result, err := predictor.Forward(ctx, inputs)
if err != nil {
    return fmt.Errorf("module failed: %w", err)
}

// Always check usage
fmt.Printf("Cost: $%.6f, Tokens: %d\n", result.Usage.Cost, result.Usage.TotalTokens)

// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
result, err := predictor.Forward(ctx, inputs)
```

Happy coding with DSGo! 🚀
