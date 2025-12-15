package main

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo"
)

// LMProvider creates LM instances for modules
type LMProvider func(ctx context.Context, modelName string) (dsgo.LM, error)

// ModuleFactory creates DSGo modules from YAML specifications
type ModuleFactory struct {
	ctx            context.Context
	defaultLM      dsgo.LM
	globalModel    ModelSpec
	lmProvider     LMProvider
	sigRegistry    *SignatureRegistry
	toolRegistry   *ToolRegistry
	globalSettings GlobalSettings
	lmCache        map[string]dsgo.LM
}

// NewModuleFactory creates a new module factory
func NewModuleFactory(ctx context.Context, defaultLM dsgo.LM, globalModel ModelSpec, lmProvider LMProvider, sigRegistry *SignatureRegistry, toolRegistry *ToolRegistry, settings GlobalSettings) *ModuleFactory {
	return &ModuleFactory{
		ctx:            ctx,
		defaultLM:      defaultLM,
		globalModel:    globalModel,
		lmProvider:     lmProvider,
		sigRegistry:    sigRegistry,
		toolRegistry:   toolRegistry,
		globalSettings: settings,
		lmCache:        make(map[string]dsgo.LM),
	}
}

// CreateModule creates a DSGo module from a YAML specification
func (f *ModuleFactory) CreateModule(name string, spec ModuleSpec) (dsgo.Module, error) {
	sig, err := f.sigRegistry.Get(spec.Signature)
	if err != nil {
		return nil, fmt.Errorf("module '%s': %w", name, err)
	}

	// Get LM for this module (may be module-specific or default)
	lm, err := f.getLMForModule(spec)
	if err != nil {
		return nil, fmt.Errorf("module '%s': %w", name, err)
	}

	// Build options with defaults from global settings, overridden by module-specific
	options := f.buildOptions(spec)

	switch spec.Type {
	case "Predict":
		return f.createPredict(sig, lm, options), nil
	case "ChainOfThought":
		return f.createChainOfThought(sig, lm, options), nil
	case "ReAct":
		return f.createReAct(sig, lm, options, spec.Options)
	default:
		return nil, fmt.Errorf("unsupported module type: %s", spec.Type)
	}
}

// getLMForModule returns the appropriate LM for a module
func (f *ModuleFactory) getLMForModule(spec ModuleSpec) (dsgo.LM, error) {
	if spec.Model == nil || spec.Model.Name == "" {
		return f.defaultLM, nil
	}

	modelName := spec.Model.Name
	if cached, exists := f.lmCache[modelName]; exists {
		return cached, nil
	}

	lm, err := f.lmProvider(f.ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create LM for model '%s': %w", modelName, err)
	}

	f.lmCache[modelName] = lm
	return lm, nil
}

// createPredict creates a Predict module
func (f *ModuleFactory) createPredict(sig *dsgo.Signature, lm dsgo.LM, options *dsgo.GenerateOptions) dsgo.Module {
	return dsgo.NewPredict(sig, lm).
		WithOptions(options).
		WithAdapter(dsgo.NewFallbackAdapter())
}

// createChainOfThought creates a ChainOfThought module
func (f *ModuleFactory) createChainOfThought(sig *dsgo.Signature, lm dsgo.LM, options *dsgo.GenerateOptions) dsgo.Module {
	return dsgo.NewChainOfThought(sig, lm).
		WithOptions(options).
		WithAdapter(dsgo.NewFallbackAdapter().WithReasoning(true))
}

// createReAct creates a ReAct module with tools
func (f *ModuleFactory) createReAct(sig *dsgo.Signature, lm dsgo.LM, options *dsgo.GenerateOptions, moduleOpts ModuleOptions) (dsgo.Module, error) {
	tools, err := f.toolRegistry.GetMultiple(moduleOpts.Tools)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tools: %w", err)
	}

	react := dsgo.NewReAct(sig, lm, tools).
		WithOptions(options).
		WithAdapter(dsgo.NewFallbackAdapter())

	if moduleOpts.MaxIterations > 0 {
		react.WithMaxIterations(moduleOpts.MaxIterations)
	}

	return react, nil
}

// buildOptions builds GenerateOptions with proper defaults
// Priority: module options > module model > global settings > global model
func (f *ModuleFactory) buildOptions(spec ModuleSpec) *dsgo.GenerateOptions {
	options := dsgo.DefaultGenerateOptions()

	// 1. Apply global model settings
	if f.globalModel.Temperature > 0 {
		options.Temperature = f.globalModel.Temperature
	}
	if f.globalModel.MaxTokens > 0 {
		options.MaxTokens = f.globalModel.MaxTokens
	}

	// 2. Apply legacy global settings (for backward compatibility)
	if f.globalSettings.Temperature > 0 {
		options.Temperature = f.globalSettings.Temperature
	}
	if f.globalSettings.MaxTokens > 0 {
		options.MaxTokens = f.globalSettings.MaxTokens
	}

	// 3. Apply module-level model settings
	if spec.Model != nil {
		if spec.Model.Temperature > 0 {
			options.Temperature = spec.Model.Temperature
		}
		if spec.Model.MaxTokens > 0 {
			options.MaxTokens = spec.Model.MaxTokens
		}
	}

	// 4. Apply module-specific options (highest priority)
	if spec.Options.Temperature > 0 {
		options.Temperature = spec.Options.Temperature
	}
	if spec.Options.MaxTokens > 0 {
		options.MaxTokens = spec.Options.MaxTokens
	}

	return options
}

// ModuleRegistry holds created modules
type ModuleRegistry struct {
	modules map[string]dsgo.Module
}

// NewModuleRegistry creates all modules from YAML specs
func NewModuleRegistry(factory *ModuleFactory, specs map[string]ModuleSpec) (*ModuleRegistry, error) {
	registry := &ModuleRegistry{
		modules: make(map[string]dsgo.Module),
	}

	for name, spec := range specs {
		module, err := factory.CreateModule(name, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create module '%s': %w", name, err)
		}
		registry.modules[name] = module
	}

	return registry, nil
}

// Get returns a module by name
func (r *ModuleRegistry) Get(name string) (dsgo.Module, error) {
	mod, exists := r.modules[name]
	if !exists {
		return nil, fmt.Errorf("module not found: %s", name)
	}
	return mod, nil
}
