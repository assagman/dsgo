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
	"github.com/assagman/dsgo/internal/yamlprogram"
)

func main() {
	// Configure DSGo logger from environment (DSGO_LOG, DSGO_LOG_LEVEL, etc.).
	dsgo.ConfigureLoggerFromEnv()

	ctx := context.Background()

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--schema" {
		b, err := yamlprogram.SchemaJSON()
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

	spec, err := yamlprogram.LoadFile(configPath)
	if err != nil {
		log.Fatalf("failed to load spec: %v", err)
	}

	builder, err := yamlprogram.NewBuilder(ctx, spec, nil)
	if err != nil {
		log.Fatalf("failed to create builder: %v", err)
	}

	res, err := builder.Build()
	if err != nil {
		log.Fatalf("failed to build program: %v", err)
	}

	displayPipeline(spec)

	if len(spec.Inputs) == 0 {
		log.Fatalf("no inputs provided")
	}

	// Run
	execCtx, cancel := context.WithTimeout(ctx, res.PipelineTimeout)
	defer cancel()

	start := time.Now()
	pred, err := res.Program.Forward(execCtx, spec.Inputs)
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

func displayPipeline(spec *yamlprogram.Spec) {
	fmt.Println("Pipeline:")
	fmt.Println(strings.Repeat("-", 72))
	for i, step := range spec.Pipeline {
		m := spec.Modules[step]
		fmt.Printf("%2d. %s (%s)\n", i+1, step, m.Kind)
		if m.Sig != "" {
			sig := spec.Signatures[m.Sig]
			fmt.Printf("    sig: %s\n", m.Sig)
			fmt.Printf("    in:  %s\n", strings.Join(sortedKeys(sig.In), ", "))
			fmt.Printf("    out: %s\n", strings.Join(sortedKeys(sig.Out), ", "))
		}
	}
	fmt.Println()
}

func sortedKeys(m map[string]yamlprogram.FieldSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
