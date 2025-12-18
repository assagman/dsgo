# Program Tracing Examples

This document demonstrates how to use DSGo's program tracing capabilities for debugging, observability, and progress reporting.

## Key Use Cases

### 1. Debugging Pipeline Issues
- Access intermediate results to identify where problems occur
- Check input/output compatibility between steps
- View detailed error messages and timing information

### 2. Observability and Monitoring
- Track execution progress in real-time
- Monitor resource usage (tokens, cost, latency)
- Collect metrics for performance analysis

### 3. UI/CLI Progress Reporting
- Show step-by-step progress to users
- Display estimated completion times
- Provide detailed status updates

### 4. Pipeline Optimization
- Identify bottlenecks with per-step timing
- Analyze cost distribution across steps
- Compare performance between different configurations

### 5. Testing and Validation
- Verify intermediate outputs meet expectations
- Test error handling and recovery
- Validate data flow through the pipeline

## Running the Examples

```bash
cd examples/program_tracing
go run tracing_examples.go
```

This will run through several examples showing:
1. **Basic Execution Tracing**: How to access execution traces and metrics
2. **Intermediate Results**: How to retrieve step-by-step outputs even though `Forward()` only returns the final result
3. **Error Debugging**: How to use execution traces to diagnose pipeline failures
4. **Metrics Collection**: How to collect and analyze performance metrics

## Key APIs Demonstrated

### Accessing Execution Traces
```go
execution := program.GetExecution()
metrics := execution.Metrics()
```

### Intermediate Results Access
```go
for i, step := range execution.Steps {
    if step.Status == dsgo.StepStatusCompleted {
        outputs := step.Prediction.Outputs
        duration := step.Duration
        // Process intermediate results
    }
}
```

### Error Diagnosis
```go
if execution.Status == dsgo.ExecutionStatusFailed {
    for i, step := range execution.Steps {
        if step.Status == dsgo.StepStatusFailed {
            fmt.Printf("Step %d failed: %v\n", i, step.Error)
            fmt.Printf("Inputs: %v\n", step.Inputs)
        }
    }
}
```

### Performance Metrics
```go
metrics := execution.Metrics()
fmt.Printf("Total duration: %v\n", metrics.TotalDuration)
fmt.Printf("Slowest step: %d (%v)\n", metrics.SlowestStepIndex, metrics.SlowestStep)
fmt.Printf("Total cost: $%.6f\n", metrics.TotalUsage.Cost)
```

## Understanding the Execution Model

DSGo's `Program.Forward()` method always returns only the **last module's prediction** for backward compatibility. However, the complete execution trace containing all intermediate results is available via `program.GetExecution()`.

This design allows:
- **Simple API**: Most use cases only need the final result
- **Full observability**: All intermediate data is accessible when needed
- **Performance**: No overhead for users who don't need tracing
- **Debugging**: Complete visibility into pipeline execution

The execution trace includes:
- Per-step timing information
- Input/output data for each step
- Error information and stack traces
- Aggregated usage metrics (tokens, cost, latency)
- Overall execution status and duration

## Integration with Observability Systems

The tracing data can be easily integrated with:
- **Logging systems**: Structured logging of execution metrics
- **Monitoring platforms**: Export metrics to Prometheus, etc.
- **Progress bars**: Real-time UI updates during long executions
- **Debugging tools**: Step-by-step pipeline analysis
- **Cost tracking**: Detailed cost breakdown per step
