package main

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo"
)

// ModuleFactory creates DSGo modules from YAML specifications.
//
// It supports per-module model overrides by creating (and caching) LM instances
// per requested model.
type ModuleFactory struct {
	ctx          context.Context
	defaultModel string
	defaultLM    dsgo.LM
	lmsByModel   map[string]dsgo.LM

	sigRegistry   *SignatureRegistry
	toolRegistry  *ToolRegistry
	modelSettings ModelSettings

	// For Parallel/MCC - need access to all module specs
	allModuleSpecs map[string]ModuleSpec
}

// NewModuleFactory creates a new module factory.
func NewModuleFactory(ctx context.Context, defaultModel string, defaultLM dsgo.LM, sigRegistry *SignatureRegistry, toolRegistry *ToolRegistry, settings ModelSettings) *ModuleFactory {
	f := &ModuleFactory{
		ctx:          ctx,
		defaultModel: defaultModel,
		defaultLM:    defaultLM,
		lmsByModel:   make(map[string]dsgo.LM),
		sigRegistry:  sigRegistry,
		toolRegistry: toolRegistry,

		modelSettings: settings,
	}

	if defaultModel != "" && defaultLM != nil {
		f.lmsByModel[defaultModel] = defaultLM
	}

	return f
}

func (f *ModuleFactory) getLM(model string) (dsgo.LM, error) {
	if model == "" {
		return f.defaultLM, nil
	}
	if f.defaultModel != "" && model == f.defaultModel {
		return f.defaultLM, nil
	}
	if lm, ok := f.lmsByModel[model]; ok {
		return lm, nil
	}

	lm, err := dsgo.NewLM(f.ctx, model)
	if err != nil {
		return nil, fmt.Errorf("failed to create LM for model %q: %w", model, err)
	}
	f.lmsByModel[model] = lm
	return lm, nil
}

// SetAllModuleSpecs sets all module specs for Parallel/MCC module creation
func (f *ModuleFactory) SetAllModuleSpecs(specs map[string]ModuleSpec) {
	f.allModuleSpecs = specs
}

// CreateModule creates a DSGo module from a YAML specification
func (f *ModuleFactory) CreateModule(name string, spec ModuleSpec) (dsgo.Module, error) {
	// Handle Parallel module separately (no LM or signature needed)
	if spec.Type == "Parallel" {
		return f.createParallel(name, spec)
	}

	// Handle MultiChainComparison module
	if spec.Type == "MultiChainComparison" {
		return f.createMultiChainComparison(name, spec)
	}

	lm, err := f.getLM(spec.Model)
	if err != nil {
		return nil, fmt.Errorf("module '%s': %w", name, err)
	}

	sig, err := f.sigRegistry.Get(spec.Signature)
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
		return f.createReAct(sig, lm, options, spec)
	default:
		return nil, fmt.Errorf("unsupported module type: %s", spec.Type)
	}
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
func (f *ModuleFactory) createReAct(sig *dsgo.Signature, lm dsgo.LM, options *dsgo.GenerateOptions, spec ModuleSpec) (dsgo.Module, error) {
	var allTools []dsgo.Tool

	// Get custom tools from options.tools
	if len(spec.Options.Tools) > 0 {
		customTools, err := f.toolRegistry.GetMultiple(spec.Options.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve custom tools: %w", err)
		}
		allTools = append(allTools, customTools...)
	}

	// Get MCP tools from per-module mcp configuration
	if len(spec.MCP) > 0 {
		mcpTools, err := f.toolRegistry.GetAllMCPToolsForModule(spec.MCP)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve MCP tools: %w", err)
		}
		allTools = append(allTools, mcpTools...)
	}

	react := dsgo.NewReAct(sig, lm, allTools).
		WithOptions(options).
		WithAdapter(dsgo.NewFallbackAdapter())

	if spec.Options.MaxIterations > 0 {
		react.WithMaxIterations(spec.Options.MaxIterations)
	}

	react.WithVerbose(spec.Options.Verbose)

	return react, nil
}

// buildOptions builds GenerateOptions with proper defaults
func (f *ModuleFactory) buildOptions(spec ModuleSpec) *dsgo.GenerateOptions {
	options := dsgo.DefaultGenerateOptions()

	// Apply model defaults first
	if f.modelSettings.Temperature > 0 {
		options.Temperature = f.modelSettings.Temperature
	}
	if f.modelSettings.MaxTokens > 0 {
		options.MaxTokens = f.modelSettings.MaxTokens
	}

	// Apply module-specific options (highest priority)
	if spec.Options.Temperature > 0 {
		options.Temperature = spec.Options.Temperature
	}
	if spec.Options.MaxTokens > 0 {
		options.MaxTokens = spec.Options.MaxTokens
	}

	return options
}

// createParallel creates a Parallel module that runs multiple modules concurrently
func (f *ModuleFactory) createParallel(name string, spec ModuleSpec) (dsgo.Module, error) {
	if len(spec.Modules) == 0 {
		return nil, fmt.Errorf("module '%s': Parallel requires modules list", name)
	}

	// Create all module instances upfront for fail-fast behavior
	instances := make([]dsgo.Module, len(spec.Modules))
	for i, modName := range spec.Modules {
		modSpec, exists := f.allModuleSpecs[modName]
		if !exists {
			return nil, fmt.Errorf("module '%s': referenced module '%s' not found", name, modName)
		}
		mod, err := f.CreateModule(modName, modSpec)
		if err != nil {
			return nil, fmt.Errorf("module '%s': failed to create '%s': %w", name, modName, err)
		}
		instances[i] = mod
	}

	parallel := dsgo.NewParallelWithInstances(instances).
		WithReturnAll(true).
		WithOnlySuccessful(true).
		WithVerbose(spec.Options.Verbose)

	if spec.MaxWorkers > 0 {
		parallel.WithMaxWorkers(spec.MaxWorkers)
	} else {
		parallel.WithMaxWorkers(len(spec.Modules))
	}

	return parallel, nil
}

// createMultiChainComparison creates a MultiChainComparison module
func (f *ModuleFactory) createMultiChainComparison(name string, spec ModuleSpec) (dsgo.Module, error) {
	lm, err := f.getLM(spec.Model)
	if err != nil {
		return nil, fmt.Errorf("module '%s': %w", name, err)
	}

	sig, err := f.sigRegistry.Get(spec.Signature)
	if err != nil {
		return nil, fmt.Errorf("module '%s': %w", name, err)
	}

	// Determine M from source_module (Parallel) or explicit m value
	m := spec.M
	if m == 0 && spec.SourceModule != "" {
		if sourceSpec, exists := f.allModuleSpecs[spec.SourceModule]; exists {
			m = len(sourceSpec.Modules)
		}
	}
	if m == 0 {
		return nil, fmt.Errorf("module '%s': MultiChainComparison requires m or source_module with modules", name)
	}

	options := f.buildOptions(spec)

	mcc := dsgo.NewMultiChainComparison(sig, lm, m).
		WithOptions(options).
		WithAdapter(dsgo.NewFallbackAdapter())

	return mcc, nil
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

	// Set all specs so Parallel/MCC can access them
	factory.SetAllModuleSpecs(specs)

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
