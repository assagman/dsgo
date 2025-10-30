package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/logging"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	fmt.Println("=== Logging & Tracing Example ===")

	// Example 1: Basic logging with automatic Request ID
	fmt.Println("--- Example 1: Automatic Request ID ---")
	basicLogging()

	fmt.Println("\n--- Example 2: Custom Request ID ---")
	customRequestID()

	fmt.Println("\n--- Example 3: Multiple API calls with same Request ID ---")
	multipleCallsWithRequestID()

	fmt.Println("\n--- Example 4: Different log levels ---")
	logLevels()
}

func basicLogging() {
	// Enable logging (by default, logging is disabled with NoOpLogger)
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

	// Create a simple sentiment analysis signature
	sig := dsgo.NewSignature("Analyze the sentiment of the given text").
		AddInput("text", dsgo.FieldTypeString, "The text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "The sentiment")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	// Forward automatically generates a Request ID
	ctx := context.Background()
	inputs := map[string]any{
		"text": "This is great!",
	}

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Result: %v\n", result.Outputs["sentiment"])
	fmt.Println("✓ Check the logs above - note the auto-generated Request ID")
}

func customRequestID() {
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

	sig := dsgo.NewSignature("Classify the topic of the text").
		AddInput("text", dsgo.FieldTypeString, "The text to classify").
		AddClassOutput("topic", []string{"technology", "sports", "politics", "entertainment"}, "The topic")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	// Set a custom Request ID for tracing
	customID := "user-request-12345"
	ctx := logging.WithRequestID(context.Background(), customID)

	inputs := map[string]any{
		"text": "The new smartphone features an impressive AI chip.",
	}

	result, err := predict.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Result: %v\n", result.Outputs["topic"])
	fmt.Printf("✓ All logs should show Request ID: [%s]\n", customID)
}

func multipleCallsWithRequestID() {
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

	sig := dsgo.NewSignature("Analyze sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	// Use the same Request ID for multiple related API calls
	requestID := "batch-job-001"
	ctx := logging.WithRequestID(context.Background(), requestID)

	texts := []string{
		"I love this!",
		"This is terrible.",
		"It's okay, I guess.",
	}

	fmt.Printf("Processing %d texts with Request ID: [%s]\n", len(texts), requestID)

	for i, text := range texts {
		result, err := predict.Forward(ctx, map[string]any{"text": text})
		if err != nil {
			log.Printf("Error on text %d: %v", i+1, err)
			continue
		}
		fmt.Printf("Text %d: %v → %v\n", i+1, text, result.Outputs["sentiment"])
	}

	fmt.Println("✓ All API calls above share the same Request ID for easy tracing")
}

func logLevels() {
	fmt.Println("Setting log level to DEBUG to see all logs:")
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelDebug))

	sig := dsgo.NewSignature("Simple task").
		AddInput("text", dsgo.FieldTypeString, "Input").
		AddOutput("summary", dsgo.FieldTypeString, "Output")

	lm := shared.GetLM(shared.GetModel())
	predict := module.NewPredict(sig, lm)

	ctx := logging.WithRequestID(context.Background(), "debug-example")
	_, err := predict.Forward(ctx, map[string]any{"text": "Hello world"})
	if err != nil {
		log.Printf("Error: %v", err)
	}

	fmt.Println("\n✓ With DEBUG level, you see both DEBUG and INFO logs")
	fmt.Println("  - DEBUG logs show prediction start/end")
	fmt.Println("  - INFO logs show API request/response details")

	fmt.Println("\nSetting log level to WARN (suppress INFO and DEBUG):")
	logging.SetLogger(logging.NewDefaultLogger(logging.LevelWarn))

	_, err = predict.Forward(ctx, map[string]any{"text": "Hello again"})
	if err != nil {
		log.Printf("Error: %v", err)
	}

	fmt.Println("✓ With WARN level, you see minimal output (only warnings and errors)")

	// Reset to NoOp to clean up
	logging.SetLogger(nil)
}
