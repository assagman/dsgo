package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

const defaultTimeout = 5 * time.Minute

func getModelName(config *PipelineConfig) string {
	if config.Model.Name != "" {
		return config.Model.Name
	}
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	return "openrouter/z-ai/glm-4.6"
}

func main() {
	ctx := context.Background()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║        DSGo YAML Pipeline Builder Example                     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load pipeline configuration
	configPath := "pipeline.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	fmt.Printf("📄 Loading pipeline configuration from: %s\n", configPath)
	config, err := LoadPipelineConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load pipeline configuration: %v", err)
	}

	fmt.Printf("✅ Loaded pipeline: %s\n", config.Name)
	fmt.Printf("   Description: %s\n", config.Description)
	fmt.Printf("   Signatures: %d\n", len(config.Signatures))
	fmt.Printf("   Modules: %d\n", len(config.Modules))
	fmt.Printf("   Pipeline steps: %d\n", len(config.Pipeline))
	fmt.Println()

	// Initialize LM
	modelName := getModelName(config)
	fmt.Printf("🤖 Initializing LM: %s\n", modelName)

	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		log.Fatalf("❌ Failed to create LM: %v", err)
	}
	fmt.Println("✅ LM initialized successfully")
	fmt.Println()

	// LM provider for module-level models
	lmProvider := func(ctx context.Context, model string) (dsgo.LM, error) {
		return dsgo.NewLM(ctx, model)
	}

	// Build program from YAML
	fmt.Println("🔧 Building program from YAML configuration...")
	builder, err := NewProgramBuilder(ctx, config, lm, lmProvider)
	if err != nil {
		log.Fatalf("❌ Failed to create program builder: %v", err)
	}

	program, err := builder.Build()
	if err != nil {
		log.Fatalf("❌ Failed to build program: %v", err)
	}
	fmt.Printf("✅ Program built successfully with %d modules\n", program.ModuleCount())
	fmt.Println()

	// Display pipeline structure
	displayPipelineStructure(config)

	// Get inputs from YAML config
	inputs := config.Inputs
	if len(inputs) == 0 {
		log.Fatalf("❌ No inputs defined in YAML configuration")
	}
	fmt.Println("📝 Inputs:")
	fmt.Println(strings.Repeat("-", 60))
	for k, v := range inputs {
		fmt.Printf("   %s: %v\n", k, v)
	}
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()

	// Run the pipeline
	fmt.Println("🚀 Executing pipeline...")
	fmt.Println()

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	startTime := time.Now()
	prediction, err := program.Forward(ctx, inputs)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Fatalf("❌ Pipeline execution failed: %v", err)
	}

	// Display results
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                       PIPELINE RESULTS                         ")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	displayResults(prediction)

	// Display execution stats
	fmt.Println()
	fmt.Println("📊 Execution Statistics:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("   Duration: %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Tokens used: %d\n", prediction.Usage.TotalTokens)
	fmt.Printf("   Cost: $%.6f\n", prediction.Usage.Cost)
	fmt.Println()

	fmt.Println("✅ Pipeline executed successfully!")
}

func displayPipelineStructure(config *PipelineConfig) {
	fmt.Println("📋 Pipeline Structure:")
	fmt.Println(strings.Repeat("-", 60))

	for i, step := range config.Pipeline {
		mod := config.Modules[step.Module]
		sig := config.Signatures[mod.Signature]

		arrow := "  │"
		if i == len(config.Pipeline)-1 {
			arrow = "  └"
		}

		fmt.Printf("  %d. %s (%s)\n", i+1, step.Module, mod.Type)
		fmt.Printf("%s── Signature: %s\n", arrow, mod.Signature)
		fmt.Printf("%s── Inputs: %s\n", arrow, formatFields(sig.Inputs))
		fmt.Printf("%s── Outputs: %s\n", arrow, formatFields(sig.Outputs))
		if mod.Options.Temperature > 0 {
			fmt.Printf("%s── Temperature: %.2f\n", arrow, mod.Options.Temperature)
		}
		fmt.Println()
	}
}

func formatFields(fields []FieldSpec) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	return strings.Join(names, ", ")
}

func displayResults(prediction *dsgo.Prediction) {
	outputs := prediction.Outputs

	for key, value := range outputs {
		fmt.Printf("📌 %s:\n", key)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("%v\n", value)
		fmt.Println()
	}
}
