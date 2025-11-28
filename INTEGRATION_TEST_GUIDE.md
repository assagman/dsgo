# DSGo Integration Test Guide

This guide explains how to run, write, and maintain integration tests for DSGo.

## Quick Start

```bash
# Run all integration tests
make integration

# Run with race detector (recommended)
go test -race ./integration/...

# Run specific test category
make integration-compose      # Module composition tests
make integration-providers    # Provider tests
make integration-performance  # Benchmarks
```

## Test Organization

```
integration/
├── fixtures/                        # Reusable test components
│   ├── signatures.go               # Common test signatures (20+)
│   ├── tools.go                    # Mock tools for ReAct tests
│   ├── lm_responses.go             # Predefined LM responses (valid/malformed/error)
│   └── collectors.go               # Test collector implementations
├── testdata/                        # Additional test data
├── assertions.go                    # Custom test assertions
├── builder.go                       # Test setup builder pattern
├── helpers.go                       # Mock LM and utilities
│
├── module_composition_test.go       # Sequential, parallel, nested modules
├── adapter_robustness_test.go       # JSON, Chat, TwoStep, Fallback adapters
├── provider_integration_test.go     # OpenAI, OpenRouter providers (mocked + real)
├── observability_test.go            # History, cost tracking, collectors
├── cache_behavior_test.go           # LRU cache, TTL, concurrency
├── error_recovery_test.go           # Retries, validation, propagation
├── streaming_integration_test.go    # Streaming with markers
├── end_to_end_test.go               # Real-world workflow scenarios
├── react_integration_test.go        # ReAct tool-using agent tests
├── program_of_thought_test.go       # Code generation module tests
├── retry_integration_test.go        # Retry package integration tests
├── concurrency_test.go              # Concurrent execution tests (100+ goroutines)
├── pipeline_observability_test.go   # Cross-module observability tests
├── jsonl_collector_test.go          # JSONL file collector tests
├── composite_collector_test.go      # Multi-sink collector tests
├── memory_test.go                   # Memory profiling and leak detection
└── performance_test.go              # Benchmarks and stress tests
```

## Running Tests

### All Integration Tests

```bash
make integration
# or
go test -v -race ./integration/...
```

### By Category

```bash
# Module composition (sequential, parallel, nested)
go test -v -run "TestSequential|TestParallel|TestNested" ./integration/...

# Adapters (JSON, Chat, Fallback)
go test -v -run "TestAdapter|TestJSON|TestChat|TestFallback" ./integration/...

# Providers
go test -v -run "TestOpenAI|TestOpenRouter|TestProvider" ./integration/...

# Caching
go test -v -run "TestCache" ./integration/...

# Error handling
go test -v -run "TestError|TestRetry|TestValidation" ./integration/...

# Streaming
go test -v -run "TestStreaming" ./integration/...

# End-to-end scenarios
go test -v -run "TestE2E" ./integration/...
```

### Performance Benchmarks

```bash
# All benchmarks
go test -bench=. -benchmem ./integration/...

# Specific benchmark
go test -bench=BenchmarkPredictModule ./integration/...

# Cache benchmarks
go test -bench=BenchmarkCache ./integration/...

# ReAct benchmarks
go test -bench=BenchmarkReAct ./integration/...

# Composition benchmarks
go test -bench=BenchmarkComposition ./integration/...

# With CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./integration/...
```

### Memory Tests

```bash
# Run memory profiling tests
go test -v -run TestMemory ./integration/...

# Memory leak detection
go test -v -run "TestMemory_NoLeaks" ./integration/...

# Streaming memory tests
go test -v -run "TestMemory_Streaming" ./integration/...
```

### Concurrency Tests

```bash
# All concurrency tests (run with race detector!)
go test -v -race -run TestConcur ./integration/...

# 100 goroutine stress test
go test -v -race -run "TestConcurrentModuleExecution_100Goroutines" ./integration/...

# Cache contention tests
go test -v -race -run "TestCacheConcurrency" ./integration/...
```

### ReAct Module Tests

```bash
# All ReAct tests
go test -v -run TestReAct ./integration/...

# Tool execution tests
go test -v -run "TestReAct_SingleTool|TestReAct_MultipleTool" ./integration/...
```

### Collector Tests

```bash
# JSONL collector tests
go test -v -run TestJSONL ./integration/...

# Composite collector tests
go test -v -run TestComposite ./integration/...
```

### With Coverage

```bash
make integration-coverage
# Generates integration_coverage.out
```

## Writing Tests

### Using the Test Builder

```go
func TestMyFeature(t *testing.T) {
    // Quick setup with builder
    test := NewTestBuilder().
        WithSignature(fixtures.SimplePredictSig).
        WithMockResponse(`{"answer": "42"}`).
        WithTimeout(5 * time.Second).
        Build(t)
    
    result, err := test.Module.Forward(test.Ctx, map[string]any{
        "question": "What is the answer?",
    })
    
    AssertNoError(t, err)
    AssertPredictionValid(t, result, []string{"answer"})
}
```

### Using Mock LM Directly

```go
func TestCustomScenario(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Create mock with multiple responses
    lm := NewMockLMWithResponses([]string{
        `{"step1": "first"}`,
        `{"step2": "second"}`,
    })
    
    sig := fixtures.SimplePredictSig
    pred := module.NewPredict(sig, lm)
    
    // First call uses first response
    result1, _ := pred.Forward(ctx, map[string]any{"question": "q1"})
    
    // Second call uses second response
    result2, _ := pred.Forward(ctx, map[string]any{"question": "q2"})
}
```

### Using Fixtures

```go
import "github.com/assagman/dsgo/integration/fixtures"

func TestWithFixtures(t *testing.T) {
    // Basic signatures
    sig := fixtures.SimplePredictSig()      // 1 input, 1 output
    sig := fixtures.ClassificationSig()     // With class field
    sig := fixtures.ChainOfThoughtSig()     // With reasoning
    sig := fixtures.ComplexOutputSig()      // Multiple typed outputs
    sig := fixtures.ReActSig()              // For tool-using agents
    
    // Advanced signatures
    sig := fixtures.MultiInputSig()         // Multiple inputs
    sig := fixtures.MultiOutputSig()        // Many output types
    sig := fixtures.MultiClassificationSig() // Multiple classifications
    sig := fixtures.AllOptionalSig()        // All optional outputs
    
    // Domain-specific signatures
    sig := fixtures.SummarizationSig()      // Text summarization
    sig := fixtures.TranslationSig()        // Translation tasks
    sig := fixtures.QASig()                 // Question answering
    sig := fixtures.CodeReviewSig()         // Code review
    
    // Basic tools
    tool := fixtures.CalculatorTool()       // Simple math
    tool := fixtures.SearchTool()           // Mock search
    tool := fixtures.DatabaseQueryTool()    // Database queries
    
    // Advanced tools
    tool := fixtures.ComplexJSONTool()      // Returns nested JSON
    tool := fixtures.DelayedTool(100*time.Millisecond) // With delay
    tool := fixtures.StatefulTool()         // Maintains state
    tool := fixtures.MultiStepTool()        // Multi-step process
    
    // LM responses
    response := fixtures.ValidSimpleAnswer()     // Basic JSON
    response := fixtures.ValidWithReasoning()    // With reasoning
    response := fixtures.MalformedSingleQuotes() // For repair testing
    response := fixtures.PartialMissingRequired() // Missing fields
    
    // Test collectors
    collector := fixtures.NewCountingCollector()      // Just counts
    collector := fixtures.NewFailingCollector(5)      // Fails after 5
    collector := fixtures.NewDelayedCollector(10*time.Millisecond, 100)
    
    // Sample history entries
    entry := fixtures.SampleHistoryEntry("id", "openai", "gpt-4")
    entry := fixtures.SampleHistoryEntryWithError("id", "openai", "gpt-4", "error msg")
    entry := fixtures.SampleHistoryEntryWithToolCalls("id", "openai", "gpt-4", 3)
}
```

### Custom Assertions

```go
// Validate prediction has expected fields
AssertPredictionValid(t, pred, []string{"answer", "confidence"})

// Check specific output
AssertOutputEquals(t, pred, "answer", "42")

// Validate usage tracking
AssertUsageTracked(t, pred)

// Check cost calculation
AssertCostInRange(t, pred, 0.001, 0.01)

// Verify cache behavior
AssertCacheHit(t, pred)

// Check history collection
AssertHistoryCollected(t, collector, 3) // 3 entries
```

## Test Patterns

### Sequential Module Composition

```go
func TestSequentialPipeline(t *testing.T) {
    ctx := context.Background()
    
    // Stage 1: Extract
    extractLM := NewMockLMWithResponse(`{"entities": ["AI", "ML"]}`)
    extractor := module.NewPredict(extractSig, extractLM)
    
    // Stage 2: Classify
    classifyLM := NewMockLMWithResponse(`{"category": "technology"}`)
    classifier := module.NewPredict(classifySig, classifyLM)
    
    // Execute pipeline
    extracted, _ := extractor.Forward(ctx, map[string]any{"text": "..."})
    entities, _ := extracted.GetString("entities")
    
    classified, _ := classifier.Forward(ctx, map[string]any{"entities": entities})
    
    // Verify end-to-end
    category, _ := classified.GetString("category")
    if category != "technology" {
        t.Errorf("expected technology, got %s", category)
    }
}
```

### Error Recovery Testing

```go
func TestRetryBehavior(t *testing.T) {
    // LM that fails twice then succeeds
    lm := &MockLM{
        Responses: []string{
            "", // triggers error
            "", // triggers error
            `{"answer": "success"}`,
        },
        Errors: []error{
            errors.New("transient error"),
            errors.New("transient error"),
            nil,
        },
    }
    
    // Wrap with retry
    retryLM := dsgo.WithRetry(lm, retry.WithMaxAttempts(3))
    
    pred := module.NewPredict(sig, retryLM)
    result, err := pred.Forward(ctx, inputs)
    
    AssertNoError(t, err)
    AssertPredictionValid(t, result, []string{"answer"})
}
```

### Concurrent Execution Testing

```go
func TestConcurrentSafety(t *testing.T) {
    lm := NewMockLMWithResponse(`{"answer": "ok"}`)
    pred := module.NewPredict(sig, lm)
    
    var wg sync.WaitGroup
    errors := make(chan error, 100)
    
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            _, err := pred.Forward(ctx, map[string]any{
                "question": fmt.Sprintf("q%d", id),
            })
            if err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    for err := range errors {
        t.Errorf("concurrent execution failed: %v", err)
    }
}
```

## Environment Variables

```bash
# Enable real provider tests (requires API keys)
USE_REAL_PROVIDERS=true

# Provider API keys
OPENAI_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-v1-...

# Debug options
DSGO_DEBUG_PARSE=1           # Show parse attempts
DSGO_SAVE_RAW_RESPONSES=1    # Save raw LM outputs
DSGO_DEBUG_MARKERS=1         # Show field markers in streaming
```

## Test Data

### Adding Mock Responses

Edit `testdata/lm_responses.go`:

```go
var MyCustomResponses = []string{
    `{"field1": "value1", "field2": 42}`,
    `{"field1": "value2", "field2": 100}`,
}
```

### Adding Test Signatures

Edit `fixtures/signatures.go`:

```go
var MyCustomSig = dsgo.NewSignature(
    "custom_task",
    "Description of the task",
).WithInputField(
    "input1", dsgo.FieldTypeString, "Input description",
).WithOutputField(
    "output1", dsgo.FieldTypeString, "Output description",
)
```

## Troubleshooting

### Test Flakiness

- Use deterministic mock responses
- Set explicit timeouts: `context.WithTimeout(ctx, 5*time.Second)`
- Avoid time-based assertions when possible

### Race Conditions

Always run with race detector:
```bash
go test -race ./integration/...
```

### Memory Issues

Run memory tests:
```bash
go test -v -run TestMemory ./integration/...
```

### Debugging Failures

```bash
# Verbose output
go test -v -run TestSpecificTest ./integration/...

# With debug logging
DSGO_DEBUG_PARSE=1 go test -v -run TestSpecificTest ./integration/...
```

## CI/CD Integration

Integration tests run automatically on:
- Every push to `main`
- Every pull request

Provider tests with real APIs run:
- Daily scheduled jobs
- Manual trigger for releases

See `.github/workflows/ci.yml` for configuration.

## Maintenance

### Quarterly Review Checklist

- [ ] Update mock responses with real-world examples
- [ ] Verify provider pricing tables
- [ ] Update performance baselines
- [ ] Review and fix any flaky tests
- [ ] Add tests for new features

### When Adding Features

1. Add unit tests first (target >90% coverage)
2. Add integration test for cross-component behavior
3. Update fixtures if new signatures needed
4. Update this guide if new patterns introduced
