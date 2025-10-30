package dsgo

import (
	"context"
	"fmt"
	"strings"
)

// ToolParameter represents a parameter for a tool
type ToolParameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
}

// Tool represents a tool/function that can be called by the LM
type Tool struct {
	Name        string
	Description string
	Parameters  []ToolParameter
	Function    ToolFunction
}

// ToolFunction is the actual function implementation
type ToolFunction func(ctx context.Context, args map[string]any) (any, error)

// NewTool creates a new tool
func NewTool(name, description string, fn ToolFunction) *Tool {
	return &Tool{
		Name:        name,
		Description: description,
		Parameters:  []ToolParameter{},
		Function:    fn,
	}
}

// AddParameter adds a parameter to the tool
func (t *Tool) AddParameter(name, paramType, description string, required bool) *Tool {
	t.Parameters = append(t.Parameters, ToolParameter{
		Name:        name,
		Type:        paramType,
		Description: description,
		Required:    required,
	})
	return t
}

// AddEnumParameter adds an enum parameter to the tool
func (t *Tool) AddEnumParameter(name, description string, enum []string, required bool) *Tool {
	t.Parameters = append(t.Parameters, ToolParameter{
		Name:        name,
		Type:        "string",
		Description: description,
		Required:    required,
		Enum:        enum,
	})
	return t
}

// Validate validates the arguments against the tool's parameters
func (t *Tool) Validate(args map[string]any) error {
	// Check required parameters are present
	for _, param := range t.Parameters {
		if param.Required {
			if _, exists := args[param.Name]; !exists {
				return fmt.Errorf("missing required parameter: %s", param.Name)
			}
		}
	}

	// Check enum values if specified
	for _, param := range t.Parameters {
		if len(param.Enum) > 0 {
			if val, exists := args[param.Name]; exists {
				valStr := fmt.Sprintf("%v", val)
				valid := false
				for _, enumVal := range param.Enum {
					if valStr == enumVal {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("parameter %s has invalid value: %v (must be one of %v)", param.Name, valStr, param.Enum)
				}
			}
		}
	}

	return nil
}

// Execute executes the tool with given arguments
func (t *Tool) Execute(ctx context.Context, args map[string]any) (any, error) {
	// Normalize arguments (convert arrays to strings for string parameters)
	normalizedArgs := t.normalizeArguments(args)

	// Validate arguments before execution
	if err := t.Validate(normalizedArgs); err != nil {
		return nil, fmt.Errorf("argument validation failed: %w", err)
	}

	return t.Function(ctx, normalizedArgs)
}

// normalizeArguments converts array arguments to strings when parameter type is string
func (t *Tool) normalizeArguments(args map[string]any) map[string]any {
	normalized := make(map[string]any)

	for key, value := range args {
		// Find the parameter definition
		var paramType string
		for _, param := range t.Parameters {
			if param.Name == key {
				paramType = param.Type
				break
			}
		}

		// If parameter is string type and value is array, convert to comma-separated string
		if paramType == "string" {
			switch v := value.(type) {
			case []interface{}:
				parts := make([]string, len(v))
				for i, val := range v {
					parts[i] = fmt.Sprintf("%v", val)
				}
				normalized[key] = strings.Join(parts, ",")
			case []string:
				normalized[key] = strings.Join(v, ",")
			case []int:
				parts := make([]string, len(v))
				for i, val := range v {
					parts[i] = fmt.Sprintf("%d", val)
				}
				normalized[key] = strings.Join(parts, ",")
			case []float64:
				parts := make([]string, len(v))
				for i, val := range v {
					parts[i] = fmt.Sprintf("%v", val)
				}
				normalized[key] = strings.Join(parts, ",")
			default:
				normalized[key] = value
			}
		} else {
			normalized[key] = value
		}
	}

	return normalized
}
