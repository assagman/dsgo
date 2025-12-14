# REFERENCE

Tables and quick lookups for DSGo.

## Environment variables

### Provider keys

| Variable | Purpose |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |

### Runtime

| Variable | Purpose |
|---|---|
| `DSGO_TIMEOUT` | Request timeout (e.g. `30s`) |
| `DSGO_MAX_RETRIES` | Max retry attempts |
| `DSGO_CACHE_SIZE` | LRU cache size |
| `DSGO_CACHE_TTL` | Cache TTL |

### Observability / debugging

| Variable | Purpose |
|---|---|
| `DSGO_LOG` | Logging output mode (see logging docs) |
| `DSGO_DEBUG_PARSE` | Show parse attempts (`1`/`true`) |
| `DSGO_SAVE_RAW_RESPONSES` | Save raw LM outputs (if supported by provider) |
| `DSGO_DEBUG_MARKERS` | Show field markers during streaming |
| `DSGO_COLLECTOR` | Collector type (`memory`, etc.) |
| `DSGO_REQUEST_ID_HEADER` | Incoming request id header name |
| `DSGO_ARTIFACT_DIR` | Where to write debug artifacts (e.g. raw exchanges) |
| `DSGO_EXAMPLE` | Optional label included in saved artifacts |

> Source-of-truth: see package docs and `internal/*`.

## Modules (high level)

### Parallel verbose logging fields

Enable:

- Code: `dsgo.NewParallel(...).WithVerbose(true)`
- Logging output: set up the DSGo logger (e.g., via `logging.ConfigureLoggerFromEnv()` or `DSGO_LOG=pretty`).

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
| Predict | `dsgo.NewPredict(sig, lm)` | Structured prediction |
| ChainOfThought | `dsgo.NewChainOfThought(sig, lm)` | Stores reasoning in `Prediction.Rationale` (and/or explicit reasoning fields) |
| ReAct | `dsgo.NewReAct(sig, lm, tools)` | Tool-using agent loop |
| Refine | `dsgo.NewRefine(sig, lm)` | Refines only when `inputs["feedback"]` (or custom refinement field) is provided |
| BestOfN | `dsgo.NewBestOfN(module, n)` | Requires `WithScorer(...)`; can parallelize and optionally return all completions |
| Program | `dsgo.NewProgram(name)` | Sequential composition |
| Parallel | `dsgo.NewParallel(module)` | Concurrent execution; per-item outputs are in `Prediction.Completions` |
| ProgramOfThought | `dsgo.NewProgramOfThought(sig, lm, language)` | Generates code-first solutions; execution is disabled by default |
| MultiChainComparison | `dsgo.NewMultiChainComparison(sig, lm, m)` | Synthesizes from `inputs["completions"]` and prepends `rationale` output |

## Adapters

| Adapter | Description |
|---|---|
| ChatAdapter | Marker-based format `[[ ## field ## ]]` |
| JSONAdapter | JSON output with schema + repair |
| TwoStepAdapter | Two-step reasoning then extraction |
| FallbackAdapter | Chains adapters for robustness |

## Links

- Getting started: [`QUICKSTART.md`](QUICKSTART.md)
- Architecture: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Development: [`DEVELOPMENT.md`](DEVELOPMENT.md)
- Examples index: [`examples/README.md`](examples/README.md)
- Core internals: [`internal/core/README.md`](internal/core/README.md)
- Module internals: [`internal/module/README.md`](internal/module/README.md)
