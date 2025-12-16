package modelcatalog

import "testing"

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

func TestRegisterModel_Idempotent(t *testing.T) {
	t.Parallel()

	m := Model{ID: "testprovider/test-model", Aliases: []string{"test-model"}}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel err = %v", err)
	}
	if err := RegisterModel(m); err != nil {
		t.Fatalf("RegisterModel (idempotent) err = %v", err)
	}
}
