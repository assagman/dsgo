package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/assagman/dsgo"
)

func main() {
	ctx := context.Background()

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("🗂️  Initializing Filesystem MCP client for: %s\n\n", cwd)

	// Create filesystem MCP client (local stdio transport)
	fsClient, err := dsgo.NewMCPFilesystemClient(cwd)
	if err != nil {
		log.Fatalf("Failed to create filesystem client: %v", err)
	}

	// Initialize client
	fmt.Println("📡 Initializing MCP client...")
	err = fsClient.Initialize(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	// List available tools
	tools := fsClient.GetTools()
	fmt.Printf("✅ Connected! Available tools: %d\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// Create LM
	lm, err := dsgo.NewLM(ctx, "openai/gpt-4o-mini")
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Create ReAct agent with filesystem tools
	sig := dsgo.NewSignature("Code analyst with filesystem access").
		AddInput("task", dsgo.FieldTypeString, "Analysis task").
		AddOutput("analysis", dsgo.FieldTypeString, "Analysis result")

	agent := dsgo.NewReAct(sig, lm, tools).
		WithMaxIterations(10).
		WithVerbose(true)

	// Run analysis task
	fmt.Println("🤖 Running ReAct agent with filesystem tools...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	result, err := agent.Forward(ctx, map[string]any{
		"task": "List all Go files in the current directory and provide a brief summary of what you find",
	})
	if err != nil {
		log.Fatalf("Agent failed: %v", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	analysis, _ := result.GetString("analysis")
	fmt.Printf("\n📊 Analysis Result:\n%s\n", analysis)
}
