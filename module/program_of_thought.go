package module

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

// ProgramOfThought generates and executes code to solve problems
// This is useful for mathematical reasoning, data processing, etc.
type ProgramOfThought struct {
	Signature        *dsgo.Signature
	LM               dsgo.LM
	Options          *dsgo.GenerateOptions
	Language         string // "python", "javascript", "go"
	AllowExecution   bool
	ExecutionTimeout int // seconds
}

// NewProgramOfThought creates a new ProgramOfThought module
func NewProgramOfThought(signature *dsgo.Signature, lm dsgo.LM, language string) *ProgramOfThought {
	return &ProgramOfThought{
		Signature:        signature,
		LM:               lm,
		Options:          dsgo.DefaultGenerateOptions(),
		Language:         language,
		AllowExecution:   false, // Disabled by default for safety
		ExecutionTimeout: 30,
	}
}

// WithOptions sets custom generation options
func (pot *ProgramOfThought) WithOptions(options *dsgo.GenerateOptions) *ProgramOfThought {
	pot.Options = options
	return pot
}

// WithAllowExecution enables code execution (use with caution!)
func (pot *ProgramOfThought) WithAllowExecution(allow bool) *ProgramOfThought {
	pot.AllowExecution = allow
	return pot
}

// WithExecutionTimeout sets the execution timeout in seconds
func (pot *ProgramOfThought) WithExecutionTimeout(seconds int) *ProgramOfThought {
	pot.ExecutionTimeout = seconds
	return pot
}

// GetSignature returns the module's signature
func (pot *ProgramOfThought) GetSignature() *dsgo.Signature {
	return pot.Signature
}

// Forward executes the program of thought
func (pot *ProgramOfThought) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	if err := pot.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	prompt, err := pot.buildPrompt(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []dsgo.Message{
		{Role: "user", Content: prompt},
	}

	// Copy options to avoid mutation
	options := pot.Options.Copy()
	// ProgramOfThought uses FallbackAdapter but prefers JSON for reliable parsing
	// Force JSON mode to ensure models follow the format specification
	options.ResponseFormat = "json"
	// Auto-generate JSON schema from signature for structured outputs
	if options.ResponseSchema == nil {
		options.ResponseSchema = pot.Signature.SignatureToJSONSchema()
	}

	result, err := pot.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Handle finish_reason: ProgramOfThought doesn't support tool execution loops
	if result.FinishReason == "tool_calls" {
		return nil, fmt.Errorf("model requested tool execution (finish_reason=tool_calls) but ProgramOfThought module doesn't support tool loops - use React module instead")
	}

	// Handle finish_reason=length: Model hit max_tokens, output truncated/incomplete
	if result.FinishReason == "length" {
		return nil, fmt.Errorf("model hit max_tokens limit (finish_reason=length) - output truncated - increase MaxTokens in options")
	}

	// Check for empty content with finish_reason=stop (actual error)
	if result.Content == "" && result.FinishReason == "stop" {
		return nil, fmt.Errorf("model returned empty content despite finish_reason=stop (model error)")
	}

	// Use FallbackAdapter to parse output
	adapter := dsgo.NewFallbackAdapter()
	outputs, err := adapter.Parse(pot.Signature, result.Content)
	if err != nil {
		// FALLBACK: If structured parsing fails, attempt text extraction for string fields
		// This makes PoT resilient to less capable models that don't follow structured formats
		extractedOutputs := pot.extractTextOutputs(result.Content)
		if len(extractedOutputs) > 0 {
			outputs = extractedOutputs
		} else {
			return nil, fmt.Errorf("failed to parse output: %w", err)
		}
	}

	// Apply output normalization before validation
	outputs = dsgo.NormalizeOutputKeys(pot.Signature, outputs)

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Execute code if enabled
	if pot.AllowExecution {
		if code, exists := outputs["code"]; exists {
			executionResult, err := pot.executeCode(ctx, fmt.Sprintf("%v", code))
			if err != nil {
				outputs["execution_error"] = err.Error()
			} else {
				outputs["execution_result"] = executionResult
			}
		}
	}

	if err := pot.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	// Build Prediction object
	prediction := dsgo.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName("ProgramOfThought").
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

func (pot *ProgramOfThought) buildPrompt(inputs map[string]any) (string, error) {
	var prompt strings.Builder

	// Add description
	if pot.Signature.Description != "" {
		prompt.WriteString(pot.Signature.Description)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString(fmt.Sprintf("Solve this problem by writing %s code.\n", pot.Language))
	prompt.WriteString("Think step-by-step and generate code that solves the problem.\n\n")

	// Add input fields
	if len(pot.Signature.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range pot.Signature.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				return "", fmt.Errorf("missing required input field: %s", field.Name)
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add output format specification
	if len(pot.Signature.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing:\n")
		prompt.WriteString(fmt.Sprintf("- code (string): The %s code to solve the problem\n", pot.Language))
		prompt.WriteString("- explanation (string): Step-by-step explanation of the code\n")
		for _, field := range pot.Signature.OutputFields {
			if field.Name == "code" || field.Name == "explanation" {
				continue // Already included
			}
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == dsgo.FieldTypeClass && len(field.Classes) > 0 {
				classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
			}
		}
	}

	return prompt.String(), nil
}

func (pot *ProgramOfThought) executeCode(ctx context.Context, code string) (string, error) {
	// Create a timeout context for code execution
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(pot.ExecutionTimeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	switch pot.Language {
	case "python":
		cmd = exec.CommandContext(execCtx, "python3", "-c", code)
	case "javascript":
		cmd = exec.CommandContext(execCtx, "node", "-e", code)
	case "go":
		// Go requires a file, so we'll skip execution for now
		return "", fmt.Errorf("go code execution not yet supported")
	default:
		return "", fmt.Errorf("unsupported language: %s", pot.Language)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("execution timeout after %d seconds", pot.ExecutionTimeout)
		}
		return string(output), fmt.Errorf("execution failed: %w", err)
	}

	return string(output), nil
}

// extractTextOutputs attempts to extract output fields from raw text when structured parsing fails
// This is a last-resort fallback for less capable models that don't follow JSON/Chat formats
// PoT typically expects 'code' and 'explanation' fields
func (pot *ProgramOfThought) extractTextOutputs(content string) map[string]any {
	outputs := make(map[string]any)
	content = strings.TrimSpace(content)

	// If content is empty or very short, cannot extract
	if len(content) < 10 {
		return nil
	}

	// Only attempt extraction for string fields
	var stringFields []dsgo.Field
	for _, field := range pot.Signature.OutputFields {
		if field.Type == dsgo.FieldTypeString {
			stringFields = append(stringFields, field)
		}
	}

	if len(stringFields) == 0 {
		return nil
	}

	// Strategy 1: Try to extract code block from markdown-style code fences
	codeField := pot.Signature.GetOutputField("code")
	if codeField != nil && codeField.Type == dsgo.FieldTypeString {
		// Look for code blocks with language tags (```python, ```javascript, etc.)
		codeBlockPattern := "```" + pot.Language
		if idx := strings.Index(content, codeBlockPattern); idx != -1 {
			start := idx + len(codeBlockPattern)
			// Skip to newline after language tag
			if newlineIdx := strings.Index(content[start:], "\n"); newlineIdx != -1 {
				start += newlineIdx + 1
			}
			if endIdx := strings.Index(content[start:], "```"); endIdx != -1 {
				code := strings.TrimSpace(content[start : start+endIdx])
				outputs["code"] = code

				// Extract explanation as the remaining text (before or after code block)
				explanationField := pot.Signature.GetOutputField("explanation")
				if explanationField != nil {
					// Use text before code block as explanation
					explanation := strings.TrimSpace(content[:idx])
					if explanation == "" {
						// If nothing before, use text after
						explanation = strings.TrimSpace(content[start+endIdx+3:])
					}
					if explanation != "" {
						outputs["explanation"] = explanation
					} else if !explanationField.Optional {
						outputs["explanation"] = "Code provided to solve the problem"
					}
				}

				// Fill in other string fields with placeholders if required
				pot.fillRequiredStringFields(outputs, stringFields)
				return outputs
			}
		}

		// Fallback: Look for generic code blocks (```)
		if idx := strings.Index(content, "```"); idx != -1 {
			start := idx + 3
			// Skip language tag if present (until newline)
			if newlineIdx := strings.Index(content[start:], "\n"); newlineIdx != -1 {
				start += newlineIdx + 1
			}
			if endIdx := strings.Index(content[start:], "```"); endIdx != -1 {
				code := strings.TrimSpace(content[start : start+endIdx])
				outputs["code"] = code

				// Extract explanation as the remaining text
				explanationField := pot.Signature.GetOutputField("explanation")
				if explanationField != nil {
					explanation := strings.TrimSpace(content[:idx])
					if explanation == "" {
						explanation = strings.TrimSpace(content[start+endIdx+3:])
					}
					if explanation != "" {
						outputs["explanation"] = explanation
					} else if !explanationField.Optional {
						outputs["explanation"] = "Code provided to solve the problem"
					}
				}

				// Fill in other string fields with placeholders if required
				pot.fillRequiredStringFields(outputs, stringFields)
				return outputs
			}
		}
	}

	// Strategy 2: Look for "Explanation:" sections to extract both code and explanation
	if codeField != nil && len(outputs) == 0 {
		// Split by common section markers
		lowerContent := strings.ToLower(content)
		explanationIdx := -1
		for _, marker := range []string{"explanation:", "**explanation**", "## explanation", "### explanation"} {
			if idx := strings.Index(lowerContent, marker); idx != -1 {
				explanationIdx = idx
				break
			}
		}

		if explanationIdx != -1 {
			// Treat everything before "Explanation:" as code
			code := strings.TrimSpace(content[:explanationIdx])
			explanation := strings.TrimSpace(content[explanationIdx:])

			// Clean up code (remove common prefixes)
			code = strings.TrimPrefix(code, "Generated Code:")
			code = strings.TrimPrefix(code, "Code:")
			code = strings.TrimPrefix(code, "**Code:**")
			code = strings.TrimSpace(code)

			if code != "" {
				outputs["code"] = code
			}

			explanationField := pot.Signature.GetOutputField("explanation")
			if explanationField != nil && explanation != "" {
				outputs["explanation"] = explanation
			}
		}
	}

	// Strategy 3: If still no code found, use entire content as code (last resort)
	// This handles cases where model outputs code directly without markdown formatting
	if codeField != nil && len(outputs) == 0 {
		outputs["code"] = content

		// Fill in other string fields with placeholders if required
		pot.fillRequiredStringFields(outputs, stringFields)
	}

	return outputs
}

// fillRequiredStringFields fills in required string fields that aren't already populated
func (pot *ProgramOfThought) fillRequiredStringFields(outputs map[string]any, stringFields []dsgo.Field) {
	for _, field := range stringFields {
		if _, exists := outputs[field.Name]; !exists && !field.Optional {
			// Provide reasonable defaults based on field name
			switch field.Name {
			case "explanation":
				outputs["explanation"] = "Code provided to solve the problem"
			case "result":
				outputs["result"] = "Computed by code execution"
			case "answer":
				outputs["answer"] = "See code output"
			case "steps":
				outputs["steps"] = "Implemented in the code above"
			case "insights":
				outputs["insights"] = "Analysis performed by code"
			default:
				outputs[field.Name] = fmt.Sprintf("Output for %s", field.Name)
			}
		}
	}
}
