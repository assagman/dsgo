# typed - Type-Safe Signatures for DSGo

The `typed` package provides generic, type-safe wrappers for DSGo modules using Go struct tags.

## Features

- ✅ Generic `Func[I, O]` module with compile-time type safety
- ✅ Automatic signature generation from struct tags
- ✅ Field type inference from Go types
- ✅ Support for all DSGo field types (string, int, float, bool, class/enum, JSON)
- ✅ Type-safe few-shot examples with `WithDemosTyped()`
- ✅ Full integration with existing DSGo features (options, adapters, history)

## Quick Start

```go
package main

import (
    "context"
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/typed"
)

// Define input and output structs with dsgo tags
type SentimentInput struct {
    Text string `dsgo:"input,desc=Text to analyze"`
}

type SentimentOutput struct {
    Sentiment string `dsgo:"output,enum=positive|negative|neutral"`
    Score     int    `dsgo:"output,desc=Confidence score 0-100"`
}

func main() {
    ctx := context.Background()
    lm, _ := dsgo.NewLM(ctx)
    
    // Create type-safe predictor
    predictor, _ := typed.NewPredict[SentimentInput, SentimentOutput](lm)
    
    // Run with type-safe input
    output, _ := predictor.Run(ctx, SentimentInput{
        Text: "I love this new feature!",
    })
    
    // Access type-safe output
    println(output.Sentiment) // "positive"
    println(output.Score)     // 95
}
```

## Struct Tags

The `dsgo` tag supports the following options:

- **Direction**: `input` or `output` (required)
- **Description**: `desc=Description text`
- **Optional**: `optional` (marks field as not required)
- **Enum**: `enum=value1|value2|value3` (for class/enum types)
- **Alias**: `alias:short=long` (for enum value synonyms)

### Examples

```go
type Input struct {
    // Required input
    Query string `dsgo:"input,desc=User query"`
    
    // Optional input
    Context string `dsgo:"input,optional,desc=Additional context"`
}

type Output struct {
    // Enum/class output
    Category string `dsgo:"output,enum=tech|science|sports"`
    
    // Optional output with description
    Confidence float64 `dsgo:"output,optional,desc=Confidence score"`
}
```

## Type Inference

Go types are automatically mapped to DSGo field types:

| Go Type | DSGo Type |
|---------|-----------|
| `string` | `FieldTypeString` |
| `int`, `int32`, `int64` | `FieldTypeInt` |
| `float32`, `float64` | `FieldTypeFloat` |
| `bool` | `FieldTypeBool` |
| `map[string]any`, `[]any` | `FieldTypeJSON` |
| `string` with `enum=...` | `FieldTypeClass` |

## Advanced Usage

### Few-Shot Examples (Type-Safe)

```go
inputs := []TranslateInput{
    {Text: "Hello", Target: "es"},
    {Text: "Goodbye", Target: "fr"},
}
outputs := []TranslateOutput{
    {Translation: "Hola"},
    {Translation: "Au revoir"},
}

predictor, _ = predictor.WithDemosTyped(inputs, outputs)
```

### Access Prediction Metadata

```go
output, pred, _ := predictor.RunWithPrediction(ctx, input)

// Access typed output
println(output.Result)

// Access prediction metadata
println("Tokens:", pred.Usage.TotalTokens)
println("Cost:", pred.Usage.Cost)
println("Rationale:", pred.Rationale)
```

### Custom Options

```go
predictor.WithOptions(&dsgo.GenerateOptions{
    Temperature: 0.3,
    MaxTokens:   100,
})
```

### Custom Adapter

```go
predictor.WithAdapter(dsgo.NewJSONAdapter())
```

### Conversation History

```go
history := dsgo.NewHistory()
predictor.WithHistory(history)
```

## API Reference

### Constructors

- `NewPredict[I, O](lm LM) (*Func[I, O], error)` - Create type-safe Predict module
- `NewCoT[I, O](lm LM) (*Func[I, O], error)` - Create type-safe ChainOfThought module
- `NewReAct[I, O](lm LM, tools []Tool) (*Func[I, O], error)` - Create type-safe ReAct module
- `NewPredictWithDescription[I, O](lm LM, desc string) (*Func[I, O], error)` - Create Predict with custom description

### Methods

- `Run(ctx, input I) (O, error)` - Execute with type-safe I/O
- `RunWithPrediction(ctx, input I) (O, *Prediction, error)` - Get output and prediction
- `WithOptions(*GenerateOptions)` - Set generation options (all modules)
- `WithAdapter(Adapter)` - Set custom adapter (all modules)
- `WithHistory(*History)` - Set conversation history (all modules)
- `WithDemos([]Example)` - Set map-based few-shot examples (all modules)
- `WithDemosTyped(inputs []I, outputs []O)` - Set type-safe few-shot examples (all modules)
- `WithMaxIterations(int)` - Set max iterations (ReAct only)
- `WithVerbose(bool)` - Enable verbose logging (ReAct only)

### Utilities

- `StructToSignature(reflect.Type, description) (*Signature, error)` - Convert struct to signature
- `StructToMap(v any) (map[string]any, error)` - Convert struct to map
- `MapToStruct(m map[string]any, target any) error` - Convert map to struct
- `ParseStructTags(structType) ([]FieldInfo, error)` - Parse dsgo tags

## Testing

The package has 81.6% test coverage with comprehensive tests for:
- Tag parsing (basic, enum, optional, aliases)
- Type inference (all Go types)
- Struct/map conversion (including type coercion)
- Generic Func execution
- Few-shot examples
- Error handling

Run tests:
```bash
go test -v ./typed/
```

## Example

See [examples/typed_signatures/](../examples/typed_signatures/) for a complete working example.
