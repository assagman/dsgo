# Codebase Analysis Example

This example demonstrates how to build a **comprehensive codebase analysis agent** using DSGo's modules and custom tools.

## Key Concepts Demonstrated

1.  **Multi-Stage Architecture**:
    *   **Stage 1 (Planning)**: Uses `dsgo.Predict` to generate a high-level analysis strategy.
    *   **Stage 2 (Execution)**: Uses `dsgo.ReAct` to autonomously execute the analysis plan.
    *   **Stage 3 (Synthesis)**: Uses `dsgo.BestOfN` to generate comprehensive reports.

2.  **Custom Tools**:
    *   File system exploration tools for discovering project structure.
    *   Code analysis tools for examining source code patterns.
    *   Dependency analysis tools for understanding relationships.

3.  **Dynamic Analysis**:
    *   Discovers relevant files by exploring the codebase.
    *   Analyzes code patterns, architecture, and dependencies.
    *   Generates comprehensive reports with actionable insights.

## Usage

### Prerequisites

*   Set your OpenAI API key in the `OPENAI_API_KEY` environment variable.
*   Ensure you have access to the DSGo codebase. The example automatically detects the project root.

### Running the Example

```bash
# Run the codebase analysis
go run main.go

# With verbose logging
DSGO_LOG=pretty go run main.go

# Save events to file
DSGO_LOG=events go run main.go > events.jsonl
```

### Output

The program will:
1.  Display the generated **Analysis Plan**.
2.  Show the agent's **Actions** as it explores the codebase.
3.  Print a **Final Analysis** with findings and recommendations.
4.  Save a detailed report to a timestamped text file.

## How it Works

```go
// 1. Initialize LM and Tools
lm, _ := dsgo.NewLM(ctx, modelName)
tools := tools.GetAllFilesystemTools()

// 2. Plan Analysis Strategy
planner := dsgo.NewPredict(planSig, lm)
plan, _ := planner.Forward(ctx, map[string]any{"task": analysisTask})

// 3. Execute Analysis
react := dsgo.NewReAct(analysisSig, lm, tools)
result, _ := react.Forward(ctx, map[string]any{
    "task": analysisTask,
    "plan": plan.GetString("strategy"),
})

// 4. Generate Comprehensive Report
synthesizer := dsgo.NewBestOfN(baseModule, 10)
finalReport, _ := synthesizer.Forward(ctx, analysisData)
```

## Features

- **Automated Discovery**: Explores codebase structure without hardcoded paths
- **Multi-Stage Analysis**: Planning, execution, and synthesis phases
- **Comprehensive Reporting**: Architecture diagrams, improvement suggestions, and recommendations
- **Tool Integration**: Uses filesystem tools for dynamic exploration
- **Error Handling**: Robust error reporting with helpful debugging information
