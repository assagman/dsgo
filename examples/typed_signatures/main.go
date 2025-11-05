package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/typed"
)

// SentimentInput defines the input structure for sentiment analysis
type SentimentInput struct {
	Text string `dsgo:"input,desc=Text to analyze for sentiment"`
}

// SentimentOutput defines the output structure for sentiment analysis
type SentimentOutput struct {
	Sentiment string `dsgo:"output,enum=positive|negative|neutral,desc=The detected sentiment"`
	Score     int    `dsgo:"output,desc=Confidence score from 0 to 100"`
}

// TranslateInput defines the input for translation
type TranslateInput struct {
	Text   string `dsgo:"input,desc=Text to translate"`
	Target string `dsgo:"input,desc=Target language code (e.g., es, fr, de)"`
}

// TranslateOutput defines the output for translation
type TranslateOutput struct {
	Translation string `dsgo:"output,desc=Translated text"`
}

// QAInput defines the input for question answering
type QAInput struct {
	Context  string `dsgo:"input,desc=Context to answer from"`
	Question string `dsgo:"input,desc=Question to answer"`
}

// QAOutput defines the output for question answering
type QAOutput struct {
	Answer     string `dsgo:"output,desc=The answer to the question"`
	Confidence string `dsgo:"output,enum=high|medium|low,desc=Confidence level"`
}

func main() {
	shared.LoadEnv()

	ctx := context.Background()

	// Get LM from shared
	lm := shared.GetLM(shared.GetModel())

	fmt.Println("=== DSGo Typed Signatures Demo ===")

	// Example 1: Sentiment Analysis
	fmt.Println("1. Sentiment Analysis (Type-Safe)")
	fmt.Println("-----------------------------------")

	sentimentFunc, err := typed.NewPredict[SentimentInput, SentimentOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create sentiment function: %v", err)
	}

	// Type-safe input
	sentiment, err := sentimentFunc.Run(ctx, SentimentInput{
		Text: "I absolutely love this new feature! It's incredibly useful.",
	})
	if err != nil {
		log.Fatalf("Sentiment analysis failed: %v", err)
	}

	// Type-safe output access
	fmt.Printf("Sentiment: %s (Score: %d/100)\n\n", sentiment.Sentiment, sentiment.Score)

	// Example 2: Translation with Few-Shot Examples
	fmt.Println("2. Translation with Typed Few-Shot Examples")
	fmt.Println("-----------------------------------")

	translateFunc, err := typed.NewPredict[TranslateInput, TranslateOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create translation function: %v", err)
	}

	// Type-safe few-shot examples
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
		log.Fatalf("Failed to add demos: %v", err)
	}

	translation, err := translateFunc.Run(ctx, TranslateInput{
		Text:   "Good morning",
		Target: "de",
	})
	if err != nil {
		log.Fatalf("Translation failed: %v", err)
	}

	fmt.Printf("Translation: %s\n\n", translation.Translation)

	// Example 3: Question Answering with Prediction Metadata
	fmt.Println("3. Question Answering with Metadata")
	fmt.Println("-----------------------------------")

	qaFunc, err := typed.NewPredict[QAInput, QAOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create QA function: %v", err)
	}

	qaContext := `The Eiffel Tower is a wrought-iron lattice tower on the Champ de Mars in Paris, France. 
It is named after the engineer Gustave Eiffel, whose company designed and built the tower. 
Constructed from 1887 to 1889, it has become a global cultural icon of France.`

	qa, pred, err := qaFunc.RunWithPrediction(ctx, QAInput{
	Context:  qaContext,
	Question: "Who designed the Eiffel Tower?",
	})
	if err != nil {
		log.Fatalf("QA failed: %v", err)
	}

	fmt.Printf("Answer: %s\n", qa.Answer)
	fmt.Printf("Confidence: %s\n", qa.Confidence)
	fmt.Printf("Tokens used: %d\n", pred.Usage.TotalTokens)
	fmt.Printf("Cost: $%.6f\n\n", pred.Usage.Cost)

	// Example 4: Custom Options
	fmt.Println("4. Typed Function with Custom Options")
	fmt.Println("-----------------------------------")

	customFunc, err := typed.NewPredictWithDescription[SentimentInput, SentimentOutput](
	lm, 
	"Advanced sentiment analyzer with confidence scoring",
	)
	if err != nil {
	log.Fatalf("Failed to create custom function: %v", err)
	}

	customFunc.WithOptions(&dsgo.GenerateOptions{
		Temperature: 0.3, // Lower temperature for more consistent results
		MaxTokens:   100,
	})

	result, err := customFunc.Run(ctx, SentimentInput{
		Text: "This is a mixed review. Some parts are good, others not so much.",
	})
	if err != nil {
		log.Fatalf("Custom function failed: %v", err)
	}

	fmt.Printf("Sentiment: %s (Score: %d/100)\n", result.Sentiment, result.Score)

	// Example 5: Multi-turn Conversation with History
	fmt.Println("\n5. Multi-turn Conversation with History")
	fmt.Println("-----------------------------------")

	history := dsgo.NewHistoryWithLimit(10) // Keep last 10 messages

	chatFunc, err := typed.NewPredict[QAInput, QAOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create chat function: %v", err)
	}
	chatFunc.WithHistory(history)

	conversationContext := `You are helping a user learn about DSGo, a Go port of DSPy for building LM programs.`

	// First turn
	answer1, err := chatFunc.Run(ctx, QAInput{
		Context:  conversationContext,
		Question: "What is DSGo used for?",
	})
	if err != nil {
		log.Fatalf("Chat turn 1 failed: %v", err)
	}
	fmt.Printf("Q1: What is DSGo used for?\n")
	fmt.Printf("A1: %s\n\n", answer1.Answer)

	// Second turn - builds on previous context
	answer2, err := chatFunc.Run(ctx, QAInput{
		Context:  conversationContext,
		Question: "Can you give me an example of using it?",
	})
	if err != nil {
		log.Fatalf("Chat turn 2 failed: %v", err)
	}
	fmt.Printf("Q2: Can you give me an example of using it?\n")
	fmt.Printf("A2: %s\n\n", answer2.Answer)

	// Third turn - references earlier conversation
	answer3, err := chatFunc.Run(ctx, QAInput{
		Context:  conversationContext,
		Question: "What about the example you just mentioned?",
	})
	if err != nil {
		log.Fatalf("Chat turn 3 failed: %v", err)
	}
	fmt.Printf("Q3: What about the example you just mentioned?\n")
	fmt.Printf("A3: %s\n", answer3.Answer)
	fmt.Printf("(History contains %d messages)\n", history.Len())

	// Example 6: Chain of Thought Reasoning with Typed Signatures
	fmt.Println("\n6. Chain of Thought with Typed Signatures")
	fmt.Println("-----------------------------------")

	cotFunc, err := typed.NewCoT[QAInput, QAOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create CoT function: %v", err)
	}

	mathContext := `A company has 120 employees. 60% work in engineering, 25% in sales, and the rest in operations.`

	cotAnswer, cotPred, err := cotFunc.RunWithPrediction(ctx, QAInput{
		Context:  mathContext,
		Question: "How many people work in operations?",
	})
	if err != nil {
		log.Fatalf("CoT failed: %v", err)
	}

	fmt.Printf("Question: How many people work in operations?\n")
	fmt.Printf("Answer: %s\n", cotAnswer.Answer)
	fmt.Printf("Confidence: %s\n", cotAnswer.Confidence)
	if cotPred.Rationale != "" {
		fmt.Printf("Reasoning: %s\n", cotPred.Rationale)
	}

	// Example 7: ReAct Agent with Typed Signatures
	fmt.Println("\n7. ReAct Agent with Typed Signatures")
	fmt.Println("-----------------------------------")

	// Define a simple search tool
	searchTool := dsgo.NewTool(
		"search",
		"Search for information about a topic",
		func(ctx context.Context, args map[string]any) (any, error) {
			query := args["query"].(string)
			// Simulate search results
			if query == "capital of France" {
				return "Paris is the capital and largest city of France.", nil
			}
			return "No results found.", nil
		},
	).AddParameter("query", "string", "The search query", true)

	reactFunc, err := typed.NewReAct[QAInput, QAOutput](lm, []dsgo.Tool{*searchTool})
	if err != nil {
		log.Fatalf("Failed to create ReAct function: %v", err)
	}

	reactFunc.WithMaxIterations(5).WithVerbose(false)

	reactAnswer, err := reactFunc.Run(ctx, QAInput{
		Context:  "Use available tools to answer the question.",
		Question: "What is the capital of France?",
	})
	if err != nil {
		log.Fatalf("ReAct failed: %v", err)
	}

	fmt.Printf("Question: What is the capital of France?\n")
	fmt.Printf("Answer: %s\n", reactAnswer.Answer)
	fmt.Printf("Confidence: %s\n", reactAnswer.Confidence)

	fmt.Println("\n✓ All typed signature examples completed successfully!")
}
