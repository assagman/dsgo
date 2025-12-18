package yamlprogram

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/module"
)

// BuildResult is the output of compiling a YAML spec.
type BuildResult struct {
	Program         *module.Program
	PipelineTimeout time.Duration
}

// LMFactory creates an LM by canonical model name.
type LMFactory func(ctx context.Context, model string) (core.LM, error)

// Builder compiles a Spec into DSGo modules.
//
// It is internal-only; the API is intentionally small and oriented around
// building an executable module.Program.
type Builder struct {
	ctx  context.Context
	spec *Spec

	lmFactory LMFactory
	lms       map[string]core.LM

	signatures map[string]*core.Signature
	modules    map[string]core.Module

	histories   map[string]*core.History
	toolSources map[string][]core.Tool
}

// NewBuilder creates a Builder for a spec.
func NewBuilder(ctx context.Context, spec *Spec, lmFactory LMFactory) (*Builder, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}
	if lmFactory == nil {
		lmFactory = core.NewLM
	}

	b := &Builder{
		ctx:         ctx,
		spec:        spec,
		lmFactory:   lmFactory,
		lms:         make(map[string]core.LM),
		signatures:  make(map[string]*core.Signature),
		modules:     make(map[string]core.Module),
		histories:   make(map[string]*core.History),
		toolSources: make(map[string][]core.Tool),
	}

	return b, nil
}

// Build compiles the spec into a module.Program.
func (b *Builder) Build() (*BuildResult, error) {
	if err := b.applyRuntimeSettings(); err != nil {
		return nil, err
	}
	if err := b.buildHistories(); err != nil {
		return nil, err
	}
	if err := b.buildToolSources(); err != nil {
		return nil, err
	}
	if err := b.buildSignatures(); err != nil {
		return nil, err
	}
	if err := b.buildAllModules(); err != nil {
		return nil, err
	}

	prog := module.NewProgram(b.spec.Name)
	for _, step := range b.spec.Pipeline {
		m, ok := b.modules[step]
		if !ok {
			return nil, fmt.Errorf("pipeline references unknown module %q", step)
		}
		prog.AddModule(m)
	}

	pipelineTimeout := 5 * time.Minute
	if b.spec.Runtime.Timeouts.Pipeline != nil && b.spec.Runtime.Timeouts.Pipeline.Duration > 0 {
		pipelineTimeout = b.spec.Runtime.Timeouts.Pipeline.Duration
	}

	return &BuildResult{Program: prog, PipelineTimeout: pipelineTimeout}, nil
}

func (b *Builder) applyRuntimeSettings() error {
	var opts []core.Option

	if b.spec.Runtime.DSGo.TimeoutSeconds != nil {
		opts = append(opts, core.WithTimeout(time.Duration(*b.spec.Runtime.DSGo.TimeoutSeconds)*time.Second))
	}
	if b.spec.Runtime.DSGo.MaxRetries != nil {
		opts = append(opts, core.WithMaxRetries(*b.spec.Runtime.DSGo.MaxRetries))
	}
	if b.spec.Runtime.DSGo.Tracing != nil {
		opts = append(opts, core.WithTracing(*b.spec.Runtime.DSGo.Tracing))
	}
	if b.spec.Runtime.DSGo.SkipModelValidation != nil {
		opts = append(opts, core.WithSkipModelValidation(*b.spec.Runtime.DSGo.SkipModelValidation))
	}

	// Structured outputs
	so := b.spec.Runtime.DSGo.StructuredOutput
	if so.Enabled != nil {
		opts = append(opts, core.WithStructuredOutputEnabled(*so.Enabled))
	}
	if so.MaxAttempts != nil {
		opts = append(opts, core.WithStructuredOutputMaxAttempts(*so.MaxAttempts))
	}
	if so.Temperature != nil {
		opts = append(opts, core.WithStructuredOutputTemperature(float32(*so.Temperature)))
	}

	// Cache
	if b.spec.Runtime.DSGo.Cache.Capacity != nil {
		opts = append(opts, core.WithCache(*b.spec.Runtime.DSGo.Cache.Capacity))
	}
	if b.spec.Runtime.DSGo.Cache.TTL != nil {
		opts = append(opts, core.WithCacheTTL(b.spec.Runtime.DSGo.Cache.TTL.Duration))
	}

	if len(opts) > 0 {
		core.Configure(opts...)
	}
	return nil
}

func (b *Builder) defaultModel() string {
	if b.spec.Defaults.Model != "" {
		return b.spec.Defaults.Model
	}
	if v := os.Getenv("EXAMPLES_DEFAULT_MODEL"); v != "" {
		return v
	}
	return "openrouter/z-ai/glm-4.6"
}

func (b *Builder) getLM(model string) (core.LM, error) {
	if model == "" {
		model = b.defaultModel()
	}
	if lm, ok := b.lms[model]; ok {
		return lm, nil
	}
	lm, err := b.lmFactory(b.ctx, model)
	if err != nil {
		return nil, fmt.Errorf("create LM %q: %w", model, err)
	}
	b.lms[model] = lm
	return lm, nil
}

func (b *Builder) buildHistories() error {
	for name, hs := range b.spec.Histories {
		if hs.Limit != nil {
			b.histories[name] = core.NewHistoryWithLimit(*hs.Limit)
		} else {
			b.histories[name] = core.NewHistory()
		}
	}
	return nil
}
