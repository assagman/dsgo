package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-friendly duration type.
//
// It accepts:
// - Go duration strings (e.g., "30s", "5m", "2h")
// - Integers interpreted as seconds
//
// Zero values mean "not set".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar value")
	}
	if value.Value == "" {
		d.Duration = 0
		return nil
	}

	// Allow integer values as seconds for convenience.
	if value.Tag == "!!int" {
		sec, err := strconv.ParseInt(value.Value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid duration seconds %q: %w", value.Value, err)
		}
		if sec < 0 {
			return fmt.Errorf("duration must be >= 0")
		}
		d.Duration = time.Duration(sec) * time.Second
		return nil
	}

	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	if dur < 0 {
		return fmt.Errorf("duration must be >= 0")
	}
	d.Duration = dur
	return nil
}

// TimeoutSettings configures timeouts for the YAML runner.
type TimeoutSettings struct {
	// Pipeline is the overall runtime timeout for the whole pipeline.
	Pipeline Duration `yaml:"pipeline,omitempty"`
	// LMHTTP controls provider HTTP client timeouts (openai/openrouter).
	LMHTTP Duration `yaml:"lm_http,omitempty"`
	// MCPHTTP controls MCP HTTP transport request timeouts.
	MCPHTTP Duration `yaml:"mcp_http,omitempty"`
	// MCPSSEPost controls the POST-side timeout for MCP SSE transports.
	MCPSSEPost Duration `yaml:"mcp_sse_post,omitempty"`
	// MCPSSEWait controls how long to wait for an SSE response.
	MCPSSEWait Duration `yaml:"mcp_sse_wait,omitempty"`
}

// ModelSettings are model generation defaults applied to modules unless overridden.
type ModelSettings struct {
	// Name is the default model identifier in "provider/model" form.
	Name        string  `yaml:"name,omitempty"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// DSGoSettings are runtime settings for the DSGo pipeline runner.
type DSGoSettings struct {
	Timeouts TimeoutSettings `yaml:"timeouts,omitempty"`
}

// PipelineConfig represents the top-level YAML structure.
//
// Backward compatibility:
// - `settings:` is the legacy key for model defaults, and may also include `timeouts:`.
// - Prefer `model:` for model defaults and `dsgo:` for runtime settings.
type PipelineConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Legacy.
	Settings GlobalSettings `yaml:"settings,omitempty"`

	// Preferred.
	Model ModelSettings `yaml:"model,omitempty"`
	DSGo  DSGoSettings  `yaml:"dsgo,omitempty"`

	MCP        map[string]MCPSpec       `yaml:"mcp,omitempty"`
	Tools      map[string]ToolSpec      `yaml:"tools,omitempty"`
	Signatures map[string]SignatureSpec `yaml:"signatures"`
	Modules    map[string]ModuleSpec    `yaml:"modules"`
	Pipeline   []PipelineStep           `yaml:"pipeline"`
	Inputs     map[string]any           `yaml:"inputs,omitempty"`
}

// GlobalSettings represents legacy global pipeline settings.
// Prefer `model:` and `dsgo:` instead.
type GlobalSettings struct {
	Temperature float64         `yaml:"temperature"`
	MaxTokens   int             `yaml:"max_tokens"`
	Timeouts    TimeoutSettings `yaml:"timeouts,omitempty"`
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
	Model     string        `yaml:"model,omitempty"`
	Signature string        `yaml:"signature"`
	Options   ModuleOptions `yaml:"options"`
}

// ModuleOptions represents module-specific options
type ModuleOptions struct {
	Temperature   float64  `yaml:"temperature"`
	MaxTokens     int      `yaml:"max_tokens"`
	MaxIterations int      `yaml:"max_iterations,omitempty"`
	Verbose       bool     `yaml:"verbose,omitempty"`
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

func (c *PipelineConfig) EffectiveModelSettings() ModelSettings {
	settings := ModelSettings{
		Temperature: c.Settings.Temperature,
		MaxTokens:   c.Settings.MaxTokens,
	}
	if c.Model.Name != "" {
		settings.Name = c.Model.Name
	}
	if c.Model.Temperature > 0 {
		settings.Temperature = c.Model.Temperature
	}
	if c.Model.MaxTokens > 0 {
		settings.MaxTokens = c.Model.MaxTokens
	}
	return settings
}

func (c *PipelineConfig) EffectiveTimeouts() TimeoutSettings {
	settings := c.Settings.Timeouts

	if c.DSGo.Timeouts.Pipeline.Duration > 0 {
		settings.Pipeline = c.DSGo.Timeouts.Pipeline
	}
	if c.DSGo.Timeouts.LMHTTP.Duration > 0 {
		settings.LMHTTP = c.DSGo.Timeouts.LMHTTP
	}
	if c.DSGo.Timeouts.MCPHTTP.Duration > 0 {
		settings.MCPHTTP = c.DSGo.Timeouts.MCPHTTP
	}
	if c.DSGo.Timeouts.MCPSSEPost.Duration > 0 {
		settings.MCPSSEPost = c.DSGo.Timeouts.MCPSSEPost
	}
	if c.DSGo.Timeouts.MCPSSEWait.Duration > 0 {
		settings.MCPSSEWait = c.DSGo.Timeouts.MCPSSEWait
	}

	return settings
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
		"shell":  true,
	}

	if !validTypes[spec.Type] {
		return fmt.Errorf("MCP '%s' has invalid type: %s (valid: exa, jina, tavily, custom, shell)", name, spec.Type)
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
