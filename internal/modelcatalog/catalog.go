package modelcatalog

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Pricing represents model pricing in USD per 1M tokens.
type Pricing struct {
	PromptPrice     float64 // Price per 1M prompt tokens (USD)
	CompletionPrice float64 // Price per 1M completion tokens (USD)
}

// Model describes a supported model identifier.
//
// ID must be in canonical form: "provider/model".
// For OpenRouter, the model portion may contain additional slashes
// (e.g. "openrouter/google/gemini-2.5-flash").
//
// The catalog is authoritative for dsgo.NewLM/core.NewLM: models must be present
// here to be considered valid.
type Model struct {
	ID      string
	Aliases []string
	Pricing Pricing
}

var (
	mu       sync.RWMutex
	models   = map[string]Model{}  // canonical (lowercased) -> model
	aliasMap = map[string]string{} // alias (lowercased) -> canonical (lowercased)
)

// RegisterModel registers a supported model.
//
// Registration is idempotent: re-registering the exact same model is allowed.
// Attempting to re-register an existing model with different aliases returns an error.
func RegisterModel(m Model) error {
	canonical, err := normalizeCanonicalID(m.ID)
	if err != nil {
		return fmt.Errorf("invalid model id %q: %w", m.ID, err)
	}

	aliases := make([]string, 0, len(m.Aliases))
	for _, a := range m.Aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		aliases = append(aliases, strings.ToLower(a))
	}

	// Normalize stored form
	m.ID = canonical
	m.Aliases = aliases

	mu.Lock()
	defer mu.Unlock()

	if existing, ok := models[canonical]; ok {
		if modelsEquivalent(existing, m) {
			return nil
		}
		return fmt.Errorf("model %q already registered", canonical)
	}

	models[canonical] = m
	aliasMap[canonical] = canonical
	for _, a := range aliases {
		if err := registerAliasLocked(a, canonical); err != nil {
			delete(models, canonical)
			delete(aliasMap, canonical)
			// best-effort cleanup for any aliases already registered
			for _, prev := range aliases {
				if aliasMap[prev] == canonical {
					delete(aliasMap, prev)
				}
			}
			return err
		}
	}

	return nil
}

// RegisterAlias adds an alias for an existing canonical model.
//
// Registration is idempotent: re-registering the same alias->canonical mapping is allowed.
func RegisterAlias(alias string, canonical string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return fmt.Errorf("alias is required")
	}
	canonicalNorm, err := normalizeCanonicalID(canonical)
	if err != nil {
		return fmt.Errorf("invalid canonical model id %q: %w", canonical, err)
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := models[canonicalNorm]; !ok {
		return fmt.Errorf("cannot register alias for unknown model %q", canonicalNorm)
	}

	return registerAliasLocked(strings.ToLower(alias), canonicalNorm)
}

func registerAliasLocked(alias, canonical string) error {
	if existing, ok := aliasMap[alias]; ok {
		if existing == canonical {
			return nil
		}
		return fmt.Errorf("alias %q already registered for %q", alias, existing)
	}
	aliasMap[alias] = canonical
	return nil
}

// Resolve returns the canonical model id for a canonical id or alias.
func Resolve(idOrAlias string) (string, bool) {
	idOrAlias = strings.TrimSpace(idOrAlias)
	if idOrAlias == "" {
		return "", false
	}

	key := strings.ToLower(idOrAlias)

	mu.RLock()
	canonical, ok := aliasMap[key]
	mu.RUnlock()
	if ok {
		return canonical, true
	}

	// If it's already canonical, normalize and check.
	canonical, err := normalizeCanonicalID(idOrAlias)
	if err != nil {
		return "", false
	}

	mu.RLock()
	_, ok = models[canonical]
	mu.RUnlock()
	if !ok {
		return "", false
	}
	return canonical, true
}

// IsValidCanonical reports whether the given model id is a known canonical model.
func IsValidCanonical(modelID string) bool {
	canonical, err := normalizeCanonicalID(modelID)
	if err != nil {
		return false
	}
	mu.RLock()
	_, ok := models[canonical]
	mu.RUnlock()
	return ok
}

// IsValid reports whether the provided id is either a known canonical id or a registered alias.
func IsValid(idOrAlias string) bool {
	_, ok := Resolve(idOrAlias)
	return ok
}

// GetPricing returns the pricing for a model by canonical ID or alias.
// Returns zero pricing and false if the model is not found.
func GetPricing(idOrAlias string) (Pricing, bool) {
	canonical, ok := Resolve(idOrAlias)
	if !ok {
		return Pricing{}, false
	}

	mu.RLock()
	m, ok := models[canonical]
	mu.RUnlock()
	if !ok {
		return Pricing{}, false
	}
	return m.Pricing, true
}

// ListModels returns all registered models, sorted by canonical ID.
func ListModels() []Model {
	mu.RLock()
	out := make([]Model, 0, len(models))
	for _, m := range models {
		out = append(out, m)
	}
	mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// ListModelsByProvider returns all models for the given provider.
func ListModelsByProvider(provider string) []Model {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return nil
	}
	prefix := provider + "/"

	mu.RLock()
	out := make([]Model, 0)
	for _, m := range models {
		if strings.HasPrefix(m.ID, prefix) {
			out = append(out, m)
		}
	}
	mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func normalizeCanonicalID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("model id is required")
	}
	parts := strings.SplitN(id, "/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("model id must be in form provider/model")
	}
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	model := strings.TrimSpace(parts[1])
	if provider == "" || model == "" {
		return "", fmt.Errorf("model id must be in form provider/model")
	}
	return provider + "/" + strings.ToLower(model), nil
}

func modelsEquivalent(a, b Model) bool {
	if a.ID != b.ID {
		return false
	}
	if len(a.Aliases) != len(b.Aliases) {
		return false
	}
	// Aliases are stored lowercased already.
	aCopy := append([]string(nil), a.Aliases...)
	bCopy := append([]string(nil), b.Aliases...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}
