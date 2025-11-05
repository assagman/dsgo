# 003_react - ReAct Agent with Tools

## Overview

Demonstrates the ReAct (Reasoning + Acting) module that combines reasoning with tool use. The agent can decide which tools to use and when to use them to answer questions.

## What it demonstrates

- Creating custom tools (search, calculator)
- Using ReAct module for agent-like behavior
- Tool selection and execution by the LM
- Multi-step reasoning with external actions
- Verbose mode to see agent's thought process

## Usage

```bash
cd examples/003_react
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
=== Final Result ===
Question: What is DSPy and how many years has it been since 2020?
Answer: DSPy is a framework for programming language models developed at Stanford. It has been 5 years since 2020.
Sources: search, calculator
```

## Key Concepts

- **ReAct**: Combines reasoning (thinking) with acting (tool use)
- **Tools**: Functions that the agent can call (search, calculator, etc.)
- **Iterations**: Agent can use multiple tools in sequence
- **Verbose Mode**: Shows the agent's reasoning at each step
- **Autonomous**: Agent decides which tools to use and when

## See Also

- [002_chain_of_thought](../002_chain_of_thought/) - Reasoning without tools
- [016_tools](../016_tools/) - Advanced tool usage
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
