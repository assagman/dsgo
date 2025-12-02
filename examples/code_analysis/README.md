# Code Analysis Agent Example

This example demonstrates how to build a **self-correcting code analysis agent** using DSGo's `ReAct` module and custom tools.

## Key Concepts Demonstrated

1.  **Two-Stage Architecture**:
    *   **Stage 1 (Planning)**: Uses `dsgo.Predict` to generate a high-level analysis strategy based on the user's request.
    *   **Stage 2 (Execution)**: Uses `dsgo.ReAct` (Reasoning + Acting) to autonomously execute the plan using provided tools.

2.  **Custom Tools**:
    *   `list_files`: Lists directory contents to discover project structure.
    *   `read_file`: Reads source code content for analysis.
    *   `search_files`: Finds files matching specific patterns (e.g., `*adapter*`).

3.  **Dynamic Exploration**:
    *   Instead of hardcoding file paths, the agent **discovers** relevant files by exploring the file system.
    *   It can follow imports, search for specific terms, and decide which files are important to read.

## Usage

### Prerequisites

*   Set your OpenAI API key in the `OPENAI_API_KEY` environment variable.
*   Ensure you have access to the DSGo codebase. The example automatically detects the project root (via `go.mod`) to correctly locate the `internal/` directory, regardless of where you run it from.

### Running the Example

```bash
# Analyze the adapter system (default)
go run main.go

# Analyze a specific topic
go run main.go "How does the caching mechanism work in internal/core?"
```

### Output

The program will:
1.  Display the generated **Analysis Plan**.
2.  Show the agent's **Actions** (tool calls) as it explores the codebase (if verbose logging is enabled).
3.  Print a **Final Analysis** summary, findings, and recommendations.
4.  Save a detailed report to a timestamped text file (e.g., `code_analysis_2023-10-25_...txt`).

## How it Works

```go
// 1. Define Tools
listFiles := dsgo.NewTool("list_files", ...)
readFile := dsgo.NewTool("read_file", ...)

// 2. Plan Strategy
planner := dsgo.NewPredict(planSig, lm)
plan, _ := planner.Forward(ctx, map[string]any{"question": userPrompt})

// 3. Execute with ReAct
react := dsgo.NewReAct(analysisSig, lm, []dsgo.Tool{*listFiles, *readFile})
result, _ := react.Forward(ctx, map[string]any{
    "task": userPrompt,
    "plan": plan.GetString("strategy"),
})
```
