package yamlprogram

import (
	"fmt"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/internal/core"
)

func (b *Builder) buildAdapter(spec AdapterSpec, moduleLM dsgo.LM) (dsgo.Adapter, error) {
	kind := spec.Kind
	if kind == "" {
		kind = "fallback"
	}

	var adapter dsgo.Adapter
	switch kind {
	case "fallback":
		adapter = dsgo.NewFallbackAdapter()
	case "chat":
		adapter = dsgo.NewChatAdapter()
	case "json":
		adapter = dsgo.NewJSONAdapter()
	case "two_step":
		extractLM := moduleLM
		if spec.ExtractionModel != "" {
			lm, err := b.getLM(spec.ExtractionModel)
			if err != nil {
				return nil, fmt.Errorf("create extraction LM %q: %w", spec.ExtractionModel, err)
			}
			extractLM = lm
		}
		adapter = dsgo.NewTwoStepAdapter(extractLM)
	default:
		return nil, fmt.Errorf("unknown adapter kind %q", kind)
	}

	if spec.Reasoning != nil {
		adapter = withReasoning(adapter, *spec.Reasoning)
	}
	return adapter, nil
}

func withReasoning(adapter dsgo.Adapter, on bool) dsgo.Adapter {
	switch a := adapter.(type) {
	case *core.ChatAdapter:
		return a.WithReasoning(on)
	case *core.JSONAdapter:
		return a.WithReasoning(on)
	case *core.TwoStepAdapter:
		return a.WithReasoning(on)
	case *core.FallbackAdapter:
		return a.WithReasoning(on)
	default:
		return adapter
	}
}
