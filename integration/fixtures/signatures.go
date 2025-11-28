package fixtures

import "github.com/assagman/dsgo"

// SimplePredictSig creates a simple predict signature
func SimplePredictSig() *dsgo.Signature {
	return dsgo.NewSignature("Simple prediction").
		AddInput("question", dsgo.FieldTypeString, "Input question").
		AddOutput("answer", dsgo.FieldTypeString, "Output answer")
}

// ClassificationSig creates a classification signature
func ClassificationSig() *dsgo.Signature {
	return dsgo.NewSignature("Classify sentiment").
		AddInput("text", dsgo.FieldTypeString, "Text to classify").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment classification")
}

// ComplexOutputSig creates a signature with multiple output types
func ComplexOutputSig() *dsgo.Signature {
	return dsgo.NewSignature("Complex processing").
		AddInput("data", dsgo.FieldTypeString, "Input data").
		AddOutput("result", dsgo.FieldTypeString, "Result").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence score").
		AddOutput("count", dsgo.FieldTypeInt, "Item count").
		AddOutput("valid", dsgo.FieldTypeBool, "Is valid")
}

// OptionalFieldsSig creates a signature with optional fields
func OptionalFieldsSig() *dsgo.Signature {
	return dsgo.NewSignature("With optional fields").
		AddInput("text", dsgo.FieldTypeString, "Input text").
		AddOutput("summary", dsgo.FieldTypeString, "Summary").
		AddOptionalOutput("keywords", dsgo.FieldTypeJSON, "Extracted keywords").
		AddOptionalOutput("sentiment", dsgo.FieldTypeString, "Detected sentiment")
}

// NestedJSONSig creates a signature with nested JSON output
func NestedJSONSig() *dsgo.Signature {
	return dsgo.NewSignature("Nested JSON processing").
		AddInput("query", dsgo.FieldTypeString, "Query").
		AddOutput("data", dsgo.FieldTypeJSON, "Complex nested data")
}

// MultiClassSig creates a signature with multiple classification fields
func MultiClassSig() *dsgo.Signature {
	return dsgo.NewSignature("Multi-class classification").
		AddInput("item", dsgo.FieldTypeString, "Item to classify").
		AddClassOutput("category", []string{"A", "B", "C"}, "Primary category").
		AddClassOutput("subcategory", []string{"1", "2", "3"}, "Sub category").
		AddClassOutput("priority", []string{"low", "medium", "high"}, "Priority level")
}

// ChainOfThoughtSig creates a signature for chain of thought reasoning
func ChainOfThoughtSig() *dsgo.Signature {
	return dsgo.NewSignature("Chain of thought reasoning").
		AddInput("problem", dsgo.FieldTypeString, "Problem to solve").
		AddOutput("reasoning", dsgo.FieldTypeString, "Step-by-step reasoning").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")
}

// ReActSig creates a signature for ReAct (tool-using) modules
func ReActSig() *dsgo.Signature {
	return dsgo.NewSignature("React with tools").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("reasoning", dsgo.FieldTypeString, "Reasoning process").
		AddOutput("answer", dsgo.FieldTypeString, "Final answer")
}

// RefineSig creates a signature for iterative refinement
func RefineSig() *dsgo.Signature {
	return dsgo.NewSignature("Refine output").
		AddInput("topic", dsgo.FieldTypeString, "Topic for generation").
		AddOutput("output", dsgo.FieldTypeString, "Generated and refined output")
}

// BestOfNSig creates a signature for best-of-N selection
func BestOfNSig() *dsgo.Signature {
	return dsgo.NewSignature("Generate and select best").
		AddInput("prompt", dsgo.FieldTypeString, "Generation prompt").
		AddOutput("result", dsgo.FieldTypeString, "Best result from N generations")
}

// ============================================================================
// Advanced Signatures
// ============================================================================

// ProgramOfThoughtSig creates a signature for code generation
func ProgramOfThoughtSig() *dsgo.Signature {
	return dsgo.NewSignature("Generate code to solve problem").
		AddInput("problem", dsgo.FieldTypeString, "Problem description").
		AddOutput("code", dsgo.FieldTypeString, "Generated code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation of approach")
}

// MultiInputSig creates a signature with multiple inputs
func MultiInputSig() *dsgo.Signature {
	return dsgo.NewSignature("Process multiple inputs").
		AddInput("context", dsgo.FieldTypeString, "Background context").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddInput("constraints", dsgo.FieldTypeString, "Any constraints").
		AddOutput("answer", dsgo.FieldTypeString, "Answer considering all inputs")
}

// MultiOutputSig creates a signature with many output types
func MultiOutputSig() *dsgo.Signature {
	return dsgo.NewSignature("Generate comprehensive output").
		AddInput("request", dsgo.FieldTypeString, "Request data").
		AddOutput("summary", dsgo.FieldTypeString, "Brief summary").
		AddOutput("details", dsgo.FieldTypeJSON, "Detailed structured data").
		AddOutput("score", dsgo.FieldTypeFloat, "Confidence score").
		AddOutput("count", dsgo.FieldTypeInt, "Number of items").
		AddOutput("approved", dsgo.FieldTypeBool, "Approval status")
}

// DeepNestedJSONSig creates a signature for deeply nested JSON output
func DeepNestedJSONSig() *dsgo.Signature {
	return dsgo.NewSignature("Generate deeply nested structure").
		AddInput("schema", dsgo.FieldTypeString, "Schema description").
		AddOutput("data", dsgo.FieldTypeJSON, "Deeply nested JSON data")
}

// MultiClassificationSig creates a signature with many classification fields
func MultiClassificationSig() *dsgo.Signature {
	return dsgo.NewSignature("Multi-dimensional classification").
		AddInput("content", dsgo.FieldTypeString, "Content to classify").
		AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment").
		AddClassOutput("topic", []string{"tech", "business", "science", "other"}, "Topic").
		AddClassOutput("urgency", []string{"low", "medium", "high", "critical"}, "Urgency").
		AddClassOutput("language", []string{"en", "es", "fr", "de", "other"}, "Language")
}

// WithAliasesSig creates a signature with classification aliases
// Note: Aliases are configured on the Field after creation
func WithAliasesSig() *dsgo.Signature {
	sig := dsgo.NewSignature("Classification with aliases").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
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
func AllOptionalSig() *dsgo.Signature {
	return dsgo.NewSignature("All optional outputs").
		AddInput("prompt", dsgo.FieldTypeString, "Prompt").
		AddOptionalOutput("title", dsgo.FieldTypeString, "Optional title").
		AddOptionalOutput("tags", dsgo.FieldTypeJSON, "Optional tags").
		AddOptionalOutput("score", dsgo.FieldTypeFloat, "Optional score").
		AddOptionalOutput("flag", dsgo.FieldTypeBool, "Optional flag")
}

// MixedRequiredOptionalSig creates a signature with mixed required/optional fields
func MixedRequiredOptionalSig() *dsgo.Signature {
	return dsgo.NewSignature("Mixed required and optional").
		AddInput("query", dsgo.FieldTypeString, "Input query").
		AddOutput("answer", dsgo.FieldTypeString, "Required answer").
		AddOptionalOutput("confidence", dsgo.FieldTypeFloat, "Optional confidence").
		AddOutput("source", dsgo.FieldTypeString, "Required source").
		AddOptionalOutput("alternatives", dsgo.FieldTypeJSON, "Optional alternatives")
}

// ============================================================================
// Domain-Specific Signatures
// ============================================================================

// SummarizationSig creates a signature for text summarization
func SummarizationSig() *dsgo.Signature {
	return dsgo.NewSignature("Summarize text").
		AddInput("text", dsgo.FieldTypeString, "Text to summarize").
		AddInput("max_length", dsgo.FieldTypeInt, "Maximum summary length").
		AddOutput("summary", dsgo.FieldTypeString, "Summarized text").
		AddOutput("key_points", dsgo.FieldTypeJSON, "Key points extracted")
}

// TranslationSig creates a signature for translation
func TranslationSig() *dsgo.Signature {
	return dsgo.NewSignature("Translate text").
		AddInput("text", dsgo.FieldTypeString, "Text to translate").
		AddInput("source_language", dsgo.FieldTypeString, "Source language code").
		AddInput("target_language", dsgo.FieldTypeString, "Target language code").
		AddOutput("translation", dsgo.FieldTypeString, "Translated text")
}

// QASig creates a signature for question answering
func QASig() *dsgo.Signature {
	return dsgo.NewSignature("Answer question from context").
		AddInput("context", dsgo.FieldTypeString, "Context document").
		AddInput("question", dsgo.FieldTypeString, "Question to answer").
		AddOutput("answer", dsgo.FieldTypeString, "Answer to the question").
		AddOutput("confidence", dsgo.FieldTypeFloat, "Confidence in answer").
		AddOutput("relevant_passage", dsgo.FieldTypeString, "Relevant passage from context")
}

// EntityExtractionSig creates a signature for entity extraction
func EntityExtractionSig() *dsgo.Signature {
	return dsgo.NewSignature("Extract entities from text").
		AddInput("text", dsgo.FieldTypeString, "Text to analyze").
		AddOutput("entities", dsgo.FieldTypeJSON, "Extracted entities").
		AddOutput("count", dsgo.FieldTypeInt, "Number of entities found")
}

// CodeReviewSig creates a signature for code review
func CodeReviewSig() *dsgo.Signature {
	return dsgo.NewSignature("Review code for issues").
		AddInput("code", dsgo.FieldTypeString, "Code to review").
		AddInput("language", dsgo.FieldTypeString, "Programming language").
		AddOutput("issues", dsgo.FieldTypeJSON, "List of issues found").
		AddOutput("suggestions", dsgo.FieldTypeJSON, "Improvement suggestions").
		AddClassOutput("quality", []string{"excellent", "good", "acceptable", "poor"}, "Overall quality")
}
