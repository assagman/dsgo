package testdata

// LMResponses contains predefined LM responses for testing
var LMResponses = struct {
	// Successful responses
	SuccessfulJSON              string
	SuccessfulPlainText         string
	SuccessfulClassification    string
	SuccessfulComplexJSON       string
	SuccessfulMultiField        string
	SuccessfulWithExtraContent  string
	SuccessfulReasoningResponse string

	// Malformed JSON responses
	MalformedMissingQuotes   string
	MalformedTrailingComma   string
	MalformedSingleQuotes    string
	MalformedUnquotedKeys    string
	MalformedExtraComma      string
	MalformedMissingBrace    string
	MalformedUnmatchedBraces string

	// Partial responses (missing fields)
	PartialMissingField          string
	PartialMultipleMissingFields string
	PartialEmptyFields           string

	// Type mismatch responses
	TypeMismatchStringAsNumber string
	TypeMismatchNumberAsString string
	TypeMismatchBoolAsString   string

	// Extra content responses
	ExtraMarkdownBefore   string
	ExtraMarkdownAfter    string
	ExtraExplanationMixed string
	ExtraFieldMarkers     string

	// Nested structure responses
	NestedJSON            string
	NestedWithArrays      string
	DeeplyNestedStructure string

	// Edge case responses
	EmptyStringValue  string
	NullValue         string
	BooleanValues     string
	NumericEdgeCases  string
	ArrayResponse     string
	LargeTextResponse string
}{
	// SuccessfulJSON
	SuccessfulJSON: `{"answer": "42", "reasoning": "The answer to life, the universe, and everything"}`,

	// SuccessfulPlainText
	SuccessfulPlainText: `{"text": "This is a plain text response"}`,

	// SuccessfulClassification
	SuccessfulClassification: `{"sentiment": "positive", "confidence": 0.95}`,

	// SuccessfulComplexJSON
	SuccessfulComplexJSON: `{"result": {"status": "success", "data": [1, 2, 3]}, "metadata": {"version": "1.0"}}`,

	// SuccessfulMultiField
	SuccessfulMultiField: `{"field1": "value1", "field2": "value2", "field3": "value3", "field4": 123}`,

	// SuccessfulWithExtraContent
	SuccessfulWithExtraContent: `Here's my response:
{"answer": "42"}
Thank you for asking!`,

	// SuccessfulReasoningResponse
	SuccessfulReasoningResponse: `{"reasoning": "Let me think about this step by step. First, I need to understand the problem. Then I can solve it.", "answer": "The solution is X"}`,

	// MalformedMissingQuotes (unquoted keys)
	MalformedMissingQuotes: `{answer: "42"}`,

	// MalformedTrailingComma
	MalformedTrailingComma: `{"answer": "42",}`,

	// MalformedSingleQuotes
	MalformedSingleQuotes: `{'answer': '42'}`,

	// MalformedUnquotedKeys
	MalformedUnquotedKeys: `{answer: 42, reason: "test"}`,

	// MalformedExtraComma
	MalformedExtraComma: `{"answer": "42", , "other": "value"}`,

	// MalformedMissingBrace
	MalformedMissingBrace: `{"answer": "42"`,

	// MalformedUnmatchedBraces
	MalformedUnmatchedBraces: `{"answer": "42"}}}`,

	// PartialMissingField
	PartialMissingField: `{"answer": "42"}`,

	// PartialMultipleMissingFields
	PartialMultipleMissingFields: `{"field1": "value1"}`,

	// PartialEmptyFields
	PartialEmptyFields: `{"answer": "", "reasoning": ""}`,

	// TypeMismatchStringAsNumber
	TypeMismatchStringAsNumber: `{"count": "not a number"}`,

	// TypeMismatchNumberAsString
	TypeMismatchNumberAsString: `{"text": 42}`,

	// TypeMismatchBoolAsString
	TypeMismatchBoolAsString: `{"active": "yes"}`,

	// ExtraMarkdownBefore
	ExtraMarkdownBefore: `## Answer
{"answer": "42"}`,

	// ExtraMarkdownAfter
	ExtraMarkdownAfter: `{"answer": "42"}
This is additional explanation`,

	// ExtraExplanationMixed
	ExtraExplanationMixed: `The response is:
{"answer": "42"}
As you can see, the answer is clearly 42`,

	// ExtraFieldMarkers
	ExtraFieldMarkers: `[[ ## answer ## ]]42[[ ## /answer ## ]]`,

	// NestedJSON
	NestedJSON: `{"user": {"name": "John", "email": "john@example.com"}}`,

	// NestedWithArrays
	NestedWithArrays: `{"items": [{"id": 1, "name": "item1"}, {"id": 2, "name": "item2"}]}`,

	// DeeplyNestedStructure
	DeeplyNestedStructure: `{"level1": {"level2": {"level3": {"level4": {"value": "deep"}}}}}`,

	// EmptyStringValue
	EmptyStringValue: `{"answer": ""}`,

	// NullValue
	NullValue: `{"answer": null}`,

	// BooleanValues
	BooleanValues: `{"active": true, "disabled": false}`,

	// NumericEdgeCases
	NumericEdgeCases: `{"zero": 0, "negative": -42, "large": 999999999, "decimal": 3.14159}`,

	// ArrayResponse
	ArrayResponse: `{"items": [1, 2, 3, 4, 5], "names": ["a", "b", "c"]}`,

	// LargeTextResponse (1KB+ response)
	LargeTextResponse: `{"content": "` + generateLargeText(800) + `"}`,
}

// generateLargeText creates a large text string for testing
func generateLargeText(sizeInChars int) string {
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	for len(text) < sizeInChars {
		text += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	}
	return text[:sizeInChars]
}

// AdapterTestCase represents a single test case for adapter robustness
type AdapterTestCase struct {
	Name            string         // Test name
	LMOutput        string         // Raw LM output
	ExpectedRepairs []string       // Repairs that should be applied
	ShouldSucceed   bool           // Whether parsing should succeed
	ExpectedParsed  map[string]any // Expected parsed output
	FieldType       string         // Expected field type
}

// AdapterTestCases contains robustness test cases for adapters
var AdapterTestCases = []AdapterTestCase{
	{
		Name:          "Valid JSON",
		LMOutput:      `{"answer": "42"}`,
		ShouldSucceed: true,
		ExpectedParsed: map[string]any{
			"answer": "42",
		},
	},
	{
		Name:            "Single quotes to double quotes",
		LMOutput:        `{'answer': '42'}`,
		ExpectedRepairs: []string{"quote_normalization"},
		ShouldSucceed:   true,
		ExpectedParsed: map[string]any{
			"answer": "42",
		},
	},
	{
		Name:            "Unquoted keys",
		LMOutput:        `{answer: "42"}`,
		ExpectedRepairs: []string{"quote_keys"},
		ShouldSucceed:   true,
		ExpectedParsed: map[string]any{
			"answer": "42",
		},
	},
	{
		Name:            "Trailing comma",
		LMOutput:        `{"answer": "42",}`,
		ExpectedRepairs: []string{"trailing_comma"},
		ShouldSucceed:   true,
		ExpectedParsed: map[string]any{
			"answer": "42",
		},
	},
	{
		Name:          "Missing closing brace",
		LMOutput:      `{"answer": "42"`,
		ShouldSucceed: false, // Should fail even after repairs
	},
	{
		Name:          "JSON with extra content",
		LMOutput:      `Here's the answer:\n{"answer": "42"}`,
		ShouldSucceed: true,
		ExpectedParsed: map[string]any{
			"answer": "42",
		},
	},
	{
		Name:            "Multiple issues",
		LMOutput:        `{answer: "42", reason: 'test',}`,
		ExpectedRepairs: []string{"quote_keys", "quote_normalization", "trailing_comma"},
		ShouldSucceed:   true,
		ExpectedParsed: map[string]any{
			"answer": "42",
			"reason": "test",
		},
	},
	{
		Name:          "Empty string value",
		LMOutput:      `{"answer": ""}`,
		ShouldSucceed: true,
		ExpectedParsed: map[string]any{
			"answer": "",
		},
	},
	{
		Name:          "Null value",
		LMOutput:      `{"answer": null}`,
		ShouldSucceed: true,
		ExpectedParsed: map[string]any{
			"answer": nil,
		},
	},
	{
		Name:          "Nested JSON",
		LMOutput:      `{"user": {"name": "John", "age": 30}}`,
		ShouldSucceed: true,
		ExpectedParsed: map[string]any{
			"user": map[string]any{
				"name": "John",
				"age":  30.0,
			},
		},
	},
}
