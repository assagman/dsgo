package module

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/assagman/dsgo"
)

// Predict is the basic prediction module
type Predict struct {
	Signature *dsgo.Signature
	LM        dsgo.LM
	Options   *dsgo.GenerateOptions
}

// NewPredict creates a new Predict module
func NewPredict(signature *dsgo.Signature, lm dsgo.LM) *Predict {
	return &Predict{
		Signature: signature,
		LM:        lm,
		Options:   dsgo.DefaultGenerateOptions(),
	}
}

// WithOptions sets custom generation options
func (p *Predict) WithOptions(options *dsgo.GenerateOptions) *Predict {
	p.Options = options
	return p
}

// GetSignature returns the module's signature
func (p *Predict) GetSignature() *dsgo.Signature {
	return p.Signature
}

// Forward executes the prediction
func (p *Predict) Forward(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if err := p.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	prompt, err := p.Signature.BuildPrompt(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []dsgo.Message{
		{Role: "user", Content: prompt},
	}

	options := p.Options
	if p.LM.SupportsJSON() {
		options.ResponseFormat = "json"
	}

	result, err := p.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	outputs, err := p.parseOutput(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := p.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	return outputs, nil
}

func (p *Predict) parseOutput(content string) (map[string]any, error) {
	content = strings.TrimSpace(content)

	// Try to extract JSON from markdown code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else if strings.Contains(content, "```") {
		// Skip non-JSON code blocks (like python, javascript, etc.)
		start := strings.Index(content, "```")
		// Check if this is a code block with a language identifier
		lineEnd := strings.Index(content[start:], "\n")
		if lineEnd > 0 {
			lang := strings.TrimSpace(content[start+3 : start+lineEnd])
			// If it's not JSON and it looks like a programming language, skip this block
			if lang != "" && lang != "json" && !strings.Contains(lang, "{") {
				// Try to find JSON after the code block
				end := strings.Index(content[start+3:], "```")
				if end > 0 {
					afterBlock := content[start+3+end+3:]
					if idx := strings.Index(afterBlock, "{"); idx >= 0 {
						content = afterBlock
					}
				}
			} else {
				// Extract from generic code block
				start := strings.Index(content, "```") + 3
				end := strings.Index(content[start:], "```")
				if end > 0 {
					content = content[start : start+end]
				}
			}
		}
	}

	// Try to find JSON object in the content
	if !strings.HasPrefix(strings.TrimSpace(content), "{") {
		start := strings.Index(content, "{")
		if start >= 0 {
			// Find matching closing brace, accounting for strings
			depth := 0
			inString := false
			escape := false
			for i := start; i < len(content); i++ {
				if escape {
					escape = false
					continue
				}
				if content[i] == '\\' {
					escape = true
					continue
				}
				if content[i] == '"' {
					inString = !inString
				}
				if !inString {
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
	}

	content = strings.TrimSpace(content)

	var outputs map[string]any
	if err := json.Unmarshal([]byte(content), &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, content)
	}

	// Coerce types to match signature expectations
	outputs = p.coerceTypes(outputs)

	return outputs, nil
}

// coerceTypes attempts to convert output values to expected types
func (p *Predict) coerceTypes(outputs map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range outputs {
		field := p.Signature.GetOutputField(key)
		if field == nil {
			result[key] = value
			continue
		}

		// Coerce arrays to strings if field expects string
		if field.Type == dsgo.FieldTypeString || field.Type == dsgo.FieldTypeClass {
			if arr, ok := value.([]any); ok {
				// Join array elements into a string
				var parts []string
				for _, item := range arr {
					parts = append(parts, fmt.Sprintf("%v", item))
				}
				result[key] = strings.Join(parts, "\n")
				continue
			}
		}

		result[key] = value
	}

	return result
}
