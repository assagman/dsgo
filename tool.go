package dsgo

import "context"

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

// Execute executes the tool with given arguments
func (t *Tool) Execute(ctx context.Context, args map[string]any) (any, error) {
	return t.Function(ctx, args)
}
