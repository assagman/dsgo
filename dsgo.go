// Package dsgo is the batteries-included distribution with all standard providers.
// It imports dsgo/internal/* and automatically registers all built-in providers (OpenAI, OpenRouter).
//
// For minimal dependencies, use github.com/assagman/dsgo/internal/core directly.
package dsgo

import (
	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/env"
	"github.com/assagman/dsgo/internal/logging"
	"github.com/assagman/dsgo/internal/module"
	"github.com/assagman/dsgo/internal/typed"

	// Import all standard providers to trigger their init() registration
	_ "github.com/assagman/dsgo/internal/providers/openai"
	_ "github.com/assagman/dsgo/internal/providers/openrouter"
)

func init() {
	// Automatically load .env files when dsgo package is imported
	// This provides zero-configuration environment variable loading
	_ = env.AutoLoad()
}

// Re-export all core types
type (
	LM                    = core.LM
	Message               = core.Message
	GenerateOptions       = core.GenerateOptions
	GenerateResult        = core.GenerateResult
	Field                 = core.Field
	Signature             = core.Signature
	Prediction            = core.Prediction
	History               = core.History
	HistoryEntry          = core.HistoryEntry
	Example               = core.Example
	Tool                  = core.Tool
	ToolCall              = core.ToolCall
	Settings              = core.Settings
	Option                = core.Option
	Collector             = core.Collector
	Cache                 = core.Cache
	ValidationDiagnostics = core.ValidationDiagnostics
	Module                = core.Module
	Adapter               = core.Adapter
	Chunk                 = core.Chunk
	Usage                 = core.Usage
	LMFactory             = core.LMFactory
	FieldType             = core.FieldType
	ExampleSet            = core.ExampleSet
	JSONAdapter           = core.JSONAdapter
	ChatAdapter           = core.ChatAdapter
	MemoryCollector       = core.MemoryCollector
	JSONLCollector        = core.JSONLCollector
	CompositeCollector    = core.CompositeCollector
	LMCache               = core.LMCache
	RequestMeta           = core.RequestMeta
	ResponseMeta          = core.ResponseMeta
	ErrorMeta             = core.ErrorMeta
	CacheMeta             = core.CacheMeta
	CacheStats            = core.CacheStats
	ToolParameter         = core.ToolParameter
)

// Re-export module types
type (
	Predict          = module.Predict
	ChainOfThought   = module.ChainOfThought
	ReAct            = module.ReAct
	Refine           = module.Refine
	BestOfN          = module.BestOfN
	BestOfNResult    = module.BestOfNResult
	Program          = module.Program
	ProgramOfThought = module.ProgramOfThought
	Parallel         = module.Parallel
	ParallelMetrics  = module.ParallelMetrics
	ScoringFunction  = module.ScoringFunction
	StreamResult     = module.StreamResult
)

// Re-export logging types
type (
	Logger        = logging.Logger
	DefaultLogger = logging.DefaultLogger
	NoOpLogger    = logging.NoOpLogger
	Level         = logging.Level
)

// Re-export typed generic type
type Func[I, O any] = typed.Func[I, O]

// Re-export typed types
type FieldInfo = typed.FieldInfo

// Re-export all core functions
var (
	NewLM                       = core.NewLM
	NewSignature                = core.NewSignature
	NewPrediction               = core.NewPrediction
	NewHistory                  = core.NewHistory
	NewHistoryWithLimit         = core.NewHistoryWithLimit
	NewExample                  = core.NewExample
	NewTool                     = core.NewTool
	Configure                   = core.Configure
	GetSettings                 = core.GetSettings
	ResetConfig                 = core.ResetConfig
	WithProvider                = core.WithProvider
	WithModel                   = core.WithModel
	WithTimeout                 = core.WithTimeout
	WithLM                      = core.WithLM
	WithAPIKey                  = core.WithAPIKey
	WithMaxRetries              = core.WithMaxRetries
	WithTracing                 = core.WithTracing
	WithCollector               = core.WithCollector
	WithCache                   = core.WithCache
	WithCacheTTL                = core.WithCacheTTL
	GenerateCacheKey            = core.GenerateCacheKey
	NewFallbackAdapter          = core.NewFallbackAdapter
	NewFallbackAdapterWithChain = core.NewFallbackAdapterWithChain
	NewJSONAdapter              = core.NewJSONAdapter
	NewChatAdapter              = core.NewChatAdapter
	NewTwoStepAdapter           = core.NewTwoStepAdapter
	RegisterLM                  = core.RegisterLM
	NewExampleSet               = core.NewExampleSet
	NewMemoryCollector          = core.NewMemoryCollector
	NewJSONLCollector           = core.NewJSONLCollector
	NewCompositeCollector       = core.NewCompositeCollector
	NewLMCache                  = core.NewLMCache
	DefaultGenerateOptions      = core.DefaultGenerateOptions
	StripMarkers                = core.StripMarkers
	NewLMCacheWithTTL           = core.NewLMCacheWithTTL
)

// Re-export module functions
var (
	NewPredict               = module.NewPredict
	NewChainOfThought        = module.NewChainOfThought
	NewReAct                 = module.NewReAct
	NewRefine                = module.NewRefine
	NewBestOfN               = module.NewBestOfN
	NewProgram               = module.NewProgram
	NewProgramOfThought      = module.NewProgramOfThought
	NewParallel              = module.NewParallel
	NewParallelWithFactory   = module.NewParallelWithFactory
	NewParallelWithInstances = module.NewParallelWithInstances
	DefaultScorer            = module.DefaultScorer
	ConfidenceScorer         = module.ConfidenceScorer
)

// Re-export logging functions
var (
	NewDefaultLogger  = logging.NewDefaultLogger
	SetLogger         = logging.SetLogger
	GetLogger         = logging.GetLogger
	WithRequestID     = logging.WithRequestID
	GetRequestID      = logging.GetRequestID
	EnsureRequestID   = logging.EnsureRequestID
	GenerateRequestID = logging.GenerateRequestID
)

// Typed constructor functions for type-safe modules
// These functions provide compile-time type safety and better IDE support
// compared to the traditional map-based approach

// NewTypedPredict creates a new typed function module using Predict with type-safe I/O
// The I and O types must be structs with dsgo struct tags defining input/output fields
// Example:
//
//	type Input struct { Query string `dsgo:"input,desc=Search query"` }
//	type Output struct { Result string `dsgo:"output,desc=Search result"` }
//
//	predictor, err := dsgo.NewTypedPredict[Input, Output](lm)
func NewTypedPredict[I, O any](lm LM) (*Func[I, O], error) {
	return typed.NewPredict[I, O](lm)
}

// NewTypedCoT creates a new typed function module using ChainOfThought with type-safe I/O
// ChainOfThought enables reasoning capabilities by having the model think step-by-step
// The I and O types must be structs with dsgo struct tags defining input/output fields
// Example:
//
//	type Input struct { Problem string `dsgo:"input,desc=Problem to solve"` }
//	type Output struct { Solution string `dsgo:"output,desc=Step-by-step solution"` }
//
//	cot, err := dsgo.NewTypedCoT[Input, Output](lm)
func NewTypedCoT[I, O any](lm LM) (*Func[I, O], error) {
	return typed.NewCoT[I, O](lm)
}

// NewTypedReAct creates a new typed function module using ReAct with type-safe I/O
// ReAct combines reasoning and acting, allowing the model to use tools to solve problems
// The I and O types must be structs with dsgo struct tags defining input/output fields
// Tools enable the model to interact with external systems (filesystem, APIs, etc.)
// Example:
//
//	type Input struct { Task string `dsgo:"input,desc=Task requiring tool usage"` }
//	type Output struct { Result string `dsgo:"output,desc=Result after tool usage"` }
//
//	react, err := dsgo.NewTypedReAct[Input, Output](lm, tools)
func NewTypedReAct[I, O any](lm LM, tools []Tool) (*Func[I, O], error) {
	return typed.NewReAct[I, O](lm, tools)
}

// NewTypedProgramOfThought creates a new typed function module using ProgramOfThought with type-safe I/O
// ProgramOfThought generates and optionally executes code to solve problems
// The I and O types must be structs with dsgo struct tags defining input/output fields
// Language specifies the target programming language ("python", "javascript", "go", etc.)
// Example:
//
//	type Input struct { Problem string `dsgo:"input,desc=Problem to solve with code"` }
//	type Output struct { Answer string `dsgo:"output,desc=Answer computed by code"` }
//
//	pot, err := dsgo.NewTypedProgramOfThought[Input, Output](lm, "python")
func NewTypedProgramOfThought[I, O any](lm LM, language string) (*Func[I, O], error) {
	return typed.NewProgramOfThought[I, O](lm, language)
}

// NewTypedPredictWithDescription creates a typed Predict function with a custom description
// This is useful when you want to provide more context about what the module does
// The I and O types must be structs with dsgo struct tags defining input/output fields
// Example:
//
//	type Input struct { Text string `dsgo:"input,desc=Text to analyze"` }
//	type Output struct { Sentiment string `dsgo:"output,desc=Detected sentiment"` }
//
//	predictor, err := dsgo.NewTypedPredictWithDescription[Input, Output](lm, "Analyze text sentiment")
func NewTypedPredictWithDescription[I, O any](lm LM, description string) (*Func[I, O], error) {
	return typed.NewPredictWithDescription[I, O](lm, description)
}

// Re-export typed non-generic utility functions
// These functions provide utilities for working with typed structs and signatures
var (
	StructToSignature = typed.StructToSignature
	StructToMap       = typed.StructToMap
	MapToStruct       = typed.MapToStruct
	ParseStructTags   = typed.ParseStructTags
)

// Re-export constants
const (
	FieldTypeString   = core.FieldTypeString
	FieldTypeInt      = core.FieldTypeInt
	FieldTypeFloat    = core.FieldTypeFloat
	FieldTypeBool     = core.FieldTypeBool
	FieldTypeClass    = core.FieldTypeClass
	FieldTypeJSON     = core.FieldTypeJSON
	FieldTypeImage    = core.FieldTypeImage
	FieldTypeDatetime = core.FieldTypeDatetime

	// Logging level constants
	LevelDebug = logging.LevelDebug
	LevelInfo  = logging.LevelInfo
	LevelWarn  = logging.LevelWarn
	LevelError = logging.LevelError
)
