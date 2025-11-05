# 006_program_of_thought - Code Generation for Problem Solving

## Overview

Demonstrates ProgramOfThought module that solves problems by generating Python code. Ideal for mathematical problems, data analysis, and tasks requiring precise calculations.

## What it demonstrates

- Creating a ProgramOfThought module
- Generating Python code to solve problems
- Code generation without execution (safe mode)
- Optional code execution for automated solving
- Explaining the approach via comments

## Usage

```bash
cd examples/006_program_of_thought
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
Problem: [3,4,5,6,1,2] -> write a program to find a target value: 5, with optimum time complexity

Generated Code:
```python
def find_target(arr, target):
    return arr.index(target) if target in arr else -1

result = find_target([3,4,5,6,1,2], 5)
print(result)  # Output: 2
```

Explanation: Uses list.index() with O(n) time complexity, optimal for unsorted arrays
```

## Key Concepts

- **Program of Thought**: Generate code instead of text reasoning
- **Language Support**: Python (extendable to other languages)
- **Execution Control**: Can enable/disable code execution
- **Timeout Protection**: Prevents infinite loops when executing
- **Precise Calculations**: Better than text-based reasoning for math

## See Also

- [002_chain_of_thought](../002_chain_of_thought/) - Text-based reasoning
- [003_react](../003_react/) - Agent with tools
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
