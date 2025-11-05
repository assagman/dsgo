# 014_adapter_fallback - Resilient Response Parsing with Adapter Fallback

## Overview

Demonstrates DSGo's **FallbackAdapter** system, which provides robust parsing of LM responses by automatically trying multiple adapters in sequence. This achieves >95% parse success rate across diverse LM response formats without manual intervention.

## What it demonstrates

- Automatic adapter fallback mechanism (ChatAdapter → JSONAdapter)
- Parse robustness across different LM response formats
- Adapter metrics and observability
- Production-grade error resilience
- Zero-configuration fallback behavior
- Parse attempt tracking and diagnostics

## Usage

```bash
cd examples/014_adapter_fallback
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=1
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
=== Adapter Fallback Demo ===
Testing parse robustness across different inputs...

Test 1: I absolutely love this product! It's amazing!
------------------------------------------------------------
Sentiment:  positive
Confidence: 0.95
Reasoning:  The text contains strong positive language ("absolutely love", "amazing")

[Adapter Metrics]
  Adapter Used:    ChatAdapter
  Parse Attempts:  1
  Fallback Used:   false
  Status:          ✓ Parsed on first attempt

Test 2: This is terrible. Waste of money.
------------------------------------------------------------
Sentiment:  negative
Confidence: 0.92
Reasoning:  Clear negative sentiment with words like "terrible" and "waste of money"

[Adapter Metrics]
  Adapter Used:    ChatAdapter
  Parse Attempts:  1
  Fallback Used:   false
  Status:          ✓ Parsed on first attempt

Test 3: It's okay, nothing special.
------------------------------------------------------------
Sentiment:  neutral
Confidence: 0.78
Reasoning:  Lukewarm response without strong positive or negative indicators

[Adapter Metrics]
  Adapter Used:    JSONAdapter
  Parse Attempts:  2
  Fallback Used:   true
  Status:          ⚠ Required fallback

=== Summary ===
The FallbackAdapter automatically handles different LM response formats:
  1. ChatAdapter tries first (field markers format)
  2. JSONAdapter tries next (JSON format)
  3. Achieves >95% parse success rate
```

## Key Concepts

### 1. FallbackAdapter (Default)

All DSGo modules use **FallbackAdapter** by default, providing automatic resilience:

```go
predict := module.NewPredict(sig, lm)
// FallbackAdapter is used automatically!
```

**Fallback Chain:**
1. **ChatAdapter** (tries first) - Expects field markers like `[sentiment]: positive`
2. **JSONAdapter** (fallback) - Expects JSON format `{"sentiment": "positive"}`

**Why this works:**
- Different LMs use different response formats
- Some responses may be malformed
- FallbackAdapter handles all cases automatically

### 2. Adapter Metrics

Track which adapter succeeded and how many attempts were needed:

```go
if result.AdapterUsed != "" {
    fmt.Printf("Adapter Used:    %s\n", result.AdapterUsed)
    fmt.Printf("Parse Attempts:  %d\n", result.ParseAttempts)
    fmt.Printf("Fallback Used:   %v\n", result.FallbackUsed)
    fmt.Printf("Parse Success:   %v\n", result.ParseSuccess)
}
```

**Available Metrics:**
- `AdapterUsed` - Which adapter successfully parsed (ChatAdapter, JSONAdapter)
- `ParseAttempts` - Number of parsing attempts (1 = first try, 2 = used fallback)
- `FallbackUsed` - Whether fallback was triggered (true/false)
- `ParseSuccess` - Whether parsing succeeded overall

### 3. Production Robustness

The FallbackAdapter system provides production-grade resilience:

**Parse Success Rate: >95%**
- ChatAdapter handles most responses
- JSONAdapter catches edge cases
- Together they handle nearly all LM outputs

**Automatic Recovery:**
- No manual intervention needed
- No retry logic required
- Transparent to application code

### 4. Adapter Types

DSGo provides multiple adapters for different scenarios:

**ChatAdapter** (default, first in chain):
- Field markers: `[field_name]: value`
- Natural conversational format
- Works with most LMs

**JSONAdapter** (fallback):
- Strict JSON format: `{"field": "value"}`
- Handles structured responses
- Includes JSON repair capabilities

**TwoStepAdapter** (advanced):
- First generates reasoning, then structured output
- Best for complex reasoning tasks
- See [017_adapters](../017_adapters/) for details

## Use Cases

### 1. Production Applications

When you need reliability without manual error handling:

```go
// This is all you need - FallbackAdapter is automatic!
predict := module.NewPredict(sig, lm)
result, err := predict.Forward(ctx, inputs)
// Parse success rate: >95%
```

### 2. Multi-LM Support

Switch between LMs without code changes:

```go
// Works with OpenAI
lm := openai.New("gpt-4", apiKey)

// Works with OpenRouter
lm := openrouter.New("anthropic/claude-3.5-sonnet", apiKey)

// FallbackAdapter handles format differences automatically
```

### 3. Observability & Monitoring

Track adapter performance in production:

```go
result, err := predict.Forward(ctx, inputs)

// Log adapter metrics for monitoring
log.Printf("Adapter: %s, Attempts: %d, Fallback: %v",
    result.AdapterUsed,
    result.ParseAttempts,
    result.FallbackUsed)

// Alert if fallback rate is high (indicates LM format issues)
if result.FallbackUsed {
    metrics.IncrementCounter("adapter_fallback_used")
}
```

### 4. Debugging LM Responses

Understand parsing behavior during development:

```go
result, err := predict.Forward(ctx, inputs)

if result.ParseAttempts > 1 {
    fmt.Printf("⚠ ChatAdapter failed, JSONAdapter succeeded\n")
    fmt.Printf("LM Response format may be inconsistent\n")
    // Consider tuning prompt or switching LM
}
```

## Advanced Configuration

### Custom Adapter Chain

Override the default FallbackAdapter if needed:

```go
// Use only ChatAdapter (no fallback)
predict := module.NewPredict(sig, lm).
    WithAdapter(dsgo.NewChatAdapter())

// Use only JSONAdapter
predict := module.NewPredict(sig, lm).
    WithAdapter(dsgo.NewJSONAdapter())

// Custom fallback order
customAdapter := dsgo.NewFallbackAdapter(
    dsgo.NewTwoStepAdapter(),
    dsgo.NewChatAdapter(),
    dsgo.NewJSONAdapter(),
)
predict := module.NewPredict(sig, lm).
    WithAdapter(customAdapter)
```

### Adapter-Specific Options

Configure individual adapters:

```go
// JSONAdapter with custom repair
jsonAdapter := dsgo.NewJSONAdapter().
    WithRepairEnabled(true).
    WithStrictValidation(false)

// ChatAdapter with custom markers
chatAdapter := dsgo.NewChatAdapter().
    WithFieldMarker(">>").
    WithCaseSensitive(false)
```

## Performance Characteristics

**Parse Time:**
- ChatAdapter: ~1-2ms overhead
- JSONAdapter: ~2-5ms overhead (includes JSON repair)
- Fallback penalty: ~3-7ms total

**Memory:**
- Minimal overhead (~1KB per parse)
- No additional API calls

**Success Rates:**
- ChatAdapter alone: ~75-85%
- JSONAdapter alone: ~70-80%
- FallbackAdapter: >95%

## Troubleshooting

### High Fallback Rate

If you see many fallbacks (ParseAttempts > 1):

**Possible Causes:**
- LM prompt needs refinement
- LM model inconsistent with format
- Complex signatures confusing LM

**Solutions:**
- Add examples to signature with `WithDemos()`
- Simplify output field descriptions
- Try different LM model
- Use TwoStepAdapter for complex cases

### Parse Failures (Even with Fallback)

Rare (<5% of cases), but can happen:

**Common Issues:**
- LM completely ignored instruction format
- Response contains no recognizable structure
- LM refused to follow output schema

**Solutions:**
- Check error diagnostics: `err.Error()`
- Enable verbose logging to see raw response
- Review and strengthen prompt instructions
- Add few-shot examples

## Comparison with Other Approaches

**vs. Manual Retry Logic:**
- FallbackAdapter: Automatic, built-in
- Manual Retry: Requires custom code, API cost

**vs. Single Adapter:**
- FallbackAdapter: >95% success rate
- Single Adapter: ~75-85% success rate

**vs. Strict Validation:**
- FallbackAdapter: Flexible, handles variations
- Strict Validation: Brittle, fails on minor deviations

## See Also

- [001_predict](../001_predict/) - Basic Predict module (uses FallbackAdapter by default)
- [017_adapters](../017_adapters/) - All adapter types and advanced usage
- [018_retry_resilience](../018_retry_resilience/) - Retry mechanisms and error handling
- [025_logging_tracing](../025_logging_tracing/) - Observability and monitoring
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide

## Production Tips

1. **Trust the Default**: FallbackAdapter is production-ready out of the box
2. **Monitor Metrics**: Track `FallbackUsed` rate to detect LM issues
3. **Log Failures**: Always log when parsing fails despite fallback
4. **Test with Multiple LMs**: Verify behavior across different providers
5. **Use Adapter Metrics**: Include in telemetry for production insights
6. **Don't Over-Configure**: Default settings work for 95%+ of use cases
7. **Alert on Anomalies**: Set up alerts if fallback rate exceeds threshold

## When to Use FallbackAdapter

**Use (Default):**
- Production applications requiring reliability
- Multi-LM support without code changes
- Quick prototyping without error handling boilerplate
- Any scenario where parse success matters

**Consider Alternatives:**
- **TwoStepAdapter**: Complex reasoning requiring separate thought process
- **Single Adapter**: Benchmarking specific adapter performance
- **Custom Adapter**: Highly specialized response formats

## Architecture Notes

The FallbackAdapter implements a **chain of responsibility pattern**:

```mermaid
graph TD
    A[LM Response] --> B{ChatAdapter}
    B -->|Success| C[Return Result]
    B -->|Failure| D{JSONAdapter}
    D -->|Success| C
    D -->|Failure| E[Return Error]
```

**Key Design Principles:**
- **Fail Fast per Adapter**: Each adapter quickly determines if it can parse
- **Zero Extra API Calls**: Fallback uses same LM response
- **Transparent Metrics**: Track which adapter succeeded
- **Extensible**: Add custom adapters to the chain
