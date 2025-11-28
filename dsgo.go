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

// Re-export typed non-generic utility functions
// Note: Generic constructor functions (NewPredict, NewCoT, NewReAct, NewPredictWithDescription)
// cannot be assigned to variables directly. Users should use:
//
//	dsgo.NewTypedPredict[Input, Output](lm)
//	dsgo.NewTypedCoT[Input, Output](lm)
//	dsgo.NewTypedReAct[Input, Output](lm)
//	dsgo.NewTypedPredictWithDesc[Input, Output](lm, description)
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
