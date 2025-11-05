package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "018_adapters", runExample)
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

	fmt.Println("=== Adapter Comparison Demo ===")
	fmt.Println("Demonstrates all adapter types and their use cases")
	fmt.Println()

	// Demo 1: JSONAdapter - Structured JSON outputs
	fmt.Println("--- Demo 1: JSONAdapter (Structured JSON) ---")
	tokens1, _, err := jsonAdapterDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("json adapter demo failed: %w", err)
	}
	totalTokens += tokens1

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: ChatAdapter - Field marker format
	fmt.Println("--- Demo 2: ChatAdapter (Field Markers) ---")
	tokens2, _, err := chatAdapterDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("chat adapter demo failed: %w", err)
	}
	totalTokens += tokens2

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: FallbackAdapter - Automatic fallback chain
	fmt.Println("--- Demo 3: FallbackAdapter (Automatic Fallback) ---")
	tokens3, _, err := fallbackAdapterDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("fallback adapter demo failed: %w", err)
	}
	totalTokens += tokens3

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 4: Custom adapter configuration
	fmt.Println("--- Demo 4: Custom Adapter Configuration ---")
	tokens4, pred4, err := customAdapterDemo(ctx, lm)
	if err != nil {
		return nil, stats, fmt.Errorf("custom adapter demo failed: %w", err)
	}
	totalTokens += tokens4

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 4
	stats.Metadata["adapters_demonstrated"] = []string{
		"JSONAdapter",
		"ChatAdapter",
		"FallbackAdapter",
		"Custom Configuration",
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Adapter capabilities:\n")
	fmt.Printf("  ✓ JSONAdapter: Reliable structured JSON parsing\n")
	fmt.Printf("  ✓ ChatAdapter: Flexible field marker format\n")
	fmt.Printf("  ✓ FallbackAdapter: Automatic retry with multiple adapters\n")
	fmt.Printf("  ✓ Custom adapters: Tailor to your LM's capabilities\n")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total demos: 4\n")
	fmt.Println()

	return pred4, stats, nil
}

func jsonAdapterDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	fmt.Println("JSONAdapter expects LMs to return structured JSON responses.")
	fmt.Println("Best for: Models good at JSON (GPT-4, Claude, etc.)")
	fmt.Println()

	// Create signature
	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score (0-1)")

	// Create Predict module with explicit JSONAdapter
	jsonAdapter := dsgo.NewJSONAdapter()
	predict := module.NewPredict(sig, lm).WithAdapter(jsonAdapter)

	inputs := map[string]any{
		"text": "This product exceeded my expectations! Great quality and fast shipping.",
	}

	fmt.Printf("Input: %s\n\n", inputs["text"])

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, fmt.Errorf("prediction failed: %w", err)
	}

	sentiment, _ := result.GetString("sentiment")
	confidence, _ := result.GetFloat("confidence")

	fmt.Printf("Output Format: JSON\n")
	fmt.Printf("Sentiment: %s\n", sentiment)
	fmt.Printf("Confidence: %.2f\n", confidence)
	fmt.Printf("\n✅ JSONAdapter used successfully\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func chatAdapterDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	fmt.Println("ChatAdapter uses field markers: [[ ## field_name ## ]]")
	fmt.Println("Best for: Models that struggle with JSON, conversational models")
	fmt.Println()

	// Create signature
	sig := dsgo.NewSignature("Generate a creative story title and opening").
		AddInput("theme", dsgo.FieldTypeString, "The story theme").
		AddOutput("title", dsgo.FieldTypeString, "Story title").
		AddOutput("opening", dsgo.FieldTypeString, "First paragraph")

	// Create Predict module with explicit ChatAdapter
	chatAdapter := dsgo.NewChatAdapter()
	predict := module.NewPredict(sig, lm).WithAdapter(chatAdapter)

	inputs := map[string]any{
		"theme": "a robot discovering emotions",
	}

	fmt.Printf("Input: %s\n\n", inputs["theme"])

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, fmt.Errorf("prediction failed: %w", err)
	}

	title, _ := result.GetString("title")
	opening, _ := result.GetString("opening")

	fmt.Printf("Output Format: Field Markers\n")
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Opening: %s\n", opening)
	fmt.Printf("\n✅ ChatAdapter used successfully\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func fallbackAdapterDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	fmt.Println("FallbackAdapter tries ChatAdapter first, then JSONAdapter if that fails.")
	fmt.Println("Best for: Production use - maximizes success rate across different LM responses")
	fmt.Println()

	// Create signature
	sig := dsgo.NewSignature("Extract key information from the product description").
		AddInput("description", dsgo.FieldTypeString, "Product description").
		AddOutput("product_name", dsgo.FieldTypeString, "Name of the product").
		AddOutput("category", dsgo.FieldTypeString, "Product category").
		AddOutput("price_range", dsgo.FieldTypeString, "Price range")

	// FallbackAdapter is the DEFAULT - no need to set it explicitly
	// But we can create it explicitly to show the pattern
	fallbackAdapter := dsgo.NewFallbackAdapter()
	predict := module.NewPredict(sig, lm).WithAdapter(fallbackAdapter)

	inputs := map[string]any{
		"description": "MacBook Pro 16-inch with M3 Max chip, 36GB RAM, 1TB SSD. Professional laptop for developers and creators. Starting at $3,499.",
	}

	fmt.Printf("Input: %s\n\n", inputs["description"])

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, fmt.Errorf("prediction failed: %w", err)
	}

	productName, _ := result.GetString("product_name")
	category, _ := result.GetString("category")
	priceRange, _ := result.GetString("price_range")

	fmt.Printf("Output Format: Automatic Fallback\n")
	fmt.Printf("Product Name: %s\n", productName)
	fmt.Printf("Category: %s\n", category)
	fmt.Printf("Price Range: %s\n", priceRange)
	if result.AdapterUsed != "" {
		fmt.Printf("\n[Adapter Metrics]\n")
		fmt.Printf("  Adapter Used: %s\n", result.AdapterUsed)
		fmt.Printf("  Parse Attempts: %d\n", result.ParseAttempts)
		fmt.Printf("  Fallback Used: %v\n", result.FallbackUsed)
	}
	fmt.Printf("\n✅ FallbackAdapter handled response automatically\n")
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)

	return result.Usage.TotalTokens, result, nil
}

func customAdapterDemo(ctx context.Context, lm dsgo.LM) (int, *dsgo.Prediction, error) {
	fmt.Println("Custom adapter configuration: You can build custom fallback chains.")
	fmt.Println("Example: Try JSONAdapter first, then ChatAdapter")
	fmt.Println()

	// Create signature
	sig := dsgo.NewSignature("Classify the programming language").
		AddInput("code_snippet", dsgo.FieldTypeString, "Code snippet").
		AddClassOutput("language", []string{"python", "go", "javascript", "rust", "java"}, "Programming language").
		AddOutput("reasoning", dsgo.FieldTypeString, "Why this classification")

	// Custom fallback chain: JSON first, then Chat
	customAdapter := dsgo.NewFallbackAdapterWithChain(
		dsgo.NewJSONAdapter(),
		dsgo.NewChatAdapter(),
	)

	predict := module.NewPredict(sig, lm).WithAdapter(customAdapter)

	inputs := map[string]any{
		"code_snippet": "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
	}

	fmt.Printf("Input:\n%s\n\n", inputs["code_snippet"])

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return 0, nil, fmt.Errorf("prediction failed: %w", err)
	}

	language, _ := result.GetString("language")
	reasoning, _ := result.GetString("reasoning")

	fmt.Printf("Output Format: Custom Chain (JSON → Chat)\n")
	fmt.Printf("Language: %s\n", language)
	fmt.Printf("Reasoning: %s\n", reasoning)
	if result.AdapterUsed != "" {
		fmt.Printf("\n[Adapter Metrics]\n")
		fmt.Printf("  Adapter Used: %s\n", result.AdapterUsed)
	}
	fmt.Printf("\n✅ Custom adapter chain configured successfully\n")
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
