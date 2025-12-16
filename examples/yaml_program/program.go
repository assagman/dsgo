package main

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo"
)

// ProgramBuilder builds a DSGo Program from YAML configuration
type ProgramBuilder struct {
	config         *PipelineConfig
	sigRegistry    *SignatureRegistry
	toolRegistry   *ToolRegistry
	moduleRegistry *ModuleRegistry
}

// NewProgramBuilder creates a new program builder from configuration
func NewProgramBuilder(ctx context.Context, config *PipelineConfig, defaultModel string, lm dsgo.LM) (*ProgramBuilder, error) {
	// Create signature registry
	sigRegistry, err := NewSignatureRegistry(config.Signatures)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature registry: %w", err)
	}

	// Create MCP client registry (if any MCP configs exist)
	var mcpRegistry *MCPClientRegistry
	if len(config.MCP) > 0 {
		mcpRegistry, err = NewMCPClientRegistry(ctx, config.MCP, config.EffectiveTimeouts())
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client registry: %w", err)
		}
	}

	// Create tool registry
	toolRegistry, err := NewToolRegistry(ctx, config.Tools, mcpRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}

	// Create module factory and registry
	factory := NewModuleFactory(ctx, defaultModel, lm, sigRegistry, toolRegistry, config.EffectiveModelSettings())
	moduleRegistry, err := NewModuleRegistry(factory, config.Modules)
	if err != nil {
		return nil, fmt.Errorf("failed to create module registry: %w", err)
	}

	return &ProgramBuilder{
		config:         config,
		sigRegistry:    sigRegistry,
		toolRegistry:   toolRegistry,
		moduleRegistry: moduleRegistry,
	}, nil
}

// Build creates the DSGo Program from the pipeline definition
func (pb *ProgramBuilder) Build() (*dsgo.Program, error) {
	program := dsgo.NewProgram(pb.config.Name)

	for i, step := range pb.config.Pipeline {
		module, err := pb.moduleRegistry.Get(step.Module)
		if err != nil {
			return nil, fmt.Errorf("pipeline step %d: %w", i+1, err)
		}
		program.AddModule(module)
	}

	return program, nil
}

// GetConfig returns the pipeline configuration
func (pb *ProgramBuilder) GetConfig() *PipelineConfig {
	return pb.config
}

// GetSignatureRegistry returns the signature registry
func (pb *ProgramBuilder) GetSignatureRegistry() *SignatureRegistry {
	return pb.sigRegistry
}

// GetModuleRegistry returns the module registry
func (pb *ProgramBuilder) GetModuleRegistry() *ModuleRegistry {
	return pb.moduleRegistry
}
