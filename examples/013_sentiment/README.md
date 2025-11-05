# 013_sentiment - Chain of Thought Reasoning

## Overview

Math word problem solving with Chain of Thought reasoning. Demonstrates step-by-step reasoning for complex tasks.

## What it demonstrates

- ChainOfThought module for reasoning
- Accessing rationale/reasoning trace
- Multi-output signatures
- Numerical output extraction

## Usage

```bash
cd examples/013_sentiment
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
```

## Expected Output

```
Problem: If John has 5 apples and gives 2 to Mary, then buys 3 more apples, how many apples does John have?
Reasoning: Let me solve this step by step...
Answer: 6
Explanation: John starts with 5, gives away 2 (5-2=3), then buys 3 more (3+3=6)
```

## Key Concepts

- **ChainOfThought**: Generates reasoning before final answer
- **Rationale**: Captured thinking process
- **Multi-output**: Both numerical answer and text explanation

## See Also

- [001_predict](../001_predict/) - Basic prediction without reasoning
- [QUICKSTART.md](../../QUICKSTART.md) - Module comparison
