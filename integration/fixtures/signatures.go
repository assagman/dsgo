package fixtures

import "github.com/assagman/dsgo/core"

// SimplePredictSig creates a simple predict signature
func SimplePredictSig() *core.Signature {
	return core.NewSignature("Simple prediction").
		AddInput("question", core.FieldTypeString, "Input question").
		AddOutput("answer", core.FieldTypeString, "Output answer")
}

// ClassificationSig creates a classification signature
func ClassificationSig() *core.Signature {
	return core.NewSignature("Classify sentiment").
		AddInput("text", core.FieldTypeString, "Text to classify").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")
}

// ComplexOutputSig creates a signature with multiple output types
func ComplexOutputSig() *core.Signature {
	return core.NewSignature("Complex processing").
		AddInput("data", core.FieldTypeString, "Input data").
		AddOutput("result", core.FieldTypeString, "Result").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence score").
		AddOutput("count", core.FieldTypeInt, "Item count").
		AddOutput("valid", core.FieldTypeBool, "Is valid")
}

// OptionalFieldsSig creates a signature with optional fields
func OptionalFieldsSig() *core.Signature {
	return core.NewSignature("With optional fields").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("summary", core.FieldTypeString, "Summary").
		AddOptionalOutput("keywords", core.FieldTypeJSON, "Extracted keywords").
		AddOptionalOutput("sentiment", core.FieldTypeString, "Detected sentiment")
}

// NestedJSONSig creates a signature with nested JSON output
func NestedJSONSig() *core.Signature {
	return core.NewSignature("Nested JSON processing").
		AddInput("query", core.FieldTypeString, "Query").
		AddOutput("data", core.FieldTypeJSON, "Complex nested data")
}

// MultiClassSig creates a signature with multiple classification fields
func MultiClassSig() *core.Signature {
	return core.NewSignature("Multi-class classification").
		AddInput("item", core.FieldTypeString, "Item to classify").
		AddClassOutput("category", []string{"A", "B", "C"}, "Primary category").
		AddClassOutput("subcategory", []string{"1", "2", "3"}, "Sub category").
		AddClassOutput("priority", []string{"low", "medium", "high"}, "Priority level")
}

// ChainOfThoughtSig creates a signature for chain of thought reasoning
func ChainOfThoughtSig() *core.Signature {
	return core.NewSignature("Chain of thought reasoning").
		AddInput("problem", core.FieldTypeString, "Problem to solve").
		AddOutput("reasoning", core.FieldTypeString, "Step-by-step reasoning").
		AddOutput("answer", core.FieldTypeString, "Final answer")
}

// ReActSig creates a signature for ReAct (tool-using) modules
func ReActSig() *core.Signature {
	return core.NewSignature("React with tools").
		AddInput("question", core.FieldTypeString, "Question to answer").
		AddOutput("reasoning", core.FieldTypeString, "Reasoning process").
		AddOutput("answer", core.FieldTypeString, "Final answer")
}

// RefineSig creates a signature for iterative refinement
func RefineSig() *core.Signature {
	return core.NewSignature("Refine output").
		AddInput("topic", core.FieldTypeString, "Topic for generation").
		AddOutput("output", core.FieldTypeString, "Generated and refined output")
}

// BestOfNSig creates a signature for best-of-N selection
func BestOfNSig() *core.Signature {
	return core.NewSignature("Generate and select best").
		AddInput("prompt", core.FieldTypeString, "Generation prompt").
		AddOutput("result", core.FieldTypeString, "Best result from N generations")
}

// ============================================================================
// Advanced Signatures
// ============================================================================

// ProgramOfThoughtSig creates a signature for code generation
func ProgramOfThoughtSig() *core.Signature {
	return core.NewSignature("Generate code to solve problem").
		AddInput("problem", core.FieldTypeString, "Problem description").
		AddOutput("code", core.FieldTypeString, "Generated code").
		AddOutput("explanation", core.FieldTypeString, "Explanation of approach")
}

// MultiInputSig creates a signature with multiple inputs
func MultiInputSig() *core.Signature {
	return core.NewSignature("Process multiple inputs").
		AddInput("context", core.FieldTypeString, "Background context").
		AddInput("question", core.FieldTypeString, "Question to answer").
		AddInput("constraints", core.FieldTypeString, "Any constraints").
		AddOutput("answer", core.FieldTypeString, "Answer considering all inputs")
}

// MultiOutputSig creates a signature with many output types
func MultiOutputSig() *core.Signature {
	return core.NewSignature("Generate comprehensive output").
		AddInput("request", core.FieldTypeString, "Request data").
		AddOutput("summary", core.FieldTypeString, "Brief summary").
		AddOutput("details", core.FieldTypeJSON, "Detailed structured data").
		AddOutput("score", core.FieldTypeFloat, "Confidence score").
		AddOutput("count", core.FieldTypeInt, "Number of items").
		AddOutput("approved", core.FieldTypeBool, "Approval status")
}

// DeepNestedJSONSig creates a signature for deeply nested JSON output
func DeepNestedJSONSig() *core.Signature {
	return core.NewSignature("Generate deeply nested structure").
		AddInput("schema", core.FieldTypeString, "Schema description").
		AddOutput("data", core.FieldTypeJSON, "Deeply nested JSON data")
}

// MultiClassificationSig creates a signature with many classification fields
func MultiClassificationSig() *core.Signature {
	return core.NewSignature("Multi-dimensional classification").
		AddInput("content", core.FieldTypeString, "Content to classify").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment").
		AddClassOutput("topic", []string{"tech", "business", "science", "other"}, "Topic").
		AddClassOutput("urgency", []string{"low", "medium", "high", "critical"}, "Urgency").
		AddClassOutput("language", []string{"en", "es", "fr", "de", "other"}, "Language")
}

// WithAliasesSig creates a signature with classification aliases
// Note: Aliases are configured on the Field after creation
func WithAliasesSig() *core.Signature {
	sig := core.NewSignature("Classification with aliases").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")

	// Configure aliases on the output field
	if len(sig.OutputFields) > 0 {
		sig.OutputFields[0].ClassAliases = map[string]string{
			"pos":   "positive",
			"good":  "positive",
			"happy": "positive",
			"+":     "positive",
			"neg":   "negative",
			"bad":   "negative",
			"sad":   "negative",
			"-":     "negative",
			"neu":   "neutral",
			"none":  "neutral",
			"0":     "neutral",
		}
	}

	return sig
}

// AllOptionalSig creates a signature with all optional outputs
func AllOptionalSig() *core.Signature {
	return core.NewSignature("All optional outputs").
		AddInput("prompt", core.FieldTypeString, "Prompt").
		AddOptionalOutput("title", core.FieldTypeString, "Optional title").
		AddOptionalOutput("tags", core.FieldTypeJSON, "Optional tags").
		AddOptionalOutput("score", core.FieldTypeFloat, "Optional score").
		AddOptionalOutput("flag", core.FieldTypeBool, "Optional flag")
}

// MixedRequiredOptionalSig creates a signature with mixed required/optional fields
func MixedRequiredOptionalSig() *core.Signature {
	return core.NewSignature("Mixed required and optional").
		AddInput("query", core.FieldTypeString, "Input query").
		AddOutput("answer", core.FieldTypeString, "Required answer").
		AddOptionalOutput("confidence", core.FieldTypeFloat, "Optional confidence").
		AddOutput("source", core.FieldTypeString, "Required source").
		AddOptionalOutput("alternatives", core.FieldTypeJSON, "Optional alternatives")
}

// ============================================================================
// Domain-Specific Signatures
// ============================================================================

// SummarizationSig creates a signature for text summarization
func SummarizationSig() *core.Signature {
	return core.NewSignature("Summarize text").
		AddInput("text", core.FieldTypeString, "Text to summarize").
		AddInput("max_length", core.FieldTypeInt, "Maximum summary length").
		AddOutput("summary", core.FieldTypeString, "Summarized text").
		AddOutput("key_points", core.FieldTypeJSON, "Key points extracted")
}

// TranslationSig creates a signature for translation
func TranslationSig() *core.Signature {
	return core.NewSignature("Translate text").
		AddInput("text", core.FieldTypeString, "Text to translate").
		AddInput("source_language", core.FieldTypeString, "Source language code").
		AddInput("target_language", core.FieldTypeString, "Target language code").
		AddOutput("translation", core.FieldTypeString, "Translated text")
}

// QASig creates a signature for question answering
func QASig() *core.Signature {
	return core.NewSignature("Answer question from context").
		AddInput("context", core.FieldTypeString, "Context document").
		AddInput("question", core.FieldTypeString, "Question to answer").
		AddOutput("answer", core.FieldTypeString, "Answer to the question").
		AddOutput("confidence", core.FieldTypeFloat, "Confidence in answer").
		AddOutput("relevant_passage", core.FieldTypeString, "Relevant passage from context")
}

// EntityExtractionSig creates a signature for entity extraction
func EntityExtractionSig() *core.Signature {
	return core.NewSignature("Extract entities from text").
		AddInput("text", core.FieldTypeString, "Text to analyze").
		AddOutput("entities", core.FieldTypeJSON, "Extracted entities").
		AddOutput("count", core.FieldTypeInt, "Number of entities found")
}

// CodeReviewSig creates a signature for code review
func CodeReviewSig() *core.Signature {
	return core.NewSignature("Review code for issues").
		AddInput("code", core.FieldTypeString, "Code to review").
		AddInput("language", core.FieldTypeString, "Programming language").
		AddOutput("issues", core.FieldTypeJSON, "List of issues found").
		AddOutput("suggestions", core.FieldTypeJSON, "Improvement suggestions").
		AddClassOutput("quality", []string{"excellent", "good", "acceptable", "poor"}, "Overall quality")
}
