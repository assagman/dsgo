package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

func main() {
	// Configure DSGo logger from environment (DSGO_LOG, DSGO_LOG_LEVEL, etc.).
	dsgo.ConfigureLoggerFromEnv()

	ctx := context.Background()

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--schema" {
		b, err := dsgo.YamlProgramSchemaJSON()
		if err != nil {
			log.Fatalf("failed to generate schema: %v", err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			log.Fatalf("failed to write schema: %v", err)
		}
		return
	}

	configPath := "software_dev.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	fmt.Println("DSGo YAML Program Runner")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Config: %s\n\n", configPath)

	start := time.Now()
	pred, err := dsgo.ExecYaml(ctx, configPath)
	if err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}
	elapsed := time.Since(start)

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("RESULTS")
	fmt.Println(strings.Repeat("=", 72))
	printOutputs(pred.Outputs)

	fmt.Println(strings.Repeat("-", 72))
	fmt.Printf("Duration: %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Tokens:   %d\n", pred.Usage.TotalTokens)
	fmt.Printf("Cost:     $%.6f\n", pred.Usage.Cost)
}

func printOutputs(outputs map[string]any) {
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("\n[%s]\n", k)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("%v\n", outputs[k])
	}
}

// Keep unused import guard for dsgo when examples are built standalone.
var _ = dsgo.DefaultGenerateOptions
