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

	result, err := pot.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Use FallbackAdapter to parse output
	adapter := dsgo.NewFallbackAdapter()
	outputs, err := adapter.Parse(pot.Signature, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

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
