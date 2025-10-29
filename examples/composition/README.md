# Module Composition Examples

This example demonstrates how to compose and combine DSGo modules to create sophisticated LM programs.

## What's Demonstrated

### 1. Program Pipeline
- Chain multiple modules together
- Outputs from one module become inputs to the next
- Build multi-step reasoning workflows

### 2. Refine Module
- Iterative improvement of outputs
- Feedback-based refinement
- Multiple refinement iterations

### 3. BestOfN Module
- Generate multiple candidates
- Score and select the best result
- Parallel or sequential execution
- Custom scoring functions

### 4. Combined Patterns
- Programs composed of multiple module types
- BestOfN within a pipeline
- ChainOfThought → BestOfN → Refine workflows

## Running the Example

```bash
# Set your API key
export OPENAI_API_KEY=your_key_here

# Run the example
cd examples/composition
go run main.go
```

## Module Composition Patterns

### Pattern 1: Sequential Pipeline
```go
program := dsgo.NewProgram("name").
    AddModule(module1).
    AddModule(module2).
    AddModule(module3)
```

### Pattern 2: Best-of-N Selection
```go
bestOf := dsgo.NewBestOfN(module, 5).
    WithScorer(customScorer).
    WithParallel(true)
```

### Pattern 3: Iterative Refinement
```go
refine := dsgo.NewRefine(signature, lm).
    WithMaxIterations(3).
    WithRefinementField("feedback")
```

### Pattern 4: Hybrid Composition
```go
program := dsgo.NewProgram("hybrid").
    AddModule(dsgo.NewChainOfThought(sig1, lm)).
    AddModule(bestOf).
    AddModule(dsgo.NewRefine(sig2, lm))
```

## Custom Scoring Functions

You can create custom scorers for BestOfN:

```go
func myScorer(inputs, outputs map[string]interface{}) (float64, error) {
    // Your scoring logic
    score := calculateScore(outputs)
    return score, nil
}

bestOf := dsgo.NewBestOfN(module, n).WithScorer(myScorer)
```

Built-in scorers:
- `dsgo.DefaultScorer()` - Length-based
- `dsgo.ConfidenceScorer(field)` - Confidence field-based

## Use Cases

- **Content Generation**: Generate → Refine → Select Best
- **Analysis Pipelines**: Extract → Analyze → Summarize
- **Problem Solving**: Understand → Generate Solutions → Pick Best
- **Quality Improvement**: Generate N → Score → Refine Best
