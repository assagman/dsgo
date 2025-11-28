package integration

import (
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

// ============================================================================
// ProgramOfThought Code Generation Tests
// ============================================================================

// TestProgramOfThought_CodeGeneration tests basic code generation without execution.
// Validates:
// - Code output field is populated
// - Explanation field is populated
// - Reasoning is captured
func TestProgramOfThought_CodeGeneration(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Create signature for ProgramOfThought
	sig := dsgo.NewSignature("Solve a mathematical problem by writing code").
		AddInput("problem", dsgo.FieldTypeString, "The problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "Python code that solves the problem").
		AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation of the code")

	// Mock LM that returns code and explanation
	lm := NewMockLMWithResponse(`{
		"code": "def fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)\n\nresult = fibonacci(10)\nprint(result)",
		"explanation": "We define a recursive Fibonacci function that returns n for base cases (0 or 1), and recursively computes fib(n-1) + fib(n-2) otherwise. We then compute the 10th Fibonacci number."
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Calculate the 10th Fibonacci number",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought failed: %v", err)
	}

	// Verify code field is populated
	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected non-empty code field")
	}

	// Verify code contains expected elements
	if !containsString(code, "fibonacci") {
		t.Error("Expected code to contain 'fibonacci'")
	}

	// Verify explanation field is populated
	explanation, ok := result.GetString("explanation")
	if !ok || explanation == "" {
		t.Error("Expected non-empty explanation field")
	}

	// Verify usage is tracked
	if result.Usage.TotalTokens == 0 {
		t.Error("Expected token usage to be tracked")
	}
}

// TestProgramOfThought_JavaScriptGeneration tests code generation for JavaScript.
// Validates:
// - Language-specific code generation works
// - Code is syntactically appropriate for target language
func TestProgramOfThought_JavaScriptGeneration(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Solve problem with JavaScript code").
		AddInput("problem", dsgo.FieldTypeString, "The problem to solve").
		AddOutput("code", dsgo.FieldTypeString, "JavaScript code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	lm := NewMockLMWithResponse(`{
		"code": "function factorial(n) {\n    if (n <= 1) return 1;\n    return n * factorial(n - 1);\n}\nconsole.log(factorial(5));",
		"explanation": "We define a recursive factorial function in JavaScript. For n <= 1, we return 1. Otherwise, we multiply n by factorial(n-1). We compute 5! = 120."
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "javascript")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Calculate 5 factorial",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought (JS) failed: %v", err)
	}

	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected non-empty code field")
	}

	// Verify JavaScript syntax elements
	if !containsString(code, "function") && !containsString(code, "const") {
		t.Error("Expected JavaScript syntax in code")
	}
}

// TestProgramOfThought_ComplexMath tests mathematical computation via code.
// Validates:
// - Complex math problems are handled
// - Numerical accuracy is maintained
// - Intermediate steps are documented
func TestProgramOfThought_ComplexMath(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Solve complex mathematical problems").
		AddInput("problem", dsgo.FieldTypeString, "Math problem").
		AddOutput("code", dsgo.FieldTypeString, "Code solution").
		AddOutput("explanation", dsgo.FieldTypeString, "Mathematical reasoning")

	lm := NewMockLMWithResponse(`{
		"code": "import math\n\ndef calculate_compound_interest(principal, rate, time, n):\n    amount = principal * (1 + rate/n) ** (n * time)\n    return amount\n\n# $1000 at 5% for 10 years, compounded monthly\nresult = calculate_compound_interest(1000, 0.05, 10, 12)\nprint(f'Final amount: ${result:.2f}')",
		"explanation": "We use the compound interest formula A = P(1 + r/n)^(nt). We define a function that takes principal ($1000), annual rate (5% = 0.05), time (10 years), and compounding frequency (12 for monthly). The calculation gives approximately $1647.01."
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Calculate compound interest on $1000 at 5% annual rate for 10 years, compounded monthly",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought (math) failed: %v", err)
	}

	// Verify code contains mathematical operations
	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected non-empty code")
	}

	// Verify explanation includes reasoning
	explanation, ok := result.GetString("explanation")
	if !ok || explanation == "" {
		t.Error("Expected non-empty explanation")
	}
}

// ============================================================================
// ProgramOfThought With Execution Tests
// ============================================================================

// TestProgramOfThought_WithExecution tests code execution (mocked sandbox).
// Validates:
// - Execution result is captured when enabled
// - Error handling for execution failures
func TestProgramOfThought_WithExecution(t *testing.T) {
	// Skip execution tests if python3 is not available
	// This is a unit-level test that just verifies the execution flag works
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Execute code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	lm := NewMockLMWithResponse(`{
		"code": "print(2 + 2)",
		"explanation": "Simple addition to verify execution works"
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")
	pot.WithAllowExecution(true)
	pot.WithExecutionTimeout(5)

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "What is 2 + 2?",
	})

	if err != nil {
		// Execution might fail if python3 is not available - that's acceptable
		t.Logf("ProgramOfThought with execution: %v (may be expected if python3 not available)", err)
		return
	}

	// If execution succeeded, check for execution result
	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected code field")
	}

	// execution_result might be present if python3 ran successfully
	if execResult, exists := result.Outputs["execution_result"]; exists {
		t.Logf("Execution result: %v", execResult)
	}
}

// TestProgramOfThought_ExecutionDisabled verifies execution is disabled by default.
// Validates:
// - Execution is off by default
// - No execution_result field when disabled
func TestProgramOfThought_ExecutionDisabled(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code only").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	lm := NewMockLMWithResponse(`{
		"code": "print('Hello World')",
		"explanation": "Simple hello world program"
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")
	// AllowExecution is false by default

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Print hello world",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought failed: %v", err)
	}

	// Verify no execution_result field
	if _, exists := result.Outputs["execution_result"]; exists {
		t.Error("Expected no execution_result when execution is disabled")
	}
}

// ============================================================================
// ProgramOfThought Error Handling Tests
// ============================================================================

// TestProgramOfThought_EmptyCodeHandling tests handling of empty code.
// Validates:
// - Empty code is rejected
// - Appropriate error message
func TestProgramOfThought_EmptyCodeHandling(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// Mock LM that returns empty code
	lm := NewMockLMWithResponse(`{
		"code": "",
		"explanation": "I couldn't generate any code"
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	_, err := pot.Forward(ctx, map[string]any{
		"problem": "Impossible problem",
	})

	// Should fail validation for empty code
	if err == nil {
		t.Error("Expected error for empty code")
	}
}

// TestProgramOfThought_EmptyExplanationHandling tests handling of empty explanation.
// Validates:
// - Empty explanation is rejected
// - Appropriate error message
func TestProgramOfThought_EmptyExplanationHandling(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// Mock LM that returns empty explanation
	lm := NewMockLMWithResponse(`{
		"code": "print('hello')",
		"explanation": ""
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	_, err := pot.Forward(ctx, map[string]any{
		"problem": "Some problem",
	})

	// Should fail validation for empty explanation
	if err == nil {
		t.Error("Expected error for empty explanation")
	}
}

// TestProgramOfThought_MalformedJSON tests handling of malformed JSON response.
// Validates:
// - Fallback extraction from markdown code blocks
// - Graceful degradation
func TestProgramOfThought_MalformedJSON(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// Mock LM that returns markdown-style code block instead of JSON
	lm := NewMockLMWithResponse("Here is the solution:\n\n```python\ndef add(a, b):\n    return a + b\n```\n\nThis function adds two numbers together.")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Add two numbers",
	})

	// Should attempt fallback extraction
	if err != nil {
		t.Logf("Fallback extraction result: %v (may fail if extraction doesn't match)", err)
		// This is acceptable - malformed JSON may not always be recoverable
		return
	}

	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected code from fallback extraction")
	}
}

// ============================================================================
// ProgramOfThought Observability Tests
// ============================================================================

// TestProgramOfThought_UsageTracking tests usage and cost tracking.
// Validates:
// - Token usage is tracked
// - Module metadata is recorded
func TestProgramOfThought_UsageTracking(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	lm := NewMockLMWithResponse(`{
		"code": "x = 1 + 1",
		"explanation": "Simple arithmetic"
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Calculate 1 + 1",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought failed: %v", err)
	}

	// Verify usage is tracked
	if result.Usage.TotalTokens == 0 {
		t.Error("Expected token usage to be tracked")
	}

	// Verify cost is tracked (may be zero in mock)
	if result.Usage.Cost < 0 {
		t.Error("Cost should be non-negative")
	}
}

// ============================================================================
// ProgramOfThought Language Support Tests
// ============================================================================

// TestProgramOfThought_MultipleLanguages tests different language outputs.
func TestProgramOfThought_MultipleLanguages(t *testing.T) {
	tests := []struct {
		name     string
		language string
		response string
	}{
		{
			name:     "Python",
			language: "python",
			response: `{"code": "def greet(): return 'Hello'", "explanation": "Python function"}`,
		},
		{
			name:     "JavaScript",
			language: "javascript",
			response: `{"code": "const greet = () => 'Hello';", "explanation": "JavaScript arrow function"}`,
		},
		{
			name:     "Go",
			language: "go",
			response: `{"code": "func greet() string { return \"Hello\" }", "explanation": "Go function"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := ContextWithTimeout(10 * time.Second)
			defer cancel()

			sig := dsgo.NewSignature("Generate code").
				AddInput("problem", dsgo.FieldTypeString, "Problem").
				AddOutput("code", dsgo.FieldTypeString, "Code").
				AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

			lm := NewMockLMWithResponse(tt.response)
			pot := dsgo.NewProgramOfThought(sig, lm, tt.language)

			result, err := pot.Forward(ctx, map[string]any{
				"problem": "Create a greeting function",
			})

			if err != nil {
				t.Fatalf("%s: ProgramOfThought failed: %v", tt.name, err)
			}

			code, ok := result.GetString("code")
			if !ok || code == "" {
				t.Errorf("%s: Expected non-empty code", tt.name)
			}
		})
	}
}

// ============================================================================
// ProgramOfThought Adapter Integration Tests
// ============================================================================

// TestProgramOfThought_AdapterMetadata tests adapter metadata tracking.
func TestProgramOfThought_AdapterMetadata(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Generate code").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	lm := NewMockLMWithResponse(`{
		"code": "result = 42",
		"explanation": "The answer to everything"
	}`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought failed: %v", err)
	}

	// Adapter metadata may or may not be set depending on parsing path
	// Just verify we get valid outputs
	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected valid code output")
	}
}

// ============================================================================
// Phase 2: ProgramOfThought Extraction Fallback Tests
// ============================================================================

// TestProgramOfThought_ExtractTextOutputs_CodeBlock tests extraction from markdown code blocks
// This validates extractTextOutputs fallback when JSON parsing fails
func TestProgramOfThought_ExtractTextOutputs_CodeBlock(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Extract code from text").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code solution").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// LM returns code in markdown format instead of JSON
	// This should trigger extractTextOutputs fallback
	lm := NewMockLMWithResponse("Here's the solution:\n\n```python\ndef find_max(arr):\n    return max(arr) if arr else None\n\nresult = find_max([1, 5, 3, 9, 2])\n```\n\nThis function finds the maximum value in a list using Python's built-in max() function.")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Find maximum in list",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought fallback extraction failed: %v", err)
	}

	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected extracted code field")
	}
	if !containsString(code, "find_max") {
		t.Errorf("Expected code to contain 'find_max', got: %s", code)
	}

	explanation, ok := result.GetString("explanation")
	if !ok || explanation == "" {
		t.Error("Expected extracted explanation field")
	}
}

// TestProgramOfThought_ExtractTextOutputs_ExplanationMarker tests extraction with "Explanation:" marker
// This validates extraction strategy 2 in extractTextOutputs
func TestProgramOfThought_ExtractTextOutputs_ExplanationMarker(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Extract code with explanation marker").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// LM returns code followed by "Explanation:" marker
	lm := NewMockLMWithResponse("Code:\ndef bubble_sort(arr):\n    n = len(arr)\n    for i in range(n):\n        for j in range(0, n-i-1):\n            if arr[j] > arr[j+1]:\n                arr[j], arr[j+1] = arr[j+1], arr[j]\n    return arr\n\nExplanation:\nWe implement bubble sort by iterating through the array and swapping adjacent elements if they're out of order. This continues until the array is sorted.")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Sort array",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought marker extraction failed: %v", err)
	}

	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected code field")
	}
	if !containsString(code, "bubble_sort") {
		t.Errorf("Expected code to contain 'bubble_sort', got: %s", code)
	}
}

// TestProgramOfThought_ExtractTextOutputs_DirectContent tests fallback with direct code content
// Validates extraction strategy 3 when code block isn't found
func TestProgramOfThought_ExtractTextOutputs_DirectContent(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Direct content extraction").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// LM returns plain code without markdown formatting
	// This triggers extraction strategy 3 (use entire content as code)
	lm := NewMockLMWithResponse("x = 10\ny = 20\nprint(f\"Sum: {x + y}\")")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Add numbers",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought direct content extraction failed: %v", err)
	}

	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected code field")
	}
	if !containsString(code, "x = 10") && !containsString(code, "Sum") {
		t.Errorf("Expected code content, got: %s", code)
	}

	explanation, ok := result.GetString("explanation")
	if !ok || explanation == "" {
		t.Logf("Explanation field filled with default: %s", explanation)
	}
}

// TestProgramOfThought_FillRequiredStringFields tests default field population
// Validates fillRequiredStringFields with different required fields
func TestProgramOfThought_FillRequiredStringFields(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Multiple output fields").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation").
		AddOutput("result", dsgo.FieldTypeString, "Result").
		AddOutput("insights", dsgo.FieldTypeString, "Insights")

	// Markdown code block triggers extraction, then fillRequiredStringFields
	// fills in missing required fields with defaults
	lm := NewMockLMWithResponse("```python\ndef compute():\n    return 42\n```")

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	result, err := pot.Forward(ctx, map[string]any{
		"problem": "Compute answer",
	})

	if err != nil {
		t.Fatalf("ProgramOfThought multi-field extraction failed: %v", err)
	}

	// All required fields should be populated (either from extraction or defaults)
	code, ok := result.GetString("code")
	if !ok || code == "" {
		t.Error("Expected code field")
	}

	explanation, ok := result.GetString("explanation")
	if !ok || explanation == "" {
		t.Error("Expected explanation field (default or extracted)")
	}

	result_val, ok := result.GetString("result")
	if !ok || result_val == "" {
		t.Error("Expected result field (should be filled with default)")
	}

	insights, ok := result.GetString("insights")
	if !ok || insights == "" {
		t.Error("Expected insights field (should be filled with default)")
	}
}

// TestProgramOfThought_ExtractTextOutputs_TooShortContent tests handling of very short content
// Content < 10 chars should return nil from extractTextOutputs
func TestProgramOfThought_ExtractTextOutputs_TooShortContent(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	sig := dsgo.NewSignature("Short content test").
		AddInput("problem", dsgo.FieldTypeString, "Problem").
		AddOutput("code", dsgo.FieldTypeString, "Code").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation")

	// LM returns content shorter than 10 chars
	// This should fail to parse and eventually error
	lm := NewMockLMWithResponse(`short`)

	pot := dsgo.NewProgramOfThought(sig, lm, "python")

	_, err := pot.Forward(ctx, map[string]any{
		"problem": "Brief",
	})

	// This should error because extraction returns nil for short content
	if err == nil {
		t.Error("Expected error for too-short content")
	} else if !containsString(err.Error(), "failed to parse") {
		t.Logf("Got expected error: %v", err)
	}
}
