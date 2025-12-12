package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

type mcpTools struct {
	Exa  []dsgo.Tool
	Jina []dsgo.Tool
}

func main() {
	// Configure DSGo logger from environment (DSGO_LOG, etc.).
	dsgo.ConfigureLoggerFromEnv()

	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Ensure we have at least one LM API key configured.
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("OPENROUTER_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Missing OPENAI_API_KEY or OPENROUTER_API_KEY.")
		fmt.Fprintln(os.Stderr, "Set one of them before running this example.")
		return errors.New("missing LM API key")
	}

	modelName := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if strings.TrimSpace(modelName) == "" {
		// Small, chat-capable default; can be overridden via EXAMPLES_DEFAULT_MODEL.
		modelName = "openrouter/moonshotai/kimi-k2-0905:exacto"
	}

	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		return fmt.Errorf("create LM (%s): %w", modelName, err)
	}

	// Shared conversation history for ReAct (multi-turn chat behavior).
	history := dsgo.NewHistoryWithLimit(50)

	// Initialize MCP tools from Exa and Jina (if configured).
	allTools, toolSets := initMCPTools(ctx)

	// Build DSGo signatures and modules.
	researchSig := buildResearchSignature()
	refineSig := buildRefineSignature()

	// MCP-powered ReAct research agent.
	react := dsgo.NewReAct(researchSig, lm, allTools).
		WithHistory(history).
		WithMaxIterations(8)

	if os.Getenv("CHAT_LOOP_VERBOSE") != "" {
		react = react.WithVerbose(true)
	}

	// Answer refiner: takes question + draft_answer (+ sources) and produces final answer.
	refiner := dsgo.NewRefine(refineSig, lm)

	// Program pipeline: ReAct (research) -> Refine (polish).
	program := dsgo.NewProgram("mcp_chat_pipeline").
		AddModule(react).
		AddModule(refiner)

	printBanner(modelName, toolSets)

	return chatLoop(ctx, program, history, toolSets)
}

func printBanner(modelName string, tools mcpTools) {
	fmt.Println("DSGo MCP Chat Loop Example")
	fmt.Printf("Model: %s\n", modelName)
	fmt.Printf("Exa MCP tools enabled: %v\n", len(tools.Exa) > 0)
	fmt.Printf("Jina MCP tools enabled: %v\n", len(tools.Jina) > 0)
	total := len(tools.Exa) + len(tools.Jina)
	fmt.Printf("Total MCP tools: %d\n", total)
	if total == 0 {
		fmt.Println("Note: No MCP tools are configured. The assistant will answer from the base model only.")
	}
	fmt.Println("Type your question and press Enter.")
	fmt.Println("Commands: /help, /tools, /history, /exit")
}

// initMCPTools initializes Exa and Jina MCP clients (if their API keys are set) and
// returns the combined tool list plus per-provider slices for introspection.
func initMCPTools(ctx context.Context) ([]dsgo.Tool, mcpTools) {
	var (
		allTools []dsgo.Tool
		sets     mcpTools
	)

	// Exa MCP tools.
	exaKey := os.Getenv("EXA_API_KEY")
	if strings.TrimSpace(exaKey) == "" {
		fmt.Fprintln(os.Stderr, "Exa MCP disabled: EXA_API_KEY not set.")
	} else {
		client, err := dsgo.NewMCPExaClient(exaKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Exa MCP client: %v\n", err)
		} else if err := client.Initialize(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize Exa MCP client: %v\n", err)
		} else {
			tools := client.GetTools()
			if len(tools) == 0 {
				fmt.Fprintln(os.Stderr, "Exa MCP client returned no tools.")
			} else {
				sets.Exa = tools
				allTools = append(allTools, tools...)
			}
		}
	}

	// Jina MCP tools.
	jinaKey := os.Getenv("JINA_API_KEY")
	if strings.TrimSpace(jinaKey) == "" {
		fmt.Fprintln(os.Stderr, "Jina MCP disabled: JINA_API_KEY not set.")
	} else {
		client, err := dsgo.NewMCPJinaClient(jinaKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create Jina MCP client: %v\n", err)
		} else if err := client.Initialize(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize Jina MCP client: %v\n", err)
		} else {
			tools := client.GetTools()
			if len(tools) == 0 {
				fmt.Fprintln(os.Stderr, "Jina MCP client returned no tools.")
			} else {
				sets.Jina = tools
				allTools = append(allTools, tools...)
			}
		}
	}

	if len(allTools) == 0 {
		fmt.Fprintln(os.Stderr, "No MCP tools initialized; running without external tools.")
	}

	return allTools, sets
}

// buildResearchSignature defines the signature for the MCP-powered ReAct research agent.
func buildResearchSignature() *dsgo.Signature {
	return dsgo.NewSignature("MCP research agent").
		AddInput("question", dsgo.FieldTypeString, "Current user question or instruction.").
		AddOutput("draft_answer", dsgo.FieldTypeString, "Research-backed draft answer to the question.").
		AddOptionalOutput("sources", dsgo.FieldTypeJSON, "Optional list of sources or citations used in the answer.")
}

// buildRefineSignature defines the signature for the answer refiner.
func buildRefineSignature() *dsgo.Signature {
	return dsgo.NewSignature("Chat-friendly answer refiner").
		AddInput("question", dsgo.FieldTypeString, "Original user question for this turn.").
		AddOptionalInput("draft_answer", dsgo.FieldTypeString, "Raw research answer from the agent.").
		AddOptionalInput("sources", dsgo.FieldTypeJSON, "Optional sources used for the answer.").
		AddOutput("answer", dsgo.FieldTypeString, "Final user-facing answer in a conversational tone.")
}

// chatLoop implements the interactive CLI chat loop and command handling.
func chatLoop(ctx context.Context, program dsgo.Module, history *dsgo.History, tools mcpTools) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")

		if !scanner.Scan() {
			// EOF or input closed.
			fmt.Println()
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			if exit := handleCommand(line, history, tools); exit {
				return nil
			}
			continue
		}

		// Normal user question.
		inputs := map[string]any{
			"question": line,
		}

		// Per-turn timeout to keep chat responsive.
		turnCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		start := time.Now()
		pred, err := program.Forward(turnCtx, inputs)
		cancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				fmt.Fprintln(os.Stderr, "Request canceled or timed out. You can try again.")
			} else {
				fmt.Fprintf(os.Stderr, "Assistant error: %v\n", err)
			}
			continue
		}

		// Prefer final refined answer; fall back to draft_answer if needed.
		answer, ok := pred.GetString("answer")
		if !ok || strings.TrimSpace(answer) == "" {
			if fallback, ok2 := pred.GetString("draft_answer"); ok2 {
				answer = fallback
			}
		}

		if strings.TrimSpace(answer) == "" {
			fmt.Println("The model did not return an answer. See logs for details.")
			continue
		}

		fmt.Printf("assistant> %s\n", answer)

		// Add a simple entry to history so future ReAct calls see prior turns.
		if history != nil {
			history.AddUserMessage(line)
			history.AddAssistantMessage(answer)
		}

		// Optionally show sources if present.
		printSources(pred)

		// Optionally show simple usage stats to stderr.
		usage := pred.Usage
		elapsed := time.Since(start).Round(10 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "[usage] tokens=%d (prompt=%d, completion=%d) cost=$%.6f latency=%s\n",
			usage.TotalTokens, usage.PromptTokens, usage.CompletionTokens, usage.Cost, elapsed)
	}
}

func handleCommand(cmd string, history *dsgo.History, tools mcpTools) bool {
	switch cmd {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /help     - Show this help message.")
		fmt.Println("  /exit     - Exit the chat.")
		fmt.Println("  /quit     - Same as /exit.")
		fmt.Println("  /history  - Show recent conversation turns.")
		fmt.Println("  /tools    - List configured MCP tools.")
	case "/exit", "/quit":
		fmt.Println("Exiting chat. Goodbye!")
		return true
	case "/history":
		printHistory(history, 10)
	case "/tools":
		printTools(tools)
	default:
		fmt.Println("Unknown command. Type /help for a list of commands.")
	}
	return false
}

// printHistory prints the last n user/assistant messages from DSGo History.
func printHistory(history *dsgo.History, n int) {
	if history == nil || history.Len() == 0 {
		fmt.Println("(no history yet)")
		return
	}

	msgs := history.GetLast(n)
	if len(msgs) == 0 {
		fmt.Println("(no history yet)")
		return
	}

	fmt.Println("Recent conversation:")
	index := 1
	for _, m := range msgs {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		prefix := "User"
		if m.Role == "assistant" {
			prefix = "Assistant"
		}
		fmt.Printf("  %d. %s: %s\n", index, prefix, m.Content)
		index++
	}
	if index == 1 {
		fmt.Println("(no user/assistant messages yet)")
	}
}

func printTools(tools mcpTools) {
	total := len(tools.Exa) + len(tools.Jina)
	if total == 0 {
		fmt.Println("No MCP tools configured. External web research is disabled.")
		return
	}

	if len(tools.Exa) > 0 {
		fmt.Println("Exa tools:")
		for _, t := range tools.Exa {
			fmt.Printf("  - %s: %s\n", t.Name, t.Description)
		}
	} else {
		fmt.Println("Exa tools: (none)")
	}

	if len(tools.Jina) > 0 {
		fmt.Println("Jina tools:")
		for _, t := range tools.Jina {
			fmt.Printf("  - %s: %s\n", t.Name, t.Description)
		}
	} else {
		fmt.Println("Jina tools: (none)")
	}

	fmt.Printf("Total tools: %d\n", total)
}

// printSources pretty-prints the `sources` field if present.
func printSources(pred *dsgo.Prediction) {
	raw, ok := pred.Outputs["sources"]
	if !ok || raw == nil {
		return
	}

	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to marshal sources:", err)
		return
	}

	if len(b) == 0 || string(b) == "null" {
		return
	}

	fmt.Println("sources:")
	fmt.Println(string(b))
}
