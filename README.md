# DSGo

**Composable LLM orchestration framework for Go** — inspired by DSPy, built for production.

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Core Concepts](#core-concepts)
  - [Signatures](#signatures)
  - [Modules](#modules)
  - [Adapters](#adapters)
  - [Tools](#tools)
- [Configuration](#configuration)
- [Observability](#observability)
- [Examples](#examples)
- [Testing](#testing)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Overview

DSGo is a Go port of [DSPy](https://github.com/stanfordnlp/dspy) that brings structured LLM programming to Go applications. It provides composable modules, robust structured output parsing, tool calling, and production-grade observability.

**Key Philosophy:**
- **Composable**: Build complex LLM behaviors from simple, reusable modules
- **Structured**: Define inputs/outputs with typed signatures, get validated results
- **Resilient**: Automatic parsing fallbacks and error recovery for real-world LLM outputs
- **Observable**: Built-in cost tracking, latency monitoring, and request logging

## Features

### 🎯 Structured I/O
- **Type-safe signatures** with validation (string, int, float, bool, json, class/enum, image, datetime)
- **Robust parsing** with automatic JSON repair and field marker extraction
- **Enum normalization** with aliases and case-insensitive matching
- **Streaming support** with cleaned output chunks

### 🔧 Modules
- **Predict** — Basic LLM prediction with structured outputs
- **ChainOfThought** — Step-by-step reasoning with rationale extraction
- **ReAct** — Tool-using agent with reasoning + acting loop
- **Refine** — Iterative output improvement with feedback
- **ProgramOfThought** — Code generation and optional execution
- **BestOfN** — Sample multiple outputs and score them
- **Program** — Compose modules into pipelines

### 🛠️ Tools & Function Calling
- Strongly typed parameters with automatic validation
- Rich type support (string, int, float, bool, json, array, enum)
- Native integration with OpenAI and OpenRouter function calling
- ReAct module for autonomous tool use

### 📊 Production Features
- **Multi-provider support** (OpenAI, OpenRouter) with auto-detection
- **Cost tracking** per request with token usage
- **LRU caching** with deterministic cache keys
- **History management** for multi-turn conversations
- **Few-shot learning** via examples
- **Observability hooks** for request/response logging
- **Streaming** with partial validation

### 🔄 Adapters
- **ChatAdapter** — Field marker-based parsing `[[ ## field ## ]]` with extensive failure-mode recovery
- **JSONAdapter** — JSON schema-based parsing with automatic repair
- **FallbackAdapter** — Chain of adapters (ChatAdapter → JSONAdapter by default)
- **TwoStepAdapter** — Reasoning-first, then structured extraction

## Installation

```bash
go get github.com/assagman/dsgo
```

### Minimal Installation (Core Only)

For minimal dependencies without auto-registered providers:

```bash
go get github.com/assagman/dsgo/core
```

## Quick Start

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
    // Configure provider (or use DSGO_PROVIDER, OPENAI_API_KEY env vars)
    dsgo.Configure(
        dsgo.WithProvider("openai"),
        dsgo.WithModel("gpt-4"),
        dsgo.WithAPIKey("sk-..."),
    )
    
    // Create LM instance
    lm, err := dsgo.NewLM(context.Background(), "gpt-4")
    if err != nil {
        log.Fatal(err)
    }
    
    // Define signature
    sig := dsgo.NewSignature("Classify sentiment").
        AddInput("text", dsgo.FieldTypeString, "Text to classify").
        AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, 
            "Sentiment classification")
    
    // Create module
    classifier := module.NewPredict(sig, lm)
    
    // Execute
    result, err := classifier.Forward(context.Background(), map[string]any{
        "text": "I love this product!",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Sentiment: %s\n", result.GetString("sentiment"))
    fmt.Printf("Cost: $%.6f, Tokens: %d\n", 
        result.Usage.Cost, result.Usage.TotalTokens)
}
```

See [QUICKSTART.md](QUICKSTART.md) for detailed tutorials and working examples.

## Architecture

DSGo follows a three-layer architecture:

```
┌─────────────────────────────────────────────────────┐
│                    Modules                          │
│  Predict │ ChainOfThought │ ReAct │ Program │ ...  │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│                  Core Layer                         │
│  Signature │ LM │ Adapter │ Tool │ History │ Cache │
└─────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────┐
│                  Providers                          │
│          OpenAI  │  OpenRouter  │  Custom           │
└─────────────────────────────────────────────────────┘
```

- **Core**: Primitives (signatures, LM interface, adapters, tools, settings)
- **Modules**: High-level behaviors (prediction, reasoning, tool use, composition)
- **Providers**: LM API implementations (OpenAI, OpenRouter, extensible)

## Core Concepts

### Signatures

Signatures define the structure of inputs and outputs:

```go
sig := dsgo.NewSignature("Answer questions with context").
    AddInput("question", dsgo.FieldTypeString, "Question to answer").
    AddInput("context", dsgo.FieldTypeString, "Background context").
    AddOutput("answer", dsgo.FieldTypeString, "Short answer").
    AddOptionalOutput("confidence", dsgo.FieldTypeFloat, "Confidence score 0-1")
```

**Field Types**: `FieldTypeString`, `FieldTypeInt`, `FieldTypeFloat`, `FieldTypeBool`, `FieldTypeJSON`, `FieldTypeClass`, `FieldTypeImage`, `FieldTypeDatetime`

### Modules

Modules implement the `Module` interface:

```go
type Module interface {
    Forward(ctx context.Context, inputs map[string]any) (*Prediction, error)
    GetSignature() *Signature
}
```

**Built-in Modules:**
- **Predict**: Basic structured prediction
- **ChainOfThought**: Adds reasoning step
- **ReAct**: Autonomous tool-using agent
- **Refine**: Iterative improvement
- **ProgramOfThought**: Code generation
- **BestOfN**: Sampling and scoring
- **Program**: Module composition

### Adapters

Adapters handle prompt formatting and output parsing:

- **ChatAdapter**: Uses field markers `[[ ## field_name ## ]]` with robust fallbacks
- **JSONAdapter**: Uses JSON schema with automatic repair
- **FallbackAdapter**: Chains adapters (default: ChatAdapter → JSONAdapter)
- **TwoStepAdapter**: Separate reasoning and extraction phases

Modules automatically select appropriate adapters, or you can customize:

```go
predictor := module.NewPredict(sig, lm).
    WithAdapter(dsgo.NewJSONAdapter())
```

### Tools

Define tools for agent modules:

```go
searchTool := dsgo.NewTool(
    "search",
    "Search the web for information",
    func(ctx context.Context, args map[string]interface{}) (string, error) {
        query := args["query"].(string)
        return performSearch(query)
    },
).AddParameter("query", "string", "Search query", true)

agent := module.NewReAct(sig, lm, []dsgo.Tool{*searchTool})
```

## Configuration

### Environment Variables

```bash
# Provider and model
export DSGO_PROVIDER=openai
export DSGO_MODEL=gpt-4

# API keys
export OPENAI_API_KEY=sk-...
export OPENROUTER_API_KEY=sk-or-v1-...

# Options
export DSGO_TIMEOUT=30s
export DSGO_MAX_RETRIES=3
export DSGO_TRACING=true

# Debugging
export DSGO_DEBUG_PARSE=1
export DSGO_SAVE_RAW_RESPONSES=1
```

### Programmatic Configuration

```go
dsgo.Configure(
    dsgo.WithProvider("openai"),
    dsgo.WithModel("gpt-4"),
    dsgo.WithAPIKey("sk-..."),
    dsgo.WithTimeout(30 * time.Second),
    dsgo.WithMaxRetries(3),
    dsgo.WithTracing(true),
)
```

### Model Auto-Detection

```go
// Auto-detects provider from model name
lm, _ := dsgo.NewLM(ctx, "gpt-4")              // → openai
lm, _ := dsgo.NewLM(ctx, "google/gemini-2.0-flash")  // → openrouter
lm, _ := dsgo.NewLM(ctx, "anthropic/claude-3.5-sonnet") // → openrouter
```

## Observability

### Cost and Usage Tracking

Every prediction includes usage statistics:

```go
result, _ := predictor.Forward(ctx, inputs)
fmt.Printf("Cost: $%.6f\n", result.Usage.Cost)
fmt.Printf("Tokens: %d (prompt: %d, completion: %d)\n",
    result.Usage.TotalTokens,
    result.Usage.PromptTokens,
    result.Usage.CompletionTokens)
fmt.Printf("Latency: %dms\n", result.Usage.Latency)
```

### Request Logging

Implement a custom collector:

```go
type MyCollector struct{}

func (c *MyCollector) Collect(entry core.HistoryEntry) {
    // Log or store entry: request/response, usage, metadata, errors
    log.Printf("Request to %s: %d tokens, $%.6f, %dms",
        entry.Provider, entry.Usage.TotalTokens, entry.Usage.Cost, entry.Usage.Latency)
}

dsgo.Configure(dsgo.WithCollector(&MyCollector{}))
```

### Streaming

```go
predictor := module.NewPredict(sig, lm)

chunks, finalPred, errCh := predictor.Stream(ctx, inputs)
for chunk := range chunks {
    fmt.Print(chunk.Content) // Clean content (no internal markers)
}

result := <-finalPred
if err := <-errCh; err != nil {
    log.Fatal(err)
}

fmt.Printf("\nFinal output: %s\n", result.GetString("answer"))
```

## Examples

See [examples/](examples/) directory:

- **[01-hello-chat](examples/01-hello-chat/)** — Basic chat interaction
- **[02-agent-tools-react](examples/02-agent-tools-react/)** — ReAct agent with tools
- **[03-quality-refine-bestof](examples/03-quality-refine-bestof/)** — Refine and BestOfN
- **[04-structured-programs](examples/04-structured-programs/)** — Module composition
- **[05-resilience-observability](examples/05-resilience-observability/)** — Production patterns

Run examples:

```bash
cd examples/01-hello-chat
go run main.go
```

## Testing

```bash
# Run all tests with race detector and coverage
make test

# Quick example validation (1 model)
make test-matrix-quick

# Sample N random models
make test-matrix-sample N=3

# Full test suite
make all
```

## Documentation

- **[README.md](README.md)** — This file (overview and reference)
- **[QUICKSTART.md](QUICKSTART.md)** — Step-by-step tutorials and patterns
- **[AGENTS.md](AGENTS.md)** — Development guide for AI agents and contributors
- **[ROADMAP.md](ROADMAP.md)** — Implementation status and future plans
- **[llms.txt](llms.txt)** — LLM-friendly documentation index

## Contributing

Contributions are welcome! Please see [AGENTS.md](AGENTS.md) for development guidelines.

Key points:
- Run `make all` before committing (format, lint, test)
- Add tests for all new features (target: >90% coverage)
- Follow Go conventions and existing code style
- Update documentation when adding features

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

**Questions?** Open an issue or check [llms.txt](llms.txt) for detailed technical documentation.
