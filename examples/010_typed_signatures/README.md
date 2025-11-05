# 010_typed_signatures - Type-Safe API with Generics

## Overview

Demonstrates the type-safe, generic API for building LM programs with compile-time type checking. Shows how to use Go generics to define strongly-typed input/output structures, eliminating runtime type assertions and providing better IDE support and refactoring safety.

## What it demonstrates

- Defining typed input/output structures with struct tags
- Creating type-safe predictor functions with generics
- Using typed few-shot examples (demos)
- Accessing prediction metadata with typed outputs
- Chain of Thought with typed signatures
- ReAct agents with typed signatures
- Compile-time type safety for LM programs
- Enum validation with struct tags

## Usage

```bash
cd examples/010_typed_signatures
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
1. Basic Sentiment Analysis (Type-Safe)
-----------------------------------
Sentiment: positive (Score: 95/100)

2. Translation with Typed Few-Shot Examples
-----------------------------------
Translation: Guten Morgen

3. Question Answering with Metadata
-----------------------------------
Answer: Gustave Eiffel
Confidence: high
Tokens used: 234

4. Chain of Thought with Typed Signatures
-----------------------------------
Question: How many people work in operations?
Answer: 18 people
Confidence: high
Reasoning: 60% work in engineering (72 people), 25% in sales (30 people)...

5. ReAct Agent with Typed Signatures
-----------------------------------
Question: What is the capital of France?
Answer: Paris
Confidence: high

📊 Type-Safe API Examples:
  Total examples executed: 5
  Total tokens used: 1850
  ✅ All typed signature examples completed successfully!
```

## Key Concepts

- **Type Safety**: Compile-time checking of inputs/outputs
- **Struct Tags**: Define field metadata with `dsgo:` tags
- **Generics**: Use `NewPredict[Input, Output](lm)` for type-safe functions
- **RunWithPrediction**: Get both typed output and prediction metadata
- **WithDemosTyped**: Add few-shot examples with type safety
- **Enum Validation**: Enforce valid values with `enum=` tag

## Defining Typed Structures

### Input Structure
```go
type SentimentInput struct {
    Text string `dsgo:"input,desc=Text to analyze for sentiment"`
}
```

### Output Structure with Enums
```go
type SentimentOutput struct {
    Sentiment string `dsgo:"output,enum=positive|negative|neutral,desc=The detected sentiment"`
    Score     int    `dsgo:"output,desc=Confidence score from 0 to 100"`
}
```

## Creating Type-Safe Functions

### Basic Predict
```go
sentimentFunc, err := typed.NewPredict[SentimentInput, SentimentOutput](lm)
if err != nil {
    log.Fatal(err)
}

// Type-safe execution
result, pred, err := sentimentFunc.RunWithPrediction(ctx, SentimentInput{
    Text: "I love this!",
})

// No type assertions needed
fmt.Printf("Sentiment: %s, Score: %d\n", result.Sentiment, result.Score)
```

### With Few-Shot Examples
```go
translateFunc, err := typed.NewPredict[TranslateInput, TranslateOutput](lm)

inputs := []TranslateInput{
    {Text: "Hello", Target: "es"},
    {Text: "Goodbye", Target: "fr"},
}
outputs := []TranslateOutput{
    {Translation: "Hola"},
    {Translation: "Au revoir"},
}

translateFunc, err = translateFunc.WithDemosTyped(inputs, outputs)
```

### Chain of Thought
```go
cotFunc, err := typed.NewCoT[QAInput, QAOutput](lm)

answer, pred, err := cotFunc.RunWithPrediction(ctx, QAInput{
    Context:  "Company has 120 employees...",
    Question: "How many work in operations?",
})

fmt.Printf("Answer: %s\nReasoning: %s\n", answer.Answer, pred.Rationale)
```

### ReAct Agent
```go
tools := []dsgo.Tool{*searchTool}
reactFunc, err := typed.NewReAct[QAInput, QAOutput](lm, tools)

reactFunc.WithMaxIterations(5).WithVerbose(false)

answer, pred, err := reactFunc.RunWithPrediction(ctx, QAInput{
    Context:  "Use tools to answer",
    Question: "What is the capital of France?",
})
```

## Struct Tag Format

```go
`dsgo:"input|output,desc=description,enum=val1|val2|val3"`
```

- **input/output**: Field direction (required)
- **desc**: Field description for the LM (optional)
- **enum**: Valid values (optional, enforces validation)

## Benefits of Typed API

1. **Compile-Time Safety**: Catch errors before runtime
2. **IDE Support**: Auto-completion and refactoring
3. **No Type Assertions**: Cleaner, safer code
4. **Self-Documenting**: Struct definitions document the API
5. **Refactoring Safety**: Type system catches breaking changes
6. **Better Testing**: Mock typed functions easily

## Use Cases

- **Production Systems**: Type safety for critical applications
- **Large Codebases**: Easier to maintain and refactor
- **Team Development**: Clear interfaces between components
- **API Wrappers**: Wrap LM calls with strong types
- **Configuration Validation**: Enum tags ensure valid values

## See Also

- [001_predict](../001_predict/) - Basic untyped Predict module
- [002_chain_of_thought](../002_chain_of_thought/) - Untyped ChainOfThought
- [003_react](../003_react/) - Untyped ReAct agent
- [014_fewshot](../014_fewshot/) - Advanced few-shot learning
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
