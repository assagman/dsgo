# REFERENCE

Tables and quick lookups for DSGo.

## Environment variables

### Provider keys

| Variable | Purpose |
|---|---|
| `DSGO_OPENAI_API_KEY` | OpenAI API key (preferred) |
| `OPENAI_API_KEY` | OpenAI API key (fallback) |
| `DSGO_OPENROUTER_API_KEY` | OpenRouter API key (preferred) |
| `OPENROUTER_API_KEY` | OpenRouter API key (fallback) |
| `OPENROUTER_SITE_NAME` | Optional OpenRouter app name header |
| `OPENROUTER_SITE_URL` | Optional OpenRouter app URL header |
| `DSGO_MOCK_BASE_URL` | Mock provider base URL (tests/dev) |
| `DSGO_MOCK_API_KEY` | Mock provider API key (optional; default "test") |

### Runtime (core defaults)

| Variable | Purpose |
|---|---|
| `DSGO_TIMEOUT` | Request timeout in seconds (e.g. `30`) |
| `DSGO_MAX_RETRIES` | Max retry attempts |
| `DSGO_TRACING` | Enable tracing (`true`/`false`) |
| `DSGO_MAX_TOKENS` | Default max tokens (GenerateOptions) |
| `DSGO_TEMPERATURE` | Default temperature (GenerateOptions) |
| `DSGO_HTTP_TIMEOUT_MS` | Provider HTTP timeout in ms |

### Cache

| Variable | Purpose |
|---|---|
| `DSGO_CACHE` | Enable/disable caching (`true`/`false`) |
| `DSGO_CACHE_TTL` | Cache TTL (e.g. `5m`, `1h`) |
| `DSGO_CACHE_MEMORY` | Memory cache capacity |
| `DSGO_CACHE_DISK` | Enable/disable disk cache |
| `DSGO_CACHEDIR` | Disk cache directory |
| `DSGO_CACHE_LIMIT` | Disk cache size limit (bytes) |

### Structured Outputs

| Variable | Purpose | Default |
|---|---|---|
| `DSGO_STRUCTURED_OUTPUTS` | Enable structured output enforcement (`true`/`false`) | `true` |
| `DSGO_STRUCTURED_MAX_ATTEMPTS` | Max validation retry attempts | `3` |
| `DSGO_STRUCTURED_TEMPERATURE` | Temperature override for structured mode | `0.0` |

### Logging

| Variable | Purpose |
|---|---|
| `DSGO_LOG_LEVEL` | Logging level (`debug|info|warn|error|fatal`) |
| `DSGO_LOG_FORMAT` | `text` or `json` |
| `DSGO_LOG_COLOR` | `auto|always|never` (text format) |
| `DSGO_LOG_MODULE_LEVELS` | Per-module overrides (`module.Predict=debug,...`) |
| `DSGO_LOG_BUFFER_SIZE` | Async buffer size |
| `DSGO_LOG_FLUSH_INTERVAL` | Flush interval (e.g. `200ms`) |
| `DSGO_LOG_FLUSH_TIMEOUT` | Flush timeout (e.g. `1s`) |
| `DSGO_LOG_BATCH_SIZE` | Max batch size per flush |
| `DSGO_LOG_DROP_WHEN_FULL` | Drop logs when buffer full (`1`/`true`) |
| `DSGO_LOG_MAX_MEMORY` | Max async buffer memory (bytes) |
| `DSGO_LOG_CACHE_SLOW_THRESHOLD` | Cache slow-log threshold |
| `DSGO_LOG_TOOL_SLOW_THRESHOLD` | Tool slow-log threshold |
| `DSGO_LOG_BLOCK_TIMEOUT` | Max block time when buffer full |

### Debugging

| Variable | Purpose |
|---|---|
| `DSGO_DEBUG_PARSE` | Show parse attempts (`1`/`true`) |
| `DSGO_DEBUG_MARKERS` | Show field markers during streaming |
| `DSGO_REACT_MAX_PROMPT_BYTES` | Override ReAct prompt budget (bytes) |

### .env loading

| Variable | Purpose |
|---|---|
| `DSGO_ENV_FILE_PATH` | Explicit path to .env file (skips auto search) |

### Legacy example overrides

| Variable | Purpose |
|---|---|
| `EXAMPLES_MAX_TOKENS` | Default max tokens for examples |
| `EXAMPLES_TEMPERATURE` | Default temperature for examples |

> Source-of-truth: see package docs; internal helpers live under `internal/*`.

## Modules (high level)

### Parallel verbose logging fields

Enable:

- Code: `module.NewParallel(...).WithVerbose(true)`
- Logging output: set up the DSGo logger (e.g., via `logging.ConfigureLoggerFromEnv()` or `DSGO_LOG_LEVEL=info`).

Parallel logs are emitted with `module=module.Parallel`. When verbose is enabled, per-task logs are emitted at INFO (otherwise DEBUG).

#### Correlation

- `parallel_id`: batch identifier (explicit field; equals the request `correlation_id`)
- `correlation_id`: task identifier on per-task logs and all downstream logs (inner modules + providers). Format:
  - `correlation_id = parallel_id + "/task/" + task_index`

This allows filtering either:

- Whole batch: `parallel_id=<id>`
- Single task: `correlation_id=<id>/task/<n>`

#### Common fields (all Parallel verbose logs)

- `parallel_id`
- `parallel_mode`: `clone|factory|instances`
- `inner_module`: best-effort module type name
- `lm_model`: best-effort LM model name
- `batch_size`
- `max_workers`
- `fail_fast`, `max_failures`, `return_all`, `only_success`, `repeat_factor`, `batch_key`, `verbose`

#### Task fields (task started/completed/failed)

- `task_index` (0-based)
- `task_total` (equals `batch_size`)
- `worker_id` (0..max_workers-1)
- `queue_wait_ms` (time spent waiting in the work queue)
- `inputs`: safe summarized inputs (strings truncated; complex types summarized as `<T>`)
- `inputs_truncated`: `true` if any input string was truncated

#### Completion fields (task completed)

- `duration_ms`
- Usage: `prompt_tokens`, `completion_tokens`, `total_tokens`, `cost`
- Parsing: `adapter_used`, `parse_attempts`, `fallback_used`, `parse_success`
- Optional bounded parse diagnostics (no raw content):
  - `missing_required_fields` (first N)
  - `missing_required_fields_count`
  - `invalid_fields_count`

#### Failure fields (task failed)

- `duration_ms`
- `error.message`
- `error.kind`: `context_canceled|deadline_exceeded|task_error`

#### Batch summary fields (batch completed)

- Outcome: `successes`, `failures`
- Latency summary: `latency_min_ms`, `latency_max_ms`, `latency_avg_ms`, `latency_p50_ms`
- Aggregated usage: `prompt_tokens`, `completion_tokens`, `total_tokens`, `cost`
- Errors: `error_count`, `error_sample` (bounded)

| Module | Constructor | Notes |
|---|---|---|
| Predict | `module.NewPredict(sig, lm)` | Structured prediction |
| ChainOfThought | `module.NewChainOfThought(sig, lm)` | Stores reasoning in `Prediction.Rationale` (and/or explicit reasoning fields) |
| ReAct | `module.NewReAct(sig, lm, tools)` | Tool-using agent loop (native tool calls + auto `finish` tool + extractor fallback) |
| Refine | `module.NewRefine(sig, lm)` | Refines only when `inputs["feedback"]` (or custom refinement field) is provided |
| BestOfN | `module.NewBestOfN(module, n)` | Requires `WithScorer(...)`; can parallelize and optionally return all completions |
| Program | `module.NewProgram(name)` | Sequential composition |
| Parallel | `module.NewParallel(module)` | Concurrent execution; per-item outputs are in `Prediction.Completions` |
| ProgramOfThought | `module.NewProgramOfThought(sig, lm, language)` | Generates code-first solutions; execution is disabled by default |
| MultiChainComparison | `module.NewMultiChainComparison(sig, lm, m)` | Synthesizes from `inputs["completions"]` and prepends `rationale` output |

### ReAct: key knobs

- **Trajectory limit (per run)**: `WithMaxIterations(n)` (default: 10)
- **Trajectory limit (multi-turn)**: `WithHistory(core.NewHistoryWithLimit(n))` to bound remembered messages
- **Prompted finishing**: ReAct guides the model to terminate by calling a synthetic `finish` tool whose arguments match your output `Signature`.
- **Extractor / schema enforcement**: configure globally via:

```go
if err := core.Configure(
  core.WithStructuredOutputEnabled(true),
  core.WithStructuredOutputMaxAttempts(3),
  core.WithStructuredOutputTemperature(0.0),
); err != nil {
  log.Fatal(err)
}
```

- **Tool result size control**: design tool schemas with explicit bounds like `max_chars`, `max_results`, and enforce them in the tool implementation.

## Adapters

| Adapter | Description |
|---|---|
| ChatAdapter | Marker-based format `[[ ## field ## ]]` |
| JSONAdapter | JSON output with schema + repair |
| TwoStepAdapter | Two-step reasoning then extraction |
| FallbackAdapter | Chains adapters for robustness |

## GenerateOptions (Advanced)

### ProviderParams

Provider-specific parameters can be passed through to LM providers using `ProviderParams`:

```go
options := core.DefaultGenerateOptions()
options.ProviderParams = map[string]any{
    "reasoning": map[string]any{
        "effort": "high", // OpenRouter reasoning effort
    },
    "custom_param": "value", // Provider-specific field
}
```

**Supported values:**
- **OpenRouter**: `"reasoning": {"effort": "high|medium|low|minimal|none"}`
- **OpenAI**: Provider-specific fields (merged with request)

**Safety rules:**
- DSGo-managed keys (`temperature`, `max_tokens`, etc.) take precedence
- ProviderParams are merged only for non-conflicting keys
- Nested objects are supported

## MCP (Model Context Protocol) Clients

DSGo supports MCP clients for accessing external tools and services:

| Client | Factory Function | Transport | Description |
|--------|------------------|-----------|-------------|
| Exa | `mcp.NewExaClient(apiKey)` | HTTP | Web search and content extraction |
| Jina | `mcp.NewJinaClient(apiKey)` | SSE | URL reading and content extraction |
| Tavily | `mcp.NewTavilyClient(apiKey)` | HTTP | Web search and content extraction |
| Filesystem | `mcp.NewFilesystemClient(dir)` | Stdio | Local filesystem operations via official MCP server |

### Filesystem MCP Client

Uses the official `@modelcontextprotocol/server-filesystem` via npx/bunx:

```go
// Create filesystem client with specific directory
fsClient, err := mcp.NewFilesystemClient("/path/to/directory")

// Or use current directory (default)
fsClient, err := mcp.NewFilesystemClient()

// Initialize and get tools
err = fsClient.Initialize(ctx)
tools := fsClient.GetTools()

// Use with ReAct agent
agent := module.NewReAct(sig, lm, tools)
```

**Available Tools**: `read_file`, `write_file`, `edit_file`, `create_directory`, `list_directory`, `directory_tree`, `move_file`, `search_files`, `get_file_info`, `list_allowed_directories`

**Prerequisites**: Node.js with `npx` or Bun with `bunx`

## Links

- Getting started: [`QUICKSTART.md`](QUICKSTART.md)
- Architecture: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Development: [`DEVELOPMENT.md`](DEVELOPMENT.md)
- Examples: not included in this layout
- Core internals: [`core/README.md`](core/README.md)
- Module internals: [`module/README.md`](module/README.md)
