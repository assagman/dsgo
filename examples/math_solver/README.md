# Math Solver with Program of Thought

Demonstrates using the **ProgramOfThought** module to solve mathematical problems by generating and analyzing code.

## Features Demonstrated

- **ProgramOfThought Module**: Generate Python code to solve math problems
- **Code Generation**: Automatic code generation for calculations
- **Multiple Problem Types**: Simple calculations, word problems, statistical analysis
- **Safety Controls**: Code execution can be disabled for security

## Use Cases

1. **Simple Calculations**: Compound interest, percentages, conversions
2. **Word Problems**: Complex multi-step mathematical reasoning
3. **Statistical Analysis**: Mean, median, standard deviation, outlier detection

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/math_solver
go run main.go
```

## What You'll Learn

- How to use ProgramOfThought for mathematical reasoning
- When code generation is more reliable than direct calculation
- How to structure prompts for code-based problem solving
- How to handle code execution safely

## Example Problems Solved

1. **Compound Interest**: Calculate returns on investments
2. **Average Speed**: Multi-stage journey calculations
3. **Statistical Analysis**: Analyze exam scores dataset

## Key Code Patterns

```go
// Create ProgramOfThought module
pot := dsgo.NewProgramOfThought(sig, lm, "python").
    WithAllowExecution(false). // Disabled for safety
    WithExecutionTimeout(30)

// The module will:
// 1. Generate Python code
// 2. Explain the code
// 3. Provide the answer
```

## Safety Note

Code execution is disabled by default (`WithAllowExecution(false)`). Only enable it in controlled environments where you trust the LM output.
