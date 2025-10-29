# DSGo Quick Start Guide

## Install & Run (30 seconds)

```bash
# 1. Get the package
go get github.com/assagman/dsgo

# 2. Set your OpenAI key
export OPENROUTER_API_KEY =sk-or-your-key-here # or OPENAI_API_KEY

# 3. Run an example
go run examples/sentiment/main.go
```

## Your First Program (2 minutes)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/examples/openai"
)

func main() {
    // 1. Define what you want (Signature)
    sig := dsgo.NewSignature("Classify the sentiment").
        AddInput("text", dsgo.FieldTypeString, "Text to analyze").
        AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

    // 2. Choose your LM
    lm := openai.NewOpenAI("gpt-4")

    // 3. Create a module
    predict := dsgo.NewPredict(sig, lm)

    // 4. Run it
    result, err := predict.Forward(context.Background(), map[string]interface{}{
        "text": "I love this framework!",
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Sentiment: %s\n", result["sentiment"])
}
```

## Core Concepts (5 minutes)

### 1. Signatures = I/O Definition

```go
sig := dsgo.NewSignature("Task description").
    AddInput("input_name", FieldType, "description").
    AddOutput("output_name", FieldType, "description")
```

**Available Types**: String, Int, Float, Bool, JSON, Class, Image, Datetime

### 2. Modules = Execution Strategy

- **Predict**: Direct answer
- **ChainOfThought**: Think step-by-step
- **ReAct**: Use tools to find answer

```go
// Simple
predict := dsgo.NewPredict(sig, lm)

// Reasoning
cot := dsgo.NewChainOfThought(sig, lm)

// With tools
react := dsgo.NewReAct(sig, lm, tools)
```

### 3. Tools = Superpowers

```go
tool := dsgo.NewTool("search", "Search the web",
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        query := args["query"].(string)
        return search(query), nil
    },
).AddParameter("query", "string", "Search query", true)
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

cot := dsgo.NewChainOfThought(sig, lm)
```

### Agent with Tools

```go
tools := []dsgo.Tool{searchTool, calculatorTool}
react := dsgo.NewReAct(sig, lm, tools).WithVerbose(true)
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
→ `examples/sentiment/` - Classification with Predict

### Need step-by-step reasoning?
→ `examples/sentiment/` - Chain of Thought

### Need to use external APIs/tools?
→ `examples/react_agent/` - ReAct with tools

### Building production research tool?
→ `examples/research_assistant/` - Complete example

## Debugging

### Enable verbose mode (see what the agent is thinking)

```go
react := dsgo.NewReAct(sig, lm, tools).
    WithVerbose(true).  // ← See all iterations
    WithMaxIterations(10)
```

### Check errors

```go
result, err := module.Forward(ctx, inputs)
if err != nil {
    log.Printf("Error: %v", err)  // Always check errors!
}
```

### Validate before running

```go
// Signature validates inputs automatically
err := sig.ValidateInputs(inputs)

// And outputs
err := sig.ValidateOutputs(outputs)
```

## Next Steps

1. ✅ Run `examples/sentiment/` - See basic usage
2. ✅ Run `examples/react_agent/` - See tools in action
3. ✅ Run `examples/research_assistant/` - See everything together
4. ✅ Read `EXAMPLES.md` - Detailed walkthrough
5. ✅ Build your own! - Use these patterns

## Cheat Sheet

| Want to... | Use this |
|------------|----------|
| Get a quick answer | `NewPredict(sig, lm)` |
| Show reasoning steps | `NewChainOfThought(sig, lm)` |
| Use external tools | `NewReAct(sig, lm, tools)` |
| Classify text | `AddClassOutput(name, []string{...}, desc)` |
| Make field optional | `AddOptionalOutput(name, type, desc)` |
| Debug agent | `.WithVerbose(true)` |
| Limit iterations | `.WithMaxIterations(n)` |
| Change temperature | `.WithOptions(&GenerateOptions{Temperature: 0.7})` |

## Common Errors

### "missing required input field"
→ Check you provided all inputs defined in signature

### "failed to parse JSON output"
→ LM didn't return valid JSON (enable verbose to see output)

### "invalid class value"
→ LM returned value not in your class list (adjust options or list)

### "API request failed with status 401"
→ Check your `OPENAI_API_KEY` environment variable

## Full Documentation

- **README.md** - Complete overview
- **EXAMPLES.md** - Example walkthrough
- **AGENTS.md** - Development guide
- **IMPLEMENTATION.md** - Technical deep dive
- **SUMMARY.md** - What was built

## Questions?

- Check the examples: `examples/*/main.go`
- Read the docs: `*.md` files
- Check inline comments: All public APIs documented
- Look at tests: `*_test.go` files

---

**You're ready!** Start with `examples/sentiment/main.go` and build from there. 🚀
