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

	err := h.Run(context.Background(), "020_streaming", runExample)
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

	fmt.Println("=== Streaming Demo ===")
	fmt.Println("Demonstrating real-time streaming output for better UX")
	fmt.Println()

	fmt.Println("--- Streaming Features ---")
	fmt.Println("✓ Real-time output - see responses as they're generated")
	fmt.Println("✓ Better user experience - no waiting for complete response")
	fmt.Println("✓ Chunk-by-chunk processing - process data incrementally")
	fmt.Println("✓ Early termination - cancel streams when needed")
	fmt.Println("✓ Progress feedback - show users that work is happening")
	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 1: Basic streaming with story generation
	fmt.Println("--- Demo 1: Story Generation (Streaming) ---")
	fmt.Println("Generating a creative story in real-time...")
	fmt.Println()

	sig := dsgo.NewSignature("Generate a creative short story based on the given prompt").
		AddInput("prompt", dsgo.FieldTypeString, "Story prompt or theme").
		AddOutput("story", dsgo.FieldTypeString, "The generated story (max 500 words)").
		AddOutput("title", dsgo.FieldTypeString, "A catchy title for the story").
		AddOutput("genre", dsgo.FieldTypeString, "The story genre")

	predict := module.NewPredict(sig, lm).
		WithOptions(&dsgo.GenerateOptions{
			MaxTokens:   2000,
			Temperature: 0.7,
		})

	result, err := predict.Stream(ctx, map[string]any{
		"prompt": "A lone astronaut discovers an ancient alien artifact on Mars",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("failed to start streaming: %w", err)
	}

	fmt.Println("📝 Streaming output:")
	fmt.Println()

	chunkCount := 0
	// Process chunks in real-time
	for chunk := range result.Chunks {
		fmt.Print(chunk.Content)
		chunkCount++

		if chunk.FinishReason != "" {
			fmt.Printf("\n\n[Stream finished: %s | %d chunks]\n", chunk.FinishReason, chunkCount)
		}
	}

	// Check for streaming errors
	select {
	case err := <-result.Errors:
		if err != nil {
			return nil, stats, fmt.Errorf("streaming error: %w", err)
		}
	default:
	}

	// Wait for final parsed prediction
	prediction := <-result.Prediction

	fmt.Println()
	fmt.Println("--- Parsed Structured Output ---")
	title, _ := prediction.GetString("title")
	genre, _ := prediction.GetString("genre")
	story, _ := prediction.GetString("story")

	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Genre: %s\n", genre)
	fmt.Printf("Story Length: %d characters\n", len(story))
	fmt.Printf("Chunks Received: %d\n", chunkCount)
	fmt.Printf("📊 Tokens used: %d\n", prediction.Usage.TotalTokens)

	totalTokens += prediction.Usage.TotalTokens

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 2: Streaming vs Non-Streaming comparison
	fmt.Println("--- Demo 2: Streaming vs Non-Streaming ---")
	fmt.Println("Comparing user experience...")
	fmt.Println()

	fmt.Println("Non-Streaming:")
	fmt.Println("  • User waits for entire response (5-10 seconds)")
	fmt.Println("  • No feedback during generation")
	fmt.Println("  • Entire response appears at once")
	fmt.Println()

	fmt.Println("Streaming:")
	fmt.Println("  • Immediate feedback (first chunk in <1 second)")
	fmt.Println("  • Progressive output visible")
	fmt.Println("  • User can start reading while generation continues")
	fmt.Println("  • Better perceived performance")
	fmt.Println()

	fmt.Println("✅ Streaming provides better UX")

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	// Demo 3: Streaming with Question Answering
	fmt.Println("--- Demo 3: Q&A Streaming ---")
	fmt.Println("Stream detailed explanation in real-time...")
	fmt.Println()

	qaSig := dsgo.NewSignature("Provide a detailed explanation to the given question").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("explanation", dsgo.FieldTypeString, "Detailed explanation").
		AddOutput("summary", dsgo.FieldTypeString, "Brief summary")

	qaPredict := module.NewPredict(qaSig, lm).
		WithOptions(&dsgo.GenerateOptions{
			MaxTokens:   1500,
			Temperature: 0.5,
		})

	qaResult, err := qaPredict.Stream(ctx, map[string]any{
		"question": "How does photosynthesis work at the molecular level?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("failed to start Q&A streaming: %w", err)
	}

	fmt.Println("🌱 Streaming explanation:")
	fmt.Println()

	qaChunkCount := 0
	for chunk := range qaResult.Chunks {
		fmt.Print(chunk.Content)
		qaChunkCount++

		if chunk.FinishReason != "" {
			fmt.Printf("\n\n[Stream finished: %s | %d chunks]\n", chunk.FinishReason, qaChunkCount)
		}
	}

	select {
	case err := <-qaResult.Errors:
		if err != nil {
			return nil, stats, fmt.Errorf("Q&A streaming error: %w", err)
		}
	default:
	}

	qaPred := <-qaResult.Prediction

	fmt.Println()
	fmt.Println("--- Structured Q&A Output ---")
	explanation, _ := qaPred.GetString("explanation")
	summary, _ := qaPred.GetString("summary")

	fmt.Printf("Explanation length: %d characters\n", len(explanation))
	fmt.Printf("Summary: %s\n", summary)
	fmt.Printf("📊 Tokens used: %d\n", qaPred.Usage.TotalTokens)

	totalTokens += qaPred.Usage.TotalTokens

	stats.TokensUsed = totalTokens
	stats.Metadata["total_demos"] = 3
	stats.Metadata["total_chunks"] = chunkCount + qaChunkCount
	stats.Metadata["story_length"] = len(story)

	fmt.Println()
	fmt.Println(repeatChar("─", 80))
	fmt.Println()

	fmt.Println("--- Use Cases for Streaming ---")
	fmt.Println("1. **Interactive Chat**: Show responses as they're typed")
	fmt.Println("2. **Long Content**: Stories, articles, reports")
	fmt.Println("3. **Code Generation**: Display code as it's written")
	fmt.Println("4. **Real-time Analysis**: Stream reasoning steps")
	fmt.Println("5. **User Feedback**: Keep users engaged during long operations")
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("Streaming capabilities:")
	fmt.Println("  ✓ Real-time output improves perceived performance")
	fmt.Println("  ✓ Chunk-by-chunk processing enables progressive rendering")
	fmt.Println("  ✓ Works with all modules (Predict, CoT, ReAct, etc.)")
	fmt.Println("  ✓ Error handling via result.Errors channel")
	fmt.Println("  ✓ Final structured output via result.Prediction channel")
	fmt.Println()
	fmt.Printf("📊 Total tokens used: %d\n", totalTokens)
	fmt.Printf("📦 Total chunks received: %d\n", chunkCount+qaChunkCount)
	fmt.Printf("🔧 Total demos: 3\n")
	fmt.Println()

	return qaPred, stats, nil
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
