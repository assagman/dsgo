# Error Handling & Analysis

## Overview

The test matrix includes sophisticated error handling to make debugging failures fast and efficient. Every failure is automatically analyzed, categorized, and documented.

## Error Collection

### Automatic Categorization

Every failed test is automatically categorized into one of 12 error types:

```
PANIC              → Go runtime panic/crash
TIMEOUT            → Test exceeded time limit
RATE_LIMIT         → HTTP 429 (too many requests)
SERVER_ERROR       → HTTP 500/503 (provider issues)
CONNECTION_ERROR   → Network connection failures
UNSUPPORTED_FEATURE → Missing model capabilities
JSON_PARSE_ERROR   → Invalid JSON responses
VALIDATION_ERROR   → Output schema violations
AUTH_ERROR         → API key/authentication issues
FORBIDDEN          → HTTP 403 (access denied)
CANCELLED          → Circuit breaker cancellation
UNKNOWN            → Unclassified errors
```

### Error Details Extraction

For each failure, the system extracts:

1. **Error Type** - Categorized error category
2. **Error Detail** - The most relevant error message
3. **Stack Trace** - Full panic stack trace (if applicable)
4. **Full Output** - Complete console output
5. **Exit Code** - Process exit code
6. **Duration** - How long the test ran before failing

## Log Files

### Individual Test Logs

Each failed test gets a detailed log file:

```
test_matrix_logs/failed/minimax-m2_006_program_of_thought_20250105_143045.log
```

**Format:**
```
================================================================================
 TEST RESULT: 006_program_of_thought
================================================================================

Example:     006_program_of_thought
Model:       openrouter/minimax/minimax-m2
Success:     false
Duration:    3.45s
Exit Code:   1

--- ERROR ANALYSIS ---

Error Type:  UNSUPPORTED_FEATURE
Error:       exit status 1
Detail:      Error: json_schema not supported by this provider

--- STACK TRACE ---

(empty if no panic)

--- FULL OUTPUT ---

(complete console output here)

================================================================================
 END OF LOG
================================================================================
```

### ERROR_SUMMARY.md

After each test run with failures, an `ERROR_SUMMARY.md` is automatically generated:

```markdown
# Test Matrix Error Summary

Generated: 2025-01-05 14:30:45

Total Failures: 17

## Error Type Breakdown

- **UNSUPPORTED_FEATURE**: 8 failures
- **TIMEOUT**: 5 failures
- **SERVER_ERROR**: 3 failures
- **UNKNOWN**: 1 failure

## Failures by Model

### minimax-m2 (5 failures)

- **006_program_of_thought** (UNSUPPORTED_FEATURE): json_schema not supported
- **027_research_assistant** (UNSUPPORTED_FEATURE): json_schema not supported
- ...

### deepseek-v3.1-terminus (4 failures)

- **003_react** (TIMEOUT): deadline exceeded
- **006_program_of_thought** (SERVER_ERROR): status 503
- ...

## Detailed Failure List

### 1. 006_program_of_thought [minimax-m2]

- **Error Type**: UNSUPPORTED_FEATURE
- **Error**: Error: json_schema not supported by this provider
- **Duration**: 3.45s
- **Exit Code**: 1
- **Log**: `test_matrix_logs/failed/minimax-m2_006_program_of_thought_...log`

---

### 2. 003_react [deepseek-v3.1-terminus]

- **Error Type**: TIMEOUT
- **Error**: context deadline exceeded
- **Duration**: 600.02s
- **Exit Code**: -1
- **Log**: `test_matrix_logs/failed/deepseek-v3.1-terminus_003_react_...log`

**Stack Trace:**
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]

goroutine 1 [running]:
main.runExample(...)
    /path/to/main.go:45
...
```

---
```

## Terminal Output

Failures are clearly marked in the terminal output with error types:

```
--------------------------------------------------------------------------------
 FAILED TESTS 
--------------------------------------------------------------------------------

1. 006_program_of_thought [minimax-m2]
   Type: UNSUPPORTED_FEATURE
   Error: Error: json_schema not supported by this provider
   Duration: 3.45s
   Exit Code: 1
   Log: test_matrix_logs/failed/minimax-m2_006_program_of_thought_20250105_143045.log

2. 003_react [deepseek-v3.1-terminus]
   Type: TIMEOUT
   Error: context deadline exceeded
   Duration: 600.02s
   Exit Code: -1
   Log: test_matrix_logs/failed/deepseek-v3.1-terminus_003_react_20250105_143050.log
```

## Debugging Workflow

### 1. Check Terminal Output

Look at the failure summary in the terminal:
- **Error Type** tells you the failure category
- **Error** gives you the key message
- **Log path** shows where to find details

### 2. Open ERROR_SUMMARY.md

Quick overview of all failures:
```bash
cat test_matrix_logs/ERROR_SUMMARY.md
```

See patterns:
- Which error types are most common?
- Which models are failing the most?
- Are there systematic issues?

### 3. Dive into Individual Logs

For detailed debugging:
```bash
cat test_matrix_logs/failed/model_example_timestamp.log
```

Each log has:
- Full console output
- Extracted error details
- Stack traces (if panic)
- Timing information

### 4. Reproduce Locally

Run the failing example directly:
```bash
export EXAMPLES_DEFAULT_MODEL="problematic-model"
cd examples/006_program_of_thought
go run main.go
```

## Error Type Patterns

### UNSUPPORTED_FEATURE

**Cause:** Model/provider lacks a required capability

**Examples:**
- json_schema not supported
- function calling not available
- structured output not supported

**Fix:**
- Use a different model
- Fall back to simpler approach
- Add adapter/workaround

### TIMEOUT

**Cause:** Test exceeded time limit (default: 10 minutes)

**Examples:**
- Slow model responses
- Network latency
- Infinite loops

**Fix:**
- Increase timeout: `-timeout=20m`
- Check for logic errors
- Optimize prompt complexity

### RATE_LIMIT

**Cause:** Too many API requests

**Examples:**
- HTTP 429 errors
- "rate limit exceeded"
- "key limit exceeded"

**Fix:**
- Reduce concurrency: `-c 5`
- Use sequential execution: `-p=false`
- Wait and retry
- Upgrade API tier

### SERVER_ERROR

**Cause:** Provider backend issues

**Examples:**
- HTTP 500/503 errors
- "internal server error"
- "service unavailable"

**Fix:**
- Wait and retry
- Check provider status page
- Report to provider if persistent

### PANIC

**Cause:** Go runtime crash

**Examples:**
- nil pointer dereference
- index out of bounds
- type assertion failure

**Fix:**
- Check stack trace for line numbers
- Fix nil checks
- Add defensive programming

## Best Practices

### 1. Always Check ERROR_SUMMARY.md

After failed runs:
```bash
cat test_matrix_logs/ERROR_SUMMARY.md
```

Gives you instant insight into failure patterns.

### 2. Group Similar Failures

If you see the same error type across many examples:
- It's likely a systematic issue
- Fix once, resolve many
- Check for common patterns

### 3. Test Fixes Individually

After fixing an issue:
```bash
export EXAMPLES_DEFAULT_MODEL="fixed-model"
go run examples/006_program_of_thought/main.go
```

Verify the fix before running full matrix.

### 4. Keep Logs for Analysis

The `test_matrix_logs/` directory builds history:
- Track improvements over time
- Compare different models
- Identify regressions

### 5. Circuit Breaker is Your Friend

If circuit breaker trips:
- Review ERROR_SUMMARY.md immediately
- Fix systematic issues first
- Then re-run full matrix

## Example: Debugging a PANIC

### Step 1: See Terminal Output

```
1. 027_research_assistant [anthropic/claude-haiku-4.5]
   Type: PANIC
   Error: panic: runtime error: invalid memory address
   Duration: 2.13s
   Exit Code: 2
   Log: test_matrix_logs/failed/claude-haiku-4.5_027_research_assistant_...log
```

### Step 2: Open Log File

```bash
cat test_matrix_logs/failed/claude-haiku-4.5_027_research_assistant_*.log
```

See full stack trace:
```
--- STACK TRACE ---

panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x10abcd0]

goroutine 1 [running]:
main.runExample(0x14000102000)
    /path/to/examples/027_research_assistant/main.go:45 +0x120
main.main()
    /path/to/examples/027_research_assistant/main.go:22 +0x58
```

### Step 3: Go to Line

Open `examples/027_research_assistant/main.go:45`

Fix the nil pointer issue.

### Step 4: Test Fix

```bash
cd examples/027_research_assistant
go run main.go
```

### Step 5: Re-run Matrix

```bash
make test-matrix-sample N=3
```

Verify fix across multiple models.

## Advanced Analysis

### Trend Analysis

Compare ERROR_SUMMARY.md across test runs:

```bash
# Save summaries with timestamps
cp test_matrix_logs/ERROR_SUMMARY.md \
   error_summaries/summary_$(date +%Y%m%d_%H%M%S).md

# Compare
diff error_summaries/summary_20250105_140000.md \
     error_summaries/summary_20250105_150000.md
```

### Model Comparison

Which models are most reliable?

```bash
grep -A 20 "## Failures by Model" test_matrix_logs/ERROR_SUMMARY.md
```

### Error Type Distribution

What are the most common failure modes?

```bash
grep -A 15 "## Error Type Breakdown" test_matrix_logs/ERROR_SUMMARY.md
```

## Summary

The test matrix error handling provides:

✅ **Automatic categorization** - Know what failed and why  
✅ **Detailed logs** - Every failure fully documented  
✅ **Stack traces** - Panic debugging made easy  
✅ **Error summary** - Quick overview of all failures  
✅ **Actionable insights** - Clear next steps for debugging  

No more hunting through massive log files. Every error is collected, categorized, and ready for analysis.
