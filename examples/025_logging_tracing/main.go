package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/logging"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "025_logging_tracing", runExample)
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

	var totalTokens int
	var lastPred *dsgo.Prediction

	fmt.Println("=== Logging & Tracing Example ===")
	fmt.Println("Demonstrates DSGo's built-in logging and Request ID tracing for observability")
	fmt.Println()

	fmt.Println("--- Logging Features ---")
	fmt.Println("✓ Automatic Request ID generation")
	fmt.Println("✓ Custom Request ID propagation")
	fmt.Println("✓ Structured logging with context")
	fmt.Println("✓ Configurable log levels (DEBUG, INFO, WARN, ERROR)")
	fmt.Println("✓ API request/response tracking")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 1: Basic logging with automatic Request ID
	fmt.Println("--- Example 1: Automatic Request ID ---")
	fmt.Println("Enable logging and observe auto-generated Request IDs")
	fmt.Println()

	// Enable logging at INFO level
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "The sentiment")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	// Forward automatically generates a Request ID
	inputs := map[string]any{
		"text": "This is great!",
	}

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		return nil, stats, fmt.Errorf("prediction failed: %w", err)
	}

	sentiment, _ := result.GetString("sentiment")
	fmt.Printf("\nSentiment: %s\n", sentiment)
	fmt.Printf("📊 Tokens used: %d\n", result.Usage.TotalTokens)
	fmt.Println("✓ Check the logs above - note the auto-generated Request ID")

	totalTokens += result.Usage.TotalTokens
	lastPred = result
	stats.Metadata["auto_request_id"] = true

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 2: Custom Request ID
	fmt.Println("--- Example 2: Custom Request ID ---")
	fmt.Println("Set your own Request ID for correlation across calls")
	fmt.Println()

	sig2 := dsgo.NewSignature("Classify the topic of the text").
		AddInput("text", dsgo.FieldTypeString, "The text to classify").
		AddClassOutput("topic", []string{"technology", "sports", "politics", "entertainment"}, "The topic")

	predict2 := module.NewPredict(sig2, lm)

	// Set a custom Request ID for tracing
	customID := "user-request-12345"
	ctx2 := logging.WithRequestID(ctx, customID)

	inputs2 := map[string]any{
		"text": "The new smartphone features an impressive AI chip.",
	}

	result2, err := predict2.Forward(ctx2, inputs2)
	if err != nil {
		return lastPred, stats, fmt.Errorf("prediction failed: %w", err)
	}

	topic, _ := result2.GetString("topic")
	fmt.Printf("\nTopic: %s\n", topic)
	fmt.Printf("📊 Tokens used: %d\n", result2.Usage.TotalTokens)
	fmt.Printf("✓ All logs should show Request ID: [%s]\n", customID)

	totalTokens += result2.Usage.TotalTokens
	lastPred = result2
	stats.Metadata["custom_request_id"] = customID

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 3: Multiple API calls with same Request ID
	fmt.Println("--- Example 3: Multiple Calls with Same Request ID ---")
	fmt.Println("Process multiple items with a shared Request ID for easy tracing")
	fmt.Println()

	sig3 := dsgo.NewSignature("Analyze sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

	predict3 := module.NewPredict(sig3, lm)

	// Use the same Request ID for multiple related API calls
	requestID := "batch-job-001"
	ctx3 := logging.WithRequestID(ctx, requestID)

	texts := []string{
		"I love this!",
		"This is terrible.",
		"It's okay, I guess.",
	}

	fmt.Printf("Processing %d texts with Request ID: [%s]\n", len(texts), requestID)

	for i, text := range texts {
		result, err := predict3.Forward(ctx3, map[string]any{"text": text})
		if err != nil {
			log.Printf("Error on text %d: %v", i+1, err)
			continue
		}
		sentiment, _ := result.GetString("sentiment")
		fmt.Printf("Text %d: %q → %s\n", i+1, text, sentiment)
		totalTokens += result.Usage.TotalTokens
		lastPred = result
	}

	fmt.Println("✓ All API calls above share the same Request ID for easy tracing")
	stats.Metadata["batch_size"] = len(texts)
	stats.Metadata["batch_request_id"] = requestID

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Example 4: Different log levels
	fmt.Println("--- Example 4: Log Levels ---")
	fmt.Println("Control log verbosity with different levels")
	fmt.Println()

	fmt.Println("Setting log level to DEBUG to see all logs:")
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelDebug))

	sig4 := dsgo.NewSignature("Simple task").
		AddInput("text", dsgo.FieldTypeString, "Input").
		AddOutput("summary", dsgo.FieldTypeString, "Output")

	predict4 := module.NewPredict(sig4, lm)

	ctx4 := logging.WithRequestID(ctx, "debug-example")
	result4, err := predict4.Forward(ctx4, map[string]any{"text": "Hello world"})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		totalTokens += result4.Usage.TotalTokens
		lastPred = result4
	}

	fmt.Println("\n✓ With DEBUG level, you see both DEBUG and INFO logs")
	fmt.Println("  - DEBUG logs show prediction start/end")
	fmt.Println("  - INFO logs show API request/response details")

	fmt.Println("\nSetting log level to WARN (suppress INFO and DEBUG):")
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelWarn))

	result5, err := predict4.Forward(ctx4, map[string]any{"text": "Hello again"})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		totalTokens += result5.Usage.TotalTokens
		lastPred = result5
	}

	fmt.Println("✓ With WARN level, you see minimal output (only warnings and errors)")
	stats.Metadata["log_levels_tested"] = []string{"INFO", "DEBUG", "WARN"}

	// Reset to NoOp to clean up
	logging.SetLogger(nil)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- Logging & Tracing Benefits ---")
	fmt.Println("✓ Automatic Request ID for distributed tracing")
	fmt.Println("✓ Custom Request IDs for correlation")
	fmt.Println("✓ Structured logs with contextual information")
	fmt.Println("✓ Performance and cost monitoring")
	fmt.Println("✓ Production debugging and observability")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("Logging & Tracing provides:")
	fmt.Println("  ✓ Request ID propagation for tracing")
	fmt.Println("  ✓ Configurable log levels")
	fmt.Println("  ✓ API usage metrics (tokens, latency)")
	fmt.Println("  ✓ Integration with external loggers")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("🔧 Total examples: 4\n")
	fmt.Println()

	stats.TokensUsed = totalTokens
	stats.Metadata["total_examples"] = 4

	return lastPred, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
