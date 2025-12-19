package yamlprogram

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo/internal/core"
)

type execConfig struct {
	lmFactory LMFactory
}

// ExecOption configures Exec behavior.
type ExecOption func(*execConfig)

// WithLMFactory overrides LM creation.
func WithLMFactory(f LMFactory) ExecOption {
	return func(c *execConfig) {
		c.lmFactory = f
	}
}

// Exec loads, builds, and executes a YAML program spec.
//
// The YAML file must include an `inputs:` map.
func Exec(ctx context.Context, path string, opts ...ExecOption) (*core.Prediction, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := &execConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	spec, err := LoadFile(path)
	if err != nil {
		return nil, err
	}
	if len(spec.Inputs) == 0 {
		return nil, fmt.Errorf("yaml spec %q: no inputs provided", path)
	}

	builder, err := NewBuilder(ctx, spec, cfg.lmFactory)
	if err != nil {
		return nil, fmt.Errorf("yaml spec %q: create builder: %w", path, err)
	}
	res, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("yaml spec %q: build: %w", path, err)
	}

	execCtx, cancel := context.WithTimeout(ctx, res.PipelineTimeout)
	defer cancel()

	pred, err := res.Program.Forward(execCtx, spec.Inputs)
	if err != nil {
		return nil, fmt.Errorf("yaml spec %q: execute pipeline: %w", path, err)
	}
	return pred, nil
}
