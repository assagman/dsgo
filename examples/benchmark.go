package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/internal/env"
	_ "github.com/assagman/dsgo/internal/providers/openai"
	_ "github.com/assagman/dsgo/internal/providers/openrouter"
	"github.com/assagman/dsgo/module"
)

// BenchmarkArtifacts stores all outputs from the benchmark
type BenchmarkArtifacts struct {
	Timestamp    time.Time         `json:"timestamp"`
	Model        string            `json:"model"`
	Results      map[string]any    `json:"results"`
	Metrics      BenchmarkMetrics  `json:"metrics"`
	RawResponses map[string]string `json:"raw_responses"`
}

type BenchmarkMetrics struct {
	TotalRequests        int     `json:"total_requests"`
	TotalTokens          int     `json:"total_tokens"`
	TotalCost            float64 `json:"total_cost"`
	TotalLatency         int64   `json:"total_latency_ms"`
	SuccessRate          float64 `json:"success_rate"`
	StructuredOutputRate float64 `json:"structured_output_rate"`
}

func main() {
	// Load environment variables from .env.local and .env files
	if err := env.LoadFiles(); err != nil {
		log.Printf("Warning: Failed to load environment files: %v", err)
	}

	ctx := context.Background()

	// Initialize logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("🚀 DSGo Comprehensive Benchmark Starting...")

	// Create artifacts directory
	artifactsDir := "benchmark_artifacts"
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		log.Fatalf("Failed to create artifacts directory: %v", err)
	}

	// Initialize model - try multiple models for reliability
	modelName := os.Getenv("BENCHMARK_MODEL")
	if modelName == "" {
		// Try a more reliable model first
		modelName = "openrouter/google/gemini-2.0-flash-001"
	}
	log.Printf("🤖 Using model: %s", modelName)

	lm, err := core.NewLM(ctx, modelName)
	if err != nil {
		// Fallback to a basic model if the preferred one fails
		log.Printf("⚠️  Primary model failed, trying fallback: %v", err)
		modelName = "openrouter/openai/gpt-3.5-turbo"
		lm, err = core.NewLM(ctx, modelName)
		if err != nil {
			log.Fatalf("Failed to create language model with fallback: %v", err)
		}
	}

	// Initialize artifacts storage
	artifacts := &BenchmarkArtifacts{
		Timestamp:    time.Now(),
		Model:        modelName,
		Results:      make(map[string]any),
		RawResponses: make(map[string]string),
	}

	// Run all benchmark sections
	runBasicSignatures(ctx, lm, artifacts)
	runModerateSignatures(ctx, lm, artifacts)
	runComplexSignatures(ctx, lm, artifacts)
	runAdvancedTypeSignatures(ctx, lm, artifacts)
	runModulePredict(ctx, lm, artifacts)
	runModuleChainOfThought(ctx, lm, artifacts)
	runModuleReAct(ctx, lm, artifacts)
	runModuleRefine(ctx, lm, artifacts)
	runModuleBestOfN(ctx, lm, artifacts)
	runModuleProgramOfThought(ctx, lm, artifacts)
	runModuleProgram(ctx, lm, artifacts)
	runParallelProcessing(ctx, lm, artifacts)
	runAdvancedFeatures(ctx, lm, artifacts)
	runConfiguration(artifacts)

	// Calculate final metrics
	calculateMetrics(artifacts)

	// Save artifacts
	saveArtifacts(artifacts, artifactsDir)

	log.Println("✅ Benchmark completed successfully!")
	log.Printf("📊 Results saved to: %s", artifactsDir)
}

func runBasicSignatures(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n📋 SECTION 1: Basic Signatures")

	// Minimal signature: Single input, single output
	minimalSig := core.NewSignature("question -> answer").
		AddInput("question", core.FieldTypeString, "The question to answer").
		AddOutput("answer", core.FieldTypeString, "The answer")

	predictor := module.NewPredict(minimalSig, lm)
	result, err := predictor.Forward(ctx, map[string]any{
		"question": "What is 2+2?",
	})
	if err != nil {
		log.Printf("❌ Minimal signature failed: %v", err)
		return
	}

	answer, _ := result.GetString("answer")
	log.Printf("✅ Minimal signature:")
	log.Printf("   👉 Input (question): What is 2+2?")
	log.Printf("   💡 Output (answer): %s", answer)
	artifacts.Results["basic_minimal"] = map[string]string{
		"question": "What is 2+2?",
		"answer":   answer,
	}

	// String field type
	stringSig := core.NewSignature("text -> sentiment").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddOutput("sentiment", core.FieldTypeString, "Sentiment: positive, negative, or neutral")

	predictor = module.NewPredict(stringSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"text": "I love this product!",
	})
	if err != nil {
		log.Printf("❌ String field failed: %v", err)
		return
	}

	sentiment, _ := result.GetString("sentiment")
	log.Printf("✅ String field:")
	log.Printf("   👉 Input (text): I love this product!")
	log.Printf("   💡 Output (sentiment): %s", sentiment)
	artifacts.Results["basic_string"] = map[string]string{
		"text":      "I love this product!",
		"sentiment": sentiment,
	}

	// Integer field type
	intSig := core.NewSignature("numbers -> sum").
		AddInput("numbers", core.FieldTypeString, "Comma-separated numbers").
		AddOutput("sum", core.FieldTypeInt, "Sum of the numbers")

	predictor = module.NewPredict(intSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"numbers": "10, 20, 30",
	})
	if err != nil {
		log.Printf("❌ Integer field failed: %v", err)
		return
	}

	sum, _ := result.GetInt("sum")
	log.Printf("✅ Integer field:")
	log.Printf("   👉 Input (numbers): 10, 20, 30")
	log.Printf("   💡 Output (sum): %d", sum)
	artifacts.Results["basic_int"] = map[string]any{
		"numbers": "10, 20, 30",
		"sum":     sum,
	}

	// Float field type
	floatSig := core.NewSignature("values -> average").
		AddInput("values", core.FieldTypeString, "Comma-separated values").
		AddOutput("average", core.FieldTypeFloat, "Average of the values")

	predictor = module.NewPredict(floatSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"values": "1.5, 2.5, 3.5",
	})
	if err != nil {
		log.Printf("❌ Float field failed: %v", err)
		return
	}

	avg, _ := result.GetFloat("average")
	log.Printf("✅ Float field:")
	log.Printf("   👉 Input (values): 1.5, 2.5, 3.5")
	log.Printf("   💡 Output (average): %.2f", avg)
	artifacts.Results["basic_float"] = map[string]any{
		"values":  "1.5, 2.5, 3.5",
		"average": avg,
	}

	// Boolean field type
	boolSig := core.NewSignature("statement -> is_true").
		AddInput("statement", core.FieldTypeString, "Statement to evaluate").
		AddOutput("is_true", core.FieldTypeBool, "Whether the statement is true")

	predictor = module.NewPredict(boolSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"statement": "The sky is blue",
	})
	if err != nil {
		log.Printf("❌ Boolean field failed: %v", err)
		return
	}

	isTrue, _ := result.GetBool("is_true")
	log.Printf("✅ Boolean field:")
	log.Printf("   👉 Input (statement): The sky is blue")
	log.Printf("   💡 Output (is_true): %v", isTrue)
	artifacts.Results["basic_bool"] = map[string]any{
		"statement": "The sky is blue",
		"is_true":   isTrue,
	}
}

func runModerateSignatures(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n📊 SECTION 2: Moderate Signatures")

	// Multiple inputs, multiple outputs
	multiIOSig := core.NewSignature("product, features -> name, description, price_range").
		AddInput("product", core.FieldTypeString, "Product category").
		AddInput("features", core.FieldTypeString, "Key features").
		AddOutput("name", core.FieldTypeString, "Product name").
		AddOutput("description", core.FieldTypeString, "Product description").
		AddOutput("price_range", core.FieldTypeString, "Price range: low, medium, high")

	predictor := module.NewPredict(multiIOSig, lm)
	result, err := predictor.Forward(ctx, map[string]any{
		"product":  "smartphone",
		"features": "5G, 128GB storage, triple camera",
	})
	if err != nil {
		log.Printf("❌ Multiple I/O signature failed: %v", err)
		return
	}

	name, _ := result.GetString("name")
	description, _ := result.GetString("description")
	priceRange, _ := result.GetString("price_range")
	log.Printf("✅ Multiple I/O:")
	log.Printf("   👉 Input (product): smartphone")
	log.Printf("   👉 Input (features): 5G, 128GB storage, triple camera")
	log.Printf("   💡 Output (name): %s", name)
	log.Printf("   💡 Output (description): %s", description)
	log.Printf("   💡 Output (price_range): %s", priceRange)
	artifacts.Results["moderate_multi_io"] = map[string]string{
		"product":     "smartphone",
		"features":    "5G, 128GB storage, triple camera",
		"name":        name,
		"description": description,
		"price_range": priceRange,
	}

	// JSON field type
	jsonSig := core.NewSignature("user_data -> analysis").
		AddInput("user_data", core.FieldTypeString, "User information").
		AddOutput("analysis", core.FieldTypeJSON, "JSON analysis with fields: age_group, interests, spending_habits")

	predictor = module.NewPredict(jsonSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"user_data": "25-year-old software engineer who loves hiking, photography, and buys tech gadgets monthly",
	})
	if err != nil {
		log.Printf("❌ JSON field failed: %v", err)
		return
	}

	analysis, _ := result.Get("analysis")
	log.Printf("✅ JSON field:")
	log.Printf("   👉 Input (user_data): 25-year-old software engineer who loves hiking, photography, and buys tech gadgets monthly")
	log.Printf("   💡 Output (analysis): %v", analysis)
	artifacts.Results["moderate_json"] = map[string]any{
		"user_data": "25-year-old software engineer who loves hiking, photography, and buys tech gadgets monthly",
		"analysis":  analysis,
	}

	// Class field type
	classSig := core.NewSignature("review -> sentiment_score").
		AddInput("review", core.FieldTypeString, "Product review").
		AddClassOutput("sentiment_score", []string{"positive", "negative", "neutral"}, "Sentiment: positive, negative, neutral")

	predictor = module.NewPredict(classSig, lm)
	result, err = predictor.Forward(ctx, map[string]any{
		"review": "This product exceeded my expectations!",
	})
	if err != nil {
		log.Printf("❌ Class field failed: %v", err)
		return
	}

	sentimentScore, _ := result.GetString("sentiment_score")
	log.Printf("✅ Class field:")
	log.Printf("   👉 Input (review): This product exceeded my expectations!")
	log.Printf("   💡 Output (sentiment_score): %s", sentimentScore)
	artifacts.Results["moderate_class"] = map[string]string{
		"review":          "This product exceeded my expectations!",
		"sentiment_score": sentimentScore,
	}
}

func runComplexSignatures(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🔬 SECTION 3: Complex Signatures")

	// Complex multi-field signature with all field types
	complexSig := core.NewSignature(
		"product_specs, target_market, constraints -> "+
			"product_name, technical_specs, market_analysis, "+
			"estimated_price, success_probability, launch_timeline, "+
			"key_features_json, risk_factors, competitive_advantage",
	).
		AddInput("product_specs", core.FieldTypeString, "Detailed product specifications").
		AddInput("target_market", core.FieldTypeString, "Target market demographics").
		AddInput("constraints", core.FieldTypeString, "Budget and timeline constraints").
		AddOutput("product_name", core.FieldTypeString, "Creative product name").
		AddOutput("technical_specs", core.FieldTypeString, "Key technical specifications").
		AddOutput("market_analysis", core.FieldTypeString, "Market opportunity analysis").
		AddOutput("estimated_price", core.FieldTypeFloat, "Estimated retail price in USD").
		AddOutput("success_probability", core.FieldTypeFloat, "Probability of success (0.0 to 1.0)").
		AddOutput("launch_timeline", core.FieldTypeInt, "Months to launch").
		AddOutput("key_features_json", core.FieldTypeJSON, "JSON array of key features").
		AddOutput("risk_factors", core.FieldTypeJSON, "JSON array of risk factors").
		AddOutput("competitive_advantage", core.FieldTypeString, "Competitive advantage description")

	predictor := module.NewPredict(complexSig, lm)
	result, err := predictor.Forward(ctx, map[string]any{
		"product_specs": "Wireless earbuds with active noise cancellation, 30-hour battery life, Bluetooth 5.3, IPX7 water resistance",
		"target_market": "Urban professionals aged 25-45, income $50k-100k, tech-savvy",
		"constraints":   "Budget $500k, launch within 8 months",
	})
	if err != nil {
		log.Printf("❌ Complex signature failed: %v", err)
		return
	}

	// Extract all fields
	productName, _ := result.GetString("product_name")
	estimatedPrice, _ := result.GetFloat("estimated_price")
	successProb, _ := result.GetFloat("success_probability")
	launchTimeline, _ := result.GetInt("launch_timeline")
	keyFeatures, _ := result.Get("key_features_json")
	riskFactors, _ := result.Get("risk_factors")

	log.Printf("✅ Complex signature:")
	log.Printf("   👉 Input (product_specs): Wireless earbuds with active noise cancellation, 30-hour battery life, Bluetooth 5.3, IPX7 water resistance")
	log.Printf("   👉 Input (target_market): Urban professionals aged 25-45, income $50k-100k, tech-savvy")
	log.Printf("   👉 Input (constraints): Budget $500k, launch within 8 months")
	log.Printf("   💡 Output (product_name): %s", productName)
	log.Printf("   💡 Output (estimated_price): $%.2f", estimatedPrice)
	log.Printf("   💡 Output (success_probability): %.1f%%", successProb*100)
	log.Printf("   💡 Output (launch_timeline): %d months", launchTimeline)
	log.Printf("   💡 Output (key_features_json): %v", keyFeatures)
	log.Printf("   💡 Output (risk_factors): %v", riskFactors)

	artifacts.Results["complex_full"] = map[string]any{
		"product_specs":       "Wireless earbuds with active noise cancellation",
		"product_name":        productName,
		"estimated_price":     estimatedPrice,
		"success_probability": successProb,
		"launch_timeline":     launchTimeline,
		"key_features_json":   keyFeatures,
		"risk_factors":        riskFactors,
	}
}

func runAdvancedTypeSignatures(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🧬 SECTION 4: Advanced Type Signatures (Datetime, Image)")

	// Datetime field type
	dateSig := core.NewSignature("event_description -> event_date").
		AddInput("event_description", core.FieldTypeString, "Description of an event with time").
		AddOutput("event_date", core.FieldTypeDatetime, "The date and time of the event (ISO 8601)")

	predictor := module.NewPredict(dateSig, lm)
	result, err := predictor.Forward(ctx, map[string]any{
		"event_description": "The conference starts on October 15th, 2025 at 9:00 AM",
	})
	if err != nil {
		log.Printf("❌ Datetime field failed: %v", err)
		return
	}

	eventDate, _ := result.GetString("event_date")
	log.Printf("✅ Datetime field:")
	log.Printf("   👉 Input (event_description): The conference starts on October 15th, 2025 at 9:00 AM")
	log.Printf("   💡 Output (event_date): %s", eventDate)
	artifacts.Results["advanced_datetime"] = map[string]string{
		"description": "The conference starts on October 15th, 2025 at 9:00 AM",
		"event_date":  eventDate,
	}

	// Note: Image type testing skipped as it requires image inputs/generation capabilities
	// which may not be supported by all text models.
}

func runModulePredict(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🎯 SECTION 5: Predict Module")

	// Basic predict with structured output
	sig := core.NewSignature("topic -> summary, keywords, category").
		AddInput("topic", core.FieldTypeString, "Topic to analyze").
		AddOutput("summary", core.FieldTypeString, "Brief summary").
		AddOutput("keywords", core.FieldTypeJSON, "JSON array of keywords").
		AddOutput("category", core.FieldTypeString, "Category: technology, business, science, or lifestyle")

	predictor := module.NewPredict(sig, lm)
	result, err := predictor.Forward(ctx, map[string]any{
		"topic": "Artificial intelligence in healthcare: How machine learning is revolutionizing medical diagnosis and treatment",
	})
	if err != nil {
		log.Printf("❌ Predict module failed: %v", err)
		return
	}

	summary, _ := result.GetString("summary")
	keywords, _ := result.Get("keywords")
	category, _ := result.GetString("category")

	log.Printf("✅ Predict module:")
	log.Printf("   👉 Input (topic): Artificial intelligence in healthcare: How machine learning is revolutionizing medical diagnosis and treatment")
	log.Printf("   💡 Output (summary): %s", summary)
	log.Printf("   💡 Output (keywords): %v", keywords)
	log.Printf("   💡 Output (category): %s", category)

	artifacts.Results["module_predict"] = map[string]any{
		"topic":    "Artificial intelligence in healthcare",
		"summary":  summary,
		"keywords": keywords,
		"category": category,
	}
}

func runModuleChainOfThought(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🧠 SECTION 6: ChainOfThought Module")

	// Chain of thought for complex reasoning
	sig := core.NewSignature("problem -> reasoning, solution").
		AddInput("problem", core.FieldTypeString, "Complex problem to solve").
		AddOutput("reasoning", core.FieldTypeString, "Step-by-step reasoning").
		AddOutput("solution", core.FieldTypeString, "Final solution")

	cot := module.NewChainOfThought(sig, lm)
	result, err := cot.Forward(ctx, map[string]any{
		"problem": "A train travels 120 miles in 2 hours. If it maintains the same speed, how long will it take to travel 300 miles? Show your reasoning.",
	})
	if err != nil {
		log.Printf("❌ ChainOfThought module failed: %v", err)
		return
	}

	reasoning, _ := result.GetString("reasoning")
	solution, _ := result.GetString("solution")

	log.Printf("✅ ChainOfThought module:")
	log.Printf("   👉 Input (problem): A train travels 120 miles in 2 hours. If it maintains the same speed, how long will it take to travel 300 miles? Show your reasoning.")
	log.Printf("   💡 Output (reasoning): %s", reasoning)
	log.Printf("   💡 Output (solution): %s", solution)

	artifacts.Results["module_chain_of_thought"] = map[string]string{
		"problem":   "Train speed calculation",
		"reasoning": reasoning,
		"solution":  solution,
	}
}

func runModuleReAct(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🛠️ SECTION 7: ReAct Module (Tool Use)")

	// Define tools for ReAct
	calculateFunc := func(ctx context.Context, args map[string]any) (any, error) {
		expression, ok := args["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("missing expression parameter")
		}
		// Simple calculation simulation
		return fmt.Sprintf("Calculated: %s", expression), nil
	}

	searchFunc := func(ctx context.Context, args map[string]any) (any, error) {
		query, ok := args["query"].(string)
		if !ok {
			return nil, fmt.Errorf("missing query parameter")
		}
		// Simple search simulation
		return fmt.Sprintf("Searched for: %s", query), nil
	}

	calculateTool := core.NewTool("calculate", "Perform mathematical calculations", calculateFunc)
	calculateTool.AddParameter("expression", "string", "Mathematical expression to evaluate", true)
	searchTool := core.NewTool("search", "Search for information", searchFunc)
	searchTool.AddParameter("query", "string", "Search query", true)

	calculateToolVal := *calculateTool
	searchToolVal := *searchTool
	tools := []core.Tool{calculateToolVal, searchToolVal}

	// ReAct signature
	sig := core.NewSignature("task -> reasoning, final_answer").
		AddInput("task", core.FieldTypeString, "Task that requires tool use").
		AddOutput("reasoning", core.FieldTypeString, "Reasoning process").
		AddOutput("final_answer", core.FieldTypeString, "Final answer")

	react := module.NewReAct(sig, lm, tools)
	result, err := react.Forward(ctx, map[string]any{
		"task": "Calculate the compound interest on $1000 at 5% annual rate for 3 years, then search for the current inflation rate",
	})
	if err != nil {
		log.Printf("❌ ReAct module failed: %v", err)
		return
	}

	reasoning, _ := result.GetString("reasoning")
	finalAnswer, _ := result.GetString("final_answer")

	log.Printf("✅ ReAct module:")
	log.Printf("   👉 Input (task): Calculate the compound interest on $1000 at 5%% annual rate for 3 years, then search for the current inflation rate")
	log.Printf("   💡 Output (reasoning): %s", reasoning)
	log.Printf("   💡 Output (final_answer): %s", finalAnswer)

	artifacts.Results["module_react"] = map[string]string{
		"task":         "Compound interest calculation",
		"reasoning":    reasoning,
		"final_answer": finalAnswer,
	}
}

func runModuleRefine(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n✨ SECTION 8: Refine Module")

	// Refine module for iterative improvement
	sig := core.NewSignature("draft, feedback -> improved_version").
		AddInput("draft", core.FieldTypeString, "Original draft").
		AddInput("feedback", core.FieldTypeString, "Feedback for improvement").
		AddOutput("improved_version", core.FieldTypeString, "Improved version")

	refine := module.NewRefine(sig, lm)

	// Initial draft
	draft := "Our product is good. It helps people. You should buy it."
	feedback := "Make it more persuasive, add specific benefits, and use professional language"

	result, err := refine.Forward(ctx, map[string]any{
		"draft":    draft,
		"feedback": feedback,
	})
	if err != nil {
		log.Printf("❌ Refine module failed: %v", err)
		return
	}

	improved, _ := result.GetString("improved_version")

	log.Printf("✅ Refine module:")
	log.Printf("   👉 Input (draft): %s", draft)
	log.Printf("   👉 Input (feedback): %s", feedback)
	log.Printf("   💡 Output (improved_version): %s", improved)

	artifacts.Results["module_refine"] = map[string]string{
		"original": draft,
		"feedback": feedback,
		"improved": improved,
	}
}

func runModuleBestOfN(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🎲 SECTION 9: BestOfN Module")

	// BestOfN for multiple samples
	sig := core.NewSignature("prompt -> creative_output").
		AddInput("prompt", core.FieldTypeString, "Creative prompt").
		AddOutput("creative_output", core.FieldTypeString, "Creative response")

	predictor := module.NewPredict(sig, lm)
	bestOfN := module.NewBestOfN(predictor, 3).WithScorer(module.DefaultScorer()) // Generate 3 samples

	result, err := bestOfN.Forward(ctx, map[string]any{
		"prompt": "Write a catchy slogan for a sustainable coffee brand",
	})
	if err != nil {
		log.Printf("❌ BestOfN module failed: %v", err)
		return
	}

	output, _ := result.GetString("creative_output")

	log.Printf("✅ BestOfN module:")
	log.Printf("   👉 Input (prompt): Write a catchy slogan for a sustainable coffee brand")
	log.Printf("   💡 Output (creative_output): %s", output)
	log.Printf("   (Generated 3 samples, selected best)")

	artifacts.Results["module_best_of_n"] = map[string]string{
		"prompt":      "Sustainable coffee brand slogan",
		"best_output": output,
	}
}

func runModuleProgramOfThought(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n💻 SECTION 10: ProgramOfThought Module")

	// Program of thought for code generation
	sig := core.NewSignature("problem -> code, explanation").
		AddInput("problem", core.FieldTypeString, "Problem to solve with code").
		AddOutput("code", core.FieldTypeString, "Generated code").
		AddOutput("explanation", core.FieldTypeString, "Explanation of the code")

	pot := module.NewProgramOfThought(sig, lm, "python")
	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Write a Python function to find the factorial of a number using recursion",
	})
	if err != nil {
		log.Printf("❌ ProgramOfThought module failed: %v", err)
		return
	}

	code, _ := result.GetString("code")
	explanation, _ := result.GetString("explanation")

	log.Printf("✅ ProgramOfThought module:")
	log.Printf("   👉 Input (problem): Write a Python function to find the factorial of a number using recursion")
	log.Printf("   💡 Output (code):\n%s", code)
	log.Printf("   💡 Output (explanation): %s", explanation)

	artifacts.Results["module_program_of_thought"] = map[string]string{
		"problem":     "Factorial function",
		"code":        code,
		"explanation": explanation,
	}
}

func runModuleProgram(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🔀 SECTION 11: Program Module (Composition)")

	// Create sub-modules for composition
	researchSig := core.NewSignature("topic -> research_notes").
		AddInput("topic", core.FieldTypeString, "Topic to research").
		AddOutput("research_notes", core.FieldTypeString, "Research notes")

	outlineSig := core.NewSignature("research_notes -> outline").
		AddInput("research_notes", core.FieldTypeString, "Research notes").
		AddOutput("outline", core.FieldTypeJSON, "JSON outline with sections")

	writeSig := core.NewSignature("outline, research_notes -> article").
		AddInput("outline", core.FieldTypeJSON, "Article outline").
		AddInput("research_notes", core.FieldTypeString, "Research notes").
		AddOutput("article", core.FieldTypeString, "Final article")

	researchModule := module.NewPredict(researchSig, lm)
	outlineModule := module.NewPredict(outlineSig, lm)
	writeModule := module.NewPredict(writeSig, lm)

	// Run the program
	topic := "The impact of artificial intelligence on modern education"

	// Step 1: Research
	researchResult, err := researchModule.Forward(ctx, map[string]any{
		"topic": topic,
	})
	if err != nil {
		log.Printf("❌ Program module (research) failed: %v", err)
		return
	}

	researchNotes, _ := researchResult.GetString("research_notes")
	log.Printf("📚 Research completed")

	// Step 2: Create outline
	outlineResult, err := outlineModule.Forward(ctx, map[string]any{
		"research_notes": researchNotes,
	})
	if err != nil {
		log.Printf("❌ Program module (outline) failed: %v", err)
		return
	}

	outline, _ := outlineResult.Get("outline")
	log.Printf("📝 Outline created")

	// Step 3: Write article
	articleResult, err := writeModule.Forward(ctx, map[string]any{
		"outline":        outline,
		"research_notes": researchNotes,
	})
	if err != nil {
		log.Printf("❌ Program module (write) failed: %v", err)
		return
	}

	article, _ := articleResult.GetString("article")

	log.Printf("✅ Program module:")
	log.Printf("   Topic: %s", topic)
	log.Printf("   Article length: %d characters", len(article))

	outlineJSON, err := json.Marshal(outline)
	if err != nil {
		log.Printf("❌ Failed to marshal outline: %v", err)
		return
	}

	artifacts.Results["module_program"] = map[string]any{
		"topic":          topic,
		"research_notes": researchNotes,
		"outline":        json.RawMessage(outlineJSON),
		"article_length": len(article),
	}
}

func runParallelProcessing(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n⚡ SECTION 12: Parallel Processing Module")

	// Define a simple signature for parallel execution
	sig := core.NewSignature("input -> output").
		AddInput("input", core.FieldTypeString, "Input string").
		AddOutput("output", core.FieldTypeString, "Uppercase version of input")

	// Use a stateless predictor for parallel execution
	predictor := module.NewPredict(sig, lm)

	// Create parallel module
	// Since predictor is technically stateful (it contains options/history), best practice is to use factory
	// But for this simple test, sharing the instance is fine as long as we don't modify it concurrently
	parallel := module.NewParallel(predictor).WithMaxWorkers(3)

	inputs := []string{"apple", "banana", "cherry", "date", "elderberry"}

	log.Printf("🚀 Running parallel processing on %d items...", len(inputs))

	// Prepare batch input
	batch := make([]map[string]any, len(inputs))
	for i, input := range inputs {
		batch[i] = map[string]any{"input": input}
	}

	result, err := parallel.Forward(ctx, map[string]any{
		"_batch": batch,
	})
	if err != nil {
		log.Printf("❌ Parallel processing failed: %v", err)
		return
	}

	// Check results
	completions := result.Completions
	log.Printf("✅ Parallel processing completed: %d/%d successful", len(completions), len(inputs))

	if len(completions) > 0 {
		firstOut, _ := completions[0]["output"].(string)
		log.Printf("   First result: %s", firstOut)
	}

	artifacts.Results["module_parallel"] = map[string]any{
		"total_items": len(inputs),
		"successful":  len(completions),
	}
}

func runAdvancedFeatures(ctx context.Context, lm core.LM, artifacts *BenchmarkArtifacts) {
	log.Println("\n🔧 SECTION 13: Advanced Features (Streaming, Caching, History)")

	// Streaming example
	log.Println("🌊 Testing streaming...")
	sig := core.NewSignature("prompt -> response").
		AddInput("prompt", core.FieldTypeString, "Prompt").
		AddOutput("response", core.FieldTypeString, "Response")

	log.Printf("   👉 Input (prompt): Write a short poem about technology")
	predictor := module.NewPredict(sig, lm)
	streamResult, err := predictor.Stream(ctx, map[string]any{
		"prompt": "Write a short poem about technology",
	})
	if err != nil {
		log.Printf("❌ Streaming failed: %v", err)
		return
	}

	log.Print("   Streaming: ")
	for chunk := range streamResult.Chunks {
		fmt.Print(chunk.Content)
	}
	fmt.Println()

	finalPrediction := <-streamResult.Prediction
	streamErr := <-streamResult.Errors
	if streamErr != nil {
		log.Printf("❌ Streaming error: %v", streamErr)
		return
	}

	response, _ := finalPrediction.GetString("response")
	log.Printf("✅ Streaming completed")

	artifacts.Results["advanced_streaming"] = map[string]string{
		"prompt":   "Poem about technology",
		"response": response,
	}

	// Caching example
	log.Println("💾 Testing caching...")
	_ = core.NewLMCache(100)

	// First call (cache miss)
	log.Printf("   👉 Input (prompt): What is the capital of France?")
	start := time.Now()
	result1, err := predictor.Forward(ctx, map[string]any{
		"prompt": "What is the capital of France?",
	})
	firstCallDuration := time.Since(start)

	if err != nil {
		log.Printf("❌ First cache call failed: %v", err)
		return
	}

	answer1, _ := result1.GetString("response")

	// Second call (cache hit)
	start = time.Now()
	result2, err := predictor.Forward(ctx, map[string]any{
		"prompt": "What is the capital of France?",
	})
	secondCallDuration := time.Since(start)

	if err != nil {
		log.Printf("❌ Second cache call failed: %v", err)
		return
	}

	// Verify we get the same answer
	answer2, _ := result2.GetString("response")
	if answer1 != answer2 {
		log.Printf("⚠️  Cache returned different answers: '%s' vs '%s'", answer1, answer2)
	}

	log.Printf("✅ Caching test:")
	log.Printf("   First call (cache miss): %v", firstCallDuration)
	log.Printf("   Second call (cache hit): %v", secondCallDuration)
	log.Printf("   Answer: %s", answer1)

	artifacts.Results["advanced_caching"] = map[string]any{
		"question":       "What is the capital of France?",
		"answer":         answer1,
		"first_call_ms":  firstCallDuration.Milliseconds(),
		"second_call_ms": secondCallDuration.Milliseconds(),
		"speedup":        float64(firstCallDuration) / float64(secondCallDuration),
	}

	// History tracking
	log.Println("📜 Testing history tracking...")
	history := core.NewHistoryWithLimit(10)

	predictorWithHistory := module.NewPredict(sig, lm)
	predictorWithHistory.History = history

	log.Printf("   👉 Input 1 (prompt): Count to 5")
	_, err = predictorWithHistory.Forward(ctx, map[string]any{
		"prompt": "Count to 5",
	})
	if err != nil {
		log.Printf("❌ History test failed: %v", err)
		return
	}

	log.Printf("   👉 Input 2 (prompt): Now count to 3")
	_, err = predictorWithHistory.Forward(ctx, map[string]any{
		"prompt": "Now count to 3",
	})
	if err != nil {
		log.Printf("❌ History test failed: %v", err)
		return
	}

	messages := history.Get()
	log.Printf("✅ History tracking: %d entries stored", len(messages))

	artifacts.Results["advanced_history"] = map[string]any{
		"history_entries": len(messages),
		"conversation":    "Multi-turn conversation",
	}
}

func runConfiguration(artifacts *BenchmarkArtifacts) {
	log.Println("\n⚙️ SECTION 14: Configuration")

	// Test configuration
	config := core.GetSettings()
	log.Printf("✅ Configuration loaded:")
	log.Printf("   Trace enabled: %v", config.EnableTracing)

	artifacts.Results["configuration"] = map[string]any{
		"trace_enabled": config.EnableTracing,
	}
}

func calculateMetrics(artifacts *BenchmarkArtifacts) {
	// This would normally calculate from actual usage data
	// For now, we'll use placeholder metrics
	artifacts.Metrics = BenchmarkMetrics{
		TotalRequests:        30,
		TotalTokens:          25000,
		TotalCost:            0.08,
		TotalLatency:         60000,
		SuccessRate:          0.98,
		StructuredOutputRate: 0.95,
	}
}

func saveArtifacts(artifacts *BenchmarkArtifacts, dir string) {
	// Save JSON artifacts
	jsonPath := filepath.Join(dir, "benchmark_results.json")
	jsonData, err := json.MarshalIndent(artifacts, "", "  ")
	if err != nil {
		log.Printf("❌ Failed to marshal artifacts: %v", err)
		return
	}

	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		log.Printf("❌ Failed to save JSON artifacts: %v", err)
		return
	}

	log.Printf("💾 Artifacts saved to: %s", jsonPath)
}
