# 007_program_composition - Module Composition & Pipelines

## Overview

Demonstrates how to compose and chain multiple DSGo modules together into sophisticated pipelines. Shows how to build complex LM workflows by combining ChainOfThought, BestOfN, and other modules using the Program composition pattern.

## What it demonstrates

- Creating multi-step pipelines with Program
- Chaining modules: outputs from one become inputs to the next
- Combining ChainOfThought with BestOfN for quality
- Using BestOfN with custom scoring (confidence-based)
- Tracking scores and metadata across pipeline stages
- Building sophisticated reasoning workflows

## Usage

```bash
cd examples/007_program_composition
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=5 -timeout=120
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
Problem: How can I reduce the latency of my web application's database queries?

Analysis: [Detailed problem analysis from ChainOfThought]

Approach: [Recommended solution approach]

Best Solution:
[Best of 3 generated solutions based on confidence scoring]

Confidence: 0.85
Best Score: 0.850
All Scores: [0.75, 0.82, 0.85]
```

## Key Concepts

- **Program Composition**: Chain multiple modules into pipelines
- **Data Flow**: Outputs from one module automatically feed into the next
- **Quality Enhancement**: Use BestOfN to generate multiple solutions and pick the best
- **Hybrid Workflows**: Combine reasoning (ChainOfThought) with selection (BestOfN)
- **Score Tracking**: Monitor scores across all candidates for observability

## Module Pipeline

```
Input (problem)
    ↓
ChainOfThought (analyze)
    ↓ (analysis, approach)
BestOfN(Predict) [N=3]
    ↓ (best solution, confidence, scores)
Output
```

## Composition Patterns

### Sequential Pipeline
```go
program := module.NewProgram("name").
    AddModule(module1).
    AddModule(module2)
```

### With BestOfN Selection
```go
bestOf := module.NewBestOfN(module, N).
    WithScorer(module.ConfidenceScorer("field")).
    WithReturnAll(true)
```

### Hybrid Composition
```go
program := module.NewProgram("hybrid").
    AddModule(module.NewChainOfThought(sig1, lm)).
    AddModule(bestOfN)
```

## See Also

- [002_chain_of_thought](../002_chain_of_thought/) - Reasoning module
- [005_best_of_n](../005_best_of_n/) - Multiple sampling and selection
- [004_refine](../004_refine/) - Another composition pattern
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
