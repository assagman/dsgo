package dsgo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ChainOfThought module encourages step-by-step reasoning
type ChainOfThought struct {
	Signature *Signature
	LM        LM
	Options   *GenerateOptions
}

// NewChainOfThought creates a new ChainOfThought module
func NewChainOfThought(signature *Signature, lm LM) *ChainOfThought {
	return &ChainOfThought{
		Signature: signature,
		LM:        lm,
		Options:   DefaultGenerateOptions(),
	}
}

// WithOptions sets custom generation options
func (cot *ChainOfThought) WithOptions(options *GenerateOptions) *ChainOfThought {
	cot.Options = options
	return cot
}

// GetSignature returns the module's signature
func (cot *ChainOfThought) GetSignature() *Signature {
	return cot.Signature
}

// Forward executes the chain of thought reasoning
func (cot *ChainOfThought) Forward(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	if err := cot.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	prompt, err := cot.buildChainOfThoughtPrompt(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []Message{
		{Role: "user", Content: prompt},
	}

	options := cot.Options
	if cot.LM.SupportsJSON() {
		options.ResponseFormat = "json"
	}

	result, err := cot.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	outputs, err := cot.parseOutput(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := cot.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	return outputs, nil
}

func (cot *ChainOfThought) buildChainOfThoughtPrompt(inputs map[string]interface{}) (string, error) {
	var prompt strings.Builder

	// Add description with CoT instruction
	if cot.Signature.Description != "" {
		prompt.WriteString(cot.Signature.Description)
		prompt.WriteString("\n\n")
	}
	
	prompt.WriteString("Think through this step-by-step before providing your final answer.\n\n")

	// Add input fields
	if len(cot.Signature.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range cot.Signature.InputFields {
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

	// Add output format with reasoning field
	if len(cot.Signature.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing:\n")
		prompt.WriteString("- reasoning (string): Your step-by-step thought process\n")
		for _, field := range cot.Signature.OutputFields {
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == FieldTypeClass && len(field.Classes) > 0 {
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

func (cot *ChainOfThought) parseOutput(content string) (map[string]interface{}, error) {
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, content)
	}

	// Always include reasoning in outputs
	outputs := make(map[string]interface{})
	for k, v := range result {
		outputs[k] = v
	}

	return outputs, nil
}
