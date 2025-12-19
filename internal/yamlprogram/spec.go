package yamlprogram

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec is the single, current YAML schema for building DSGo programs.
//
// Notes:
//   - YAML decoding is strict (unknown fields are errors).
//   - Scalar option fields use pointers so that explicit zero values (e.g. temperature: 0)
//     are representable and not confused with "unset".
//   - This package is internal-only; execute YAML via dsgo.ExecYaml(ctx, path).
//
// The schema is intentionally declarative: it describes signatures, tools, modules,
// and a top-level pipeline (Program) that runs modules in order.
//
// Example skeleton:
//
//	name: "my-pipeline"
//	defaults:
//	  model: openrouter/z-ai/glm-4.6
//	  gen: {temperature: 0.6, max_tokens: 4096}
//	tool_sources: {...}
//	signatures: {...}
//	modules: {...}
//	pipeline: [step1, step2]
//	inputs: {...}
//
// See SchemaJSON() for a JSON Schema reference.
//
// (The schema generator is provided via SchemaJSON(); we intentionally avoid codegen hooks.)
//
// IMPORTANT: schema keys use snake_case.
//
//go:generate go test ./... -run TestNonExistent -count=0
//nolint:revive // YAML schema types prefer "Spec" names.
type Spec struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	Defaults Defaults `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Runtime  Runtime  `yaml:"runtime,omitempty" json:"runtime,omitempty"`

	// ToolSources define where tools come from (builtin registry or MCP clients).
	ToolSources ToolSources `yaml:"tool_sources,omitempty" json:"tool_sources,omitempty"`

	// Histories defines reusable conversation histories.
	Histories map[string]HistorySpec `yaml:"histories,omitempty" json:"histories,omitempty"`

	Signatures map[string]SignatureSpec `yaml:"signatures" json:"signatures"`
	Modules    map[string]ModuleSpec    `yaml:"modules" json:"modules"`

	// Pipeline is the ordered list of module names to execute.
	Pipeline []string `yaml:"pipeline" json:"pipeline"`

	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

type Defaults struct {
	Model string              `yaml:"model,omitempty" json:"model,omitempty"`
	Gen   GenerateOptionsSpec `yaml:"gen,omitempty" json:"gen,omitempty"`
	// Adapter is the default adapter used by modules when not overridden.
	Adapter AdapterSpec `yaml:"adapter,omitempty" json:"adapter,omitempty"`
}

type Runtime struct {
	// DSGo global settings (applied via core.Configure; dsgo.Configure re-exports it).
	DSGo DSGoRuntime `yaml:"dsgo,omitempty" json:"dsgo,omitempty"`

	// Timeouts for the YAML runner itself.
	Timeouts TimeoutSettings `yaml:"timeouts,omitempty" json:"timeouts,omitempty"`
}

type DSGoRuntime struct {
	TimeoutSeconds      *int  `yaml:"default_timeout_seconds,omitempty" json:"default_timeout_seconds,omitempty"`
	MaxRetries          *int  `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	Tracing             *bool `yaml:"tracing,omitempty" json:"tracing,omitempty"`
	SkipModelValidation *bool `yaml:"skip_model_validation,omitempty" json:"skip_model_validation,omitempty"`

	StructuredOutput StructuredOutputSpec `yaml:"structured_output,omitempty" json:"structured_output,omitempty"`
	Cache            CacheSpec            `yaml:"cache,omitempty" json:"cache,omitempty"`
}

type StructuredOutputSpec struct {
	Enabled     *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxAttempts *int     `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	Temperature *float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
}

type CacheSpec struct {
	// Simple memory-only cache. Set capacity<=0 to disable.
	Capacity *int      `yaml:"capacity,omitempty" json:"capacity,omitempty"`
	TTL      *Duration `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

// TimeoutSettings configure runner-level timeouts.
// These are not the same as provider-level HTTP client timeouts.
//
// NOTE: these are currently used by the example runner to bound the overall pipeline.
// They may also be used in future to configure MCP client timeouts.
type TimeoutSettings struct {
	Pipeline *Duration `yaml:"pipeline,omitempty" json:"pipeline,omitempty"`
}

// Duration is a YAML-friendly duration type.
//
// It accepts:
// - Go duration strings (e.g., "30s", "5m", "2h")
// - Integers interpreted as seconds
//
// When embedded as a pointer (*Duration), nil means "not set".
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
		sec, err := parseInt64(value.Value)
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

// ToolSources groups tool providers by kind.
//
// Example:
//
//	tool_sources:
//	  mcp:
//	    tavily:
//	      api_key_env: TAVILY_API_KEY
//	    filesystem:
//	      allowed_dirs: ["/tmp"]
//	  builtin: [current_datetime, calculate]
type ToolSources struct {
	MCP     map[string]MCPToolSource `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	Builtin []string                 `yaml:"builtin,omitempty" json:"builtin,omitempty"`
}

// MCPToolSource configures an MCP tool provider.
// The map key determines the provider type (exa, jina, tavily, filesystem, shell, custom).
type MCPToolSource struct {
	// APIKey is the API key for the MCP provider.
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	// APIKeyEnv is the environment variable name containing the API key.
	APIKeyEnv string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
	// URL is required for custom MCP providers.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// AllowedDirs is required for filesystem and shell providers.
	AllowedDirs []string `yaml:"allowed_dirs,omitempty" json:"allowed_dirs,omitempty"`
}

// HistorySpec defines a reusable DSGo History instance.
type HistorySpec struct {
	Limit *int `yaml:"limit,omitempty" json:"limit,omitempty"`
}

// SignatureSpec defines a DSGo signature.
// Inputs/Outputs use a concise map form.
type SignatureSpec struct {
	Desc string               `yaml:"desc" json:"desc"`
	In   map[string]FieldSpec `yaml:"in" json:"in"`
	Out  map[string]FieldSpec `yaml:"out" json:"out"`
}

// FieldSpec supports both compact scalar and expanded object forms.
//
// Compact:
//
//	text: string
//
// Expanded:
//
//	sentiment:
//	  type: enum
//	  values: [positive, negative, neutral]
//	  desc: "Sentiment"
//
// Supported types:
// - string, int, float, bool, json, image, datetime, enum, array
//
// enum uses Values, array uses Items (element type).
type FieldSpec struct {
	Type     string   `yaml:"type" json:"type"`
	Desc     string   `yaml:"desc,omitempty" json:"desc,omitempty"`
	Optional bool     `yaml:"optional,omitempty" json:"optional,omitempty"`
	Values   []string `yaml:"values,omitempty" json:"values,omitempty"` // For enum types: allowed values
	Items    string   `yaml:"items,omitempty" json:"items,omitempty"`   // For array types: element type (string, int, float, bool, json)
}

func (f *FieldSpec) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}

	// Shorthand: field: string
	if value.Kind == yaml.ScalarNode {
		f.Type = value.Value
		return nil
	}

	type raw FieldSpec
	var r raw
	if err := value.Decode(&r); err != nil {
		return err
	}
	*f = FieldSpec(r)
	return nil
}

// ModuleSpec defines a DSGo module.
//
// kind is one of:
// - predict
// - chain_of_thought
// - react
// - refine
// - best_of_n
// - program_of_thought
// - program
// - parallel
// - multi_chain_comparison
//
// Each module kind has an optional sub-block for its parameters.
type ModuleSpec struct {
	Kind string `yaml:"kind" json:"kind"`
	Sig  string `yaml:"sig,omitempty" json:"sig,omitempty"`

	Model   string              `yaml:"model,omitempty" json:"model,omitempty"`
	Gen     GenerateOptionsSpec `yaml:"gen,omitempty" json:"gen,omitempty"`
	Adapter AdapterSpec         `yaml:"adapter,omitempty" json:"adapter,omitempty"`
	Verbose *bool               `yaml:"verbose,omitempty" json:"verbose,omitempty"`

	History string        `yaml:"history,omitempty" json:"history,omitempty"`
	Demos   []ExampleSpec `yaml:"demos,omitempty" json:"demos,omitempty"`

	ReAct                ReactSpec                `yaml:"react,omitempty" json:"react,omitempty"`
	Refine               RefineSpec               `yaml:"refine,omitempty" json:"refine,omitempty"`
	BestOfN              BestOfNSpec              `yaml:"best_of_n,omitempty" json:"best_of_n,omitempty"`
	ProgramOfThought     ProgramOfThoughtSpec     `yaml:"program_of_thought,omitempty" json:"program_of_thought,omitempty"`
	Program              ProgramSpec              `yaml:"program,omitempty" json:"program,omitempty"`
	Parallel             ParallelSpec             `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	MultiChainComparison MultiChainComparisonSpec `yaml:"multi_chain_comparison,omitempty" json:"multi_chain_comparison,omitempty"`
}

type ExampleSpec struct {
	Inputs  map[string]any `yaml:"inputs" json:"inputs"`
	Outputs map[string]any `yaml:"outputs" json:"outputs"`
}

// GenerateOptionsSpec mirrors dsgo.GenerateOptions, but uses pointers for scalars.
// This makes explicit zero values representable.
type GenerateOptionsSpec struct {
	Temperature      *float64       `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	MaxTokens        *int           `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	TopP             *float64       `yaml:"top_p,omitempty" json:"top_p,omitempty"`
	Stop             []string       `yaml:"stop,omitempty" json:"stop,omitempty"`
	ResponseFormat   *string        `yaml:"response_format,omitempty" json:"response_format,omitempty" jsonschema:"enum=text,enum=json"`
	ResponseSchema   map[string]any `yaml:"response_schema,omitempty" json:"response_schema,omitempty"`
	ToolChoice       *string        `yaml:"tool_choice,omitempty" json:"tool_choice,omitempty"`
	FrequencyPenalty *float64       `yaml:"frequency_penalty,omitempty" json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64       `yaml:"presence_penalty,omitempty" json:"presence_penalty,omitempty"`
	ProviderParams   map[string]any `yaml:"provider_params,omitempty" json:"provider_params,omitempty"`

	Retry RetrySpec `yaml:"retry,omitempty" json:"retry,omitempty"`
}

type RetrySpec struct {
	MaxRetries     *int      `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	InitialBackoff *Duration `yaml:"initial_backoff,omitempty" json:"initial_backoff,omitempty"`
	MaxBackoff     *Duration `yaml:"max_backoff,omitempty" json:"max_backoff,omitempty"`
	JitterFactor   *float64  `yaml:"jitter_factor,omitempty" json:"jitter_factor,omitempty"`
}

// AdapterSpec describes which adapter to use.
//
// kind:
// - fallback (default)
// - chat
// - json
// - two_step
//
// reasoning toggles inclusion/extraction of rationale fields for adapters that support it.
type AdapterSpec struct {
	Kind      string `yaml:"kind,omitempty" json:"kind,omitempty" jsonschema:"enum=fallback,enum=chat,enum=json,enum=two_step"`
	Reasoning *bool  `yaml:"reasoning,omitempty" json:"reasoning,omitempty"`

	// two_step-specific. If set, this model will be used to extract/structure outputs.
	ExtractionModel string `yaml:"extraction_model,omitempty" json:"extraction_model,omitempty"`
}

// ReactSpec parameters.
type ReactSpec struct {
	MaxIterations *int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`

	// Tool selections for this agent.
	//
	// Example:
	//   tools:
	//     - source: fs
	//       include: ["*"]
	//     - source: builtin
	//       include: [calculate, current_datetime]
	Tools []ToolSelection `yaml:"tools,omitempty" json:"tools,omitempty"`
}

type ToolSelection struct {
	Source  string   `yaml:"source" json:"source"`
	Include []string `yaml:"include" json:"include"`
}

// RefineSpec parameters.
type RefineSpec struct {
	MaxIterations   *int    `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
	RefinementField *string `yaml:"refinement_field,omitempty" json:"refinement_field,omitempty"`
	TrackHistory    *bool   `yaml:"track_history,omitempty" json:"track_history,omitempty"`
}

// BestOfNSpec parameters.
type BestOfNSpec struct {
	// Of is the module name to execute N times.
	Of string `yaml:"of" json:"of"`

	N           int      `yaml:"n" json:"n"`
	Parallel    *bool    `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	ReturnAll   *bool    `yaml:"return_all,omitempty" json:"return_all,omitempty"`
	MaxFailures *int     `yaml:"max_failures,omitempty" json:"max_failures,omitempty"`
	Threshold   *float64 `yaml:"threshold,omitempty" json:"threshold,omitempty"`

	Scorer ScorerSpec `yaml:"scorer" json:"scorer"`
}

type ScorerSpec struct {
	Kind  string `yaml:"kind" json:"kind" jsonschema:"enum=default,enum=confidence"`
	Field string `yaml:"field,omitempty" json:"field,omitempty"`
}

// ProgramOfThoughtSpec parameters.
type ProgramOfThoughtSpec struct {
	Language                string `yaml:"language" json:"language"`
	AllowExecution          *bool  `yaml:"allow_execution,omitempty" json:"allow_execution,omitempty"`
	ExecutionTimeoutSeconds *int   `yaml:"execution_timeout_seconds,omitempty" json:"execution_timeout_seconds,omitempty"`
}

// ProgramSpec defines a nested program (a sequential pipeline).
// Steps reference modules by name.
type ProgramSpec struct {
	Steps []string `yaml:"steps" json:"steps"`
}

// ParallelSpec parameters.
type ParallelSpec struct {
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty" jsonschema:"enum=clone,enum=instances,enum=factory"`

	// clone mode
	Module string `yaml:"module,omitempty" json:"module,omitempty"`

	// instances mode
	Instances []string `yaml:"instances,omitempty" json:"instances,omitempty"`

	// factory mode: list of module names.
	// When the input does not specify a batch, Parallel probes this list and executes
	// one task per factory entry.
	Factory []string `yaml:"factory,omitempty" json:"factory,omitempty"`

	MaxWorkers     *int    `yaml:"max_workers,omitempty" json:"max_workers,omitempty"`
	MaxFailures    *int    `yaml:"max_failures,omitempty" json:"max_failures,omitempty"`
	FailFast       *bool   `yaml:"fail_fast,omitempty" json:"fail_fast,omitempty"`
	ReturnAll      *bool   `yaml:"return_all,omitempty" json:"return_all,omitempty"`
	OnlySuccessful *bool   `yaml:"only_successful,omitempty" json:"only_successful,omitempty"`
	BatchKey       *string `yaml:"batch_key,omitempty" json:"batch_key,omitempty"`
	Repeat         *int    `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	Verbose        *bool   `yaml:"verbose,omitempty" json:"verbose,omitempty"`
}

// MultiChainComparisonSpec parameters.
type MultiChainComparisonSpec struct {
	Attempts        int     `yaml:"attempts" json:"attempts"`
	AttemptTemplate *string `yaml:"attempt_template,omitempty" json:"attempt_template,omitempty"`
}
