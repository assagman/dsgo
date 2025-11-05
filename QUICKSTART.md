# DSGo Quick Start Guide

Get started with DSGo in under 1 minute. Build LM-powered apps with structured signatures and composable modules.

## Install & Run (30 seconds)

```bash
# 1. Get the package
go get github.com/assagman/dsgo

# 2. Set your API key (choose one)
export OPENAI_API_KEY=sk-...          # For OpenAI
# or
export OPENROUTER_API_KEY=sk-or-...   # For OpenRouter
# or use DSGO_* prefixed variables
export DSGO_OPENAI_API_KEY=sk-...
export DSGO_OPENROUTER_API_KEY=sk-or-...

# 3. Optional: Configure defaults via environment
export DSGO_PROVIDER=openai
export DSGO_MODEL=gpt-4
export DSGO_TIMEOUT=30
export DSGO_MAX_RETRIES=3

# 4. Run the quick test suite
make test-matrix-quick
```

## Your First Program (2 minutes)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
    "github.com/assagman/dsgo/providers/openai"
)

func main() {
    // Optional: Configure global settings
    dsgo.Configure(
        dsgo.WithProvider("openai"),
        dsgo.WithModel("gpt-4"),
    )

    // 1. Define what you want (Signature)
    sig := dsgo.NewSignature("Classify the sentiment").
        AddInput("text", dsgo.FieldTypeString, "Text to analyze").
        AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

    // 2. Choose your LM
    lm := openai.NewOpenAI("gpt-4")

    // 3. Create a module
    predict := module.NewPredict(sig, lm)

    // 4. Run it
    result, err := predict.Forward(context.Background(), map[string]interface{}{
        "text": "I love this framework!",
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Sentiment: %s\n", result.GetString("sentiment"))
}
```

## Core Concepts (5 minutes)

DSGo uses three main building blocks:

```mermaid
graph LR
    A[1. Signature<br/>Define I/O] --> B[2. Module<br/>Execution Strategy]
    B --> C[3. Language Model<br/>AI Provider]
    D[Tools<br/>Optional] -.-> B
    
    classDef sig fill:#ffd166,stroke:#073b4c,color:#000
    classDef mod fill:#06d6a0,stroke:#073b4c,color:#000
    classDef lm fill:#118ab2,stroke:#073b4c,color:#fff
    classDef tool fill:#ef476f,stroke:#073b4c,color:#fff
    
    class A sig
    class B mod
    class C lm
    class D tool
```

### 1. Signatures = I/O Definition

Define what you want the LM to do:

```go
sig := dsgo.NewSignature("Task description").
    AddInput("input_name", FieldType, "description").
    AddOutput("output_name", FieldType, "description")
```

**Available Types**: String, Int, Float, Bool, JSON, Class, Image, Datetime

### 2. Modules = Execution Strategy

Choose how the LM should approach the task:

```mermaid
graph TD
    START[Choose Module] --> SIMPLE{Need reasoning?}
    SIMPLE -->|No| PRED[Predict<br/>Direct answer]
    SIMPLE -->|Yes| REASON{Need tools?}
    REASON -->|No| COT[ChainOfThought<br/>Think step-by-step]
    REASON -->|Yes| REACT[ReAct<br/>Reason + Tools]
    
    classDef start fill:#ffd166,stroke:#073b4c,color:#000
    classDef module fill:#06d6a0,stroke:#073b4c,color:#000
    classDef decision fill:#118ab2,stroke:#073b4c,color:#fff
    
    class START start
    class PRED,COT,REACT module
    class SIMPLE,REASON decision
```

```go
// Simple - Direct answer
predict := module.NewPredict(sig, lm)

// Reasoning - Think step-by-step
cot := module.NewChainOfThought(sig, lm)

// With tools - Reason and act
react := module.NewReAct(sig, lm, tools)
```

### 3. Tools = Superpowers

Give your LM agent the ability to take actions:

```go
tool := dsgo.NewTool("search", "Search the web",
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        query := args["query"].(string)
        return search(query), nil
    },
).AddParameter("query", "string", "Search query", true)
```

**Flow with Tools:**

```mermaid
sequenceDiagram
    participant User
    participant ReAct
    participant LM
    participant Tool
    
    User->>ReAct: Ask question
    ReAct->>LM: What should I do?
    LM->>ReAct: Use search tool
    ReAct->>Tool: search("query")
    Tool->>ReAct: Results
    ReAct->>LM: Here are results
    LM->>ReAct: Final answer
    ReAct->>User: Answer
```

## Common Patterns

### Classification

```go
sig := dsgo.NewSignature("Classify").
    AddInput("text", dsgo.FieldTypeString, "Input").
    AddClassOutput("category", []string{"A", "B", "C"}, "Category")
```

### Reasoning

```go
sig := dsgo.NewSignature("Solve problem").
    AddInput("problem", dsgo.FieldTypeString, "Problem").
    AddOutput("answer", dsgo.FieldTypeString, "Answer")

cot := module.NewChainOfThought(sig, lm)
```

### Agent with Tools

```go
tools := []dsgo.Tool{searchTool, calculatorTool}
react := module.NewReAct(sig, lm, tools).WithVerbose(true)
```

### Complex Inputs/Outputs

```go
sig := dsgo.NewSignature("Research topic").
    AddInput("topic", dsgo.FieldTypeString, "Topic").
    AddInput("depth", dsgo.FieldTypeInt, "Depth level").
    AddInput("detailed", dsgo.FieldTypeBool, "Include details").
    AddOutput("summary", dsgo.FieldTypeString, "Summary").
    AddOutput("score", dsgo.FieldTypeInt, "Quality score").
    AddClassOutput("confidence", []string{"high", "low"}, "Confidence")
```

## Examples by Use Case

### Need to classify something?
→ `examples/001_predict/` - Classification with Predict

### Need step-by-step reasoning?
→ `examples/013_sentiment/` - Chain of Thought reasoning

### Need caching for performance?
→ `examples/022_caching/` - LM cache demonstration

### Need to use external APIs/tools?
→ `examples/react_agent/` - ReAct with tools

### Building production research tool?
→ `examples/research_assistant/` - Complete example

### Need logging and tracing?
→ `examples/logging_tracing/` - Request ID propagation & observability

## Debugging

### Enable verbose mode (see what the agent is thinking)

```go
react := module.NewReAct(sig, lm, tools).
    WithVerbose(true).  // ← See all iterations
    WithMaxIterations(10)
```

### Check errors

```go
result, err := predict.Forward(ctx, inputs)
if err != nil {
    log.Printf("Error: %v", err)  // Always check errors!
}
```

### Check parse diagnostics

```go
// Inspect parsing diagnostics if available
if diags := result.ParseDiagnostics; diags != nil {
    log.Printf("Missing fields: %v", diags.MissingFields)
    log.Printf("Type errors: %v", diags.TypeErrors)
}
```

## Next Steps

1. **Run Examples** - Explore `examples/` directory:
   - `001_predict/` - Basic Predict
   - `013_sentiment/` - ChainOfThought reasoning
   - `015_fewshot/` - Few-shot learning
   - `022_caching/` - Performance optimization
   - `003_react/` - ReAct with tools
   - `007_program_composition/` - Module pipelines
2. **Read Docs** - Check README.md for complete overview
3. **Build!** - Start with a simple Predict module and expand

## New Harness Infrastructure

Numbered examples (001-028) use the new harness for better observability:

```bash
cd examples/001_predict
go run main.go -verbose -format=json
```

**Harness Features:**
- Concurrent execution (up to 50 workers)
- Automatic error dumps with full History
- Detailed statistics (tokens, cache hits, timing)
- Multiple output formats (summary, json, ndjson)
- CLI flags and environment variable support

## Cheat Sheet

| Want to... | Use this |
|------------|----------|
| Get a quick answer | `module.NewPredict(sig, lm)` |
| Show reasoning steps | `module.NewChainOfThought(sig, lm)` |
| Use external tools | `module.NewReAct(sig, lm, tools)` |
| Classify text | `sig.AddClassOutput(name, []string{...}, desc)` |
| Make field optional | `sig.AddOptionalOutput(name, type, desc)` |
| Debug agent | `.WithVerbose(true)` |
| Limit iterations | `.WithMaxIterations(n)` |
| Few-shot examples | Use `Example` with `module.WithDemos([]dsgo.Example{...})` |

## Common Errors

### "missing required input field"
→ Check you provided all inputs defined in signature

### "failed to parse JSON output"
→ LM didn't return valid JSON (enable verbose to see output; automatic JSON repair may help)

### "invalid class value"
→ LM returned value not in your class list (class normalization handles case/whitespace automatically)

### "API request failed with status 401"
→ Ensure your env var (`OPENAI_API_KEY` or `OPENROUTER_API_KEY`) matches the provider you're using

## Full Documentation

- **README.md** - Complete overview and architecture
- **ROADMAP.md** - Implementation status and future plans
- **AGENTS.md** - Development and testing guide

## Questions?

- Check the examples: `examples/*/main.go`
- Read the docs: `*.md` files
- Check inline comments: All public APIs documented
- Look at tests: `*_test.go` files

---

**You're ready!** Start with `examples/sentiment/main.go` and build from there. 🚀
