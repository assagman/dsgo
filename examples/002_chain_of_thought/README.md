# 002_chain_of_thought - Chain of Thought Reasoning

## Overview

Demonstrates Chain of Thought (CoT) reasoning for solving math word problems. The model explicitly shows its reasoning process before arriving at an answer.

## What it demonstrates

- Creating a Signature for complex reasoning tasks
- Using ChainOfThought module for step-by-step reasoning
- Accessing the reasoning trace via Rationale field
- Multi-output structured results (answer + explanation)

## Usage

```bash
cd examples/002_chain_of_thought
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=10 -timeout=60
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
Problem: If John has 5 apples and gives 2 to Mary, then buys 3 more apples, how many apples does John have?
Reasoning: Let me work through this step by step...
Answer: 6
Explanation: John starts with 5 apples, gives away 2 (leaving 3), then buys 3 more (3+3=6)
```

## Key Concepts

- **Chain of Thought**: Forces the model to reason explicitly before answering
- **Rationale**: Internal reasoning is captured in result.Rationale
- **Multi-Output**: Returns both numerical answer and text explanation
- **Complex Reasoning**: Better accuracy on tasks requiring multiple steps

## See Also

- [001_predict](../001_predict/) - Basic prediction without reasoning
- [003_react](../003_react/) - ReAct agent with tool use
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
