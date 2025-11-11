package cost

import (
	"math"
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		wantCost         float64
	}{
		{
			name:             "gpt-4o",
			model:            "openai/gpt-4o",
			promptTokens:     1000,
			completionTokens: 500,
			wantCost:         0.0075, // (1000 * 2.5 + 500 * 10) / 1M = 0.0075
		},
		{
			name:             "gpt-3.5-turbo",
			model:            "gpt-3.5-turbo",
			promptTokens:     10000,
			completionTokens: 5000,
			wantCost:         0.0125, // (10000 * 0.5 + 5000 * 1.5) / 1M = 0.0125
		},
		{
			name:             "llama-3.1-70b",
			model:            "meta/llama-3.1-70b",
			promptTokens:     100000,
			completionTokens: 50000,
			wantCost:         0.055000, // (100000 * 0.35 + 50000 * 0.40) / 1M = 0.055
		},
		{
			name:             "zero tokens",
			model:            "openai/gpt-4o",
			promptTokens:     0,
			completionTokens: 0,
			wantCost:         0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewCalculator()
			got := calc.Calculate(tt.model, tt.promptTokens, tt.completionTokens)

			if math.Abs(got-tt.wantCost) > 0.000001 {
				t.Errorf("Calculate() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestDefaultCalculate(t *testing.T) {
	cost := Calculate("openai/gpt-4o", 1000, 500)
	expected := 0.0075

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() = %f, want %f", cost, expected)
	}
}

func TestSetModelPricing(t *testing.T) {
	calc := NewCalculator()

	customPricing := ModelPricing{
		PromptPrice:     10.0,
		CompletionPrice: 20.0,
	}

	calc.SetModelPricing("custom-model", customPricing)

	cost := calc.Calculate("custom-model", 1000, 500)
	expected := 0.020 // (1000 * 10 + 500 * 20) / 1M = 0.02

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() = %f, want %f", cost, expected)
	}
}

func TestHasPricing(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"known model", "openai/gpt-4o", true},
		{"known model with prefix", "gpt-3.5-turbo", true},
		{"unknown model", "unknown-model-xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.HasPricing(tt.model)
			if got != tt.want {
				t.Errorf("HasPricing(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetPricing(t *testing.T) {
	calc := NewCalculator()

	t.Run("known model", func(t *testing.T) {
		pricing, ok := calc.GetPricing("openai/gpt-4o")
		if !ok {
			t.Error("GetPricing(openai/gpt-4o) returned ok=false")
		}
		if pricing.PromptPrice != 2.5 {
			t.Errorf("PromptPrice = %f, want 2.5", pricing.PromptPrice)
		}
		if pricing.CompletionPrice != 10.0 {
			t.Errorf("CompletionPrice = %f, want 10.0", pricing.CompletionPrice)
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		_, ok := calc.GetPricing("unknown-model")
		if ok {
			t.Error("GetPricing(unknown-model) returned ok=true")
		}
	})
}

func TestFindPricingByPattern(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name          string
		model         string
		expectNonZero bool
	}{
		{"exact match", "openai/gpt-4o", true},
		{"case insensitive", "OPENAI/GPT-4O", true},
		{"contains pattern", "openai/gpt-4o-something", true},
		{"no match", "completely-unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := calc.findPricingByPattern(tt.model)
			hasNonZero := pricing.PromptPrice > 0 || pricing.CompletionPrice > 0

			if hasNonZero != tt.expectNonZero {
				t.Errorf("findPricingByPattern(%q) hasNonZero = %v, want %v", tt.model, hasNonZero, tt.expectNonZero)
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
			_ = calc.Calculate("openai/gpt-4o", 1000, 500)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCalculate_PatternMatch(t *testing.T) {
	calc := NewCalculator()

	// Test Calculate with model that doesn't have exact key but matches pattern
	cost := calc.Calculate("openai/gpt-4o-something-new", 1000, 500)

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
	calc := NewCalculator()

	// Test GetPricing with model that matches via pattern
	pricing, ok := calc.GetPricing("meta/llama-3.1-70b-derivative")

	if !ok {
		t.Error("GetPricing() with pattern match returned ok=false, expected ok=true")
	}

	// Should match "meta/llama-3.1-70b" pricing
	if pricing.PromptPrice != 0.35 {
		t.Errorf("PromptPrice = %f, want 0.35", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 0.40 {
		t.Errorf("CompletionPrice = %f, want 0.40", pricing.CompletionPrice)
	}
}
