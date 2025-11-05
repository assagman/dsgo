package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/typed"
)

type SentimentInput struct {
	Text string `dsgo:"input,desc=Text to analyze for sentiment"`
}

type SentimentOutput struct {
	Sentiment string `dsgo:"output,enum=positive|negative|neutral,desc=The detected sentiment"`
	Score     int    `dsgo:"output,desc=Confidence score from 0 to 100"`
}

type TranslateInput struct {
	Text   string `dsgo:"input,desc=Text to translate"`
	Target string `dsgo:"input,desc=Target language code (e.g., es, fr, de)"`
}

type TranslateOutput struct {
	Translation string `dsgo:"output,desc=Translated text"`
}

type QAInput struct {
	Context  string `dsgo:"input,desc=Context to answer from"`
	Question string `dsgo:"input,desc=Question to answer"`
}

type QAOutput struct {
	Answer     string `dsgo:"output,desc=The answer to the question"`
	Confidence string `dsgo:"output,enum=high|medium|low,desc=Confidence level"`
}

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "010_typed_signatures", runExample)
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

	// Example 1: Basic Sentiment Analysis
	fmt.Println("1. Basic Sentiment Analysis (Type-Safe)")
	fmt.Println("-----------------------------------")

	sentimentFunc, err := typed.NewPredict[SentimentInput, SentimentOutput](lm)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to create sentiment function: %w", err)
	}

	sentiment, pred, err := sentimentFunc.RunWithPrediction(ctx, SentimentInput{
		Text: "I absolutely love this new feature! It's incredibly useful.",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("sentiment analysis failed: %w", err)
	}

	fmt.Printf("Sentiment: %s (Score: %d/100)\n", sentiment.Sentiment, sentiment.Score)
	totalTokens += pred.Usage.TotalTokens

	// Example 2: Translation with Few-Shot Examples
	fmt.Println("\n2. Translation with Typed Few-Shot Examples")
	fmt.Println("-----------------------------------")

	translateFunc, err := typed.NewPredict[TranslateInput, TranslateOutput](lm)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to create translation function: %w", err)
	}

	inputs := []TranslateInput{
		{Text: "Hello", Target: "es"},
		{Text: "Goodbye", Target: "fr"},
	}
	outputs := []TranslateOutput{
		{Translation: "Hola"},
		{Translation: "Au revoir"},
	}

	translateFunc, err = translateFunc.WithDemosTyped(inputs, outputs)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to add demos: %w", err)
	}

	translation, pred, err := translateFunc.RunWithPrediction(ctx, TranslateInput{
		Text:   "Good morning",
		Target: "de",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("translation failed: %w", err)
	}

	fmt.Printf("Translation: %s\n", translation.Translation)
	totalTokens += pred.Usage.TotalTokens

	// Example 3: Question Answering
	fmt.Println("\n3. Question Answering with Metadata")
	fmt.Println("-----------------------------------")

	qaFunc, err := typed.NewPredict[QAInput, QAOutput](lm)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to create QA function: %w", err)
	}

	qaContext := `The Eiffel Tower is a wrought-iron lattice tower on the Champ de Mars in Paris, France. 
It is named after the engineer Gustave Eiffel, whose company designed and built the tower. 
Constructed from 1887 to 1889, it has become a global cultural icon of France.`

	qa, pred, err := qaFunc.RunWithPrediction(ctx, QAInput{
		Context:  qaContext,
		Question: "Who designed the Eiffel Tower?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("QA failed: %w", err)
	}

	fmt.Printf("Answer: %s\n", qa.Answer)
	fmt.Printf("Confidence: %s\n", qa.Confidence)
	fmt.Printf("Tokens used: %d\n", pred.Usage.TotalTokens)
	totalTokens += pred.Usage.TotalTokens

	// Example 4: Chain of Thought with Typed Signatures
	fmt.Println("\n4. Chain of Thought with Typed Signatures")
	fmt.Println("-----------------------------------")

	cotFunc, err := typed.NewCoT[QAInput, QAOutput](lm)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to create CoT function: %w", err)
	}

	mathContext := `A company has 120 employees. 60% work in engineering, 25% in sales, and the rest in operations.`

	cotAnswer, pred, err := cotFunc.RunWithPrediction(ctx, QAInput{
		Context:  mathContext,
		Question: "How many people work in operations?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("CoT failed: %w", err)
	}

	fmt.Printf("Question: How many people work in operations?\n")
	fmt.Printf("Answer: %s\n", cotAnswer.Answer)
	fmt.Printf("Confidence: %s\n", cotAnswer.Confidence)
	if pred.Rationale != "" {
		fmt.Printf("Reasoning: %s\n", pred.Rationale)
	}
	totalTokens += pred.Usage.TotalTokens

	// Example 5: ReAct Agent with Typed Signatures
	fmt.Println("\n5. ReAct Agent with Typed Signatures")
	fmt.Println("-----------------------------------")

	searchTool := dsgo.NewTool(
		"search",
		"Search for information about a topic",
		func(ctx context.Context, args map[string]any) (any, error) {
			query := args["query"].(string)
			if query == "capital of France" {
				return "Paris is the capital and largest city of France.", nil
			}
			return "No results found.", nil
		},
	).AddParameter("query", "string", "The search query", true)

	reactFunc, err := typed.NewReAct[QAInput, QAOutput](lm, []dsgo.Tool{*searchTool})
	if err != nil {
		return nil, stats, fmt.Errorf("failed to create ReAct function: %w", err)
	}

	reactFunc.WithMaxIterations(5).WithVerbose(false)

	reactAnswer, pred, err := reactFunc.RunWithPrediction(ctx, QAInput{
		Context:  "Use available tools to answer the question.",
		Question: "What is the capital of France?",
	})
	if err != nil {
		return nil, stats, fmt.Errorf("ReAct failed: %w", err)
	}

	fmt.Printf("Question: What is the capital of France?\n")
	fmt.Printf("Answer: %s\n", reactAnswer.Answer)
	fmt.Printf("Confidence: %s\n", reactAnswer.Confidence)
	totalTokens += pred.Usage.TotalTokens

	stats.TokensUsed = totalTokens
	stats.Metadata["total_examples"] = 5
	stats.Metadata["sentiment_score"] = sentiment.Score
	stats.Metadata["translation"] = translation.Translation
	stats.Metadata["qa_confidence"] = qa.Confidence

	fmt.Printf("\n📊 Type-Safe API Examples:\n")
	fmt.Printf("  Total examples executed: 5\n")
	fmt.Printf("  Total tokens used: %d\n", totalTokens)
	fmt.Printf("  ✅ All typed signature examples completed successfully!\n")

	return pred, stats, nil
}
