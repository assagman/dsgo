package module

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// MultiChainComparison synthesizes the best answer from M reasoning attempts.
// It follows DSPy's design: accepts pre-generated completions and performs synthesis.
// The signature is transformed to include reasoning attempts as INPUT fields.
type MultiChainComparison struct {
	BaseSignature     *core.Signature // Original signature without transformations
	internalSignature *core.Signature // Internal signature with reasoning_attempt INPUT fields
	predict           *Predict        // Underlying Predict module for synthesis
	LM                core.LM         // Language model for synthesis
	M                 int             // Number of reasoning attempts
	lastKey           string          // Name of the last output field
	AttemptTemplate   string          // Template for formatting attempts
	Options           *core.GenerateOptions
	Adapter           core.Adapter
	History           *core.History  // Optional conversation history
	Demos             []core.Example // Optional few-shot examples
}

// NewMultiChainComparison creates a new MultiChainComparison module.
// It accepts a base signature, language model, and number of reasoning attempts M.
func NewMultiChainComparison(baseSignature *core.Signature, lm core.LM, m int) *MultiChainComparison {
	// Constructor validation
	if baseSignature == nil {
		panic("baseSignature cannot be nil")
	}
	if m <= 0 {
		panic("m must be positive")
	}
	if len(baseSignature.OutputFields) == 0 {
		panic("signature must have at least one output field")
	}

	// Build internal signature with M INPUT fields for reasoning attempts
	internalSig, lastKey := buildMCCSignature(baseSignature, m)

	// Create underlying Predict module
	predict := NewPredict(internalSig, lm)

	return &MultiChainComparison{
		BaseSignature:     baseSignature,
		internalSignature: internalSig,
		predict:           predict,
		LM:                lm,
		M:                 m,
		lastKey:           lastKey,
		AttemptTemplate:   "I'm trying to {rationale} I'm not sure but my prediction is {answer}",
		Options:           core.DefaultGenerateOptions(),
		Adapter:           core.NewFallbackAdapter(),
	}
}

// WithTemperature sets the temperature for synthesis.
func (mcc *MultiChainComparison) WithTemperature(temp float64) *MultiChainComparison {
	mcc.Options.Temperature = temp
	return mcc
}

// WithOptions sets custom generation options.
func (mcc *MultiChainComparison) WithOptions(options *core.GenerateOptions) *MultiChainComparison {
	mcc.Options = options
	return mcc
}

// WithAdapter sets a custom adapter.
func (mcc *MultiChainComparison) WithAdapter(adapter core.Adapter) *MultiChainComparison {
	mcc.Adapter = adapter
	return mcc
}

// WithAttemptTemplate sets the template for formatting reasoning attempts.
func (mcc *MultiChainComparison) WithAttemptTemplate(template string) *MultiChainComparison {
	mcc.AttemptTemplate = template
	return mcc
}

// WithHistory sets conversation history for the synthesis module.
func (mcc *MultiChainComparison) WithHistory(history *core.History) *MultiChainComparison {
	mcc.History = history
	mcc.predict.WithHistory(history)
	return mcc
}

// WithDemos sets few-shot examples for the synthesis module.
func (mcc *MultiChainComparison) WithDemos(demos []core.Example) *MultiChainComparison {
	mcc.Demos = demos
	mcc.predict.WithDemos(demos)
	return mcc
}

// GetSignature returns the base signature (not the internal transformed one).
// This preserves the original API contract for composability.
func (mcc *MultiChainComparison) GetSignature() *core.Signature {
	return mcc.BaseSignature
}

// Forward executes multi-chain comparison synthesis.
// It expects completions to be provided via inputs["completions"].
func (mcc *MultiChainComparison) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleMultiChainComparison, mcc.BaseSignature.Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleMultiChainComparison, time.Since(startTime), predErr)
	}()

	// Extract and validate completions from inputs["completions"]
	completions, err := mcc.extractCompletions(inputs)
	if err != nil {
		predErr = fmt.Errorf("failed to extract completions: %w", err)
		return nil, predErr
	}
	if len(completions) != mcc.M {
		predErr = fmt.Errorf("expected %d completions, got %d", mcc.M, len(completions))
		return nil, predErr
	}

	logging.GetLogger().Debug(ctx, "MCC synthesizing", map[string]any{
		"completions": len(completions),
	})

	// Build new inputs with reasoning_attempt_N fields
	newInputs := make(map[string]any)
	// Copy original inputs (excluding completions)
	for _, f := range mcc.BaseSignature.InputFields {
		if v, ok := inputs[f.Name]; ok {
			newInputs[f.Name] = v
		}
	}
	// Format and add reasoning attempts
	for i, comp := range completions {
		attemptStr := mcc.formatAttempt(comp)
		newInputs[fmt.Sprintf("reasoning_attempt_%d", i+1)] = attemptStr
	}

	// Call underlying Predict
	result, err := mcc.predict.Forward(ctx, newInputs)
	if err != nil {
		predErr = fmt.Errorf("synthesis failed: %w", err)
		return nil, predErr
	}

	// Map synthesis outputs back to original signature
	finalOutputs := make(map[string]any)

	// Copy original output fields from synthesis result
	for _, field := range mcc.BaseSignature.OutputFields {
		if value, exists := result.Outputs[field.Name]; exists {
			finalOutputs[field.Name] = value
		}
	}

	// Add rationale if it exists in synthesis result
	if rationale, exists := result.Outputs["rationale"]; exists {
		finalOutputs["rationale"] = rationale
	}

	// Create final prediction
	prediction := core.NewPrediction(finalOutputs).
		WithModuleName(logging.ModuleMultiChainComparison).
		WithInputs(inputs).
		WithUsage(result.Usage)

	return prediction, nil
}

// extractCompletions extracts completions from inputs in various formats.
// Supports []*core.Prediction, []map[string]any, and []any (from JSON unmarshaling).
func (mcc *MultiChainComparison) extractCompletions(inputs map[string]any) ([]map[string]any, error) {
	completionsValue, exists := inputs["completions"]
	if !exists {
		return nil, fmt.Errorf("completions field is required")
	}

	switch completions := completionsValue.(type) {
	case []*core.Prediction:
		// Convert []*Prediction to []map[string]any
		result := make([]map[string]any, len(completions))
		for i, pred := range completions {
			result[i] = pred.Outputs
		}
		return result, nil

	case []map[string]any:
		// Already in correct format
		return completions, nil

	case []any:
		// Convert []any to []map[string]any
		result := make([]map[string]any, len(completions))
		for i, item := range completions {
			if m, ok := item.(map[string]any); ok {
				result[i] = m
			} else {
				return nil, fmt.Errorf("completion at index %d is not a map", i)
			}
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported completions type: %T", completionsValue)
	}
}

// formatAttempt formats a completion using the attempt template.
// It extracts rationale and answer from the completion using DSPy-style aliases.
func (mcc *MultiChainComparison) formatAttempt(completion map[string]any) string {
	// Extract rationale using DSPy aliases
	var rationale string
	for _, key := range []string{"rationale", "reasoning", "thought", "explanation"} {
		if value, exists := completion[key]; exists {
			if str, ok := value.(string); ok {
				rationale = firstLine(str)
				break
			}
		}
	}

	// Extract answer (use first non-rationale field)
	var answer string
	for key, value := range completion {
		if key == "rationale" || key == "reasoning" || key == "thought" || key == "explanation" {
			continue
		}
		if str, ok := value.(string); ok {
			answer = firstLine(str)
			break
		}
	}

	// Apply template
	template := mcc.AttemptTemplate
	template = strings.ReplaceAll(template, "{rationale}", rationale)
	template = strings.ReplaceAll(template, "{answer}", answer)

	return template
}

// firstLine extracts the first line from a string, trimming whitespace.
func firstLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// buildMCCSignature builds the internal signature for MultiChainComparison.
// It adds M reasoning_attempt INPUT fields and prepends rationale as first OUTPUT.
func buildMCCSignature(baseSig *core.Signature, m int) (*core.Signature, string) {
	// Start with base signature description
	sig := core.NewSignature(baseSig.Description + " (with multi-chain comparison)")

	// Copy original input fields
	for _, field := range baseSig.InputFields {
		sig.AddInput(field.Name, field.Type, field.Description)
	}

	// Add M reasoning_attempt INPUT fields with "Student Attempt" framing
	for i := 0; i < m; i++ {
		fieldName := fmt.Sprintf("reasoning_attempt_%d", i+1)
		description := fmt.Sprintf("Student Attempt #%d: A previous reasoning attempt for this task", i+1)
		sig.AddInput(fieldName, core.FieldTypeString, description)
	}

	// PREPEND rationale as first output field (DSPy design)
	sig.AddOutput("rationale", core.FieldTypeString, "Rationale for the synthesized answer")

	// Copy original output fields
	var lastKey string
	for _, field := range baseSig.OutputFields {
		sig.AddOutput(field.Name, field.Type, field.Description)
		lastKey = field.Name
	}

	return sig, lastKey
}

// RequiresCompletions implements CompletionsConsumer interface
func (mcc *MultiChainComparison) RequiresCompletions() bool {
	return true
}

// Clone creates an independent copy of MultiChainComparison module.
func (mcc *MultiChainComparison) Clone() core.Module {
	cloned := &MultiChainComparison{
		BaseSignature:     mcc.BaseSignature,
		internalSignature: mcc.internalSignature,
		predict:           mcc.predict.Clone().(*Predict),
		LM:                mcc.LM,
		M:                 mcc.M,
		lastKey:           mcc.lastKey,
		AttemptTemplate:   mcc.AttemptTemplate,
		Options:           mcc.Options.Copy(),
		Adapter:           mcc.Adapter,
		History:           mcc.History,
		Demos:             append([]core.Example{}, mcc.Demos...),
	}
	return cloned
}
