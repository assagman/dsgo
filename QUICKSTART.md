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
# OpenAI (for GPT models) - DSGO_* preferred, OPENAI_* supported
export DSGO_OPENAI_API_KEY=sk-...
export OPENAI_API_KEY=sk-...

# OpenRouter (access to 100+ models) - DSGO_* preferred, OPENROUTER_* supported
export DSGO_OPENROUTER_API_KEY=sk-or-v1-...
export OPENROUTER_API_KEY=sk-or-v1-...
```

API keys must be provided via environment variables; `core.Configure()` returns an error if none are set.

Providers must be imported for registration:

```go
import (
    _ "github.com/assagman/dsgo/provider/openai"
    _ "github.com/assagman/dsgo/provider/openrouter"
)
```

### Choosing a Model

DSGo uses the `provider/model` format:

```go
// OpenAI models
lm, _ := core.NewLM(ctx, "openai/gpt-4o-mini")
lm, _ := core.NewLM(ctx, "openai/gpt-4o")

// OpenRouter models (access to 100+ models)
lm, _ := core.NewLM(ctx, "openrouter/google/gemini-2.5-flash")
lm, _ := core.NewLM(ctx, "openrouter/meta-llama/llama-3.1-8b-instruct")
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

    "github.com/assagman/dsgo/core"
    "github.com/assagman/dsgo/module"
    _ "github.com/assagman/dsgo/provider/openrouter"
)

func main() {
    // Create LM instance
    if err := core.Configure(); err != nil {
        log.Fatal(err)
    }
    lm, err := core.NewLM(context.Background(), "openrouter/google/gemini-2.5-flash")
    if err != nil {
        log.Fatal(err)
    }
    
    // Define signature: inputs and outputs
    sig := core.NewSignature("Answer a question").
        AddInput("question", core.FieldTypeString, "The question to answer").
        AddOutput("answer", core.FieldTypeString, "A helpful answer")
    
    // Create predictor module
    predictor := module.NewPredict(sig, lm)
    
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
sig := core.NewSignature("Process data").
    // Basic types
    AddInput("text", core.FieldTypeString, "Text input").
    AddInput("count", core.FieldTypeInt, "Integer input").
    AddInput("score", core.FieldTypeFloat, "Float score 0-1").
    AddInput("active", core.FieldTypeBool, "Boolean flag").
    
    // Structured types
    AddInput("metadata", core.FieldTypeJSON, "JSON data").
    AddInput("timestamp", core.FieldTypeDatetime, "Timestamp").
    
    // Classification (enums)
    AddClassInput("priority", []string{"low", "medium", "high"}, "Task priority").
    AddClassOutput("category", []string{"bug", "feature", "docs"}, "Issue category").
    
    // Outputs
    AddOutput("result", core.FieldTypeString, "Processing result").
    AddOptionalOutput("error_info", core.FieldTypeString, "Error details if any")
```

### Optional Fields

```go
sig := core.NewSignature("Summarize text").
    AddInput("text", core.FieldTypeString, "Text to summarize").
    AddOutput("summary", core.FieldTypeString, "Summary").
    AddOptionalOutput("keywords", core.FieldTypeJSON, "Extracted keywords")

// Optional fields can be missing
result, _ := module.NewPredict(sig, lm).Forward(ctx, inputs)
summary := result.GetString("summary")          // Always present
keywords, hasKeywords := result.GetJSON("keywords") // May be nil
```

### Classification with Aliases

```go
// Flexible matching for classification tasks
sig := core.NewSignature("Classify sentiment").
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
sig := core.NewSignature("Translate text").
    AddInput("text", core.FieldTypeString, "Text to translate").
    AddInput("target_language", core.FieldTypeString, "Target language").
    AddOutput("translation", core.FieldTypeString, "Translated text")

translator := module.NewPredict(sig, lm)

result, _ := translator.Forward(ctx, map[string]any{
    "text": "Hello world",
    "target_language": "Spanish",
})
fmt.Println(result.GetString("translation")) // "Hola mundo"
```

### ChainOfThought - Step-by-Step Reasoning

For complex tasks requiring reasoning:

```go
sig := core.NewSignature("Solve math problem").
    AddInput("problem", core.FieldTypeString, "Math problem to solve").
    AddOutput("reasoning", core.FieldTypeString, "Step-by-step reasoning").
    AddOutput("answer", core.FieldTypeString, "Final numerical answer")

solver := module.NewChainOfThought(sig, lm)

result, _ := solver.Forward(ctx, map[string]any{
    "problem": "If Alice has 5 apples and Bob has 3, how many do they have together?",
})

fmt.Println("Reasoning:", result.GetString("reasoning"))
fmt.Println("Answer:", result.GetString("answer"))
```

### ReAct - Tool-Using Agents

ReAct is DSGo’s native tool-calling agent loop. It’s designed to keep tool trajectories bounded and to reliably produce outputs that match your `Signature`.

```go
// Tool: keep result sizes bounded (cost + context control).
func searchWeb(ctx context.Context, args map[string]any) (any, error) {
    query := args["query"].(string)

    // Size control: default to a small summary.
    maxChars := 800
    if v, ok := args["max_chars"].(float64); ok { // JSON numbers decode as float64
        maxChars = int(v)
    }

    result := fmt.Sprintf("Search results for %q: Wikipedia, news, etc...", query)
    if len(result) > maxChars {
        result = result[:maxChars] + "...(truncated)"
    }
    return result, nil
}

searchTool := core.NewTool(
    "search",
    "Search the web (return a short summary)",
    searchWeb,
).
    AddParameter("query", "string", "Search query", true).
    AddParameter("max_chars", "integer", "Max chars to return (size control)", false)

sig := core.NewSignature("Answer questions using tools").
    AddInput("question", core.FieldTypeString, "Question to answer").
    AddOutput("answer", core.FieldTypeString, "Final answer based on tool results")

history := core.NewHistoryWithLimit(20) // trajectory limit for multi-turn chats

agent := module.NewReAct(sig, lm, []core.Tool{*searchTool}).
    WithHistory(history).
    WithMaxIterations(8) // trajectory limit for a single run

pred, err := agent.Forward(ctx, map[string]any{
    "question": "Who is the current president of France?",
})
if err != nil {
    fmt.Println("error:", err)
    return
}
fmt.Println(pred.GetString("answer"))
```

#### ReAct output guarantees

- **Native tool calling first**: Tools execute only when the selected model/provider supports native tool calls. For non-tool models, use `Predict`/`ChainOfThought` or switch models.
- **Structured finish (recommended)**: DSGo auto-injects a synthetic `finish` tool (unless you provide one) whose arguments mirror your output fields. The model can end the loop by calling `finish(answer=...)`.
- **Forced final mode**: On the last iteration DSGo requests a final JSON object matching your output signature and disallows further tool calls.
- **Extractor fallback**: If parsing/validation fails or the loop hits limits, DSGo runs a post-loop extraction step that synthesizes a valid final JSON answer from the full tool trajectory.

Structured output enforcement is enabled by default and controls the extractor’s retry behavior:

```go
if err := core.Configure(
    core.WithStructuredOutputEnabled(true),
    core.WithStructuredOutputMaxAttempts(3),
    core.WithStructuredOutputTemperature(0.0),
); err != nil {
    log.Fatal(err)
}
```

#### ReAct behavioral details

- **Bounded trajectories**: Tool results are truncated to `MaxToolResultBytes` (default 16KB) and encoded as JSON envelopes. The trajectory is rendered within a soft prompt budget (`MaxPromptBytes`, default 256KB), keeping only the newest steps that fit.
- **Context overflow recovery**: If a provider returns a context-length error, ReAct drops the oldest trajectory steps and retries (up to 3 times).
- **Strict tool schemas**: Tool parameter schemas include `additionalProperties: false`, so providers will reject tool calls with extra keys not in your schema.
- **Termination policies**: The loop terminates early on repeated identical tool calls (3 in a row), repeated identical observations (stagnation), or consecutive tool errors (2 in a row). Termination reason is available in `Prediction.Metadata["termination_reason"]`.

### Refine - Iterative Improvement

For improving outputs through iteration:

```go
sig := core.NewSignature("Write professional email").
    AddInput("topic", core.FieldTypeString, "Email topic").
    AddInput("tone", core.FieldTypeString, "Desired tone (formal/casual)").
    AddOutput("email", core.FieldTypeString, "Final email")

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
sig := core.NewSignature("Generate marketing slogan").
    AddInput("product", core.FieldTypeString, "Product name").
    AddOutput("slogan", core.FieldTypeString, "Marketing slogan")

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

### ChainOfThought - Step-by-Step Reasoning

For complex tasks requiring reasoning:

```go
sig := core.NewSignature("Solve math problem").
    AddInput("problem", core.FieldTypeString, "Math problem to solve").
    AddOutput("reasoning", core.FieldTypeString, "Step-by-step reasoning").
    AddOutput("answer", core.FieldTypeString, "Final numerical answer")

solver := module.NewChainOfThought(sig, lm)

result, _ := solver.Forward(ctx, map[string]any{
    "problem": "If Alice has 5 apples and Bob has 3, how many do they have together?",
})

fmt.Println("Reasoning:", result.GetString("reasoning"))
fmt.Println("Answer:", result.GetString("answer"))
```

### ReAct - Tool-Using Agents

Same API, but constructed from the `module` package:

```go
searchTool := core.NewTool(
    "search",
    "Search the web (return a short summary)",
    searchWeb,
).
    AddParameter("query", "string", "Search query", true).
    AddParameter("max_chars", "integer", "Max chars to return (size control)", false)

sig := core.NewSignature("Answer questions using tools").
    AddInput("question", core.FieldTypeString, "Question to answer").
    AddOutput("answer", core.FieldTypeString, "Final answer based on tool results")

agent := module.NewReAct(sig, lm, []core.Tool{*searchTool}).WithMaxIterations(8)

pred, err := agent.Forward(ctx, map[string]any{
    "question": "Who is the current president of France?",
})
if err != nil {
    fmt.Println("error:", err)
    return
}
fmt.Println(pred.GetString("answer"))
```

### Refine - Iterative Improvement

For improving outputs through iteration:

```go
sig := core.NewSignature("Write professional email").
    AddInput("topic", core.FieldTypeString, "Email topic").
    AddInput("tone", core.FieldTypeString, "Desired tone (formal/casual)").
    AddOutput("email", core.FieldTypeString, "Final email")

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
sig := core.NewSignature("Generate marketing slogan").
    AddInput("product", core.FieldTypeString, "Product name").
    AddOutput("slogan", core.FieldTypeString, "Marketing slogan")

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

calcTool := core.NewTool("calculate", "Perform mathematical operations", calculate).
    AddParameter("operation", "string", "Operation (add/multiply/divide)", true).
    AddParameter("a", "number", "First number", true).
    AddParameter("b", "number", "Second number", true).
    AddParameter("precision", "integer", "Decimal places", false) // Optional
```

#### Tool schema best practices

- Keep parameter schemas **small and explicit**: prefer a few required fields over a single “blob” argument.
- Use constrained types:
  - `AddEnumParameter(...)` for flags/modes
  - `AddArrayParameter(...)` for lists
- Put operational constraints in the schema (so the model can self-regulate): `max_results`, `max_chars`, `page`, `timeout_ms`.

#### Tool result size controls

Tool outputs become part of the ReAct trajectory. Treat them like *prompt tokens*:

- Default to **summaries**, not raw payloads.
- Add and enforce a `max_chars`/`max_bytes` argument and truncate server-side.
- Prefer “preview” tools (e.g. `read_file(path, max_bytes)`) over returning full files.

### Multi-Tool Agents

```go
weatherTool := core.NewTool("get_weather", "Get current weather", getWeatherFunc).
    AddParameter("location", "string", "City name", true)

tools := []core.Tool{*calcTool, *weatherTool}

sig := core.NewSignature("Helpful assistant").
    AddInput("request", core.FieldTypeString, "User request").
    AddOutput("response", core.FieldTypeString, "Helpful response")

agent := module.NewReAct(sig, lm, tools)
```

### Using MCP Tools

MCP (Model Context Protocol) allows your agents to access external tools:

```go
// Initialize Exa MCP for web search
exaClient, err := mcp.NewExaClient(os.Getenv("EXA_API_KEY"))
if err != nil {
    log.Fatal(err)
}
if err := exaClient.Initialize(ctx); err != nil {
    log.Fatal(err)
}

// Create ReAct agent with MCP tools
tools := exaClient.GetTools()
agent := module.NewReAct(sig, lm, tools)

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
outlineSig := core.NewSignature("Generate article outline").
    AddInput("topic", core.FieldTypeString, "Article topic").
    AddOutput("outline", core.FieldTypeString, "Numbered outline")

outliner := module.NewPredict(outlineSig, lm)

// Stage 2: Expand outline
expandSig := core.NewSignature("Expand outline to full article").
    AddInput("outline", core.FieldTypeString, "Article outline").
    AddOutput("article", core.FieldTypeString, "Full article text")

expander := module.NewPredict(expandSig, lm)

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
program := module.NewProgram("ArticleGenerator").
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
predictor := module.NewPredict(sig, lm)
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
fmt.Println(modelcatalog.IsValid("openai/gpt-4o-mini"))
for _, m := range modelcatalog.ListModelsByProvider("openai") {
    pricing, ok := cost.DefaultCalculator.GetPricing(m.ID)
    if !ok {
        fmt.Println(m.ID)
        continue
    }

    fmt.Printf("%s price=%+v ctx=%d out=%d tools=%v json=%v\n",
        m.ID,
        pricing,
        m.Limits.ContextTokens,
        m.Limits.OutputTokens,
        m.Capabilities.ToolCall,
        m.Capabilities.StructuredOutput,
    )
}
```

If you register a custom provider via `core.RegisterLM`, you must also register its supported models:

```go
_ = modelcatalog.RegisterModel(modelcatalog.Model{ID: "myprovider/my-model"})
```

### Caching

DSGo includes automatic LRU caching:

```go
// Configure cache
if err := core.Configure(
    core.WithCache(1000),              // 1000 entry capacity
    core.WithCacheTTL(5 * time.Minute), // 5 minute TTL
); err != nil {
    log.Fatal(err)
}

predictor := module.NewPredict(sig, lm)

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
predictor := module.NewPredict(sig, lm)
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
predictor := module.NewPredict(sig, lm).
    WithDemos([]core.Example{
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
jsonPredictor := module.NewPredict(sig, lm).WithAdapter(core.NewJSONAdapter())

// Use fallback adapter for robustness (default behavior: JSON → Chat)
fallbackPredictor := module.NewPredict(sig, lm).WithAdapter(
    core.NewFallbackAdapterWithChain(
        core.NewJSONAdapter(),   // Try first
        core.NewChatAdapter(),   // Fallback
    ),
)
```

> **OpenAI Compatibility**: DSGo automatically detects OpenAI providers and uses OpenAI-compliant JSON schemas. No manual configuration needed - structured outputs work seamlessly with GPT models.

*Note: The JSON adapter and other adapter types live in `core/`.*

### Observability

Track all LLM interactions automatically when a collector is configured:

```go
type ProductionCollector struct{}

func (c *ProductionCollector) Collect(entry core.HistoryEntry) {
    log.Printf("LLM Call: provider=%s model=%s tokens=%d cost=$%.6f latency=%dms",
        entry.Provider, entry.Model, entry.Usage.TotalTokens,
        entry.Usage.Cost, entry.Usage.Latency)
}

// Observability is automatically enabled when a collector is configured
if err := core.Configure(core.WithCollector(&ProductionCollector{})); err != nil {
    log.Fatal(err)
}
```

*Note: The HistoryEntry and observability infrastructure live in `core/` and `logging/`.*

### Parallel Execution

Run multiple modules concurrently:

```go
// Create multiple modules
translator := module.NewPredict(translateSig, lm)
// Create a module to run in parallel
classifier := module.NewPredict(classifySig, lm)

// Execute in parallel with automatic state isolation
parallel := module.NewParallel(classifier).WithMaxWorkers(3)

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
if err := core.Configure(
    core.WithStructuredOutputEnabled(false),
); err != nil {
    log.Fatal(err)
}
```

#### Tune Structured Outputs

```go
// Increase retry attempts
if err := core.Configure(
    core.WithStructuredOutputMaxAttempts(5),
); err != nil {
    log.Fatal(err)
}

// Use slightly higher temperature for more variation during retries
if err := core.Configure(
    core.WithStructuredOutputTemperature(0.1),
); err != nil {
    log.Fatal(err)
}

// Or all together
if err := core.Configure(
    core.WithStructuredOutputEnabled(true),
    core.WithStructuredOutputMaxAttempts(3),
    core.WithStructuredOutputTemperature(0.0),
); err != nil {
    log.Fatal(err)
}
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
- **Read [REFERENCE.md](REFERENCE.md)** — Tables / quick reference
- **Read [README.md](README.md)** — Index + diagrams + tiny example
- **Check [AGENTS.md](AGENTS.md)** — Development guide
- **Review [ROADMAP.md](ROADMAP.md)** — Feature status

### Quick Reference

| Task | Module | Example |
|------|--------|---------|
| Simple I/O | Predict | `module.NewPredict(sig, lm)` |
| Reasoning | ChainOfThought | `module.NewChainOfThought(sig, lm)` |
| Tools | ReAct | `module.NewReAct(sig, lm, tools)` |
| Improvement | Refine | `module.NewRefine(sig, lm).WithMaxIterations(n)` *(uses `inputs["feedback"]`)* |
| Quality | BestOfN | `module.NewBestOfN(base, n).WithScorer(module.DefaultScorer())` |
| Composition | Program | `module.NewProgram("name").AddModule(...)` |
| Parallel | Parallel | `module.NewParallel(module).WithMaxWorkers(n)` |
| Reasoning (code) | ProgramOfThought | `module.NewProgramOfThought(sig, lm, "python")` |
| Synthesis | MultiChainComparison | `module.NewMultiChainComparison(sig, lm, m)` *(expects `inputs["completions"]`)* |

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
