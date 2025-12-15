package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PipelineConfig represents the top-level YAML structure
type PipelineConfig struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Model       ModelSpec                `yaml:"model,omitempty"`
	Settings    GlobalSettings           `yaml:"settings"`
	MCP         map[string]MCPSpec       `yaml:"mcp,omitempty"`
	Tools       map[string]ToolSpec      `yaml:"tools,omitempty"`
	Signatures  map[string]SignatureSpec `yaml:"signatures"`
	Modules     map[string]ModuleSpec    `yaml:"modules"`
	Pipeline    []PipelineStep           `yaml:"pipeline"`
	Inputs      map[string]any           `yaml:"inputs,omitempty"`
}

// ModelSpec represents a model configuration (global or module-level)
type ModelSpec struct {
	Name        string  `yaml:"name"`
	Temperature float64 `yaml:"temperature,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
}

// GlobalSettings represents global pipeline settings
type GlobalSettings struct {
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// SignatureSpec represents a signature definition in YAML
type SignatureSpec struct {
	Description string      `yaml:"description"`
	Inputs      []FieldSpec `yaml:"inputs"`
	Outputs     []FieldSpec `yaml:"outputs"`
}

// FieldSpec represents a field definition in YAML
type FieldSpec struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Optional    bool     `yaml:"optional"`
	Classes     []string `yaml:"classes,omitempty"` // For class/enum types
}

// ModuleSpec represents a module definition in YAML
type ModuleSpec struct {
	Type      string        `yaml:"type"`
	Signature string        `yaml:"signature"`
	Model     *ModelSpec    `yaml:"model,omitempty"`
	Options   ModuleOptions `yaml:"options"`
}

// ModuleOptions represents module-specific options
type ModuleOptions struct {
	Temperature   float64  `yaml:"temperature"`
	MaxTokens     int      `yaml:"max_tokens"`
	MaxIterations int      `yaml:"max_iterations,omitempty"`
	Tools         []string `yaml:"tools,omitempty"`
}

// MCPSpec represents an MCP client configuration
type MCPSpec struct {
	Type   string `yaml:"type"`
	APIKey string `yaml:"api_key,omitempty"`
	URL    string `yaml:"url,omitempty"`
}

// ToolSpec represents a tool definition
type ToolSpec struct {
	Type        string          `yaml:"type"`
	Source      string          `yaml:"source,omitempty"`
	Name        string          `yaml:"name,omitempty"`
	Description string          `yaml:"description,omitempty"`
	Parameters  []ToolParamSpec `yaml:"parameters,omitempty"`
}

// ToolParamSpec represents a tool parameter definition
type ToolParamSpec struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// PipelineStep represents a step in the pipeline
type PipelineStep struct {
	Module string `yaml:"module"`
}

// LoadPipelineConfig loads a pipeline configuration from a YAML file
func LoadPipelineConfig(filename string) (*PipelineConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	var config PipelineConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// validateConfig validates the pipeline configuration
func validateConfig(config *PipelineConfig) error {
	if config.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}

	if len(config.Signatures) == 0 {
		return fmt.Errorf("at least one signature must be defined")
	}

	if len(config.Modules) == 0 {
		return fmt.Errorf("at least one module must be defined")
	}

	if len(config.Pipeline) == 0 {
		return fmt.Errorf("pipeline must have at least one step")
	}

	// Validate signatures
	for name, sig := range config.Signatures {
		if err := validateSignature(name, sig); err != nil {
			return err
		}
	}

	// Validate MCP configurations
	for name, mcpSpec := range config.MCP {
		if err := validateMCP(name, mcpSpec); err != nil {
			return err
		}
	}

	// Validate tool definitions
	for name, toolSpec := range config.Tools {
		if err := validateTool(name, toolSpec, config.MCP); err != nil {
			return err
		}
	}

	// Validate modules reference existing signatures
	for name, mod := range config.Modules {
		if err := validateModule(name, mod, config.Signatures, config.Tools); err != nil {
			return err
		}
	}

	// Validate pipeline steps reference existing modules
	for i, step := range config.Pipeline {
		if _, exists := config.Modules[step.Module]; !exists {
			return fmt.Errorf("pipeline step %d references undefined module: %s", i+1, step.Module)
		}
	}

	return nil
}

// validateMCP validates an MCP client configuration
func validateMCP(name string, spec MCPSpec) error {
	validTypes := map[string]bool{
		"exa":    true,
		"jina":   true,
		"tavily": true,
		"custom": true,
	}

	if !validTypes[spec.Type] {
		return fmt.Errorf("MCP '%s' has invalid type: %s (valid: exa, jina, tavily, custom)", name, spec.Type)
	}

	if spec.Type == "custom" && spec.URL == "" {
		return fmt.Errorf("MCP '%s' is custom type but has no URL defined", name)
	}

	return nil
}

// validateTool validates a tool definition
func validateTool(name string, spec ToolSpec, mcpClients map[string]MCPSpec) error {
	validTypes := map[string]bool{
		"mcp":        true,
		"filesystem": true,
		"function":   true,
	}

	if !validTypes[spec.Type] {
		return fmt.Errorf("tool '%s' has invalid type: %s (valid: mcp, filesystem, function)", name, spec.Type)
	}

	if spec.Type == "mcp" {
		if spec.Source == "" {
			return fmt.Errorf("tool '%s' is MCP type but has no source (MCP client reference) defined", name)
		}
		if _, exists := mcpClients[spec.Source]; !exists {
			return fmt.Errorf("tool '%s' references undefined MCP client: %s", name, spec.Source)
		}
	}

	if spec.Type == "function" {
		validFunctions := map[string]bool{
			"current_datetime": true,
			"calculate":        true,
			"random_number":    true,
			"string_length":    true,
			"word_count":       true,
			"environment_info": true,
		}
		funcName := spec.Name
		if funcName == "" {
			funcName = name
		}
		if !validFunctions[funcName] {
			return fmt.Errorf("tool '%s' references unknown function: %s (valid: current_datetime, calculate, random_number, string_length, word_count, environment_info)", name, funcName)
		}
	}

	return nil
}

// validateSignature validates a signature definition
func validateSignature(name string, sig SignatureSpec) error {
	if sig.Description == "" {
		return fmt.Errorf("signature '%s' must have a description", name)
	}

	if len(sig.Inputs) == 0 {
		return fmt.Errorf("signature '%s' must have at least one input", name)
	}

	if len(sig.Outputs) == 0 {
		return fmt.Errorf("signature '%s' must have at least one output", name)
	}

	// Validate field types
	validTypes := map[string]bool{
		"string":   true,
		"int":      true,
		"float":    true,
		"bool":     true,
		"json":     true,
		"class":    true,
		"image":    true,
		"datetime": true,
	}

	for _, field := range sig.Inputs {
		if !validTypes[field.Type] {
			return fmt.Errorf("signature '%s' input '%s' has invalid type: %s", name, field.Name, field.Type)
		}
	}

	for _, field := range sig.Outputs {
		if !validTypes[field.Type] {
			return fmt.Errorf("signature '%s' output '%s' has invalid type: %s", name, field.Name, field.Type)
		}
		if field.Type == "class" && len(field.Classes) == 0 {
			return fmt.Errorf("signature '%s' output '%s' is class type but has no classes defined", name, field.Name)
		}
	}

	return nil
}

// validateModule validates a module definition
func validateModule(name string, mod ModuleSpec, signatures map[string]SignatureSpec, tools map[string]ToolSpec) error {
	validTypes := map[string]bool{
		"Predict":        true,
		"ChainOfThought": true,
		"ReAct":          true,
	}

	if !validTypes[mod.Type] {
		return fmt.Errorf("module '%s' has invalid type: %s (valid: Predict, ChainOfThought, ReAct)", name, mod.Type)
	}

	if _, exists := signatures[mod.Signature]; !exists {
		return fmt.Errorf("module '%s' references undefined signature: %s", name, mod.Signature)
	}

	if mod.Type == "ReAct" && len(mod.Options.Tools) == 0 {
		return fmt.Errorf("module '%s' is ReAct type but has no tools defined", name)
	}

	for _, toolRef := range mod.Options.Tools {
		if _, exists := tools[toolRef]; !exists {
			return fmt.Errorf("module '%s' references undefined tool: %s", name, toolRef)
		}
	}

	return nil
}
