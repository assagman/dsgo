package fixtures

// LMResponses provides predefined LM response strings for testing various scenarios.
// These are categorized by type: valid, malformed, partial, error responses.

// ============================================================================
// Valid Responses
// ============================================================================

// ValidSimpleAnswer returns a valid simple answer response
func ValidSimpleAnswer() string {
	return `{"answer": "This is a valid answer"}`
}

// ValidWithReasoning returns a valid response with reasoning
func ValidWithReasoning() string {
	return `{"reasoning": "Let me think step by step. First, I analyze the problem. Then, I consider the solution.", "answer": "The answer is 42"}`
}

// ValidClassification returns a valid classification response
func ValidClassification() string {
	return `{"sentiment": "positive"}`
}

// ValidComplexOutput returns a valid response with multiple field types
func ValidComplexOutput() string {
	return `{"result": "success", "confidence": 0.95, "count": 42, "valid": true}`
}

// ValidNestedJSON returns a valid nested JSON response
func ValidNestedJSON() string {
	return `{
		"data": {
			"users": [
				{"id": 1, "name": "Alice", "active": true},
				{"id": 2, "name": "Bob", "active": false}
			],
			"metadata": {
				"total": 2,
				"page": 1,
				"hasMore": false
			}
		}
	}`
}

// ValidWithOptionalFields returns a response with optional fields populated
func ValidWithOptionalFields() string {
	return `{"summary": "This is a summary", "keywords": ["key1", "key2"], "sentiment": "positive"}`
}

// ValidWithOptionalFieldsMissing returns a response with optional fields omitted
func ValidWithOptionalFieldsMissing() string {
	return `{"summary": "This is a summary"}`
}

// ============================================================================
// Chat Marker Format Responses
// ============================================================================

// ValidChatMarkerResponse returns a response using chat marker format
func ValidChatMarkerResponse() string {
	return `[[ ## answer ## ]]
This is the answer in chat marker format.`
}

// ValidChatMarkerWithReasoning returns a chat marker response with reasoning
func ValidChatMarkerWithReasoning() string {
	return `[[ ## reasoning ## ]]
Let me think about this step by step.
First, I consider the problem.
Then, I work through the solution.

[[ ## answer ## ]]
The final answer is 42.`
}

// ValidChatMarkerMultiField returns a chat marker response with multiple fields
func ValidChatMarkerMultiField() string {
	return `[[ ## result ## ]]
Operation completed successfully

[[ ## confidence ## ]]
0.95

[[ ## count ## ]]
42

[[ ## valid ## ]]
true`
}

// ============================================================================
// Malformed Responses (for testing repair/recovery)
// ============================================================================

// MalformedSingleQuotes returns JSON with single quotes (should be repaired)
func MalformedSingleQuotes() string {
	return `{'answer': 'response with single quotes'}`
}

// MalformedUnquotedKeys returns JSON with unquoted keys (should be repaired)
func MalformedUnquotedKeys() string {
	return `{answer: "response with unquoted key"}`
}

// MalformedTrailingComma returns JSON with trailing comma (should be repaired)
func MalformedTrailingComma() string {
	return `{"answer": "response", "extra": "field",}`
}

// MalformedMissingCloseBrace returns incomplete JSON
func MalformedMissingCloseBrace() string {
	return `{"answer": "incomplete response"`
}

// MalformedNestedQuotes returns JSON with nested quote issues
func MalformedNestedQuotes() string {
	return `{"answer": "She said \"hello\" to me"}`
}

// MalformedMarkdownWrapped returns JSON wrapped in markdown code blocks
func MalformedMarkdownWrapped() string {
	return "```json\n{\"answer\": \"wrapped in markdown\"}\n```"
}

// MalformedWithPreamble returns JSON with text before the JSON
func MalformedWithPreamble() string {
	return `Here is my response:
{"answer": "the actual answer"}`
}

// MalformedWithPostamble returns JSON with text after the JSON
func MalformedWithPostamble() string {
	return `{"answer": "the actual answer"}
Hope this helps!`
}

// ============================================================================
// Partial Responses (for testing partial validation)
// ============================================================================

// PartialMissingRequired returns a response missing a required field
func PartialMissingRequired() string {
	return `{"reasoning": "I thought about it"}`
}

// PartialWrongType returns a response with wrong field type
func PartialWrongType() string {
	return `{"answer": 123}`
}

// PartialEmptyString returns a response with empty string value
func PartialEmptyString() string {
	return `{"answer": ""}`
}

// PartialNullValue returns a response with null value
func PartialNullValue() string {
	return `{"answer": null}`
}

// PartialInvalidClass returns a response with invalid classification value
func PartialInvalidClass() string {
	return `{"sentiment": "very_happy"}`
}

// ============================================================================
// Error Responses
// ============================================================================

// ErrorEmptyResponse returns an empty string
func ErrorEmptyResponse() string {
	return ""
}

// ErrorWhitespaceOnly returns whitespace only
func ErrorWhitespaceOnly() string {
	return "   \n\t\n   "
}

// ErrorInvalidJSON returns completely invalid JSON
func ErrorInvalidJSON() string {
	return "This is not JSON at all"
}

// ErrorTruncated returns truncated/corrupted content
func ErrorTruncated() string {
	return `{"answer": "This response was cut off mid-sen`
}

// ErrorBinaryGarbage returns non-text content
func ErrorBinaryGarbage() string {
	return "\x00\x01\x02\x03"
}

// ============================================================================
// Tool Call Responses
// ============================================================================

// ValidToolCallResponse returns a response with tool call intent
func ValidToolCallResponse() string {
	return `{"thought": "I need to search for this", "action": "search", "action_input": "query terms"}`
}

// ValidFinishResponse returns a finish/final answer response
func ValidFinishResponse() string {
	return `{"thought": "I have all the information", "action": "finish", "answer": "The final answer is here"}`
}

// ============================================================================
// Large Responses
// ============================================================================

// LargeResponse returns a response with significant content size
func LargeResponse(sizeKB int) string {
	content := ""
	unit := "This is a line of content that will be repeated many times to create a large response. "
	for len(content) < sizeKB*1024 {
		content += unit
	}
	return `{"answer": "` + content[:sizeKB*1024] + `"}`
}

// ============================================================================
// Special Character Responses
// ============================================================================

// ValidWithUnicode returns a response with Unicode characters
func ValidWithUnicode() string {
	return `{"answer": "Response with Unicode: 你好世界 🌍 émojis café"}`
}

// ValidWithNewlines returns a response with embedded newlines
func ValidWithNewlines() string {
	return `{"answer": "Line 1\nLine 2\nLine 3"}`
}

// ValidWithTabs returns a response with embedded tabs
func ValidWithTabs() string {
	return `{"answer": "Column1\tColumn2\tColumn3"}`
}

// ValidWithEscapes returns a response with various escape sequences
func ValidWithEscapes() string {
	return `{"answer": "Escapes: \\n \\t \\\" \\\\ \\/"}`
}
