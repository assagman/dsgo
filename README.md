```
╔══════════════════════════════════════════════════════════════════════╗
║                                                                      ║
║   ██████╗ ███████╗ ██████╗  ██████╗                                  ║
║   ██╔══██╗██╔════╝██╔════╝ ██╔═══██╗                                 ║
║   ██║  ██║███████╗██║  ███╗██║   ██║                                 ║
║   ██║  ██║╚════██║██║   ██║██║   ██║                                 ║
║   ██████╔╝███████║╚██████╔╝╚██████╔╝                                 ║
║   ╚═════╝ ╚══════╝ ╚═════╝  ╚═════╝                                  ║
║                                                                      ║
║   DSPy Framework for Go                                              ║
║   Programming Language Models with Signatures, Modules & Tools       ║
║                                                                      ║
╚══════════════════════════════════════════════════════════════════════╝
```

✨ Features:
  • Type-Safe Signatures (8 field types)
  • Composable Modules (Predict, ChainOfThought, ReAct)
  • Tool/Function Calling
  • OpenAI and Openrouter support
  • Experiment-Ready

📚 Documentation:
  • QUICKSTART.md  - Get started in 30 seconds
  • AGENTS.md      - Development guide
  • README.md      - Complete overview

# DSGo - DSPy for Go

DSGo is a Go implementation of the [DSPy framework](https://github.com/stanfordnlp/dspy) for programming language models. It provides a structured approach to building LM-based applications through signatures, modules, and composable patterns.

## Features

- ✅ **Signatures**: Define structured inputs and outputs for LM calls
- ✅ **Type Safety**: Strong typing with validation for inputs and outputs
- ✅ **Modules**: Composable building blocks for LM programs
  - `Predict`: Basic prediction module
  - `ChainOfThought`: Step-by-step reasoning
  - `ReAct`: Reasoning and Acting with tool support
  - `Refine`: Iterative refinement of predictions
  - `BestOfN`: Generate N solutions and select the best
  - `ProgramOfThought`: Code generation and execution for reasoning
  - `Program`: Compose modules into pipelines
- ✅ **LM Abstraction**: Easy integration with different language models
- ✅ **Tool Support**: Define and use tools in ReAct agents
- ✅ **Structured Outputs**: JSON-based structured responses with validation

## Quick Start

### Installation

```bash
go get github.com/assagman/dsgo
```

### Three Progressive Examples

1. **Sentiment Analysis** (Beginner) - Basic prediction and chain-of-thought
2. **ReAct Agent** (Intermediate) - Tools and iterative reasoning
3. **Research Assistant** (Advanced) - All features: complex signatures, multiple types, tools, reasoning

### Basic Example: Sentiment Analysis

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
    // Create signature
    sig := dsgo.NewSignature("Analyze the sentiment of the given text").
        AddInput("text", dsgo.FieldTypeString, "The text to analyze").
        AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "The sentiment").
        AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score")

    // Create language model
    lm := openai.NewOpenAI("gpt-4")

    // Create Predict module
    predict := module.NewPredict(sig, lm)

    // Execute
    ctx := context.Background()
    inputs := map[string]interface{}{
        "text": "I love this product!",
    }

    outputs, err := predict.Forward(ctx, inputs)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Sentiment: %v (Confidence: %v)\n",
        outputs["sentiment"], outputs["confidence"])
}
```

### Chain of Thought Example

```go
sig := dsgo.NewSignature("Solve the math word problem").
    AddInput("problem", dsgo.FieldTypeString, "The problem").
    AddOutput("answer", dsgo.FieldTypeFloat, "The answer").
    AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step solution")

lm := openai.NewOpenAI("gpt-4")
cot := module.NewChainOfThought(sig, lm)

outputs, err := cot.Forward(ctx, map[string]interface{}{
    "problem": "If John has 5 apples and gives 2 away, how many does he have?",
})
```

### ReAct Agent with Tools

```go
// Define a search tool
searchTool := dsgo.NewTool(
    "search",
    "Search for information",
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        query := args["query"].(string)
        return performSearch(query), nil
    },
).AddParameter("query", "string", "Search query", true)

// Create ReAct module
sig := dsgo.NewSignature("Answer questions using available tools").
    AddInput("question", dsgo.FieldTypeString, "The question").
    AddOutput("answer", dsgo.FieldTypeString, "The answer")

lm := openai.NewOpenAI("gpt-4")
react := module.NewReAct(sig, lm, []dsgo.Tool{*searchTool}).
    WithMaxIterations(5).
    WithVerbose(true)

outputs, err := react.Forward(ctx, map[string]interface{}{
    "question": "What is DSPy?",
})
```

### Advanced: Custom Signatures with Multiple Types

```go
// Complex signature with diverse input/output types
sig := dsgo.NewSignature("Research and analyze a topic").
    // Multiple input types
    AddInput("topic", dsgo.FieldTypeString, "Research topic").
    AddInput("depth_level", dsgo.FieldTypeInt, "Depth: 1-3").
    AddInput("include_stats", dsgo.FieldTypeBool, "Include statistics").
    // Multiple output types with constraints
    AddOutput("summary", dsgo.FieldTypeString, "Executive summary").
    AddOutput("key_findings", dsgo.FieldTypeString, "Main discoveries").
    AddClassOutput("confidence", []string{"high", "medium", "low"}, "Confidence").
    AddOutput("sources_count", dsgo.FieldTypeInt, "Number of sources").
    AddOptionalOutput("statistics", dsgo.FieldTypeString, "Stats if requested")

// Use with ReAct and multiple tools
tools := []dsgo.Tool{searchTool, statsTool, factCheckTool}
react := module.NewReAct(sig, lm, tools).WithMaxIterations(7)

outputs, err := react.Forward(ctx, map[string]interface{}{
    "topic":         "AI in software development",
    "depth_level":   2,
    "include_stats": true,
})
```

## Core Concepts

### Signatures

Signatures define the structure of your LM program's inputs and outputs:

```go
sig := dsgo.NewSignature("Description of the task").
    AddInput("field_name", dsgo.FieldTypeString, "Field description").
    AddOutput("result", dsgo.FieldTypeString, "Result description").
    AddClassOutput("category", []string{"A", "B", "C"}, "Classification")
```

Supported field types:
- `FieldTypeString`
- `FieldTypeInt`
- `FieldTypeFloat`
- `FieldTypeBool`
- `FieldTypeJSON`
- `FieldTypeClass` (enum/classification)
- `FieldTypeImage`
- `FieldTypeDatetime`

### Modules

Modules are composable building blocks:

- **Predict**: Direct prediction based on signature
- **ChainOfThought**: Encourages step-by-step reasoning
- **ReAct**: Combines reasoning with tool usage
- **Refine**: Iteratively improve predictions with feedback
- **BestOfN**: Generate multiple candidates and select the best
- **ProgramOfThought**: Generate and execute code for reasoning
- **Program**: Chain modules into pipelines

### Language Models

Implement the `LM` interface to add support for different providers:

```go
type LM interface {
    Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)
    Name() string
    SupportsJSON() bool
    SupportsTools() bool
}
```

Currently included:
- OpenAI (GPT-3.5, GPT-4)

### Tools

Define tools for ReAct agents:

```go
tool := dsgo.NewTool(
    "tool_name",
    "Description of what the tool does",
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        // Tool implementation
        return result, nil
    },
).AddParameter("param", "string", "Parameter description", true)
```

## Architecture

DSGo follows the DSPy philosophy:

1. **Declarative**: Define what you want, not how to prompt
2. **Modular**: Compose complex behaviors from simple modules
3. **Type-Safe**: Strong typing with validation
4. **Tool-Enabled**: Easy integration with external tools

## Project Structure

```
dsgo/
├── signature.go             # Signature system (InputField, OutputField)
├── lm.go                    # Language Model interface
├── module.go                # Module interface
├── prediction.go            # Prediction wrapper with metadata
├── history.go               # Conversation history management
├── example.go               # Few-shot learning support
├── tool.go                  # Tool/function definitions
├── *_test.go                # Unit tests
├── module/
│   ├── predict.go           # Basic Predict module
│   ├── chain_of_thought.go  # ChainOfThought module
│   ├── react.go             # ReAct module with tool support
│   ├── refine.go            # Refine module for iterative improvement
│   ├── best_of_n.go         # BestOfN module for multiple sampling
│   ├── program_of_thought.go # ProgramOfThought module for code generation
│   └── program.go           # Program structure for module composition
├── providers/
│   ├── openai/              # OpenAI LM provider
│   └── openrouter/          # OpenRouter LM provider
├── examples/
│   ├── shared/              # Shared provider utilities
│   ├── sentiment/           # Basic prediction & chain-of-thought
│   ├── react_agent/         # ReAct agent with tools
│   ├── research_assistant/  # Advanced: complex signatures + tools + reasoning
│   └── composition/         # Module composition and pipelines
├── AGENTS.md                # Development guide
└── README.md                # This file
```

## Roadmap

### Core Modules (Complete ✅)
- [x] Predict, ChainOfThought, ReAct
- [x] Refine, BestOfN, ProgramOfThought
- [x] Program composition

### Future Enhancements
- [ ] Additional LM providers (Anthropic, Google, Ollama)
- [ ] Optimizers (MIPROv2, COPRO, BootstrapFewShot)
- [ ] Evaluation framework
- [ ] Caching layer
- [ ] Observability/tracing
- [ ] Multi-modal support

## Documentation Index

- **[QUICKSTART.md](QUICKSTART.md)** - Get started in 30 seconds
- **[AGENTS.md](AGENTS.md)** - Development and testing guide

## Contributing

Contributions are welcome! This is an early-stage implementation.

## License

MIT License

## Inspiration

- [DSPy](https://github.com/stanfordnlp/dspy) - Original Python implementation
- [ax](https://github.com/ax-llm/ax) - TypeScript variant
- [dspy.rb](https://github.com/vicentereig/dspy.rb) - Ruby variant

## References

- [DSPy Documentation](https://dspy.ai/)
- [DSPy API Reference](https://dspy.ai/api/)
