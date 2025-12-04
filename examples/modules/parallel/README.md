# Parallel Module Example

This example demonstrates DSGo Parallel module's capabilities using only the public API, including automatic state isolation and various execution patterns.

## Features Demonstrated

### 1. Basic Parallel Execution
- Simple text classification using map-of-slices input
- Automatic task distribution across workers
- Aggregated cost and token usage tracking

### 2. State Isolation with History
- Parallel execution of stateful modules with History
- Automatic cloning per task prevents race conditions
- Each parallel task gets isolated state

### 3. Parallel with Tools (ReAct)
- Parallel ReAct agents with tool usage
- Complex reasoning tasks executed concurrently
- Tool calls properly isolated per task

### 4. Advanced Patterns
- Error handling and resilience
- Usage statistics and metrics
- Different input modes (map-of-slices)

## Key Concepts

### Automatic State Isolation
The Parallel module automatically clones modules per task, ensuring:
- No shared History between parallel tasks
- Thread-safe execution by default
- DSPy-like semantic behavior

### Input Modes
- **Map-of-slices**: `map[string]any{"text": []string{"a", "b", "c"}}`
- **Batch mode**: `map[string]any{"_batch": []map[string]any{...}}`
- **Repeat mode**: Use `WithRepeat(n)` to duplicate single input

### Result Access
```go
result, err := parallel.Forward(ctx, inputs)
if err == nil {
    // Primary result (first successful)
    answer := result.GetString("answer")
    
    // All results (when WithReturnAll(true))
    if completions := result.Completions; completions != nil {
        for _, completion := range completions {
            sentiment := completion["sentiment"].(string)
        }
    }
    
    // Usage statistics
    cost := result.Usage.Cost
    tokens := result.Usage.TotalTokens
}
```

## Running the Example

```bash
# From examples/modules/parallel directory
export OPENAI_API_KEY=sk-...
go run main.go
```

## Expected Output

The example will run 4 different scenarios:
1. **Basic Classification**: Classify 6 texts for sentiment
2. **Stateful Conversations**: Run 4 conversations with isolated History
3. **Math Problem Solving**: Solve 4 math problems using ReAct agents
4. **Advanced Classification**: Text classification with error handling

Each section shows:
- Number of tasks processed
- Total cost and token usage
- Individual results
- Performance metrics

## Configuration Options

```go
parallel := dsgo.NewParallel(module).
    WithMaxWorkers(3).        // Control concurrency
    WithReturnAll(true).       // Get all results
    WithOnlySuccessful(true).   // Filter failures
    WithFailFast(false).       // Continue on errors
    WithMaxFailures(2).        // Max failures allowed
    WithBatchKey("_batch").     // Custom batch key
    WithRepeat(5)              // Repeat single input
```

## Thread Safety

All Parallel module operations are thread-safe:
- Automatic module cloning per task
- No shared state between parallel executions
- Safe with stateful modules (History, etc.)
- No race conditions detected with `go test -race`

## Performance Considerations

- **Cloning overhead**: Minimal (~50ns per clone)
- **Memory usage**: Each task gets isolated module instance
- **Concurrency**: Limited by `WithMaxWorkers()` setting
- **Cost tracking**: Automatically aggregated across all tasks

This example showcases the production-ready parallel execution capabilities of DSGo, with automatic state isolation making it safe for complex workflows.
