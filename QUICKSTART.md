# DSGo Quick Start Guide

Learn DSGo through practical tutorials, from basics to production patterns.

## Table of Contents

- [1. Basic Setup](#1-basic-setup)
- [2. Your First Prediction](#2-your-first-prediction)
- [3. Signatures & Field Types](#3-signatures--field-types)
- [4. Structured Outputs (Classification)](#4-structured-outputs-classification)
- [5. Chain of Thought Reasoning](#5-chain-of-thought-reasoning)
- [6. Tools and ReAct Agents](#6-tools-and-react-agents)
- [7. Module Composition](#7-module-composition)
- [8. Production Patterns](#8-production-patterns)

---

## 1. Basic Setup

### Installation

```bash
go get github.com/assagman/dsgo
```

### Configuration

Set up your API keys and provider:

```bash
export OPENAI_API_KEY=sk-...
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
        dsgo.WithAPIKey("sk-..."),
    )
}
```

---

## 2. Your First Prediction

### Simple Text Generation

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

func main() {
    // Setup
    lm, err := dsgo.NewLM(context.Background(), "gpt-4o-mini")
    if err != nil {
        log.Fatal(err)
    }
    
    // Define signature: inputs and outputs
    sig := dsgo.NewSignature("Answer a question").
        AddInput("question", dsgo.FieldTypeString, "The question to answer").
        AddOutput("answer", dsgo.FieldTypeString, "A helpful answer")
    
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
    fmt.Println(result.GetString("answer"))
    
    // Track usage
    fmt.Printf("Cost: $%.6f, Tokens: %d\n", 
        result.Usage.Cost, result.Usage.TotalTokens)
}
```

**Key Points:**
- `Signature` defines inputs/outputs
- `Predict` module handles basic LLM calls
- `Forward()` executes the module
- `Usage` tracks cost and tokens automatically

---

## 3. Signatures & Field Types

### Available Field Types

```go
sig := dsgo.NewSignature("Process data").
    // Scalar types
    AddInput("text", dsgo.FieldTypeString, "Text input").
    AddInput("count", dsgo.FieldTypeInt, "Integer input").
    AddInput("score", dsgo.FieldTypeFloat, "Float score 0-1").
    AddInput("active", dsgo.FieldTypeBool, "Boolean flag").
    
    // Structured types
    AddInput("metadata", dsgo.FieldTypeJSON, "JSON data").
    AddInput("date", dsgo.FieldTypeDatetime, "Timestamp").
    
    // Classification
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

// Optional fields can be missing - returns nil if not set
result, _ := module.NewPredict(sig, lm).Forward(ctx, inputs)
summary := result.GetString("summary")           // Always present
keywords, ok := result.GetJSON("keywords")       // May be nil
```

### Class/Enum Fields

```go
// With aliases for flexible matching
sig := dsgo.NewSignature("Classify sentiment").
    AddClassOutput("sentiment", 
        []string{"positive", "negative", "neutral"},
        "Sentiment classification").
    WithAlias("sentiment", "pos", "positive").
    WithAlias("sentiment", "neg", "negative").
    WithAlias("sentiment", "neutral", "neutral")

// LM can say "pos" → automatically normalized to "positive"
```

---

## 4. Structured Outputs (Classification)

### Single Classification

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

func main() {
    lm, _ := dsgo.NewLM(context.Background(), "gpt-4o-mini")
    
    sig := dsgo.NewSignature("Classify sentiment").
        AddInput("text", dsgo.FieldTypeString, "Text to classify").
        AddClassOutput("sentiment", 
            []string{"positive", "negative", "neutral"},
            "Sentiment classification")
    
    classifier := module.NewPredict(sig, lm)
    
    result, _ := classifier.Forward(context.Background(), map[string]any{
        "text": "I absolutely love this product!",
    })
    
    sentiment := result.GetString("sentiment")
    fmt.Printf("Sentiment: %s\n", sentiment) // "positive"
}
```

### Multiple Classifications

```go
sig := dsgo.NewSignature("Analyze issue").
    AddInput("issue_text", dsgo.FieldTypeString, "Issue description").
    AddClassOutput("type", []string{"bug", "feature", "docs"}, "Issue type").
    AddClassOutput("priority", []string{"low", "medium", "high"}, "Priority level").
    AddOutput("summary", dsgo.FieldTypeString, "One-line summary")

analyzer := module.NewPredict(sig, lm)
result, _ := analyzer.Forward(ctx, map[string]any{
    "issue_text": "Login button doesn't work on mobile",
})

issueType := result.GetString("type")     // "bug"
priority := result.GetString("priority")  // "high"
summary := result.GetString("summary")
```

### Validation & Error Handling

```go
result, err := classifier.Forward(ctx, inputs)
if err != nil {
    log.Printf("Prediction failed: %v", err)
    return
}

// Check for validation errors
if result.ParseDiagnostics != nil {
    fmt.Printf("Missing fields: %v\n", result.ParseDiagnostics.MissingFields)
    fmt.Printf("Type errors: %v\n", result.ParseDiagnostics.TypeErrors)
}

// Safe getters return zero values if missing
sentiment := result.GetString("sentiment")  // "" if missing
confidence := result.GetFloat("confidence") // 0.0 if missing
```

---

## 5. Chain of Thought Reasoning

### Add Reasoning Steps

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

func main() {
    lm, _ := dsgo.NewLM(context.Background(), "gpt-4o-mini")
    
    sig := dsgo.NewSignature("Solve a math problem").
        AddInput("problem", dsgo.FieldTypeString, "Math problem to solve").
        AddOutput("reasoning", dsgo.FieldTypeString, "Step-by-step reasoning").
        AddOutput("answer", dsgo.FieldTypeString, "Final numerical answer")
    
    // ChainOfThought prompts for reasoning first
    solver := module.NewChainOfThought(sig, lm)
    
    result, _ := solver.Forward(context.Background(), map[string]any{
        "problem": "If Alice has 5 apples and Bob has 3, how many do they have together?",
    })
    
    fmt.Println("Reasoning:")
    fmt.Println(result.GetString("reasoning"))
    fmt.Printf("\nAnswer: %s\n", result.GetString("answer"))
}
```

**What ChainOfThought does:**
1. Prompts the LM to think step-by-step
2. Extracts both reasoning and final answer
3. Better for complex reasoning tasks
4. Slightly higher token usage than basic Predict

---

## 6. Tools and ReAct Agents

### Define a Tool

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

// Simple search tool
func searchWeb(ctx context.Context, args map[string]interface{}) (string, error) {
    query := args["query"].(string)
    // In real code, call a search API
    return fmt.Sprintf("Results for '%s': Wikipedia, Google, etc.", query), nil
}

func main() {
    lm, _ := dsgo.NewLM(context.Background(), "gpt-4o-mini")
    
    // Define tool
    searchTool := dsgo.NewTool(
        "search",
        "Search the web for information",
        searchWeb,
    ).AddParameter("query", dsgo.FieldTypeString, "Search query", true)
    
    sig := dsgo.NewSignature("Answer questions using available tools").
        AddInput("question", dsgo.FieldTypeString, "Question to answer").
        AddOutput("answer", dsgo.FieldTypeString, "Final answer based on tool results")
    
    // ReAct: Reason + Act loop
    agent := module.NewReAct(sig, lm, []dsgo.Tool{*searchTool})
    
    result, _ := agent.Forward(context.Background(), map[string]any{
        "question": "Who is the current president of France?",
    })
    
    fmt.Printf("Answer: %s\n", result.GetString("answer"))
}
```

### Tool Parameters

```go
tool := dsgo.NewTool("calculate", "Perform calculations", calcFunc).
    AddParameter("operation", dsgo.FieldTypeString, "add/multiply/divide", true).
    AddParameter("a", dsgo.FieldTypeFloat, "First number", true).
    AddParameter("b", dsgo.FieldTypeFloat, "Second number", true).
    AddParameter("precision", dsgo.FieldTypeInt, "Decimal places", false) // Optional
```

### Multiple Tools

```go
searchTool := dsgo.NewTool("search", "Search web", searchFunc).
    AddParameter("query", dsgo.FieldTypeString, "Search query", true)

calculatorTool := dsgo.NewTool("calculate", "Math operations", calcFunc).
    AddParameter("expression", dsgo.FieldTypeString, "Math expression", true)

agent := module.NewReAct(sig, lm, []dsgo.Tool{
    *searchTool,
    *calculatorTool,
})
```

---

## 7. Module Composition

### Chain Modules into Pipelines

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

func main() {
    lm, _ := dsgo.NewLM(context.Background(), "gpt-4o-mini")
    
    // Stage 1: Generate initial response
    stage1Sig := dsgo.NewSignature("Generate article outline").
        AddInput("topic", dsgo.FieldTypeString, "Article topic").
        AddOutput("outline", dsgo.FieldTypeString, "Numbered outline")
    
    stage1 := module.NewPredict(stage1Sig, lm)
    
    // Stage 2: Refine/improve with feedback
    stage2Sig := dsgo.NewSignature("Improve outline").
        AddInput("outline", dsgo.FieldTypeString, "Original outline").
        AddInput("feedback", dsgo.FieldTypeString, "Improvement suggestions").
        AddOutput("improved_outline", dsgo.FieldTypeString, "Refined outline")
    
    stage2 := module.NewPredict(stage2Sig, lm)
    
    // Execute pipeline
    stage1Result, _ := stage1.Forward(context.Background(), map[string]any{
        "topic": "Machine Learning Basics",
    })
    
    outline := stage1Result.GetString("outline")
    
    stage2Result, _ := stage2.Forward(context.Background(), map[string]any{
        "outline": outline,
        "feedback": "Add more details about neural networks",
    })
    
    finalOutline := stage2Result.GetString("improved_outline")
    fmt.Println(finalOutline)
}
```

### Using Program Module

```go
// More composable approach with Program
program := module.NewProgram().
    AddModule("generate", stage1).
    AddModule("improve", stage2).
    SetInputMapping("generate", map[string]string{"topic": "topic"}).
    SetInputMapping("improve", map[string]string{
        "outline": "generate:outline",      // Use output from generate
        "feedback": "feedback",              // Direct input
    }).
    SetOutput("final", "improve:improved_outline")

// result contains outputs from all stages
result, _ := program.Forward(ctx, map[string]any{
    "topic": "ML Basics",
    "feedback": "Add details on neural nets",
})

fmt.Println(result.GetString("final"))
```

---

## 8. Production Patterns

### Cost Tracking

```go
predictor := module.NewPredict(sig, lm)
result, _ := predictor.Forward(ctx, inputs)

fmt.Printf("Token usage:\n")
fmt.Printf("  Prompt:     %d\n", result.Usage.PromptTokens)
fmt.Printf("  Completion: %d\n", result.Usage.CompletionTokens)
fmt.Printf("  Total:      %d\n", result.Usage.TotalTokens)
fmt.Printf("  Latency:    %dms\n", result.Usage.Latency)
fmt.Printf("  Cost:       $%.6f\n", result.Usage.Cost)

// Accumulate across batch
totalCost += result.Usage.Cost
totalTokens += result.Usage.TotalTokens
```

### Custom Logging

```go
type ProductionCollector struct {
    logger Logger
}

func (c *ProductionCollector) Collect(entry core.HistoryEntry) {
    c.logger.Log("llm_call", map[string]interface{}{
        "provider":     entry.Provider,
        "model":        entry.Model,
        "tokens":       entry.Usage.TotalTokens,
        "cost":         entry.Usage.Cost,
        "latency_ms":   entry.Usage.Latency,
        "request_id":   entry.RequestID,
        "cache_hit":    entry.CacheHit,
    })
}

dsgo.Configure(dsgo.WithCollector(&ProductionCollector{logger: myLogger}))
```

### Streaming with Callbacks

```go
predictor := module.NewPredict(sig, lm)
streamResult, _ := predictor.Stream(ctx, inputs)

// Process chunks as they arrive
for chunk := range streamResult.Chunks {
    fmt.Print(chunk.Content) // Clean content
}

// Get final result
result := <-streamResult.Prediction
err := <-streamResult.Errors

if err != nil {
    log.Printf("Streaming error: %v", err)
}

fmt.Printf("\nFinal: %s\n", result.GetString("output"))
```

### Retry with BestOfN

```go
sig := dsgo.NewSignature("Generate creative idea").
    AddInput("topic", dsgo.FieldTypeString, "Topic for ideas").
    AddOutput("idea", dsgo.FieldTypeString, "Creative idea")

// Generate 3 variations, pick best
bestof := module.NewBestOfN(sig, lm, 3)

// With scoring function
bestof = bestof.WithScorer(func(ctx context.Context, pred *core.Prediction) (float64, error) {
    idea := pred.GetString("idea")
    // Score based on length, uniqueness, etc.
    return float64(len(idea)) / 100.0, nil
})

result, _ := bestof.Forward(ctx, map[string]any{
    "topic": "sustainable technology",
})

fmt.Println(result.GetString("idea"))
```

### Few-Shot Learning

```go
sig := dsgo.NewSignature("Classify document").
    AddInput("text", dsgo.FieldTypeString, "Document text").
    AddClassOutput("category", []string{"sales", "support", "billing"}, "Department")

predictor := module.NewPredict(sig, lm)

// Add examples to help the model
predictor = predictor.WithExamples([]core.Example{
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

### Caching

```go
// DSGo caches by default with deterministic keys
// Cache includes all parameters (messages, model, options, etc.)

predictor := module.NewPredict(sig, lm)

// Same input = cache hit
result1, _ := predictor.Forward(ctx, map[string]any{"text": "Hello"})
result2, _ := predictor.Forward(ctx, map[string]any{"text": "Hello"})

// result2 from cache (faster, no tokens charged)
fmt.Printf("Cache hit: %v\n", result2.CacheHit)
```

### Error Handling

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

// Proceed even with partial results
answer := result.GetString("answer")
confidence := result.GetFloat("confidence") // 0.0 if missing
```

---

## Next Steps

- **See [examples/](examples/)** — Working code for all patterns
- **Read [README.md](README.md)** — Full API reference
- **Check [AGENTS.md](AGENTS.md)** — Development guide
- **Review [ROADMAP.md](ROADMAP.md)** — Feature status
