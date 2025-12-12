# Examples

Runnable examples for DSGo.

| Folder | Goal | Run | Key concepts |
|---|---|---|---|
| `codebase_analysis/` | Analyze a codebase with structured output | `cd examples/codebase_analysis && go run main.go` | Predict/CoT, signatures |
| `modules/parallel/` | Show concurrent execution patterns | `cd examples/modules/parallel && go run main.go` | Parallel module |
| `project_review/` | Multi-stage review pipeline | `cd examples/project_review && go run main.go` | Program composition |
| `react_experiment/` | Tool-using ReAct agent | `cd examples/react_experiment && go run main.go` | Tools, ReAct loop |
| `multi_chain_comparison/` | Synthesize answers from multiple chains | `cd examples/multi_chain_comparison && go run main.go` | MultiChainComparison |
| `chat_loop_mcp/` | ReAct-like loop with MCP tools | `cd examples/chat_loop_mcp && go run main.go` | MCP integration |
| `package_analysis/` | Analyze a Go package | `cd examples/package_analysis && go run main.go` | Signatures, parsing |
| `security_scan/` | Security scanning workflow | `cd examples/security_scan && go run main.go` | Program patterns |
| `snowflake_trainer/` | Trainer experiment | `cd examples/snowflake_trainer && go run main.go` | Trainers |

Notes:
- Use provider-prefixed model IDs (e.g. `openai/gpt-4o-mini`, `openrouter/anthropic/claude-3-opus`).
- Most examples require `OPENAI_API_KEY` or `OPENROUTER_API_KEY`.
