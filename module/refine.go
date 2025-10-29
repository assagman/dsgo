package module

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/assagman/dsgo"
)

// Refine implements iterative refinement of predictions
// It takes an initial prediction and refines it based on feedback or additional context
type Refine struct {
	Signature       *dsgo.Signature
	LM              dsgo.LM
	Options         *dsgo.GenerateOptions
	MaxIterations   int
	RefinementField string // Field name to use for refinement feedback
}

// NewRefine creates a new Refine module
func NewRefine(signature *dsgo.Signature, lm dsgo.LM) *Refine {
	return &Refine{
		Signature:       signature,
		LM:              lm,
		Options:         dsgo.DefaultGenerateOptions(),
		MaxIterations:   3,
		RefinementField: "feedback",
	}
}

// WithOptions sets custom generation options
func (r *Refine) WithOptions(options *dsgo.GenerateOptions) *Refine {
	r.Options = options
	return r
}

// WithMaxIterations sets the maximum number of refinement iterations
func (r *Refine) WithMaxIterations(max int) *Refine {
	r.MaxIterations = max
	return r
}

// WithRefinementField sets the field name for refinement feedback
func (r *Refine) WithRefinementField(field string) *Refine {
	r.RefinementField = field
	return r
}

// GetSignature returns the module's signature
func (r *Refine) GetSignature() *dsgo.Signature {
	return r.Signature
}

// Forward executes the refinement loop
func (r *Refine) Forward(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if err := r.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Generate initial prediction
	outputs, err := r.generatePrediction(ctx, inputs, nil)
	if err != nil {
		return nil, fmt.Errorf("initial prediction failed: %w", err)
	}

	// Check if feedback is provided for refinement
	feedback, hasFeedback := inputs[r.RefinementField]
	if !hasFeedback || r.MaxIterations <= 1 {
		return outputs, nil
	}

	// Refinement loop
	for i := 0; i < r.MaxIterations-1; i++ {
		// Generate refinement prompt
		refined, err := r.generateRefinement(ctx, inputs, outputs, fmt.Sprintf("%v", feedback))
		if err != nil {
			// If refinement fails, return the last valid output
			return outputs, nil
		}

		outputs = refined
	}

	return outputs, nil
}

func (r *Refine) generatePrediction(ctx context.Context, inputs map[string]any, previousOutput map[string]any) (map[string]any, error) {
	var prompt strings.Builder

	// Add description
	if r.Signature.Description != "" {
		prompt.WriteString(r.Signature.Description)
		prompt.WriteString("\n\n")
	}

	// Add input fields
	if len(r.Signature.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range r.Signature.InputFields {
			if field.Name == r.RefinementField {
				continue // Skip feedback field in initial prediction
			}
			value, exists := inputs[field.Name]
			if !exists {
				continue
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add previous output if refining
	if previousOutput != nil {
		prompt.WriteString("--- Previous Output ---\n")
		for k, v := range previousOutput {
			prompt.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		prompt.WriteString("\n")
	}

	// Add output format specification
	if len(r.Signature.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing:\n")
		for _, field := range r.Signature.OutputFields {
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

	messages := []dsgo.Message{
		{Role: "user", Content: prompt.String()},
	}

	options := r.Options
	if r.LM.SupportsJSON() {
		options.ResponseFormat = "json"
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	outputs, err := r.parseOutput(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	return outputs, nil
}

func (r *Refine) generateRefinement(ctx context.Context, inputs map[string]any, previousOutput map[string]any, feedback string) (map[string]any, error) {
	var prompt strings.Builder

	prompt.WriteString("Refine the previous output based on the following feedback:\n\n")
	prompt.WriteString(fmt.Sprintf("Feedback: %s\n\n", feedback))

	// Add previous output
	prompt.WriteString("--- Previous Output ---\n")
	for k, v := range previousOutput {
		prompt.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	prompt.WriteString("\n")

	// Add original inputs for context
	prompt.WriteString("--- Original Inputs ---\n")
	for _, field := range r.Signature.InputFields {
		if field.Name == r.RefinementField {
			continue
		}
		value, exists := inputs[field.Name]
		if !exists {
			continue
		}
		prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
	}
	prompt.WriteString("\n")

	// Add output format
	prompt.WriteString("--- Improved Output Format ---\n")
	prompt.WriteString("Respond with a JSON object containing the refined version:\n")
	for _, field := range r.Signature.OutputFields {
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

	messages := []dsgo.Message{
		{Role: "user", Content: prompt.String()},
	}

	options := r.Options
	if r.LM.SupportsJSON() {
		options.ResponseFormat = "json"
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, err
	}

	outputs, err := r.parseOutput(result.Content)
	if err != nil {
		return nil, err
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, err
	}

	return outputs, nil
}

func (r *Refine) parseOutput(content string) (map[string]any, error) {
	content = strings.TrimSpace(content)

	// Try to extract JSON from markdown code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else {
		// Try to find JSON object in the content
		start := strings.Index(content, "{")
		if start >= 0 {
			// Find matching closing brace
			depth := 0
			for i := start; i < len(content); i++ {
				if content[i] == '{' {
					depth++
				} else if content[i] == '}' {
					depth--
					if depth == 0 {
						content = content[start : i+1]
						break
					}
				}
			}
		}
	}

	content = strings.TrimSpace(content)

	var outputs map[string]any
	if err := json.Unmarshal([]byte(content), &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, content)
	}

	return outputs, nil
}
