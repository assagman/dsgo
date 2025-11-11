# Test Matrix - Multi-Model Example Validation

## Overview

The Test Matrix is a **first-class tool** in the DSGo examples directory that validates all numbered examples (001-028) across multiple models. It features improved logging, colored output, and intelligent failure detection.

## Quick Start

```bash
# From project root
make test-matrix-quick          # Single model (2-3 min)
make test-matrix-sample N=3     # 3 random models (5-10 min)
make test-matrix                # All models (30-60 min)
```

## Features

✅ **Beautiful Output** - Colored, structured test results with progress tracking  
✅ **Smart Logging** - Detailed logs saved to `test_matrix_logs/` automatically  
✅ **Error Categorization** - Automatic classification of failures (PANIC, TIMEOUT, etc.)  
✅ **Error Summary Report** - Comprehensive `ERROR_SUMMARY.md` with breakdown by type/model  
✅ **Stack Trace Extraction** - Automatic capture and storage of panic stack traces  
✅ **Circuit Breaker** - Early termination on systematic failures (85% success threshold)  
✅ **Parallel Execution** - Concurrent testing with configurable limits (default: 20)  
✅ **Rate Limit Handling** - Automatically skips rate-limited tests  
✅ **Model Scoring** - Ranked results showing best-performing models  

## Usage

### Basic Commands

```bash
# Default: single model
go run examples/test_matrix/main.go

# 5 random models
go run examples/test_matrix/main.go -n 5

# All models (comprehensive)
go run examples/test_matrix/main.go -n 0
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-n` | 1 | Number of models: 1=default, N=random, 0=all |
| `-v` | false | Verbose output (show each test) |
| `-timeout` | 10m | Timeout per example |
| `-p` | true | Run tests in parallel |
| `-c` | 20 | Max concurrent executions |
| `-no-color` | false | Disable colored output (for CI) |

### Examples

```bash
# Verbose output for debugging
go run examples/test_matrix/main.go -v

# Increase timeout for slow connections
go run examples/test_matrix/main.go -timeout=20m

# Reduce concurrency to avoid rate limits
go run examples/test_matrix/main.go -c 5

# Sequential execution (no parallelism)
go run examples/test_matrix/main.go -p=false

# CI-friendly (no colors)
go run examples/test_matrix/main.go -no-color
```

## Output Examples

### Success

```
================================================================================
 TESTING DEFAULT MODEL (1) 
================================================================================

  Models:         gemini-2.5-flash
  Examples:       28
  Total Tests:    28
  Parallel:       true
  Max Concurrent: 20
  Timeout:        10m0s

[  28/28] ✅ PASS 020_streaming            gemini-2.5-flash 2.34s

================================================================================
 TEST RESULTS 
================================================================================

  Total Tests:    28
  ✅ Passed:      28
  ⏱️  Duration:    2m15s

--------------------------------------------------------------------------------
 MODEL SCORES 
--------------------------------------------------------------------------------

   1. gemini-2.5-flash                                28/28 (100.0%) ✅

================================================================================
```

### Failures

```
--------------------------------------------------------------------------------
 FAILED TESTS 
--------------------------------------------------------------------------------

1. 006_program_of_thought [minimax-m2]
   Error: json_schema not supported by provider
   Duration: 3.45s
   Exit Code: 1
   Log: test_matrix_logs/failed/minimax-m2_006_program_of_thought_20250105_143045.log

2. 003_react [deepseek-v3.1-terminus]
   Error: provider connection timeout
   Duration: 10.02s
   Exit Code: 1
   Log: test_matrix_logs/failed/deepseek-v3.1-terminus_003_react_20250105_143055.log
```

### Circuit Breaker

```
🚨 CIRCUIT BREAKER TRIPPED 🚨
Reason: 16.1% failure rate exceeds 15.0% threshold (45/280 failed)
```

## Environment Variables

### EXAMPLES_DEFAULT_MODEL

Override the default model used for testing:

```bash
export EXAMPLES_DEFAULT_MODEL="openrouter/google/gemini-2.5-flash"
go run examples/test_matrix/main.go
```

### OPENROUTER_API_KEY

Required for testing:

```bash
export OPENROUTER_API_KEY="your-api-key"
go run examples/test_matrix/main.go
```

## Supported Models

The test matrix includes 12 models spanning different performance tiers:

### Tier 1 (High Performance)
- `openrouter/google/gemini-2.5-flash` ⭐ (default)
- `openrouter/google/gemini-2.5-pro`
- `openrouter/qwen/qwen3-235b-a22b-2507`

### Tier 2 (Good Performance)
- `openrouter/z-ai/glm-4.6:exacto`
- `openrouter/minimax/minimax-m2`
- `openrouter/qwen/qwen3-30b-a3b`
- `openrouter/google/gemini-2.0-flash-lite-001`

### Tier 3 (Experimental)
- `openrouter/moonshotai/kimi-k2-0905:exacto`
- `openrouter/deepseek/deepseek-v3.1-terminus:exacto`
- `openrouter/openai/gpt-oss-120b:exacto`
- `openrouter/meta-llama/llama-3.1-8b-instruct`

## Architecture

```
Test Matrix
    │
    ├── Model Selection
    │   ├── Single (default)
    │   ├── Random N
    │   └── All
    │
    ├── Circuit Breaker
    │   └── 85% success threshold
    │
    ├── Parallel Executor
    │   ├── Semaphore (max 20 concurrent)
    │   └── Progress tracking
    │
    ├── Test Runner
    │   ├── go run examples/NNN_name/main.go
    │   ├── EXAMPLES_DEFAULT_MODEL=model
    │   └── 10 minute timeout
    │
    └── Results
        ├── Model scores (ranked)
        ├── Failure analysis
        └── Detailed logs
```

## Log Files

Test results are automatically saved to `test_matrix_logs/`:

```
test_matrix_logs/
├── ERROR_SUMMARY.md          # ⭐ Comprehensive error analysis (generated on failures)
├── passed/
│   ├── gemini-2.5-flash_001_predict_20250105_143022.log
│   ├── gemini-2.5-flash_002_chain_of_thought_20250105_143025.log
│   └── ...
└── failed/
    ├── minimax-m2_006_program_of_thought_20250105_143045.log
    └── ...
```

### Individual Test Logs

Each log contains:
- Example name and model
- Success status and duration
- Exit code and error categorization
- **Error analysis** (type, detail, stack trace)
- Complete output

### ERROR_SUMMARY.md

Generated automatically when failures occur:
- **Error type breakdown** - Count of each error category
- **Failures by model** - Which models had which failures
- **Detailed failure list** - Every failure with full context
- **Stack traces** - Included inline for easy debugging

## Adding Models

Edit `main.go` to add new models to the test matrix:

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

## Error Types

The test matrix automatically categorizes failures into these types:

| Error Type | Description | Common Causes |
|------------|-------------|---------------|
| **PANIC** | Go panic/crash | nil pointer, out of bounds, assertion failure |
| **TIMEOUT** | Test exceeded time limit | Slow model, network issues, infinite loop |
| **RATE_LIMIT** | HTTP 429 response | Too many requests, quota exceeded |
| **SERVER_ERROR** | HTTP 500/503 | Provider outage, backend issues |
| **CONNECTION_ERROR** | Network failure | Connection refused/reset, DNS failure |
| **UNSUPPORTED_FEATURE** | Missing capability | json_schema, function calling not supported |
| **JSON_PARSE_ERROR** | Invalid JSON | Malformed response, parsing failure |
| **VALIDATION_ERROR** | Output validation failed | Missing fields, wrong types |
| **AUTH_ERROR** | Authentication failed | Invalid API key, expired token |
| **FORBIDDEN** | HTTP 403 | Access denied, disabled endpoint |
| **CANCELLED** | Circuit breaker tripped | Systematic failures detected |
| **UNKNOWN** | Unclassified error | Novel error type |

Use the ERROR_SUMMARY.md to see which error types are most common in your test runs.

## Troubleshooting

### All Tests Fail

**Issue:** Every test fails with API errors

**Solution:**
```bash
# Check API key
echo $OPENROUTER_API_KEY

# Set if missing
export OPENROUTER_API_KEY="your-key"
```

### Timeouts

**Issue:** Tests timeout frequently

**Solution:**
```bash
# Increase timeout
go run examples/test_matrix/main.go -timeout=20m
```

### Rate Limits

**Issue:** HTTP 429 errors

**Solution:**
```bash
# Reduce concurrency
go run examples/test_matrix/main.go -c 5

# Or run sequentially
go run examples/test_matrix/main.go -p=false
```

### Circuit Breaker Trips

**Issue:** Tests stop early

**Solution:**
1. Check failed test logs for patterns
2. Fix underlying issues
3. Re-run after fixes

## Integration

### Makefile

The test matrix is integrated into the project Makefile:

```makefile
test-matrix-quick:
	@go run examples/test_matrix/main.go -n 1

test-matrix-sample:
	@go run examples/test_matrix/main.go -n $(N)

test-matrix:
	@go run examples/test_matrix/main.go -n 0
```

### CI/CD

Example GitHub Actions integration:

```yaml
- name: Test Examples
  env:
    OPENROUTER_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
  run: |
    go run examples/test_matrix/main.go -n 3 -no-color
```

## See Also

- [TESTING.md](../TESTING.md) - Comprehensive testing guide
- [TESTING_QUICK_REFERENCE.md](../TESTING_QUICK_REFERENCE.md) - Quick reference card
- [MIGRATION_PLAN.md](../MIGRATION_PLAN.md) - Examples reorganization

## Design Philosophy

The test matrix tool embodies these principles:

1. **First-class tool** - Lives in `examples/` as a peer to tested examples
2. **Beautiful output** - Colored, structured, easy to scan
3. **Smart defaults** - Works out of the box, customizable when needed
4. **Fast feedback** - Parallel execution with progress tracking
5. **Production-ready** - Circuit breaker, logging, error handling
6. **CI-friendly** - Non-interactive, clean exit codes, optional colors
