# Example 06: Parallel Batch Processing

**Key Concepts**: Parallel module, batch processing, worker pools, concurrent execution

## What This Demonstrates

- **Parallel Module**: Execute module across multiple inputs concurrently
- **Worker Pool**: Configure maximum concurrent workers
- **Error Handling**: Set failure thresholds and fail-fast behavior
- **Batch Input Modes**: Map-of-slices pattern for batch processing
- **Metrics Tracking**: Per-task latency, success/failure counts
- **Factory Pattern**: Isolate state for stateful modules

## The Story

You're building a customer feedback analysis system that needs to process hundreds of reviews efficiently. The Parallel module enables concurrent processing with configurable parallelism and robust error handling.

## Code Walkthrough

### 1. Define Signature

```go
sig := core.NewSignature("Analyze customer review sentiment").
    AddInput("review", core.FieldTypeString, "Customer review text").
    AddClassOutput("sentiment", []string{"positive", "neutral", "negative"}, "Overall sentiment").
    AddOutput("reason", core.FieldTypeString, "Brief explanation")
```

Standard signature for sentiment analysis.

### 2. Create Base Module

```go
predictor := module.NewPredict(sig, lm)
```

Create the module that will be executed in parallel.

### 3. Configure Parallel Module

```go
parallel := module.NewParallel(predictor).
    WithMaxWorkers(3).         // Process up to 3 reviews concurrently
    WithMaxFailures(1).        // Allow 1 failure without stopping
    WithReturnAll(true).       // Return all results
    WithOnlySuccessful(true)   // Only include successful results
```

**Key Options**:
- `WithMaxWorkers(n)` - Maximum concurrent executions (default: NumCPU)
- `WithMaxFailures(n)` - Maximum failures before error (default: 0)
- `WithFailFast(bool)` - Cancel on first failure
- `WithReturnAll(bool)` - Include all results in Completions
- `WithOnlySuccessful(bool)` - Filter failures from Completions
- `WithRepeat(n)` - Repeat same input N times

### 4. Prepare Batch Inputs

**Map-of-slices pattern**:
```go
batchInputs := map[string]any{
    "review": []any{review1, review2, review3, ...},
}
```

All slices must have equal length. Scalar values are broadcast to all tasks.

**Alternative patterns**:
```go
// Explicit batch array
inputs := map[string]any{
    "_batch": []map[string]any{
        {"review": review1},
        {"review": review2},
        {"review": review3},
    },
}

// Custom batch key
parallel.WithBatchKey("items")
inputs := map[string]any{
    "items": []map[string]any{...},
}
```

### 5. Execute and Process Results

```go
result, err := parallel.Forward(ctx, batchInputs)

// Access individual completions
for i, completion := range result.Completions {
    sentiment := completion["sentiment"].(string)
    reason := completion["reason"].(string)
}

// Access aggregated metrics
totalTokens := result.Usage.TotalTokens
totalCost := result.Usage.Cost
```

### 6. Parallel Metrics

```go
if metrics, ok := result.Outputs["__parallel_metrics"].(module.ParallelMetrics); ok {
    fmt.Printf("Total: %d, Successes: %d, Failures: %d\n",
        metrics.Total, metrics.Successes, metrics.Failures)
    fmt.Printf("Latency: min=%dms, avg=%dms, max=%dms, p50=%dms\n",
        metrics.Latency.MinMs, metrics.Latency.AvgMs,
        metrics.Latency.MaxMs, metrics.Latency.P50Ms)
}
```

## Factory Pattern for Stateful Modules

⚠️ **Important**: Modules with internal state (e.g., `Predict` with `History`) are NOT thread-safe when shared.

**Solution**: Use factory pattern to create isolated instances:

```go
factory := func(i int) core.Module {
    history := core.NewHistory()
    return module.NewPredict(sig, lm).WithHistory(history)
}

parallel := module.NewParallelWithFactory(factory)
```

**Alternative**: Pre-create instances:

```go
instances := make([]core.Module, 10)
for i := 0; i < 10; i++ {
    instances[i] = module.NewPredict(sig, lm).WithHistory(core.NewHistory())
}

parallel := module.NewParallelWithInstances(instances)
```

## Error Handling Strategies

### Fail-Fast (Cancel on First Failure)

```go
parallel := module.NewParallel(predictor).
    WithFailFast(true)  // Cancels all remaining tasks on first failure
```

Use when: Any failure makes the entire batch invalid.

### Partial Success (Collect All Results)

```go
parallel := module.NewParallel(predictor).
    WithMaxFailures(5)  // Allow up to 5 failures
```

Use when: You can proceed with partial results.

### Strict (No Failures Allowed)

```go
parallel := module.NewParallel(predictor).
    WithMaxFailures(0)  // Default: any failure errors
```

Use when: All tasks must succeed.

## Performance Considerations

1. **Worker Pool Size**
   - Default: `runtime.NumCPU()`
   - Increase for I/O-bound tasks (API calls)
   - Decrease for CPU-bound tasks
   - Set to `len(batch)` for unlimited parallelism

2. **Token Costs**
   - Parallel execution uses same total tokens as sequential
   - Aggregated cost in `result.Usage.Cost`

3. **Latency**
   - Total latency ≈ `max(individual latencies)` (vs sum for sequential)
   - Check `metrics.Latency.MaxMs` for bottlenecks

## Running the Example

```bash
cd examples/06-parallel
EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" go run main.go
```

## Expected Output

```
=== Parallel Sentiment Analysis ===
Processing 5 reviews in parallel...

Successfully processed 5/5 reviews

Review 1:
  Text: The product quality is amazing! Fast shipping too. Highly recommend.
  Sentiment: positive
  Reason: Expresses strong satisfaction with product and service

Review 2:
  Text: Disappointed with the customer service. Product is okay but nothing special.
  Sentiment: negative
  Reason: Disappointment with service outweighs neutral product opinion

[...]

=== Summary ===
Positive: 3
Neutral: 1
Negative: 1

=== Performance Metrics ===
Total tokens: 450 (prompt: 250, completion: 200)
Total cost: $0.000675
Latency: 1234ms

Parallel execution:
  Total tasks: 5
  Successes: 5
  Failures: 0
  Latency: min=823ms, avg=987ms, max=1234ms, p50=945ms
```

## DSPy Parity Note

DSGo's Parallel differs from DSPy in return semantics:
- **DSPy**: Returns `list[Any]` with `None` for failures
- **DSGo**: Returns `Prediction` with first successful result as primary output + all results in `Completions`

This matches DSGo's module conventions (BestOfN, Refine) where `Prediction` is the standard return type. The `Completions` field provides access to all parallel results, while the primary `Outputs` contains the first successful execution for convenience.

## Key Takeaways

✅ **Parallel module** enables efficient batch processing with worker pools

✅ **Multiple input modes** support different batch patterns (explicit batch, map-of-slices, repeat)

✅ **Configurable error handling** with fail-fast, partial success, and strict modes

✅ **Factory pattern** ensures thread-safety for stateful modules

✅ **Rich metrics** track per-task latency and success/failure counts

✅ **Usage aggregation** sums tokens and costs across all parallel tasks
