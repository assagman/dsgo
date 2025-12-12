# Chat Loop with MCP Tools (`examples/chat_loop_mcp/`)

Interactive CLI chat loop built with **DSGo** that demonstrates:

- A multi-step **`Program`** pipeline (ReAct → Refine)
- A `ReAct` agent that can use **MCP tools from Exa and Jina** for web research
- Simple **conversation memory** across turns using DSGo `History`
- Basic **CLI commands** to inspect tools and history

---

## Features

- **CLI chat loop**  
  Type questions or follow-ups; the assistant replies and keeps track of the conversation.

- **Program pipeline**  
  A DSGo `Program` composes:
  - A `ReAct` step (`MCP research agent`) wired to MCP tools
  - A `Refine` step (`Chat-friendly answer refiner`) that turns the draft into a polished answer

- **MCP-based tools**  
  Uses DSGo's MCP clients to expose:
  - Exa tools (semantic web search, web content)
  - Jina tools (URL reading and content extraction)

- **Conversation memory**  
  Uses `dsgo.History` so each turn can see previous messages and behave like a chat.

- **Graceful degradation**  
  If Exa/Jina API keys are missing, the example still runs as an LM-only chat and prints a clear notice that external tools are disabled.

---

## Requirements

- Go **1.25+**
- At least one DSGo-supported LLM provider:
  - `OPENAI_API_KEY` (for OpenAI models), or
  - `OPENROUTER_API_KEY` (for OpenRouter models)

Optional for MCP tools:

- `EXA_API_KEY` – enables Exa MCP tools (search + web content)
- `JINA_API_KEY` – enables Jina MCP tools (URL reading)

---

## Environment Variables

### LLM configuration

```bash
# One of these must be set:

# For OpenAI models (e.g. gpt-4o-mini)
export OPENAI_API_KEY=sk-...

# OR for OpenRouter models
export OPENROUTER_API_KEY=sk-or-...
```

You can choose the model via:

```bash
# Optional: override default model
export EXAMPLES_DEFAULT_MODEL="gpt-4o-mini"
```

If `EXAMPLES_DEFAULT_MODEL` is not set, the example defaults to `gpt-4o-mini`.

### MCP tools

```bash
# Optional: enables Exa MCP tools
export EXA_API_KEY=exa_...

# Optional: enables Jina MCP tools
export JINA_API_KEY=jina_...
```

If either key is missing, that provider's tools are simply disabled. The chat still works using the base model only.

### DSGo debug & logging (optional)

```bash
# Pretty logging while experimenting
export DSGO_LOG=pretty

# Show more verbose ReAct step-by-step logs
export CHAT_LOOP_VERBOSE=1
```

---

## Running the Example

From the repository root:

```bash
cd examples/chat_loop_mcp
go run ./...
```

With all providers configured:

```bash
EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" \
OPENAI_API_KEY=sk-... \
EXA_API_KEY=exa_... \
JINA_API_KEY=jina_... \
go run ./...
```

On startup you should see something like:

```text
DSGo MCP Chat Loop Example
Model: gpt-4o-mini
Exa MCP tools enabled: true
Jina MCP tools enabled: true
Total MCP tools: 3
Type your question and press Enter.
Commands: /help, /tools, /history, /exit
```

---

## CLI Commands

Inside the chat loop, the following commands are available:

- `/help`  
  Show a short help message listing all commands.

- `/tools`  
  List MCP tools grouped by provider (Exa, Jina) and show their names/descriptions.

- `/history`  
  Print a compact summary of recent user/assistant messages from DSGo `History`.

- `/exit` or `/quit`  
  Exit the program.

Any line that does **not** start with `/` is treated as a normal user message and sent through the `Program` pipeline.

---

## How It Works

1. **Initialization**
   - Configures the logger via `dsgo.ConfigureLoggerFromEnv()`.
   - Resolves the model from `EXAMPLES_DEFAULT_MODEL` (default `gpt-4o-mini`).
   - Creates the LM instance with `dsgo.NewLM(ctx, modelName)`.
   - Creates a shared `History` with `dsgo.NewHistoryWithLimit(50)`.

2. **MCP tools**
   - If `EXA_API_KEY` is set, `dsgo.NewMCPExaClient` is initialized and its tools are added.
   - If `JINA_API_KEY` is set, `dsgo.NewMCPJinaClient` is initialized and its tools are added.
   - All MCP tools are passed into the `ReAct` module; if there are none, the chat still works, but without external web research.

3. **Program pipeline**
   - `researchSig` (`"MCP research agent"`) defines:
     - Input: `question` (string)
     - Optional outputs: `draft_answer` (string), `sources` (JSON)
   - `refineSig` (`"Chat-friendly answer refiner"`) defines:
     - Inputs: `question`, `draft_answer`, optional `sources`
     - Output: `answer` (string, user-facing)
   - `react := dsgo.NewReAct(researchSig, lm, tools).WithHistory(history).WithMaxIterations(8)`
   - `refine := dsgo.NewRefine(refineSig, lm)`
   - `program := dsgo.NewProgram("mcp_chat_pipeline").AddModule(react).AddModule(refine)`

4. **Conversation memory**
   - In the loop, each successful turn appends user/assistant messages to `history` using `AddUserMessage` and `AddAssistantMessage`.
   - ReAct uses `history` (via `WithHistory`) to prepend previous messages for multi-turn reasoning.
   - `/history` reads from `history.GetLast(n)` and prints recent user/assistant turns.

5. **Answer flow**
   - Each user message calls `program.Forward(ctx, {"question": line})`.
   - The ReAct step may call Exa/Jina MCP tools to gather information and produce `draft_answer` + `sources`.
   - The Refine step produces a polished final `answer`.
   - The CLI prints `answer` (or falls back to `draft_answer` if needed) and optionally shows structured `sources`.

---

## Sample Session

_Actual responses will vary; this is illustrative._

```text
$ cd examples/chat_loop_mcp
$ EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" \
  OPENAI_API_KEY=sk-... \
  EXA_API_KEY=exa_... \
  JINA_API_KEY=jina_... \
  go run ./...

DSGo MCP Chat Loop Example
Model: gpt-4o-mini
Exa MCP tools enabled: true
Jina MCP tools enabled: true
Total MCP tools: 3
Type your question and press Enter.
Commands: /help, /tools, /history, /exit

> Hi, who are you?
assistant> I'm an example assistant built with DSGo. I run in a CLI chat loop and can optionally use web tools via MCP to answer questions.

> In one sentence, what is DSGo?
assistant> DSGo is a Go framework for building structured LLM applications using composable modules like Predict, ReAct, and Program.

> Using web tools, find a recent article about Go-based LLM orchestration and summarize it briefly.
assistant> I found an article describing how Go developers can orchestrate LLMs with type-safe signatures, modular pipelines, and observability. In short, it shows how to build reliable, composable AI workflows in Go.

sources:
[
  {
    "title": "Go-based LLM orchestration frameworks",
    "url": "https://example.com/go-llm-orchestration"
  }
]

> /tools
Exa tools:
  - search: Search the web using Exa.
  - browse: Fetch and summarize a specific result.
Jina tools:
  - read_url: Fetch and clean content from a URL.
Total tools: 3

> /history
Recent conversation:
  1. User: Hi, who are you?
     Assistant: I'm an example assistant built with DSGo...
  2. User: In one sentence, what is DSGo?
     Assistant: DSGo is a Go framework...
  3. User: Using web tools, find a recent article...
     Assistant: I found an article describing how Go developers...

> /exit
Exiting chat. Goodbye!
```

---

## Notes

- If `EXA_API_KEY` or `JINA_API_KEY` is not set, the example prints a warning and continues in LM-only mode.
- If no LM API key (`OPENAI_API_KEY`/`OPENROUTER_API_KEY`) is set, the program exits with a clear error instead of panicking.
- For deeper DSGo concepts, see the root `README.md`, `AGENTS.md`, and `llms.txt` files.
