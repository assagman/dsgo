// Package dsgo is the batteries-included distribution with all standard providers.
// It imports dsgo/core and automatically registers all built-in providers (OpenAI, OpenRouter).
//
// For minimal dependencies, use github.com/assagman/dsgo/core directly.
package dsgo

import (
	"github.com/assagman/dsgo/core"

	// Import all standard providers to trigger their init() registration
	_ "github.com/assagman/dsgo/providers/openai"
	_ "github.com/assagman/dsgo/providers/openrouter"
)

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
)

// Re-export all functions
var (
	NewLM               = core.NewLM
	NewSignature        = core.NewSignature
	NewPrediction       = core.NewPrediction
	NewHistory          = core.NewHistory
	NewHistoryWithLimit = core.NewHistoryWithLimit
	NewExample          = core.NewExample
	NewTool             = core.NewTool
	Configure           = core.Configure
	GetSettings         = core.GetSettings
	ResetConfig         = core.ResetConfig
	WithProvider        = core.WithProvider
	WithModel           = core.WithModel
	WithTimeout         = core.WithTimeout
	WithLM              = core.WithLM
	WithAPIKey          = core.WithAPIKey
	WithMaxRetries      = core.WithMaxRetries
	WithTracing         = core.WithTracing
	WithCollector       = core.WithCollector
	GenerateCacheKey    = core.GenerateCacheKey
	NewFallbackAdapter  = core.NewFallbackAdapter
	NewJSONAdapter      = core.NewJSONAdapter
	NewChatAdapter      = core.NewChatAdapter
	NewTwoStepAdapter   = core.NewTwoStepAdapter
	RegisterLM          = core.RegisterLM
	NewLMWrapper        = core.NewLMWrapper
)

// Re-export constants
const (
	FieldTypeString = core.FieldTypeString
	FieldTypeInt    = core.FieldTypeInt
	FieldTypeFloat  = core.FieldTypeFloat
	FieldTypeBool   = core.FieldTypeBool
	FieldTypeClass  = core.FieldTypeClass
	FieldTypeJSON   = core.FieldTypeClass // FieldTypeJSON is an alias for FieldTypeClass
)
