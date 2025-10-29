# Customer Support Response Generator

Demonstrates how to use **Refine** and **BestOfN** modules to generate high-quality customer support responses.

## Features Demonstrated

- **Refine Module**: Iteratively improve responses based on feedback
- **BestOfN Module**: Generate multiple responses and select the best one
- **Custom Scoring**: Balance empathy and professionalism
- **Program Pipeline**: Multi-stage workflow (classify → generate → select)

## Use Cases

1. **Refine Response**: Start with a basic response and improve it based on specific feedback
2. **Best Response Selection**: Generate multiple responses and automatically choose the best one
3. **Combined Workflow**: Full pipeline from issue classification to response generation

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/customer_support
go run main.go
```

## What You'll Learn

- How to use the Refine module for iterative improvement
- How to create custom scoring functions for BestOfN
- How to balance multiple quality metrics (empathy, professionalism)
- How to build multi-stage workflows with Program

## Example Output

The example generates professional customer support responses that balance:
- Empathy (60% weight)
- Professionalism (40% weight)
- Specific tone requirements
- Appropriate compensation offers

## Key Code Patterns

```go
// Refine with feedback
refine := dsgo.NewRefine(sig, lm).
    WithMaxIterations(2).
    WithRefinementField("feedback")

// BestOfN with custom scorer
bestOf := dsgo.NewBestOfN(predict, 3).
    WithScorer(customScorer).
    WithReturnAll(true)

// Multi-stage pipeline
pipeline := dsgo.NewProgram("Support").
    AddModule(classify).
    AddModule(bestResponse)
```
