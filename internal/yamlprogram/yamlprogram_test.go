package yamlprogram

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/module"
)

type fakeLM struct{}

func (f *fakeLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	return &core.GenerateResult{Content: ""}, nil
}

func (f *fakeLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunks := make(chan core.Chunk)
	errs := make(chan error, 1)
	close(chunks)
	close(errs)
	return chunks, errs
}

func (f *fakeLM) Name() string        { return "fake" }
func (f *fakeLM) SupportsJSON() bool  { return true }
func (f *fakeLM) SupportsTools() bool { return true }
func (f *fakeLM) IsOpenAI() bool      { return false }

func TestLoad_StrictUnknownFields(t *testing.T) {
	y := []byte(`
name: test
signatures:
  s:
    desc: hi
    in: {x: string}
    out: {y: string}
modules:
  m: {kind: predict, sig: s}
pipeline: [m]
unknown_top_level_key: 123
`)

	_, err := Load(bytes.NewReader(y))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "unknown_top_level_key") {
		t.Fatalf("expected unknown field error, got: %v", err)
	}
}

func TestNormalize_DefaultTemperature(t *testing.T) {
	s := &Spec{
		Name:       "x",
		Signatures: map[string]SignatureSpec{"s": {Desc: "d", In: map[string]FieldSpec{"x": {Type: "string"}}, Out: map[string]FieldSpec{"y": {Type: "string"}}}},
		Modules:    map[string]ModuleSpec{"m": {Kind: "predict", Sig: "s"}},
		Pipeline:   []string{"m"},
	}

	Normalize(s)
	if s.Defaults.Gen.Temperature == nil {
		t.Fatalf("expected default temperature")
	}
	if *s.Defaults.Gen.Temperature != 0.6 {
		t.Fatalf("expected 0.6, got %v", *s.Defaults.Gen.Temperature)
	}
}

func TestBuild_ExplicitTemperatureZeroIsApplied(t *testing.T) {
	zero := 0.0
	s := &Spec{
		Name:     "x",
		Defaults: Defaults{Gen: GenerateOptionsSpec{Temperature: ptrFloat(0.6)}},
		Signatures: map[string]SignatureSpec{
			"s": {
				Desc: "d",
				In:   map[string]FieldSpec{"x": {Type: "string"}},
				Out:  map[string]FieldSpec{"y": {Type: "string"}},
			},
		},
		Modules: map[string]ModuleSpec{
			"m": {Kind: "predict", Sig: "s", Gen: GenerateOptionsSpec{Temperature: &zero}},
		},
		Pipeline: []string{"m"},
	}

	Normalize(s)
	if err := Validate(s); err != nil {
		t.Fatalf("validate: %v", err)
	}

	b, err := NewBuilder(context.Background(), s, func(ctx context.Context, model string) (core.LM, error) {
		return &fakeLM{}, nil
	})
	if err != nil {
		t.Fatalf("builder: %v", err)
	}
	_, err = b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	pm, ok := b.modules["m"]
	if !ok {
		t.Fatalf("module not built")
	}
	pred, ok := pm.(*module.Predict)
	if !ok {
		t.Fatalf("expected *module.Predict, got %T", pm)
	}
	if pred.Options.Temperature != 0 {
		t.Fatalf("expected temperature=0, got %v", pred.Options.Temperature)
	}
}

func ptrFloat(v float64) *float64 { return &v }
