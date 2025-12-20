# DSGo

DSPy-inspired AI Agent Framework for Go. Build structured LLM workflows and tool-using agents with type-safe signatures, composable modules, and pluggable providers.

## Why DSGo

- Structured output: Define explicit input/output signatures with types and validation.
- Compose modules (Predict, ChainOfThought, ReAct, Refine, BestOfN, Program, Parallel, ProgramOfThought, MultiChainComparison).
- Parse outputs reliably with adapters (JSON, Chat, TwoStep, Fallback).
- Swap LLM providers (OpenAI, OpenRouter, Mock) via a shared LM interface.

## Quick start

Requires Go 1.25+.

Install:

```bash
go get github.com/assagman/dsgo
```

Set an API key (DSGO_* preferred):

```bash
export DSGO_OPENAI_API_KEY=sk-...
# or
export DSGO_OPENROUTER_API_KEY=sk-or-v1-...
```

Minimal example:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/assagman/dsgo/core"
    "github.com/assagman/dsgo/module"
    _ "github.com/assagman/dsgo/provider/openai"
)

func main() {
    ctx := context.Background()

    if err := core.Configure(); err != nil {
        log.Fatal(err)
    }

    lm, err := core.NewLM(ctx, "openai/gpt-4o-mini")
    if err != nil {
        log.Fatal(err)
    }

    sig := core.NewSignature("Classify sentiment").
        AddInput("text", core.FieldTypeString, "Text to classify").
        AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

    pred, err := module.NewPredict(sig, lm).Forward(ctx, map[string]any{
        "text": "I love this product",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Sentiment:", pred.GetString("sentiment"))
}
```

## Documentation index

Start here:

- [`QUICKSTART.md`](QUICKSTART.md) - step-by-step tutorials
- [`REFERENCE.md`](REFERENCE.md) - tables and quick lookup
- [`ARCHITECTURE.md`](ARCHITECTURE.md) - layers and design
- [`DEVELOPMENT.md`](DEVELOPMENT.md) - contributor workflow
- [`ROADMAP.md`](ROADMAP.md) - current status and planned work
- [`AGENTS.md`](AGENTS.md) - env vars and AI agent workflow
- [`llms.txt`](llms.txt) - LLM-friendly documentation

Package docs:

- [`core/README.md`](core/README.md) - signatures, adapters, prediction, LM interface
- [`module/README.md`](module/README.md) - module catalog and usage
- [`provider/openai/README.md`](provider/openai/README.md) - OpenAI provider
- [`provider/openrouter/README.md`](provider/openrouter/README.md) - OpenRouter provider
- [`provider/mock/README.md`](provider/mock/README.md) - mock provider for tests
- [`signature_typed/README.md`](signature_typed/README.md) - typed signatures and helpers
- [`logging/README.md`](logging/README.md) - structured logging
- [`mcp/README.md`](mcp/README.md) - Model Context Protocol integration
- [`modelcatalog/README.md`](modelcatalog/README.md) - model registry
- [`cost/README.md`](cost/README.md) - pricing tables
- [`scripts/README.md`](scripts/README.md) - build/test scripts
