package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// Demonstrates: ReAct, Tools, Typed signatures, JSON adapter
// Story: Travel helper agent with search, currency converter, timezone tools

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(2)
	}
	envFilePath := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "examples", ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// If not found in examples/, check cwd/.env.local
	if envFilePath == "" {
		candidate := filepath.Join(cwd, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
		}
	}
	if envFilePath == "" {
		fmt.Printf("Could not find .env.local file\n")
		os.Exit(3)
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(3)
	}

	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "travel_agent", map[string]interface{}{
		"scenario": "multi_tool_agent",
	})
	defer runSpan.End(nil)

	// Setup tools
	searchTool := dsgo.NewTool(
		"search",
		"Search the web for information about destinations, flights, hotels",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			_, span := observe.Start(ctx, observe.SpanKindTool, "search", map[string]interface{}{
				"query": args["query"],
			})
			defer span.End(nil)

			query := args["query"].(string)
			// Simulate search
			time.Sleep(100 * time.Millisecond)
			results := map[string]interface{}{
				"query":   query,
				"results": []string{
					"Barcelona weekend trips: avg $450-650 (flights+hotel)",
					"Peak season: June-August, shoulder: Apr-May, Sep-Oct",
					"Top activities: Sagrada Familia, Park Güell, Gothic Quarter",
				},
			}
			return results, nil
		},
	).AddParameter("query", "string", "Search query", true)

	currencyTool := dsgo.NewTool(
		"convert_currency",
		"Convert amount from one currency to another",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			_, span := observe.Start(ctx, observe.SpanKindTool, "convert_currency", map[string]interface{}{
				"amount": args["amount"],
				"from":   args["from"],
				"to":     args["to"],
			})
			defer span.End(nil)

			amount := args["amount"].(float64)
			from := args["from"].(string)
			to := args["to"].(string)

			// Simulate conversion (mock rates)
			rates := map[string]float64{"USD": 1.0, "EUR": 0.92, "GBP": 0.79}
			converted := amount * (rates[to] / rates[from])

			return map[string]interface{}{
				"amount":   amount,
				"from":     from,
				"to":       to,
				"result":   fmt.Sprintf("%.2f", converted),
				"rate":     rates[to] / rates[from],
			}, nil
		},
	).
		AddParameter("amount", "number", "Amount to convert", true).
		AddParameter("from", "string", "Source currency (USD, EUR, GBP)", true).
		AddParameter("to", "string", "Target currency (USD, EUR, GBP)", true)

	timezoneTool := dsgo.NewTool(
		"local_time",
		"Get local time for a city",
		func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			_, span := observe.Start(ctx, observe.SpanKindTool, "local_time", map[string]interface{}{
				"city": args["city"],
			})
			defer span.End(nil)

			city := args["city"].(string)
			// Simulate timezone lookup
			offsets := map[string]int{"barcelona": 1, "paris": 1, "london": 0, "new york": -5}
			cityLower := strings.ToLower(city)
			
			var offset int
			for k, v := range offsets {
				if strings.Contains(cityLower, k) {
					offset = v
					break
				}
			}

			now := time.Now().UTC().Add(time.Duration(offset) * time.Hour)
			return map[string]interface{}{
				"city":       city,
				"time":       now.Format("15:04"),
				"timezone":   fmt.Sprintf("UTC%+d", offset),
				"day_of_week": now.Format("Monday"),
			}, nil
		},
	).AddParameter("city", "string", "City name", true)

	tools := []dsgo.Tool{*searchTool, *currencyTool, *timezoneTool}

	// Setup ReAct agent
	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("failed to create LM: %v", err)
	}
	
	sig := dsgo.NewSignature("You are a helpful travel assistant. Use tools to find accurate information.").
		AddInput("question", dsgo.FieldTypeString, "User's travel question").
		AddOutput("answer", dsgo.FieldTypeString, "Detailed answer with cited sources")

	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(8).
		WithVerbose(true)

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	// Turn 1: Multi-step planning query
	fmt.Println("\n=== Turn 1: Weekend Trip Planning ===")
	turn1Ctx, turn1Span := observe.Start(ctx, observe.SpanKindModule, "turn1", map[string]interface{}{
		"tools_available": len(tools),
	})

	userQuestion1 := "What's a good weekend trip from London to Barcelona? Convert the typical price from USD to EUR."
	fmt.Printf("User: %s\n", userQuestion1)

	result1, err := react.Forward(turn1Ctx, map[string]interface{}{
		"question": userQuestion1,
	})
	if err != nil {
		log.Fatal(err)
	}

	answer1, _ := result1.GetString("answer")
	fmt.Printf("\n✓ Final Answer:\n%s\n", answer1)
	usage1 := result1.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage1.PromptTokens, usage1.CompletionTokens)
	totalPromptTokens += usage1.PromptTokens
	totalCompletionTokens += usage1.CompletionTokens
	turn1Span.End(nil)

	// Turn 2: Follow-up with timezone
	fmt.Println("\n=== Turn 2: Timezone Query ===")
	turn2Ctx, turn2Span := observe.Start(ctx, observe.SpanKindModule, "turn2", nil)

	userQuestion2 := "If it's 9 AM in London on Saturday, what time is it in Barcelona?"
	fmt.Printf("User: %s\n", userQuestion2)

	result2, err := react.Forward(turn2Ctx, map[string]interface{}{
		"question": userQuestion2,
	})
	if err != nil {
		log.Fatal(err)
	}

	answer2, _ := result2.GetString("answer")
	fmt.Printf("\n✓ Final Answer:\n%s\n", answer2)
	usage2 := result2.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage2.PromptTokens, usage2.CompletionTokens)
	totalPromptTokens += usage2.PromptTokens
	totalCompletionTokens += usage2.CompletionTokens
	turn2Span.End(nil)

	// Summary
	fmt.Println("\n=== Agent Summary ===")
	fmt.Println("Tools used: search, convert_currency, local_time")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ ReAct agent (reasoning + acting)")
	fmt.Println("  ✓ Multiple typed tools")
	fmt.Println("  ✓ JSON adapter (tool args/results)")
	fmt.Println("  ✓ Verbose mode (see iterations)")
	fmt.Println("  ✓ Event logging for tool calls")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
