package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "017_tools", runExample)
	if err != nil {
		log.Fatal(err)
	}

	if err := h.OutputResults(); err != nil {
		log.Fatal(err)
	}
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
	stats := &harness.ExecutionStats{
		Metadata: make(map[string]any),
	}

	lm := shared.GetLM(shared.GetModel())

	var totalTokens int

	fmt.Println("=== Tool Definition & Usage Demo ===")
	fmt.Println("Demonstrates creating tools with different parameter types and integrating them with modules")
	fmt.Println()

	// Demo 1: Simple tools with basic parameters
	fmt.Println("--- Demo 1: Basic Tools (String Parameters) ---")
	tokens1, _, err := basicTools(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("basic tools failed: %w", err)
	}
	totalTokens += tokens1

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: Tools with multiple parameter types
	fmt.Println("--- Demo 2: Advanced Tools (Multiple Parameter Types) ---")
	tokens2, _, err := advancedTools(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("advanced tools failed: %w", err)
	}
	totalTokens += tokens2

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Tools with optional parameters
	fmt.Println("--- Demo 3: Tools with Optional Parameters ---")
	tokens3, pred3, err := optionalParameterTools(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("optional parameter tools failed: %w", err)
	}
	totalTokens += tokens3

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 3
	stats.Metadata["features_demonstrated"] = []string{
		"Tool creation with NewTool()",
		"Required and optional parameters",
		"Multiple parameter types (string, number, boolean)",
		"Tool integration with ReAct",
		"Tool execution and result handling",
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Tool capabilities:\n")
	fmt.Printf("  ✓ Custom function definitions\n")
	fmt.Printf("  ✓ Rich parameter types and validation\n")
	fmt.Printf("  ✓ Required and optional parameters\n")
	fmt.Printf("  ✓ Seamless integration with modules\n")
	fmt.Printf("  ✓ Error handling and robustness\n")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 3\n")
	fmt.Println()

	return pred3, stats, nil
}

func basicTools(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	// Create simple search tool
	searchTool := dsgo.NewTool(
		"search",
		"Search for information on the internet",
		func(ctx context.Context, args map[string]any) (any, error) {
			query, ok := args["query"].(string)
			if !ok {
				return nil, fmt.Errorf("query parameter must be a string")
			}
			// Simulate search results
			results := map[string]string{
				"dsgo":   "DSGo is a Go port of DSPy, a framework for programming language models.",
				"golang": "Go is a statically typed, compiled programming language designed at Google.",
				"tools":  "Tools in DSGo allow agents to perform actions like searching, calculating, or calling APIs.",
			}
			for key, result := range results {
				if strings.Contains(strings.ToLower(query), key) {
					return fmt.Sprintf("Search results for '%s': %s", query, result), nil
				}
			}
			return fmt.Sprintf("No specific results found for '%s'", query), nil
		},
	).AddParameter("query", "string", "The search query", true)

	// Create calculator tool
	calculatorTool := dsgo.NewTool(
		"calculator",
		"Perform basic arithmetic calculations",
		func(ctx context.Context, args map[string]any) (any, error) {
			expression, ok := args["expression"].(string)
			if !ok {
				return nil, fmt.Errorf("expression parameter must be a string")
			}
			// Simple calculator for addition
			if strings.Contains(expression, "+") {
				parts := strings.Split(expression, "+")
				if len(parts) == 2 {
					var num1, num2 int
					_, err1 := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &num1)
					_, err2 := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &num2)
					if err1 == nil && err2 == nil {
						return fmt.Sprintf("%d", num1+num2), nil
					}
				}
			}
			return fmt.Sprintf("Unable to calculate: %s (only addition with + supported)", expression), nil
		},
	).AddParameter("expression", "string", "Mathematical expression (e.g., '5 + 3')", true)

	tools := []dsgo.Tool{*searchTool, *calculatorTool}

	sig := dsgo.NewSignature("Answer the question using available tools").
		AddInput("question", dsgo.FieldTypeString, "The question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "The final answer")

	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(3).
		WithVerbose(false)

	question := "What is DSGo and what is 15 + 27?"
	fmt.Printf("Question: %s\n\n", question)

	result, err := react.Forward(ctx, map[string]any{
		"question": question,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("react failed: %w", err)
	}

	answer, _ := result.GetString("answer")
	fmt.Printf("Answer: %s\n", answer)
	fmt.Printf("\n✅ Tools used: search, calculator\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func advancedTools(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	// Tool with mixed parameter types
	weatherTool := dsgo.NewTool(
		"get_weather",
		"Get weather information for a location",
		func(ctx context.Context, args map[string]any) (any, error) {
			location, ok := args["location"].(string)
			if !ok {
				return nil, fmt.Errorf("location parameter must be a string")
			}

			// Simulate weather data
			weather := map[string]string{
				"san francisco": "Sunny, 68°F",
				"new york":      "Cloudy, 55°F",
				"london":        "Rainy, 50°F",
				"tokyo":         "Clear, 72°F",
			}

			locationLower := strings.ToLower(location)
			for city, conditions := range weather {
				if strings.Contains(locationLower, city) {
					return fmt.Sprintf("Weather in %s: %s", location, conditions), nil
				}
			}
			return fmt.Sprintf("Weather data not available for %s", location), nil
		},
	).AddParameter("location", "string", "City or location name", true)

	// Tool with date/time
	dateTool := dsgo.NewTool(
		"get_current_date",
		"Get the current date and time",
		func(ctx context.Context, args map[string]any) (any, error) {
			now := time.Now()
			return fmt.Sprintf("Current date and time: %s", now.Format("Monday, January 2, 2006 at 3:04 PM MST")), nil
		},
	)

	tools := []dsgo.Tool{*weatherTool, *dateTool}

	sig := dsgo.NewSignature("Answer questions about weather and time").
		AddInput("question", dsgo.FieldTypeString, "The question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "The final answer")

	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(3).
		WithVerbose(false)

	question := "What's the weather in San Francisco and what time is it now?"
	fmt.Printf("Question: %s\n\n", question)

	result, err := react.Forward(ctx, map[string]any{
		"question": question,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("react failed: %w", err)
	}

	answer, _ := result.GetString("answer")
	fmt.Printf("Answer: %s\n", answer)
	fmt.Printf("\n✅ Tools used: get_weather, get_current_date\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func optionalParameterTools(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	// Tool with optional parameters
	formatTool := dsgo.NewTool(
		"format_text",
		"Format text with optional styling",
		func(ctx context.Context, args map[string]any) (any, error) {
			text, ok := args["text"].(string)
			if !ok {
				return nil, fmt.Errorf("text parameter must be a string")
			}

			// Optional parameters with defaults
			uppercase := false
			if val, ok := args["uppercase"].(bool); ok {
				uppercase = val
			}

			prefix := ""
			if val, ok := args["prefix"].(string); ok {
				prefix = val
			}

			result := text
			if uppercase {
				result = strings.ToUpper(result)
			}
			if prefix != "" {
				result = prefix + " " + result
			}

			return result, nil
		},
	).
		AddParameter("text", "string", "The text to format", true).
		AddParameter("uppercase", "boolean", "Convert to uppercase", false).
		AddParameter("prefix", "string", "Prefix to add", false)

	tools := []dsgo.Tool{*formatTool}

	sig := dsgo.NewSignature("Format text according to instructions").
		AddInput("instruction", dsgo.FieldTypeString, "How to format the text").
		AddOutput("formatted_text", dsgo.FieldTypeString, "The formatted result")

	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(2).
		WithVerbose(false)

	instruction := "Format 'hello world' in uppercase with the prefix 'Greeting:'"
	fmt.Printf("Instruction: %s\n\n", instruction)

	result, err := react.Forward(ctx, map[string]any{
		"instruction": instruction,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("react failed: %w", err)
	}

	formatted, _ := result.GetString("formatted_text")
	fmt.Printf("Result: %s\n", formatted)
	fmt.Printf("\n✅ Tool with optional parameters demonstrated\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
