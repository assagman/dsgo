package modelcatalog

import (
	"fmt"
	"sync"
	"testing"
)

func TestResolve_Defaults(t *testing.T) {
	t.Parallel()

	canonical, ok := Resolve("gpt-4o-mini")
	if !ok {
		t.Fatal("expected alias to resolve")
	}
	if canonical != "openai/gpt-4o-mini" {
		t.Fatalf("canonical = %q, want %q", canonical, "openai/gpt-4o-mini")
	}

	if !IsValidCanonical("openai/gpt-4o") {
		t.Fatal("expected openai/gpt-4o to be valid")
	}

	if IsValidCanonical("openai/does-not-exist") {
		t.Fatal("expected unknown model to be invalid")
	}
}

func TestOpenAIModelFields_Populated(t *testing.T) {
	t.Parallel()

	m, ok := GetModel("openai/gpt-4o-mini")
	if !ok {
		t.Fatal("expected openai/gpt-4o-mini to exist")
	}

	if m.Metadata.Name == "" {
		t.Error("expected Metadata.Name to be populated")
	}
	if m.Limits.ContextTokens <= 0 {
		t.Errorf("expected Limits.ContextTokens > 0, got %d", m.Limits.ContextTokens)
	}
	if m.Limits.OutputTokens <= 0 {
		t.Errorf("expected Limits.OutputTokens > 0, got %d", m.Limits.OutputTokens)
	}
	if m.Pricing.PromptPrice <= 0 {
		t.Errorf("expected Pricing.PromptPrice > 0, got %v", m.Pricing.PromptPrice)
	}
	if m.Pricing.CompletionPrice <= 0 {
		t.Errorf("expected Pricing.CompletionPrice > 0, got %v", m.Pricing.CompletionPrice)
	}
	if len(m.Modalities.Input) == 0 {
		t.Error("expected Modalities.Input to be populated")
	}
	if len(m.Modalities.Output) == 0 {
		t.Error("expected Modalities.Output to be populated")
	}
}

func TestNormalizeStringSlice(t *testing.T) {
	t.Parallel()

	got := normalizeStringSlice([]string{" Image ", "text", "TEXT", ""})
	want := []string{"image", "text", "text"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] = %q, want %q (got=%v)", i, got[i], want[i], got)
		}
	}
}

func TestRegisterModel_Idempotent(t *testing.T) {
	t.Parallel()

	m := Model{
		ID:         "testprovider/test-model",
		Aliases:    []string{"test-model"},
		Limits:     Limits{ContextTokens: 1, OutputTokens: 1},
		Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}},
		Metadata:   Metadata{Name: "Test Model", Family: "test"},
	}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel err = %v", err)
	}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel (idempotent) err = %v", err)
	}
}

func TestRegisterModel_EmptyID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		model   Model
		wantErr bool
	}{
		{"empty ID", Model{ID: ""}, true},
		{"whitespace ID", Model{ID: "   "}, true},
		{"missing provider", Model{ID: "model-only"}, true},
		{"missing model", Model{ID: "provider/"}, true},
		{"slash only", Model{ID: "/"}, true},
		{"valid model", Model{ID: "provider/model", Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "Valid Model", Family: "test"}}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := RegisterModel(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterModel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterModel_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		model   Model
		wantErr bool
	}{
		{
			name:    "missing context tokens",
			model:   Model{ID: "req/context", Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "X"}},
			wantErr: true,
		},
		{
			name:    "missing modalities",
			model:   Model{ID: "req/modalities", Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Metadata: Metadata{Name: "X"}},
			wantErr: true,
		},
		{
			name:    "missing metadata name",
			model:   Model{ID: "req/metadata", Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := RegisterModel(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterModel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterModel_ConflictingAliases(t *testing.T) {
	t.Parallel()

	m1 := Model{ID: "provider1/model1", Aliases: []string{"shared-alias"}, Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "Model 1", Family: "test"}}
	if err := RegisterModel(m1); err != nil {
		t.Fatalf("RegisterModel(m1) err = %v", err)
	}

	m2 := Model{ID: "provider2/model2", Aliases: []string{"shared-alias"}, Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "Model 2", Family: "test"}}
	err := RegisterModel(m2)
	if err == nil {
		t.Fatal("expected error registering conflicting alias")
	}
}

func TestRegisterAlias(t *testing.T) {
	t.Parallel()

	m := Model{ID: "provider/alias-test-model", Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "Alias Test Model", Family: "test"}}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel err = %v", err)
	}

	tests := []struct {
		name      string
		alias     string
		canonical string
		wantErr   bool
	}{
		{"valid alias", "my-alias", "provider/alias-test-model", false},
		{"idempotent alias", "my-alias", "provider/alias-test-model", false},
		{"empty alias", "", "provider/alias-test-model", true},
		{"whitespace alias", "   ", "provider/alias-test-model", true},
		{"unknown canonical model", "new-alias", "unknown/model", true},
		{"invalid canonical format", "new-alias2", "invalid-format", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := RegisterAlias(tt.alias, tt.canonical)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterAlias() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolve_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		wantOk bool
	}{
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"unknown model", "unknown/model", false},
		{"invalid format", "invalid-format", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, ok := Resolve(tt.input)
			if ok != tt.wantOk {
				t.Errorf("Resolve(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
		})
	}
}

func TestListModels(t *testing.T) {
	t.Parallel()

	models := ListModels()
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}

	// Check sorting
	for i := 1; i < len(models); i++ {
		if models[i-1].ID >= models[i].ID {
			t.Errorf("models not sorted: %q >= %q", models[i-1].ID, models[i].ID)
		}
	}
}

func TestListModelsByProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		provider     string
		wantMinCount int
	}{
		{"openai provider", "openai", 3},
		{"openrouter provider", "openrouter", 5},
		{"case insensitive", "OpenAI", 3},
		{"unknown provider", "unknown", 0},
		{"empty provider", "", 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			models := ListModelsByProvider(tt.provider)
			if len(models) < tt.wantMinCount {
				t.Errorf("ListModelsByProvider(%q) returned %d models, want at least %d", tt.provider, len(models), tt.wantMinCount)
			}

			// Check sorting
			for i := 1; i < len(models); i++ {
				if models[i-1].ID >= models[i].ID {
					t.Errorf("models not sorted: %q >= %q", models[i-1].ID, models[i].ID)
				}
			}
		})
	}
}

func TestConcurrentRegistration(t *testing.T) {
	t.Parallel()

	const goroutines = 10
	const modelsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < modelsPerGoroutine; j++ {
				m := Model{
					ID:         fmt.Sprintf("concurrent-test/model-%d-%d", i, j),
					Aliases:    []string{fmt.Sprintf("concurrent-alias-%d-%d", i, j)},
					Limits:     Limits{ContextTokens: 1, OutputTokens: 1},
					Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}},
					Metadata:   Metadata{Name: "Concurrent Test", Family: "test"},
				}
				_ = RegisterModel(m)
			}
		}()
	}

	wg.Wait()

	models := ListModels()
	if len(models) == 0 {
		t.Error("expected models after concurrent registration")
	}
}

func TestConcurrentResolve(t *testing.T) {
	t.Parallel()

	m := Model{ID: "concurrent-read/model", Aliases: []string{"concurrent-read-alias"}, Limits: Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: Metadata{Name: "Concurrent Read", Family: "test"}}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel err = %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = Resolve("concurrent-read-alias")
				_ = IsValidCanonical("concurrent-read/model")
				_ = IsValid("concurrent-read-alias")
				_ = ListModels()
				_ = ListModelsByProvider("concurrent-read")
			}
		}()
	}

	wg.Wait()
}
