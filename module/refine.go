package module

import (
	"context"
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
	Adapter         dsgo.Adapter
	MaxIterations   int
	RefinementField string // Field name to use for refinement feedback
}

// NewRefine creates a new Refine module
func NewRefine(signature *dsgo.Signature, lm dsgo.LM) *Refine {
	return &Refine{
		Signature:       signature,
		LM:              lm,
		Options:         dsgo.DefaultGenerateOptions(),
		Adapter:         dsgo.NewFallbackAdapter(),
		MaxIterations:   3,
		RefinementField: "feedback",
	}
}

// WithOptions sets custom generation options
func (r *Refine) WithOptions(options *dsgo.GenerateOptions) *Refine {
	r.Options = options
	return r
}

// WithAdapter sets a custom adapter
func (r *Refine) WithAdapter(adapter dsgo.Adapter) *Refine {
	r.Adapter = adapter
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
func (r *Refine) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	if err := r.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Generate initial prediction
	prediction, err := r.generatePrediction(ctx, inputs, nil)
	if err != nil {
		return nil, fmt.Errorf("initial prediction failed: %w", err)
	}

	// Check if feedback is provided for refinement
	feedback, hasFeedback := inputs[r.RefinementField]
	if !hasFeedback || r.MaxIterations <= 1 {
		return prediction, nil
	}

	// Refinement loop
	for i := 0; i < r.MaxIterations-1; i++ {
		// Generate refinement prompt
		refined, err := r.generateRefinement(ctx, inputs, prediction.Outputs, fmt.Sprintf("%v", feedback))
		if err != nil {
			// If refinement fails, return the last valid prediction
			return prediction, nil
		}

		prediction = refined
	}

	return prediction, nil
}

func (r *Refine) generatePrediction(ctx context.Context, inputs map[string]any, previousOutput map[string]any) (*dsgo.Prediction, error) {
	// Build custom prompt for refinement context
	var messages []dsgo.Message

	if previousOutput != nil {
		// If we have previous output, build custom refinement prompt
		var prompt strings.Builder

		prompt.WriteString("Refine the previous output based on the context:\n\n")

		// Add previous output
		prompt.WriteString("--- Previous Output ---\n")
		for k, v := range previousOutput {
			prompt.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		prompt.WriteString("\n")

		// Add inputs for context
		prompt.WriteString("--- Context ---\n")
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

		messages = []dsgo.Message{{Role: "user", Content: prompt.String()}}
	} else {
		// Initial prediction, use adapter
		var err error
		messages, err = r.Adapter.Format(r.Signature, inputs, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to format messages: %w", err)
		}
	}

	// Copy options to avoid mutation
	options := r.Options.Copy()
	if r.LM.SupportsJSON() {
		if _, isJSON := r.Adapter.(*dsgo.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				options.ResponseSchema = r.Signature.SignatureToJSONSchema()
			}
		}
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Use adapter to parse output
	outputs, err := r.Adapter.Parse(r.Signature, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := dsgo.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName("Refine").
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

func (r *Refine) generateRefinement(ctx context.Context, inputs map[string]any, previousOutput map[string]any, feedback string) (*dsgo.Prediction, error) {
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

	// Copy options to avoid mutation
	options := r.Options.Copy()
	if r.LM.SupportsJSON() {
		if _, isJSON := r.Adapter.(*dsgo.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				options.ResponseSchema = r.Signature.SignatureToJSONSchema()
			}
		}
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, err
	}

	// Use adapter to parse output
	outputs, err := r.Adapter.Parse(r.Signature, result.Content)
	if err != nil {
		return nil, err
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, err
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := dsgo.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName("Refine").
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}
