package yamlprogram

import (
	"fmt"

	"github.com/assagman/dsgo"
)

func (b *Builder) buildAllModules() error {
	for name := range b.spec.Modules {
		if _, err := b.getModule(name); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) getModule(name string) (dsgo.Module, error) {
	if m, ok := b.modules[name]; ok {
		return m, nil
	}
	ms, ok := b.spec.Modules[name]
	if !ok {
		return nil, fmt.Errorf("unknown module %q", name)
	}

	m, err := b.buildModule(name, ms)
	if err != nil {
		return nil, err
	}
	b.modules[name] = m
	return m, nil
}

func (b *Builder) buildModule(name string, spec ModuleSpec) (dsgo.Module, error) {
	switch spec.Kind {
	case "predict":
		return b.buildPredict(name, spec)
	case "chain_of_thought":
		return b.buildChainOfThought(name, spec)
	case "react":
		return b.buildReAct(name, spec)
	case "refine":
		return b.buildRefine(name, spec)
	case "best_of_n":
		return b.buildBestOfN(name, spec)
	case "program_of_thought":
		return b.buildProgramOfThought(name, spec)
	case "program":
		return b.buildProgram(name, spec)
	case "parallel":
		return b.buildParallel(name, spec)
	case "multi_chain_comparison":
		return b.buildMultiChainComparison(name, spec)
	default:
		return nil, fmt.Errorf("module %q: unsupported kind %q", name, spec.Kind)
	}
}

func (b *Builder) getSignature(name, sigName string) (*dsgo.Signature, error) {
	sig, ok := b.signatures[sigName]
	if !ok {
		return nil, fmt.Errorf("module %q: unknown signature %q", name, sigName)
	}
	return sig, nil
}

func (b *Builder) getHistory(ref string) *dsgo.History {
	if ref == "" {
		return nil
	}
	return b.histories[ref]
}
