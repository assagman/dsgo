# 012_math_solver - Math Solver with Program of Thought

## Overview

Demonstrates using the **ProgramOfThought** module to solve mathematical problems by generating Python code. This example shows how code generation can be more reliable than direct calculation for complex math problems, statistical analysis, and word problems.

## What it demonstrates

- ProgramOfThought module for code-based reasoning
- Python code generation for mathematical calculations
- Multiple problem types (finance, physics, statistics)
- Safety controls (execution disabled by default)
- Step-by-step explanations of generated code
- Structured output with code, explanation, and answer

## Usage

```bash
cd examples/012_math_solver
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=1 -timeout=120
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
=== Math Solver with Program of Thought ===

--- Example 1: Compound Interest Calculation ---
Problem: Calculate the compound interest on $1000 invested at 5% annually for 3 years

Generated Code:
principal = 1000
rate = 0.05
time = 3
amount = principal * (1 + rate) ** time
compound_interest = amount - principal
print(f"Compound Interest: ${compound_interest:.2f}")
print(f"Total Amount: ${amount:.2f}")

Explanation: This code calculates compound interest using the formula A = P(1 + r)^t...
Answer: The compound interest is $157.63, and the total amount is $1157.63

--- Example 2: Average Speed Word Problem ---
Problem: A train travels 120 km in 2 hours, then 180 km in 3 hours. What is the average speed for the entire journey?

Python Code:
distance1 = 120
time1 = 2
distance2 = 180
time2 = 3
total_distance = distance1 + distance2
total_time = time1 + time2
average_speed = total_distance / total_time
print(f"Average Speed: {average_speed} km/h")

Explanation:
First, we calculate the total distance (120 + 180 = 300 km)
Then the total time (2 + 3 = 5 hours)
Average speed = total distance / total time = 300 / 5 = 60 km/h

Answer: 60 km/h

--- Example 3: Statistical Analysis ---
Data: Dataset of exam scores: [75, 82, 90, 68, 85, 92, 78, 88, 95, 72]
Analysis: mean, median, standard deviation, and identify outliers

Generated Code:
import statistics
scores = [75, 82, 90, 68, 85, 92, 78, 88, 95, 72]
mean = statistics.mean(scores)
median = statistics.median(scores)
stdev = statistics.stdev(scores)
# Outliers: values > mean + 2*stdev or < mean - 2*stdev
outlier_threshold = 2 * stdev
outliers = [x for x in scores if abs(x - mean) > outlier_threshold]
print(f"Mean: {mean:.2f}")
print(f"Median: {median}")
print(f"Standard Deviation: {stdev:.2f}")
print(f"Outliers: {outliers}")

Explanation: Uses Python's statistics module to calculate mean, median, and standard deviation...
Interpretation: The mean score is 82.5, median is 83.5, with a standard deviation of 9.1. No significant outliers detected.

📊 Summary:
  Problems solved: 3
  Total tokens used: 3200
  ✅ All mathematical problems solved successfully!
```

## Key Concepts

### 1. ProgramOfThought Module

ProgramOfThought generates executable code to solve problems:

```go
sig := dsgo.NewSignature("Solve the mathematical problem using Python code").
    AddInput("problem", dsgo.FieldTypeString, "The problem to solve").
    AddOutput("code", dsgo.FieldTypeString, "Python code solution").
    AddOutput("explanation", dsgo.FieldTypeString, "Explanation").
    AddOutput("answer", dsgo.FieldTypeString, "Final answer")

pot := module.NewProgramOfThought(sig, lm, "python").
    WithAllowExecution(false) // Don't execute for safety
```

**Why use code generation?**
- More reliable for complex calculations
- Shows work (the code is the explanation)
- Handles edge cases better
- Can use libraries (statistics, numpy, etc.)

### 2. Safety Controls

**Execution is disabled by default** for security:

```go
pot := module.NewProgramOfThought(sig, lm, "python").
    WithAllowExecution(false) // Safety first!
```

**When to enable execution:**
- Controlled environments (sandboxes)
- Trusted LM outputs
- With proper validation

### 3. Problem Types

The example solves three types of problems:

**Financial Calculations:**
- Compound interest
- ROI calculations
- Loan amortization

**Physics/Word Problems:**
- Speed, distance, time
- Multi-stage calculations
- Unit conversions

**Statistical Analysis:**
- Mean, median, mode
- Standard deviation
- Outlier detection

### 4. Structured Outputs

ProgramOfThought provides three outputs:

```go
outputs.Outputs["code"]         // Generated Python code
outputs.Outputs["explanation"]  // What the code does
outputs.Outputs["answer"]       // Final numerical result
```

## Use Cases

### Educational Applications
- Math tutoring systems
- Problem-solving assistants
- Step-by-step explanations
- Homework helpers

### Data Analysis
- Quick statistical calculations
- Dataset exploration
- Hypothesis testing
- Data validation

### Business Calculations
- Financial modeling
- ROI analysis
- Budget planning
- Forecasting

### Scientific Computing
- Physics problems
- Chemistry calculations
- Engineering formulas
- Research computations

## Advanced Features

### Custom Code Languages

```go
// Python (default)
pot := module.NewProgramOfThought(sig, lm, "python")

// JavaScript
pot := module.NewProgramOfThought(sig, lm, "javascript")

// R for statistics
pot := module.NewProgramOfThought(sig, lm, "r")
```

### Execution Control

```go
pot := module.NewProgramOfThought(sig, lm, "python").
    WithAllowExecution(true).        // Enable execution
    WithExecutionTimeout(30).         // 30 second timeout
    WithSandbox(customSandbox)        // Custom sandbox environment
```

### Complex Signatures

```go
sig := dsgo.NewSignature("Advanced analysis").
    AddInput("data", dsgo.FieldTypeString, "Dataset").
    AddInput("analysis_type", dsgo.FieldTypeString, "Analysis to perform").
    AddInput("constraints", dsgo.FieldTypeString, "Constraints or requirements").
    AddOutput("code", dsgo.FieldTypeString, "Implementation").
    AddOutput("explanation", dsgo.FieldTypeString, "Code explanation").
    AddOutput("visualization_code", dsgo.FieldTypeString, "Optional plot code").
    AddOutput("results", dsgo.FieldTypeString, "Analysis results")
```

## Comparison with Other Modules

**vs. Predict:**
- Predict: Direct text generation
- ProgramOfThought: Code generation → more reliable for math

**vs. ChainOfThought:**
- ChainOfThought: Natural language reasoning
- ProgramOfThought: Code-based reasoning → better for calculations

**vs. ReAct:**
- ReAct: Tool-based actions
- ProgramOfThought: Code generation → more flexible for custom logic

## See Also

- [006_program_of_thought](../006_program_of_thought/) - Core ProgramOfThought module introduction
- [002_chain_of_thought](../002_chain_of_thought/) - Natural language reasoning
- [003_react](../003_react/) - Tool-based problem solving
- [007_program_composition](../007_program_composition/) - Combining PoT with other modules
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide

## Production Tips

1. **Safety First**: Always disable execution unless in a controlled environment
2. **Validation**: Validate generated code before execution
3. **Timeouts**: Set appropriate execution timeouts to prevent hangs
4. **Sandboxing**: Use proper sandboxing when enabling execution
5. **Error Handling**: Handle code generation failures gracefully
6. **Code Review**: Log generated code for quality monitoring
7. **Testing**: Test with diverse problem types to ensure robustness

## When to Use ProgramOfThought

**Use when:**
- Complex mathematical calculations
- Multi-step numerical reasoning
- Statistical analysis
- Need to show work/explanation
- Precision is critical

**Don't use when:**
- Simple text generation tasks
- Creative or subjective outputs
- Real-time conversational responses
- No need for code execution
