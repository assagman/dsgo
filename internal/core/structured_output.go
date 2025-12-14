package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// StructuredOutputStrategy represents the strategy being used for structured output enforcement
type StructuredOutputStrategy int

const (
	StrategyJSONSchema StructuredOutputStrategy = iota
	StrategyJSONObject
	StrategyPlainJSON
)

// StructuredMetaKey is the reserved output field name used to inject structured
// output metadata. User signatures MUST NOT use this field name as an output field.
// When structured outputs are enabled, this key is always present in the output map
// and contains metadata about the generation process (attempts, strategy, convergence).
const StructuredMetaKey = "__structured_meta"

func (s StructuredOutputStrategy) String() string {
	switch s {
	case StrategyJSONSchema:
		return "json_schema"
	case StrategyJSONObject:
		return "json_object"
	case StrategyPlainJSON:
		return "plain_json"
	default:
		return "unknown"
	}
}

// StructuredOutputMeta holds metadata about structured output enforcement attempts
type StructuredOutputMeta struct {
	Attempts         int                      // Number of attempts made
	LastStrategy     StructuredOutputStrategy // Last strategy attempted
	LastError        string                   // Last error encountered
	StrategyFallback bool                     // Whether strategy fallback occurred
	Converged        bool                     // Whether validation succeeded
}

// BuildValidationDiagnosticsString converts ValidationDiagnostics to a readable string
func BuildValidationDiagnosticsString(diag *ValidationDiagnostics) string {
	if diag == nil || !diag.HasErrors() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Validation errors:\n")

	if len(diag.MissingFields) > 0 {
		sb.WriteString(fmt.Sprintf("  Missing required fields: %v\n", diag.MissingFields))
	}

	if len(diag.TypeErrors) > 0 {
		sb.WriteString("  Type errors:\n")
		for field, err := range diag.TypeErrors {
			sb.WriteString(fmt.Sprintf("    - %s: %v\n", field, err))
		}
	}

	if len(diag.ClassErrors) > 0 {
		sb.WriteString("  Class/enum errors:\n")
		for field, err := range diag.ClassErrors {
			sb.WriteString(fmt.Sprintf("    - %s: %v\n", field, err))
		}
	}

	return sb.String()
}

// BuildRepairMessages builds repair prompt messages for structured output retry
// Expected format: JSON schema with required fields and types
// Unexpected format: previous model output (truncated)
// Errors: concrete validation failures
func BuildRepairMessages(sig *Signature, previousOutput string, diag *ValidationDiagnostics) []Message {
	var messages []Message

	// Build the repair instruction
	var repairPrompt strings.Builder
	repairPrompt.WriteString("The previous response was incomplete or invalid. Please provide a corrected response.\n\n")

	// Expected format: JSON schema
	repairPrompt.WriteString("Expected JSON format:\n")
	repairPrompt.WriteString("```json\n")
	repairPrompt.WriteString(buildJSONSchemaDescription(sig))
	repairPrompt.WriteString("\n```\n\n")

	// Unexpected format: previous output (truncated to 500 chars)
	repairPrompt.WriteString("Previous response (for context):\n")
	truncated := previousOutput
	if len(truncated) > 500 {
		truncated = truncated[:500] + "..."
	}
	repairPrompt.WriteString(fmt.Sprintf("```\n%s\n```\n\n", truncated))

	// Errors: validation failures
	if diag != nil && diag.HasErrors() {
		repairPrompt.WriteString("Errors to fix:\n")
		if len(diag.MissingFields) > 0 {
			repairPrompt.WriteString(fmt.Sprintf("  - Missing required fields: %v\n", diag.MissingFields))
		}
		if len(diag.TypeErrors) > 0 {
			for field, err := range diag.TypeErrors {
				repairPrompt.WriteString(fmt.Sprintf("  - %s: %v\n", field, err))
			}
		}
		if len(diag.ClassErrors) > 0 {
			for field, err := range diag.ClassErrors {
				repairPrompt.WriteString(fmt.Sprintf("  - %s: %v\n", field, err))
			}
		}
		repairPrompt.WriteString("\n")
	}

	// Hard instruction
	repairPrompt.WriteString("Return ONLY valid JSON. No commentary. No code fences. No markdown.\n")

	messages = append(messages, Message{
		Role:    "user",
		Content: repairPrompt.String(),
	})

	return messages
}

// buildJSONSchemaDescription builds a human-readable JSON schema description for repair prompts
func buildJSONSchemaDescription(sig *Signature) string {
	var sb strings.Builder
	sb.WriteString("{\n")

	for i, field := range sig.OutputFields {
		required := ""
		if !field.Optional {
			required = " (required)"
		}

		typeStr := fieldTypeToString(field.Type)
		if field.Type == FieldTypeClass && len(field.Classes) > 0 {
			typeStr = fmt.Sprintf("string, one of: %v", field.Classes)
		}

		sb.WriteString(fmt.Sprintf("  \"%s\": %s%s", field.Name, typeStr, required))
		if field.Description != "" {
			sb.WriteString(fmt.Sprintf(" // %s", field.Description))
		}

		if i < len(sig.OutputFields)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}")
	return sb.String()
}

// fieldTypeToString converts FieldType to a string representation for schema
func fieldTypeToString(ft FieldType) string {
	switch ft {
	case FieldTypeString:
		return "string"
	case FieldTypeInt:
		return "integer"
	case FieldTypeFloat:
		return "number"
	case FieldTypeBool:
		return "boolean"
	case FieldTypeJSON:
		return "object"
	case FieldTypeClass:
		return "string (enum)"
	case FieldTypeImage:
		return "string (base64 or URL)"
	case FieldTypeDatetime:
		return "string (ISO 8601)"
	default:
		return "string"
	}
}

// TruncateString truncates a string to maxLen characters with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ExtractJSONFromResponse attempts to extract valid JSON from a response string
// This handles cases where the model returns JSON with surrounding text
func ExtractJSONFromResponse(content string) (string, error) {
	// Try to find JSON object
	start := strings.Index(content, "{")
	if start == -1 {
		return "", fmt.Errorf("no JSON object found in response")
	}

	// Find matching closing brace
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(content); i++ {
		ch := content[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '"' && !escaped {
			inString = !inString
			continue
		}

		if !inString {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return content[start : i+1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("no complete JSON object found in response")
}

// ValidateJSONStructure validates that a string is valid JSON
func ValidateJSONStructure(content string) error {
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// GenerateStructuredOptions contains options for structured output generation
type GenerateStructuredOptions struct {
	// Adapter to use for formatting and parsing (required)
	Adapter Adapter

	// BaseOptions contains generation options to propagate (MaxTokens, TopP, Stop, etc.)
	// Temperature and StreamCallback are overridden by structured-specific settings.
	BaseOptions *GenerateOptions

	// Maximum number of attempts for structured output validation
	MaxAttempts int

	// Temperature override for structured mode
	Temperature float32

	// Whether to use JSON response format (controls strategy selection)
	// If false, only StrategyPlainJSON is used.
	UseJSONFormat bool

	// Callback for streaming (optional)
	StreamCallback StreamCallback
}

// GenerateStructuredResult holds the result of structured output generation
type GenerateStructuredResult struct {
	// Outputs from the LM (may be partial if validation failed)
	Outputs map[string]any

	// Validation diagnostics (if partial output returned)
	Diagnostics *ValidationDiagnostics

	// Structured output metadata
	Meta StructuredOutputMeta

	// Token usage
	Usage Usage

	// Raw content from the last LM attempt (useful for History)
	Content string

	// Adapter metrics from parsing (if available)
	AdapterUsed   string
	ParseAttempts int
	FallbackUsed  bool

	// Whether the output fully converged (passed strict validation)
	Converged bool
}

// GenerateStructured performs structured output generation with retry loop
// This is the core enforcement mechanism for structured outputs.
//
// The function attempts to generate valid structured output from an LM,
// with automatic retries and strategy fallback. It returns metadata
// including attempt count and convergence status for all calls.
func GenerateStructured(
	ctx context.Context,
	lm LM,
	sig *Signature,
	inputs map[string]any,
	demos []Example,
	opts GenerateStructuredOptions,
) (*GenerateStructuredResult, error) {
	// Validate required parameters
	if opts.Adapter == nil {
		return nil, fmt.Errorf("GenerateStructured: Adapter is required")
	}

	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}

	result := &GenerateStructuredResult{
		Meta: StructuredOutputMeta{
			Attempts:     0,
			LastStrategy: StrategyJSONSchema,
		},
		Outputs: make(map[string]any),
	}

	var lastOutput string
	var lastDiagnostics *ValidationDiagnostics
	var lastParsedOutputs map[string]any

	// Strategy ladder: try from most strict to most lenient
	// If UseJSONFormat is false, only use plain JSON strategy
	var strategies []StructuredOutputStrategy
	if opts.UseJSONFormat && lm.SupportsJSON() {
		strategies = []StructuredOutputStrategy{
			StrategyJSONSchema,
			StrategyJSONObject,
			StrategyPlainJSON,
		}
	} else {
		strategies = []StructuredOutputStrategy{
			StrategyPlainJSON,
		}
	}
	strategyIndex := 0

	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		result.Meta.Attempts = attempt + 1
		result.Meta.LastStrategy = strategies[strategyIndex]

		// Build messages for this attempt
		messages, err := opts.Adapter.Format(sig, inputs, demos)
		if err != nil {
			return nil, fmt.Errorf("failed to format prompt: %w", err)
		}

		// Add repair messages on retry
		if attempt > 0 && lastDiagnostics != nil && lastDiagnostics.HasErrors() {
			repairMessages := BuildRepairMessages(sig, lastOutput, lastDiagnostics)
			messages = append(messages, repairMessages...)
		}

		// Build generation options from BaseOptions if provided, then overlay structured-specific settings
		genOpts := &GenerateOptions{}
		if opts.BaseOptions != nil {
			// Copy base options to preserve MaxTokens, TopP, Stop, etc.
			genOpts.MaxTokens = opts.BaseOptions.MaxTokens
			genOpts.TopP = opts.BaseOptions.TopP
			genOpts.Stop = opts.BaseOptions.Stop
			genOpts.FrequencyPenalty = opts.BaseOptions.FrequencyPenalty
			genOpts.PresencePenalty = opts.BaseOptions.PresencePenalty
			genOpts.ProviderParams = opts.BaseOptions.ProviderParams
			// Propagate Tools/ToolChoice for Bedrock compatibility: when conversation
			// history contains tool calls, some providers (e.g., Amazon Bedrock) require
			// toolConfig to be present. We propagate but don't enable tool loops.
			genOpts.Tools = opts.BaseOptions.Tools
			genOpts.ToolChoice = opts.BaseOptions.ToolChoice
		}

		// Override with structured-specific settings
		genOpts.Temperature = float64(opts.Temperature)
		genOpts.Stream = opts.StreamCallback != nil
		genOpts.StreamCallback = opts.StreamCallback

		// Apply strategy-specific response format settings
		switch strategies[strategyIndex] {
		case StrategyJSONSchema:
			genOpts.ResponseFormat = "json"
			if lm.IsOpenAI() {
				genOpts.ResponseSchema = sig.SignatureToOpenAIJSONSchema()
			} else {
				genOpts.ResponseSchema = sig.SignatureToJSONSchema()
			}
		case StrategyJSONObject:
			genOpts.ResponseFormat = "json"
			genOpts.ResponseSchema = nil
		case StrategyPlainJSON:
			genOpts.ResponseFormat = ""
			genOpts.ResponseSchema = nil
		}

		previousOutput := lastOutput

		// Generate
		genResult, err := lm.Generate(ctx, messages, genOpts)
		if err != nil {
			return nil, fmt.Errorf("LM generation failed: %w", err)
		}

		lastOutput = genResult.Content
		result.Content = genResult.Content
		result.Usage = addUsage(result.Usage, genResult.Usage)

		// Parse output
		parsedOutputs, parseErr := opts.Adapter.Parse(sig, lastOutput)
		if parseErr != nil {
			lastDiagnostics = nil
			// Try next strategy if parsing failed
			if strategyIndex < len(strategies)-1 {
				strategyIndex++
				result.Meta.StrategyFallback = true
				continue
			}
			// Last strategy failed - store best effort and continue to lenient completion
			break
		}

		// Normalize keys
		parsedOutputs = NormalizeOutputKeys(sig, parsedOutputs)

		// Extract adapter metadata
		adapterUsed, parseAttempts, fallbackUsed := ExtractAdapterMetadata(parsedOutputs)
		result.AdapterUsed = adapterUsed
		result.ParseAttempts = parseAttempts
		result.FallbackUsed = fallbackUsed

		lastParsedOutputs = parsedOutputs

		// Strict validation
		diag := sig.ValidateOutputsPartial(parsedOutputs)

		// Check for convergence
		if !diag.HasErrors() {
			result.Outputs = parsedOutputs
			result.Converged = true
			result.Meta.Converged = true
			injectStructuredMeta(result)
			return result, nil
		}

		// Check for repeated failures (early stop)
		if lastDiagnostics != nil && isSameValidationError(lastDiagnostics, diag) {
			// Same error twice - stop retrying
			result.Outputs = parsedOutputs
			result.Diagnostics = diag
			result.Meta.Converged = false
			injectStructuredMeta(result)
			return result, nil
		}

		// Check if output unchanged
		if attempt > 0 && lastOutput == previousOutput {
			// Output unchanged - stop retrying
			result.Outputs = parsedOutputs
			result.Diagnostics = diag
			result.Meta.Converged = false
			injectStructuredMeta(result)
			return result, nil
		}

		lastDiagnostics = diag
		result.Meta.LastError = BuildValidationDiagnosticsString(diag)
	}

	// All attempts exhausted
	// If no parse ever succeeded, return an error (not a partial result)
	if lastParsedOutputs == nil {
		return nil, fmt.Errorf("structured output parsing failed: no valid JSON produced after %d attempts", result.Meta.Attempts)
	}

	// Return best-effort output with diagnostics
	result.Outputs = lastParsedOutputs
	result.Diagnostics = lastDiagnostics
	result.Meta.Converged = false

	// Apply lenient validation if no diagnostics yet
	if result.Diagnostics == nil {
		result.Diagnostics = sig.ValidateOutputsPartial(result.Outputs)
	}

	injectStructuredMeta(result)
	return result, nil
}

// injectStructuredMeta adds structured output metadata to the result outputs.
// This is always called before returning, providing visibility into the
// structured output enforcement process for both successful and partial results.
// Uses [StructuredMetaKey] as the field name.
func injectStructuredMeta(result *GenerateStructuredResult) {
	if result.Outputs == nil {
		result.Outputs = make(map[string]any)
	}
	result.Outputs[StructuredMetaKey] = map[string]any{
		"attempts":          result.Meta.Attempts,
		"strategy":          result.Meta.LastStrategy.String(),
		"converged":         result.Meta.Converged,
		"strategy_fallback": result.Meta.StrategyFallback,
		"last_error":        result.Meta.LastError,
	}
}

// isSameValidationError checks if two validation diagnostics report the same errors
func isSameValidationError(d1, d2 *ValidationDiagnostics) bool {
	if d1 == nil || d2 == nil {
		return false
	}

	// Compare missing fields (order-insensitive)
	if len(d1.MissingFields) != len(d2.MissingFields) {
		return false
	}
	m1 := append([]string(nil), d1.MissingFields...)
	m2 := append([]string(nil), d2.MissingFields...)
	sort.Strings(m1)
	sort.Strings(m2)
	for i := range m1 {
		if m1[i] != m2[i] {
			return false
		}
	}

	// Compare type errors
	if len(d1.TypeErrors) != len(d2.TypeErrors) {
		return false
	}
	for k, v := range d1.TypeErrors {
		if v2, ok := d2.TypeErrors[k]; !ok || v.Error() != v2.Error() {
			return false
		}
	}

	// Compare class errors
	if len(d1.ClassErrors) != len(d2.ClassErrors) {
		return false
	}
	for k, v := range d1.ClassErrors {
		if v2, ok := d2.ClassErrors[k]; !ok || v.Error() != v2.Error() {
			return false
		}
	}

	return true
}

// addUsage accumulates usage across multiple LM calls.
func addUsage(a, b Usage) Usage {
	a.PromptTokens += b.PromptTokens
	a.CompletionTokens += b.CompletionTokens
	a.TotalTokens += b.TotalTokens
	a.Cost += b.Cost
	a.Latency += b.Latency
	return a
}
