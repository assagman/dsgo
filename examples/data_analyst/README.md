# AI Data Analyst

Demonstrates **ReAct agent with multiple analytical tools** for data analysis tasks.

## Features Demonstrated

- **ReAct Module**: Reasoning + Acting pattern
- **Multiple Tools**: Statistics, outliers, comparison, trends
- **Iterative Analysis**: Multi-step analytical reasoning
- **Real Calculations**: Actual statistical computations

## Tools Provided

1. **calculate_statistics**: Mean, median, std dev, min, max
2. **find_outliers**: IQR-based outlier detection
3. **compare_datasets**: Compare two datasets
4. **analyze_trend**: Detect trends in time series data

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/data_analyst
go run main.go
```

## What You'll Learn

- How to build analytical tools for ReAct agents
- How the agent decides which tools to use
- How to implement statistical calculations
- How agents combine multiple tool results

## Example Analyses

### Example 1: Sales Data Analysis
```
Data: 45000, 47000, 46500, 48000, 51000, 49000, 52000, 95000, 53000...
Question: Are there outliers? What's the trend?
```

The agent will:
1. Calculate basic statistics
2. Detect outliers (the 95000 spike)
3. Analyze overall trend
4. Provide recommendations

### Example 2: Product Rating Comparison
```
Product A: 4.2, 4.5, 4.3, 4.6, 4.4, 4.7
Product B: 3.8, 3.9, 4.0, 3.7, 3.9, 3.8
Question: Which performs better and by how much?
```

The agent will:
1. Calculate statistics for each product
2. Compare the datasets
3. Quantify the difference
4. Make recommendations

## Key Code Patterns

```go
// Define analytical tools
statsTool := dsgo.NewTool(
    "calculate_statistics",
    "Calculate statistics for a dataset",
    func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
        // Actual statistical calculations
        return results, nil
    },
).AddParameter("data", "string", "Comma-separated numbers", true)

// Create ReAct agent with tools
react := dsgo.NewReAct(sig, lm, tools).
    WithMaxIterations(5).
    WithVerbose(true)  // See the agent's reasoning
```

## Statistical Methods

- **Mean & Median**: Central tendency
- **Standard Deviation**: Data spread
- **IQR Method**: Outlier detection
- **Percentiles**: Q1, Q3 calculation
- **Trend Detection**: Increase/decrease patterns

## Output Format

Structured reports with:
- Detailed analysis
- Key insights
- Actionable recommendations
- Supporting statistics
