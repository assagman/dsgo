# Examples Testing Guide

## Overview

DSGo provides comprehensive testing infrastructure for validating examples across multiple models and configurations. The test matrix system enables:

- **Single model testing** - Fast validation with default model
- **Random sampling** - Test N random models for broader coverage
- **Full matrix** - Exhaustive testing across all supported models
- **Parallel execution** - Concurrent test runs with resource limits
- **Circuit breaker** - Early termination on systematic failures
- **Compatibility matrix** - Skip known incompatible model/example combinations

## Quick Start

### Test with Single Model (Fast - 2-3 minutes)

```bash
make test-matrix-quick
```

Uses default model (`gemini-2.5-flash`) for all 28 examples. Perfect for:
- Pre-commit validation
- Quick sanity checks
- Development iteration

### Test with Random Models (Medium - 5-15 minutes)

```bash
make test-matrix-sample N=3
```

Tests all examples against 3 randomly selected models. Good for:
- CI/CD pipelines
- Broader validation without full cost
- Finding model-specific issues

### Test All Models (Slow - 30-60 minutes)

```bash
make test-matrix
```

Comprehensive test across all 14 models × 28 examples = 392 test runs. Use for:
- Release validation
- Model compatibility updates
- Comprehensive quality assurance

## Test Matrix Architecture

### Directory Structure

```
dsgo/
├── examples/
│   ├── test_matrix/             # ⭐ Test matrix tool (first-class example tool)
│   │   └── main.go
│   ├── 001_predict/             # Numbered examples (new architecture)
│   ├── 002_chain_of_thought/
│   └── ...
├── test_matrix_logs/            # Auto-generated test results
│   ├── passed/
│   │   └── model_example_*.log
│   └── failed/
│       └── model_example_*.log
└── Makefile                     # Convenient targets
```

### How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                     make test-matrix-quick                       │
│                  (or test-matrix-sample N=3)                     │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
                ┌──────────────────────┐
                │  Select Models        │
                │  - Single: default    │
                │  - Sample: random N   │
                │  - All: all 14 models │
                └──────────┬────────────┘
                           │
                           ▼
         ┌─────────────────────────────────────┐
         │    Create Circuit Breaker           │
         │  Overall: 85% success required      │
         │  Per-Model: 65% success required    │
         └──────────┬──────────────────────────┘
                    │
                    ▼
    ┌───────────────────────────────────────────────┐
    │  For Each Model × Example Combination         │
    │  (Skip incompatible pairs from matrix)        │
    └──────────┬────────────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────────┐
│          Parallel Execution Pool                  │
│  Max 20 concurrent (configurable with -c)         │
└──────────┬───────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────┐
│  Run: go run examples/NNN_name/main.go           │
│  Env: OPENROUTER_MODEL=model                     │
│  Timeout: 10 minutes (configurable)              │
└──────────┬───────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────┐
│  Collect Results + Update Circuit Breaker        │
│  - Count passes/failures per model               │
│  - Check thresholds                              │
│  - Trip breaker if exceeded                      │
└──────────┬───────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────┐
│  Generate Report                                  │
│  - Summary table (model scores)                  │
│  - Failed test details                           │
│  - Save logs to test_matrix_logs/               │
└──────────────────────────────────────────────────┘
```

## Usage Examples

### Basic Usage

```bash
# Single model (default: gemini-2.5-flash)
make test-matrix-quick

# 3 random models
make test-matrix-sample N=3

# 5 random models
make test-matrix-sample N=5

# All models
make test-matrix
```

### Advanced Usage (Direct Tool)

```bash
# Verbose output
go run examples/test_matrix/main.go -n 1 -v

# Custom timeout (20 minutes per example)
go run examples/test_matrix/main.go -n 3 -timeout=20m

# Sequential execution (no parallelism)
go run examples/test_matrix/main.go -n 1 -p=false

# Custom concurrency limit (50 concurrent)
go run examples/test_matrix/main.go -n 0 -c 50

# Disable colored output
go run examples/test_matrix/main.go -n 1 -no-color

# Combine flags
go run examples/test_matrix/main.go -n 5 -v -timeout=15m -c 30
```

### Environment Variables

```bash
# Override default model for single-model tests
export EXAMPLES_DEFAULT_MODEL="openrouter/anthropic/claude-haiku-4.5"
make test-matrix-quick

# Set API key
export OPENROUTER_API_KEY="your-api-key"
make test-matrix-sample N=3
```

## Test Matrix Script Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n` | int | 1 | Number of models: `1` = single, `N` = random N, `0` = all |
| `-v` | bool | false | Verbose output (show each test result) |
| `-timeout` | duration | 10m | Timeout per example execution |
| `-p` | bool | true | Run tests in parallel |
| `-c` | int | 20 | Max concurrent test executions |
| `-no-color` | bool | false | Disable colored output |

## Circuit Breaker

The test matrix includes a **circuit breaker** that stops testing early if systematic failures are detected:

### Thresholds

- **Overall**: Fails if >15% of all tests fail (requires 85% success rate)
- **Per-Model**: Fails if >35% of any model's tests fail (requires 65% per-model success)

### Behavior

When circuit breaker trips:
1. ✋ Cancels all pending tests
2. 📊 Displays current failure statistics
3. 📝 Dumps all failed test outputs to stderr
4. ❌ Exits with error code

### Example Output

```
🚨 CIRCUIT BREAKER TRIPPED 🚨
Reason: Overall failure threshold exceeded: 62 failures out of 392 tests (15.8% failed, max allowed: 15.0%)

Cancelling remaining tests...

=== Failed Test Outputs ===

[1] program_of_thought [openrouter/minimax/minimax-m2]
Exit Code: 1
Error: exit status 1
Output:
Error: json_schema not supported by provider
---

[2] react_agent [openrouter/deepseek/deepseek-v3.1-terminus:exacto]
Exit Code: 1
Error: exit status 1
Output:
Error: provider connection timeout
---
```

### Disabling Circuit Breaker

To test all examples regardless of failures, modify `examples/test_matrix/main.go`:

```go
// Set thresholds to 100% (never trip)
overallThreshold: 1.0,
```

## Understanding Test Results

### Success Criteria

A test is considered **successful** if:
- ✅ Exit code is 0
- ✅ No panic or fatal error
- ✅ Example completes within timeout

**Note**: Tests are also marked successful (skipped) for:
- Known incompatible model/example pairs
- Rate limit errors (HTTP 429/403)

### Test Logs

All test results are saved to `test_matrix_logs/`:

```
test_matrix_logs/
├── passed/
│   ├── gemini-2.5-flash_001_predict_20250105_143022.log
│   ├── gemini-2.5-flash_002_chain_of_thought_20250105_143025.log
│   └── ...
└── failed/
    ├── minimax-m2_006_program_of_thought_20250105_143045.log
    └── ...
```

Each log contains:
- Example name and model
- Success status and duration
- Exit code and error (if any)
- Complete output

### Report Format

```
=== Test Results ===

Total: 392 tests
Passed: 375 (95.7%)
Failed: 17 (4.3%)
Duration: 12m34s

=== Model Scores (sorted by success rate) ===

  1. openrouter/google/gemini-2.5-flash:           28/28 (100.0%) ✅
  2. openrouter/anthropic/claude-haiku-4.5:        28/28 (100.0%) ✅
  3. openrouter/google/gemini-2.5-pro:             28/28 (100.0%) ✅
  4. openrouter/qwen/qwen3-235b-a22b-2507:         27/28 (96.4%) ✅
  5. openrouter/z-ai/glm-4.6:exacto:               26/28 (92.9%) ✅
  ...

=== Failed Tests (17) ===

 1. program_of_thought [minimax-m2]
    Error: json_schema not supported
    Log: test_matrix_logs/failed/minimax-m2_006_program_of_thought_*.log

 2. react_agent [deepseek-v3.1-terminus]
    Error: provider connection timeout
    Log: test_matrix_logs/failed/deepseek-v3.1-terminus_003_react_*.log
```

## Supported Models

Current test matrix includes 14 models:

### High Performance (Tier 1)
- `openrouter/google/gemini-2.5-flash` ⭐ (default)
- `openrouter/google/gemini-2.5-pro`
- `openrouter/anthropic/claude-haiku-4.5`
- `openrouter/qwen/qwen3-235b-a22b-2507`

### Good Performance (Tier 2)
- `openrouter/z-ai/glm-4.6:exacto`
- `openrouter/minimax/minimax-m2`
- `openrouter/qwen/qwen3-30b-a3b`
- `openrouter/google/gemini-2.0-flash-lite-001`

### Experimental (Tier 3)
- `openrouter/moonshotai/kimi-k2-0905:exacto` (limited json_schema)
- `openrouter/deepseek/deepseek-v3.1-terminus:exacto` (unreliable provider)
- `openrouter/openai/gpt-oss-120b:exacto` (unreliable provider)
- `openrouter/meta-llama/llama-3.1-8b-instruct`

See `examples/test_matrix/main.go` for the authoritative list.

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test Examples Matrix

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test-matrix:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Test with sample models
        env:
          OPENROUTER_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
        run: make test-matrix-sample N=3
      
      - name: Upload test logs
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: test-matrix-logs
          path: test_matrix_logs/
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running quick example validation..."
make test-matrix-quick

if [ $? -ne 0 ]; then
    echo "❌ Example tests failed. Fix before committing."
    exit 1
fi

echo "✅ All examples passed"
```

## Troubleshooting

### All Tests Failing

**Symptom:** Every test fails with API errors

**Diagnosis:**
- Check API key: `echo $OPENROUTER_API_KEY`
- Verify network connectivity
- Check OpenRouter status

**Solution:**
```bash
export OPENROUTER_API_KEY="your-actual-key"
make test-matrix-quick
```

### Timeouts

**Symptom:** Many tests fail with "timeout exceeded"

**Diagnosis:**
- Network latency issues
- Model response time varies
- Default 10m timeout too short

**Solution:**
```bash
# Increase timeout to 20 minutes
go run scripts/test_examples_matrix/main.go -n 1 -timeout=20m
```

### Rate Limits

**Symptom:** Tests fail with HTTP 429 or "rate limit exceeded"

**Diagnosis:**
- Too many concurrent requests
- API tier limitations
- Model-specific rate limits

**Solution:**
```bash
# Reduce concurrency
go run scripts/test_examples_matrix/main.go -n 1 -c 5

# Or run sequentially
go run scripts/test_examples_matrix/main.go -n 1 -p=false
```

### Circuit Breaker Trips Early

**Symptom:** Test run stops before completion

**Diagnosis:**
- Systematic failures (>15% overall or >35% per-model)
- Model incompatibilities not in matrix
- Provider issues

**Solution:**
1. Check failed test logs in stderr output
2. Add incompatible pairs to `incompatibleCombos` in script
3. Fix underlying issues before re-running
4. Temporarily raise thresholds if expected

### Out of Memory

**Symptom:** Script crashes with OOM error

**Diagnosis:**
- Too many concurrent executions
- Large example outputs

**Solution:**
```bash
# Reduce concurrency
go run scripts/test_examples_matrix/main.go -n 3 -c 10
```

## Advanced Topics

### Adding New Models

Edit `examples/test_matrix/main.go`:

```go
var allModels = []string{
    // ... existing models ...
    "openrouter/new-provider/new-model",
}
```

Test the new model:
```bash
export EXAMPLES_DEFAULT_MODEL="openrouter/new-provider/new-model"
make test-matrix-quick
```

### Custom Test Subsets

Run specific examples only by modifying the script temporarily:

```go
var allExamples = []string{
    "examples/001_predict",
    "examples/002_chain_of_thought",
    // Only test these two
}
```

### Performance Benchmarking

Track execution time trends:

```bash
# Run full matrix and time it
time make test-matrix > results_$(date +%Y%m%d).log

# Compare across runs
grep "Duration:" results_*.log
```

## Cost Estimation

### Token Usage

Approximate token usage per example:
- **Simple** (predict, sentiment): 500-2,000 tokens
- **Medium** (CoT, refine): 2,000-8,000 tokens  
- **Complex** (ReAct, research): 8,000-30,000 tokens

**Total per run:**
- Single model: ~150,000-300,000 tokens
- 3 models: ~450,000-900,000 tokens
- All models (14): ~2,100,000-4,200,000 tokens

### Cost Estimates

Based on OpenRouter pricing (varies by model):

| Test Type | Models | Total Tokens | Est. Cost |
|-----------|--------|--------------|-----------|
| Quick | 1 | ~200K | $0.02-0.20 |
| Sample (N=3) | 3 | ~600K | $0.06-0.60 |
| Sample (N=5) | 5 | ~1M | $0.10-1.00 |
| Full Matrix | 14 | ~3M | $0.30-3.00 |

**Note:** Actual costs depend on:
- Model pricing (flash < pro < opus)
- Example complexity
- Retry attempts
- Cache hits

## Best Practices

### Development Workflow

1. **During development**: `make test-matrix-quick` after each change
2. **Before PR**: `make test-matrix-sample N=3` for broader validation
3. **Before release**: `make test-matrix` for full coverage

### CI/CD Strategy

- **PR validation**: Quick test (1 model)
- **Main branch**: Sample test (3 models)
- **Release tags**: Full matrix (all models)
- **Nightly**: Full matrix + detailed analysis

### Debugging Failed Tests

1. Check test log: `cat test_matrix_logs/failed/model_example_*.log`
2. Run example manually: `cd examples/NNN_name && go run main.go`
3. Test with different model: `OPENROUTER_MODEL=other-model go run main.go`
4. Enable verbose mode: `go run main.go -verbose`
5. Check harness output: `go run main.go -format=json`

### Maintaining Compatibility Matrix

- Update `incompatibleCombos` when you discover issues
- Document reasons (missing features, provider bugs)
- Re-test incompatible pairs after provider updates
- Remove entries when issues are resolved

## See Also

- [MIGRATION_PLAN.md](MIGRATION_PLAN.md) - Examples reorganization history
- [QUICKSTART.md](../QUICKSTART.md) - Getting started with DSGo
- [shared/_harness/](shared/_harness/) - Harness infrastructure code
- [test_matrix/main.go](test_matrix/main.go) - Test matrix tool source

## Future Enhancements

**Phase 5** (planned - see MIGRATION_PLAN.md):
- [ ] HTML test report generation
- [ ] JSON result export for analysis
- [ ] Cost tracking per model/example
- [ ] Performance regression detection
- [ ] Example-specific timeout configuration
- [ ] Parallel model testing (test same example across models concurrently)
- [ ] Better failure categorization (timeout vs API error vs logic error)
- [ ] Historical trend analysis
