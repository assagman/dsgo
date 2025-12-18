package yamlprogram

import (
	"fmt"
	"maps"
	"slices"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/module"
)

func (b *Builder) buildPredict(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	m := module.NewPredict(sig, lm).WithOptions(opts).WithAdapter(adapter)
	if h := b.getHistory(spec.History); h != nil {
		m.WithHistory(h)
	}
	if demos := convertDemos(spec.Demos); demos != nil {
		m.WithDemos(demos)
	}
	return m, nil
}

func (b *Builder) buildChainOfThought(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	m := module.NewChainOfThought(sig, lm).WithOptions(opts).WithAdapter(adapter)
	if h := b.getHistory(spec.History); h != nil {
		m.WithHistory(h)
	}
	if demos := convertDemos(spec.Demos); demos != nil {
		m.WithDemos(demos)
	}
	return m, nil
}

func (b *Builder) buildReAct(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	tools, err := b.resolveTools(spec.ReAct.Tools)
	if err != nil {
		return nil, fmt.Errorf("module %q: resolve tools: %w", name, err)
	}

	m := module.NewReAct(sig, lm, tools).WithOptions(opts).WithAdapter(adapter)
	if spec.ReAct.MaxIterations != nil {
		m.WithMaxIterations(*spec.ReAct.MaxIterations)
	}
	if spec.Verbose != nil {
		m.WithVerbose(*spec.Verbose)
	}
	return m, nil
}

func (b *Builder) buildRefine(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	m := module.NewRefine(sig, lm).WithOptions(opts).WithAdapter(adapter)
	if spec.Refine.MaxIterations != nil {
		m.WithMaxIterations(*spec.Refine.MaxIterations)
	}
	if spec.Refine.RefinementField != nil {
		m.WithRefinementField(*spec.Refine.RefinementField)
	}
	if spec.Refine.TrackHistory != nil {
		m.WithHistoryTracking(*spec.Refine.TrackHistory)
	}
	if h := b.getHistory(spec.History); h != nil {
		m.WithHistory(h)
	}
	if demos := convertDemos(spec.Demos); demos != nil {
		m.WithDemos(demos)
	}
	return m, nil
}

func (b *Builder) buildBestOfN(name string, spec ModuleSpec) (core.Module, error) {
	inner, err := b.getModule(spec.BestOfN.Of)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}

	m := module.NewBestOfN(inner, spec.BestOfN.N)

	switch spec.BestOfN.Scorer.Kind {
	case "default":
		m.WithScorer(module.DefaultScorer())
	case "confidence":
		m.WithScorer(module.ConfidenceScorer(spec.BestOfN.Scorer.Field))
	default:
		return nil, fmt.Errorf("module %q: unsupported scorer.kind %q", name, spec.BestOfN.Scorer.Kind)
	}

	if spec.BestOfN.Parallel != nil {
		m.WithParallel(*spec.BestOfN.Parallel)
	}
	if spec.BestOfN.ReturnAll != nil {
		m.WithReturnAll(*spec.BestOfN.ReturnAll)
	}
	if spec.BestOfN.MaxFailures != nil {
		m.WithMaxFailures(*spec.BestOfN.MaxFailures)
	}
	if spec.BestOfN.Threshold != nil {
		m.WithThreshold(*spec.BestOfN.Threshold)
	}
	return m, nil
}

func (b *Builder) buildProgramOfThought(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	_ = adapter // ProgramOfThought currently does not expose adapter overrides.
	m := module.NewProgramOfThought(sig, lm, spec.ProgramOfThought.Language).WithOptions(opts)
	if spec.ProgramOfThought.AllowExecution != nil {
		m.WithAllowExecution(*spec.ProgramOfThought.AllowExecution)
	}
	if spec.ProgramOfThought.ExecutionTimeoutSeconds != nil {
		m.WithExecutionTimeout(*spec.ProgramOfThought.ExecutionTimeoutSeconds)
	}
	return m, nil
}

func (b *Builder) buildProgram(name string, spec ModuleSpec) (core.Module, error) {
	p := module.NewProgram(name)
	for _, step := range spec.Program.Steps {
		m, err := b.getModule(step)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", name, err)
		}
		p.AddModule(m)
	}
	return p, nil
}

func (b *Builder) buildParallel(name string, spec ModuleSpec) (core.Module, error) {
	mode := spec.Parallel.Mode
	if mode == "" {
		mode = "clone"
	}

	var p *module.Parallel
	switch mode {
	case "clone":
		inner, err := b.getModule(spec.Parallel.Module)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", name, err)
		}
		p = module.NewParallel(inner)
	case "instances":
		instances := make([]core.Module, 0, len(spec.Parallel.Instances))
		for _, in := range spec.Parallel.Instances {
			m, err := b.getModule(in)
			if err != nil {
				return nil, fmt.Errorf("module %q: %w", name, err)
			}
			instances = append(instances, m.Clone())
		}
		p = module.NewParallelWithInstances(instances)
	case "factory":
		mods := make([]core.Module, 0, len(spec.Parallel.Factory))
		for _, use := range spec.Parallel.Factory {
			m, err := b.getModule(use)
			if err != nil {
				return nil, fmt.Errorf("module %q: %w", name, err)
			}
			mods = append(mods, m)
		}
		p = module.NewParallelWithFactory(func(i int) core.Module {
			if i < 0 || i >= len(mods) {
				return nil
			}
			return mods[i].Clone()
		})
	default:
		return nil, fmt.Errorf("module %q: unsupported parallel.mode %q", name, mode)
	}

	// Apply options
	if spec.Parallel.MaxWorkers != nil {
		p.WithMaxWorkers(*spec.Parallel.MaxWorkers)
	}
	if spec.Parallel.MaxFailures != nil {
		p.WithMaxFailures(*spec.Parallel.MaxFailures)
	}
	if spec.Parallel.FailFast != nil {
		p.WithFailFast(*spec.Parallel.FailFast)
	}
	if spec.Parallel.ReturnAll != nil {
		p.WithReturnAll(*spec.Parallel.ReturnAll)
	}
	if spec.Parallel.OnlySuccessful != nil {
		p.WithOnlySuccessful(*spec.Parallel.OnlySuccessful)
	}
	if spec.Parallel.BatchKey != nil {
		p.WithBatchKey(*spec.Parallel.BatchKey)
	}
	if spec.Parallel.Repeat != nil {
		p.WithRepeat(*spec.Parallel.Repeat)
	}
	if spec.Parallel.Verbose != nil {
		p.WithVerbose(*spec.Parallel.Verbose)
	}

	return p, nil
}

func (b *Builder) buildMultiChainComparison(name string, spec ModuleSpec) (core.Module, error) {
	sig, err := b.getSignature(name, spec.Sig)
	if err != nil {
		return nil, err
	}
	lm, err := b.getLM(spec.Model)
	if err != nil {
		return nil, err
	}
	adapter, err := b.buildAdapter(spec.Adapter, lm)
	if err != nil {
		return nil, fmt.Errorf("module %q: %w", name, err)
	}
	opts := buildGenerateOptions(spec.Gen)

	m := module.NewMultiChainComparison(sig, lm, spec.MultiChainComparison.Attempts).WithOptions(opts).WithAdapter(adapter)
	if spec.MultiChainComparison.AttemptTemplate != nil {
		m.WithAttemptTemplate(*spec.MultiChainComparison.AttemptTemplate)
	}
	if h := b.getHistory(spec.History); h != nil {
		m.WithHistory(h)
	}
	if demos := convertDemos(spec.Demos); demos != nil {
		m.WithDemos(demos)
	}
	return m, nil
}

func (b *Builder) resolveTools(selections []ToolSelection) ([]core.Tool, error) {
	byName := map[string]core.Tool{}

	for _, sel := range selections {
		tools, ok := b.toolSources[sel.Source]
		if !ok {
			return nil, fmt.Errorf("unknown tool source %q", sel.Source)
		}
		if slices.Contains(sel.Include, "*") {
			for _, t := range tools {
				byName[t.Name] = t
			}
			continue
		}

		set := map[string]bool{}
		for _, n := range sel.Include {
			set[n] = true
		}
		for _, t := range tools {
			if set[t.Name] {
				byName[t.Name] = t
			}
		}
	}

	out := make([]core.Tool, 0, len(byName))
	for t := range maps.Values(byName) {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b core.Tool) int { return stringsCompare(a.Name, b.Name) })
	return out, nil
}

func stringsCompare(a, b string) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func convertDemos(in []ExampleSpec) []core.Example {
	if len(in) == 0 {
		return nil
	}
	out := make([]core.Example, 0, len(in))
	for _, ex := range in {
		out = append(out, core.Example{Inputs: ex.Inputs, Outputs: ex.Outputs})
	}
	return out
}
