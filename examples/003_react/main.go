package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "003_react", runExample)
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

	searchTool := dsgo.NewTool(
		"search",
		"Search for information on the internet",
		func(ctx context.Context, args map[string]any) (any, error) {
			query := args["query"].(string)
			return fmt.Sprintf("Search results for '%s': DSPy is a framework for programming language models, developed at Stanford.", query), nil
		},
	).AddParameter("query", "string", "The search query", true)

	calculatorTool := dsgo.NewTool(
		"calculator",
		"Perform mathematical calculations",
		func(ctx context.Context, args map[string]any) (any, error) {
			expression := args["expression"].(string)
			parts := strings.Split(strings.ReplaceAll(expression, " ", ""), "-")
			if len(parts) == 2 {
				var num1, num2 int
				_, err1 := fmt.Sscanf(parts[0], "%d", &num1)
				_, err2 := fmt.Sscanf(parts[1], "%d", &num2)
				if err1 == nil && err2 == nil {
					return fmt.Sprintf("%d", num1-num2), nil
				}
			}
			return fmt.Sprintf("Unable to calculate: %s", expression), nil
		},
	).AddParameter("expression", "string", "The mathematical expression to evaluate", true)

	tools := []dsgo.Tool{*searchTool, *calculatorTool}

	sig := dsgo.NewSignature("Answer the question using available tools").
		AddInput("question", dsgo.FieldTypeString, "The question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "The final answer").
		AddOutput("sources", dsgo.FieldTypeString, "Sources used to answer the question")

	lm := shared.GetLM(shared.GetModel())

	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(5).
		WithVerbose(true)

	inputs := map[string]any{
		"question": "What is DSPy and how many years has it been since 2020?",
	}

	result, err := react.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("react failed: %w", err)
	}

	stats.TokensUsed = result.Usage.TotalTokens

	answer, _ := result.GetString("answer")
	sources, _ := result.GetString("sources")

	stats.Metadata["question"] = inputs["question"]
	stats.Metadata["answer"] = answer
	stats.Metadata["sources"] = sources

	fmt.Printf("\n=== Final Result ===\n")
	fmt.Printf("Question: %s\n", inputs["question"])
	fmt.Printf("Answer: %s\n", answer)
	fmt.Printf("Sources: %s\n", sources)

	return result, stats, nil
}
