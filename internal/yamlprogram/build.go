package yamlprogram

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/assagman/dsgo"
)

// BuildResult is the output of compiling a YAML spec.
type BuildResult struct {
	Program         *dsgo.Program
	PipelineTimeout time.Duration
}

// LMFactory creates an LM by canonical model name.
type LMFactory func(ctx context.Context, model string) (dsgo.LM, error)

// Builder compiles a Spec into DSGo modules.
//
// It is internal-only; the API is intentionally small and oriented around
// building an executable dsgo.Program.
type Builder struct {
	ctx  context.Context
	spec *Spec

	lmFactory LMFactory
	lms       map[string]dsgo.LM

	signatures map[string]*dsgo.Signature
	modules    map[string]dsgo.Module

	histories   map[string]*dsgo.History
	toolSources map[string][]dsgo.Tool
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
		lmFactory = dsgo.NewLM
	}

	b := &Builder{
		ctx:         ctx,
		spec:        spec,
		lmFactory:   lmFactory,
		lms:         make(map[string]dsgo.LM),
		signatures:  make(map[string]*dsgo.Signature),
		modules:     make(map[string]dsgo.Module),
		histories:   make(map[string]*dsgo.History),
		toolSources: make(map[string][]dsgo.Tool),
	}

	return b, nil
}

// Build compiles the spec into a dsgo.Program.
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

	prog := dsgo.NewProgram(b.spec.Name)
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
	var opts []dsgo.Option

	if b.spec.Runtime.DSGo.TimeoutSeconds != nil {
		opts = append(opts, dsgo.WithTimeout(time.Duration(*b.spec.Runtime.DSGo.TimeoutSeconds)*time.Second))
	}
	if b.spec.Runtime.DSGo.MaxRetries != nil {
		opts = append(opts, dsgo.WithMaxRetries(*b.spec.Runtime.DSGo.MaxRetries))
	}
	if b.spec.Runtime.DSGo.Tracing != nil {
		opts = append(opts, dsgo.WithTracing(*b.spec.Runtime.DSGo.Tracing))
	}
	if b.spec.Runtime.DSGo.SkipModelValidation != nil {
		opts = append(opts, dsgo.WithSkipModelValidation(*b.spec.Runtime.DSGo.SkipModelValidation))
	}

	// Structured outputs
	so := b.spec.Runtime.DSGo.StructuredOutput
	if so.Enabled != nil {
		opts = append(opts, dsgo.WithStructuredOutputEnabled(*so.Enabled))
	}
	if so.MaxAttempts != nil {
		opts = append(opts, dsgo.WithStructuredOutputMaxAttempts(*so.MaxAttempts))
	}
	if so.Temperature != nil {
		opts = append(opts, dsgo.WithStructuredOutputTemperature(float32(*so.Temperature)))
	}

	// Cache
	if b.spec.Runtime.DSGo.Cache.Capacity != nil {
		opts = append(opts, dsgo.WithCache(*b.spec.Runtime.DSGo.Cache.Capacity))
	}
	if b.spec.Runtime.DSGo.Cache.TTL != nil {
		opts = append(opts, dsgo.WithCacheTTL(b.spec.Runtime.DSGo.Cache.TTL.Duration))
	}

	if len(opts) > 0 {
		dsgo.Configure(opts...)
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

func (b *Builder) getLM(model string) (dsgo.LM, error) {
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
			b.histories[name] = dsgo.NewHistoryWithLimit(*hs.Limit)
		} else {
			b.histories[name] = dsgo.NewHistory()
		}
	}
	return nil
}
