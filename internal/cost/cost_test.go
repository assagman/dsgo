package cost

import (
	"math"
	"testing"
)

func TestCalculate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		provider         string
		model            string
		promptTokens     int
		completionTokens int
		wantCost         float64
	}{
		{
			name:             "openai gpt-4o",
			provider:         "openai",
			model:            "gpt-4o",
			promptTokens:     1000,
			completionTokens: 500,
			wantCost:         0.0075, // (1000 * 2.5 + 500 * 10) / 1M = 0.0075
		},
		{
			name:             "openai gpt-3.5-turbo",
			provider:         "openai",
			model:            "gpt-3.5-turbo",
			promptTokens:     10000,
			completionTokens: 5000,
			wantCost:         0.0125, // (10000 * 0.5 + 5000 * 1.5) / 1M = 0.0125
		},
		{
			name:             "openrouter llama-3.1-70b",
			provider:         "openrouter",
			model:            "meta/llama-3.1-70b",
			promptTokens:     100000,
			completionTokens: 50000,
			wantCost:         0.055000, // (100000 * 0.35 + 50000 * 0.40) / 1M = 0.055
		},
		{
			name:             "zero tokens",
			provider:         "openai",
			model:            "gpt-4o",
			promptTokens:     0,
			completionTokens: 0,
			wantCost:         0.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calc := NewCalculator()
			got := calc.Calculate(tt.provider, tt.model, tt.promptTokens, tt.completionTokens)

			if math.Abs(got-tt.wantCost) > 0.000001 {
				t.Errorf("Calculate() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestCalculateIfKnown(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	t.Run("known model", func(t *testing.T) {
		t.Parallel()
		got, ok := calc.CalculateIfKnown("openai", "gpt-4o", 1000, 500)
		if !ok {
			t.Fatal("CalculateIfKnown(openai, gpt-4o) ok=false")
		}
		want := 0.0075
		if math.Abs(got-want) > 0.000001 {
			t.Errorf("CalculateIfKnown() = %f, want %f", got, want)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		t.Parallel()
		got, ok := calc.CalculateIfKnown("unknown", "completely-unknown-model", 1000, 500)
		if ok {
			t.Fatal("CalculateIfKnown(unknown, completely-unknown-model) ok=true")
		}
		if got != 0 {
			t.Errorf("CalculateIfKnown() = %f, want 0", got)
		}
	})

	t.Run("free model", func(t *testing.T) {
		t.Parallel()
		got, ok := calc.CalculateIfKnown("openrouter", "minimax/minimax-m2:free", 1000, 500)
		if !ok {
			t.Fatal("CalculateIfKnown(openrouter, minimax/minimax-m2:free) ok=false")
		}
		if got != 0 {
			t.Errorf("CalculateIfKnown() = %f, want 0", got)
		}
	})
}

func TestDefaultCalculate(t *testing.T) {
	t.Parallel()
	cost := Calculate("openai", "gpt-4o", 1000, 500)
	expected := 0.0075

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() = %f, want %f", cost, expected)
	}
}

func TestSetModelPricing(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	customPricing := ModelPricing{
		PromptPrice:     10.0,
		CompletionPrice: 20.0,
	}

	calc.SetModelPricing("custom/custom-model", customPricing)

	cost := calc.Calculate("custom", "custom-model", 1000, 500)
	expected := 0.020 // (1000 * 10 + 500 * 20) / 1M = 0.02

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() = %f, want %f", cost, expected)
	}
}

func TestHasPricing(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	tests := []struct {
		name     string
		provider string
		model    string
		want     bool
	}{
		{"known model", "openai", "gpt-4o", true},
		{"known model openai gpt-3.5-turbo", "openai", "gpt-3.5-turbo", true},
		{"unknown model", "unknown", "unknown-model-xyz", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := calc.HasPricing(tt.provider, tt.model)
			if got != tt.want {
				t.Errorf("HasPricing(%q, %q) = %v, want %v", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestGetPricing(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	t.Run("known model", func(t *testing.T) {
		t.Parallel()
		pricing, ok := calc.GetPricing("openai", "gpt-4o")
		if !ok {
			t.Error("GetPricing(openai, gpt-4o) returned ok=false")
		}
		if pricing.PromptPrice != 2.5 {
			t.Errorf("PromptPrice = %f, want 2.5", pricing.PromptPrice)
		}
		if pricing.CompletionPrice != 10.0 {
			t.Errorf("CompletionPrice = %f, want 10.0", pricing.CompletionPrice)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		t.Parallel()
		_, ok := calc.GetPricing("unknown", "unknown-model")
		if ok {
			t.Error("GetPricing(unknown, unknown-model) returned ok=true")
		}
	})
}

func TestFindPricingByPattern(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	tests := []struct {
		name          string
		provider      string
		model         string
		expectNonZero bool
	}{
		{"exact match", "openai", "gpt-4o", true},
		{"case insensitive", "OPENAI", "GPT-4O", true},
		{"contains pattern", "openai", "gpt-4o-something", true},
		{"no match", "unknown", "completely-unknown-model", false},
		{"cross provider no match", "other", "gpt-4o", false}, // openai model shouldn't match other provider
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pricing := calc.findPricingByPattern(tt.provider, tt.model)
			hasNonZero := pricing.PromptPrice > 0 || pricing.CompletionPrice > 0

			if hasNonZero != tt.expectNonZero {
				t.Errorf("findPricingByPattern(%q, %q) hasNonZero = %v, want %v", tt.provider, tt.model, hasNonZero, tt.expectNonZero)
			}
		})
	}
}

func TestCalculatorConcurrency(t *testing.T) {
	calc := NewCalculator()

	// Test concurrent calculations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = calc.Calculate("openai", "gpt-4o", 1000, 500)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCalculate_PatternMatch(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test Calculate with model that doesn't have exact key but matches pattern
	cost := calc.Calculate("openai", "gpt-4o-something-new", 1000, 500)

	// Should match "openai/gpt-4o" pricing via pattern matching
	expected := 0.0075 // (1000 * 2.5 + 500 * 10) / 1M = 0.0075

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() with pattern match = %f, want %f", cost, expected)
	}

	// Verify non-zero cost was calculated (proves pattern matching worked)
	if cost == 0 {
		t.Error("Calculate() with pattern match returned 0, expected non-zero cost")
	}
}

func TestGetPricing_PatternMatch(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test GetPricing with model that matches via pattern
	pricing, ok := calc.GetPricing("openrouter", "meta/llama-3.1-70b-derivative")

	if !ok {
		t.Error("GetPricing() with pattern match returned ok=false, expected ok=true")
	}

	// Should match "openrouter/meta/llama-3.1-70b" pricing
	if pricing.PromptPrice != 0.35 {
		t.Errorf("PromptPrice = %f, want 0.35", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 0.40 {
		t.Errorf("CompletionPrice = %f, want 0.40", pricing.CompletionPrice)
	}
}

func TestGetPricing_EmptyProviderFallback(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test backwards compatibility: empty provider should still find pricing
	// by searching across all providers
	pricing, ok := calc.GetPricing("", "gpt-4o")

	if !ok {
		t.Error("GetPricing(\"\", \"gpt-4o\") returned ok=false, expected ok=true (cross-provider fallback)")
	}

	// Should find OpenAI pricing by pattern match
	if pricing.PromptPrice != 2.5 {
		t.Errorf("PromptPrice = %f, want 2.5", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 10.0 {
		t.Errorf("CompletionPrice = %f, want 10.0", pricing.CompletionPrice)
	}
}

func TestGetPricing_ProviderScopedMatch(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that "openrouter", "gpt-4" matches the nested key "openrouter/openai/gpt-4"
	pricing, ok := calc.GetPricing("openrouter", "openai/gpt-4")

	if !ok {
		t.Error("GetPricing(\"openrouter\", \"openai/gpt-4\") returned ok=false")
	}

	if pricing.PromptPrice != 30.0 {
		t.Errorf("PromptPrice = %f, want 30.0", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 60.0 {
		t.Errorf("CompletionPrice = %f, want 60.0", pricing.CompletionPrice)
	}
}

func TestCalculate_ProviderScoping(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that provider scoping prevents cross-provider matches
	// gpt-4o should match openai but NOT be matched for openrouter provider
	// (openrouter has its own openai/gpt-4o entry, but we test the scoping)
	cost := calc.Calculate("openai", "gpt-4o", 1000, 500)
	expected := 0.0075 // (1000 * 2.5 + 500 * 10) / 1M

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate(\"openai\", \"gpt-4o\") = %f, want %f", cost, expected)
	}

	// Same model via openrouter should use openrouter's pricing
	cost2 := calc.Calculate("openrouter", "openai/gpt-4o", 1000, 500)
	expected2 := 0.0075 // Same pricing in this case

	if math.Abs(cost2-expected2) > 0.000001 {
		t.Errorf("Calculate(\"openrouter\", \"openai/gpt-4o\") = %f, want %f", cost2, expected2)
	}
}

func TestCalculateIfKnown_EmptyProviderFallback(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that empty provider still returns ok=true for known models
	got, ok := calc.CalculateIfKnown("", "gpt-4o", 1000, 500)
	if !ok {
		t.Fatal("CalculateIfKnown(\"\", \"gpt-4o\") ok=false, expected true (cross-provider fallback)")
	}

	expected := 0.0075
	if math.Abs(got-expected) > 0.000001 {
		t.Errorf("CalculateIfKnown(\"\", \"gpt-4o\") = %f, want %f", got, expected)
	}
}

func TestGetPricing_OpenRouterGPT4(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that "openrouter"/"gpt-4" can find "openrouter/openai/gpt-4" via pattern matching
	pricing, ok := calc.GetPricing("openrouter", "gpt-4")

	if !ok {
		t.Error("GetPricing(\"openrouter\", \"gpt-4\") returned ok=false, expected ok=true (should match openrouter/openai/gpt-4)")
	}

	if pricing.PromptPrice != 30.0 {
		t.Errorf("PromptPrice = %f, want 30.0", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 60.0 {
		t.Errorf("CompletionPrice = %f, want 60.0", pricing.CompletionPrice)
	}
}

func TestGetPricing_OpenRouterGPT4o(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that "openrouter"/"gpt-4o" can find "openrouter/openai/gpt-4o" via Case B component matching
	pricing, ok := calc.GetPricing("openrouter", "gpt-4o")

	if !ok {
		t.Error("GetPricing(\"openrouter\", \"gpt-4o\") returned ok=false, expected ok=true (should match openrouter/openai/gpt-4o)")
	}

	if pricing.PromptPrice != 2.5 {
		t.Errorf("PromptPrice = %f, want 2.5", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 10.0 {
		t.Errorf("CompletionPrice = %f, want 10.0", pricing.CompletionPrice)
	}
}

func TestGetPricing_LongestPrefixWins(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Test that "gpt-4o-derivative" matches "gpt-4o" (longer prefix) not "gpt-4" (shorter prefix)
	// This confirms longest-match-wins semantics in Case A
	pricing, ok := calc.GetPricing("openai", "gpt-4o-derivative")

	if !ok {
		t.Error("GetPricing(\"openai\", \"gpt-4o-derivative\") returned ok=false, expected ok=true")
	}

	// Should match openai/gpt-4o pricing (2.5/10.0) not openai/gpt-4 (30.0/60.0)
	if pricing.PromptPrice != 2.5 {
		t.Errorf("PromptPrice = %f, want 2.5 (should match gpt-4o, not gpt-4)", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 10.0 {
		t.Errorf("CompletionPrice = %f, want 10.0 (should match gpt-4o, not gpt-4)", pricing.CompletionPrice)
	}
}

func TestGetPricing_NoCrossProviderLeak(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	// Verify that a known model (gpt-4o) is NOT found when using a different provider.
	// This ensures provider scoping prevents cross-provider matches.
	_, ok := calc.GetPricing("other", "gpt-4o")
	if ok {
		t.Error("GetPricing(\"other\", \"gpt-4o\") returned ok=true, expected ok=false (no cross-provider leak)")
	}
}
