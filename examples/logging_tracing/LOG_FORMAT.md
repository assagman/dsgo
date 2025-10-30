# Log Format Guide

## Format Structure

```
[Prefix] Timestamp [Level] [RequestID] Message | key1=value1 key2=value2
```

### Components

| Component | Example | Description |
|-----------|---------|-------------|
| **Prefix** | `[DSGo]` | Framework identifier |
| **Timestamp** | `2025-10-31 00:21:08.557` | ISO 8601 format with milliseconds |
| **Level** | `[INFO]` | Log level: DEBUG, INFO, WARN, ERROR |
| **RequestID** | `[5834ee8446b98b3a]` | 16-character hex string for request tracing |
| **Message** | `API request started` | Human-readable log message |
| **Fields** | `model=gpt-4 prompt_length=100` | Structured key-value pairs |

## Example Logs

### Single API Call

```
[DSGo] 2025-10-31 00:21:08.557 [INFO] [5834ee8446b98b3a] API request started | model=deepseek/deepseek-v3.1-terminus:exacto prompt_length=367
[DSGo] 2025-10-31 00:21:11.733 [INFO] [5834ee8446b98b3a] API request completed | model=deepseek/deepseek-v3.1-terminus:exacto status_code=200 duration_ms=3175 prompt_tokens=87 completion_tokens=7 total_tokens=94
```

**Notice:** Same Request ID (`5834ee8446b98b3a`) for the entire request/response cycle.

### Multiple Calls with Same Request ID

```
[DSGo] 2025-10-31 00:21:12.568 [INFO] [batch-job-001] API request started | model=deepseek/deepseek-v3.1-terminus:exacto prompt_length=324
[DSGo] 2025-10-31 00:21:13.566 [INFO] [batch-job-001] API request completed | model=deepseek/deepseek-v3.1-terminus:exacto status_code=200 duration_ms=998 prompt_tokens=78 completion_tokens=7 total_tokens=85
[DSGo] 2025-10-31 00:21:13.566 [INFO] [batch-job-001] API request started | model=deepseek/deepseek-v3.1-terminus:exacto prompt_length=329
[DSGo] 2025-10-31 00:21:14.184 [INFO] [batch-job-001] API request completed | duration_ms=617 prompt_tokens=78 completion_tokens=7 total_tokens=85 model=deepseek/deepseek-v3.1-terminus:exacto status_code=200
```

**Notice:** All logs share the custom Request ID (`batch-job-001`) for easy correlation.

### DEBUG Level Logs

```
[DSGo] 2025-10-31 00:16:41.126 [DEBUG] [debug-example] Prediction started | module=Predict signature=Simple task
[DSGo] 2025-10-31 00:16:41.126 [INFO] [debug-example] API request started | model=deepseek/deepseek-v3.1-terminus:exacto prompt_length=276
[DSGo] 2025-10-31 00:16:42.641 [INFO] [debug-example] API request completed | prompt_tokens=67 completion_tokens=13 total_tokens=80 model=deepseek/deepseek-v3.1-terminus:exacto status_code=200 duration_ms=1515
[DSGo] 2025-10-31 00:16:42.641 [DEBUG] [debug-example] Prediction completed | module=Predict duration_ms=1515
```

**Notice:** DEBUG level shows the full prediction lifecycle (start → API call → completion).

## Request ID Types

### Auto-Generated (16-char hex)
```
[5834ee8446b98b3a]  ← Automatically created by DSGo
[cdc1e21eef392ce6]  ← Each prediction gets a unique ID
```

### Custom (any string)
```
[user-request-12345]  ← Set by your application
[batch-job-001]       ← Useful for batch processing
[session-abc-def]     ← For session tracking
```

## Structured Fields

### API Request Started
- `model` - LM model name
- `prompt_length` - Character count of the prompt

### API Request Completed
- `model` - LM model name
- `status_code` - HTTP status code (200, 429, etc.)
- `duration_ms` - Request duration in milliseconds
- `prompt_tokens` - Input token count
- `completion_tokens` - Output token count
- `total_tokens` - Sum of prompt + completion tokens

### Prediction Flow (DEBUG only)
- `module` - Module name (Predict, ChainOfThought, etc.)
- `signature` - Signature description
- `duration_ms` - Total prediction duration

### Error Logs
- `model` - LM model name
- `error` - Error message

## Parsing Tips

### Extract Request ID
```bash
grep "API request" log.txt | grep -o '\[[a-f0-9]\{16\}\]'
```

### Filter by Request ID
```bash
grep "[batch-job-001]" log.txt
```

### Calculate Average Duration
```bash
grep "API request completed" log.txt | grep -o 'duration_ms=[0-9]*' | cut -d= -f2 | awk '{sum+=$1; count+=1} END {print sum/count}'
```

### Sum Token Usage
```bash
grep "total_tokens" log.txt | grep -o 'total_tokens=[0-9]*' | cut -d= -f2 | awk '{sum+=$1} END {print sum}'
```

## Best Practices

1. **Use Custom Request IDs for:**
   - Batch jobs
   - User sessions
   - Multi-step workflows
   - Distributed tracing

2. **Use Auto-Generated IDs for:**
   - Single API calls
   - Quick scripts
   - Testing

3. **Log Level Guidelines:**
   - **DEBUG**: Development and troubleshooting
   - **INFO**: Production monitoring
   - **WARN**: Potential issues (high latency, rate limits)
   - **ERROR**: Failures and exceptions

4. **Filtering in Production:**
   ```go
   // Start with INFO, increase to DEBUG if needed
   logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))
   ```

## Integration with Log Aggregators

The structured format works well with tools like:
- **Splunk**: Parse fields after `|` character
- **Elasticsearch**: Use Logstash to parse key-value pairs
- **Datadog**: Extract Request ID for trace correlation
- **CloudWatch**: Create metric filters on `duration_ms`, `total_tokens`

Example Logstash grok pattern:
```
\[DSGo\] %{TIMESTAMP_ISO8601:timestamp} \[%{LOGLEVEL:level}\] \[%{DATA:request_id}\] %{DATA:message}( \| %{GREEDYDATA:fields})?
```
