package cost

import (
	"math"
	"testing"
)

func TestCalculate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		wantCost         float64
	}{
		{
			name:             "gpt-4o standard",
			model:            "openai/gpt-4o",
			promptTokens:     1000,
			completionTokens: 500,
			wantCost:         0.0075, // (1000 * 2.5 + 500 * 10) / 1M
		},
		{
			name:             "gpt-3.5-turbo alias resolves",
			model:            "gpt-3.5-turbo",
			promptTokens:     10000,
			completionTokens: 5000,
			wantCost:         0.0125, // (10000 * 0.5 + 5000 * 1.5) / 1M
		},
		{
			name:             "openrouter gemini",
			model:            "openrouter/google/gemini-2.5-flash",
			promptTokens:     1000,
			completionTokens: 500,
			wantCost:         0.00155, // (1000 * 0.30 + 500 * 2.50) / 1M
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calc := NewCalculator()
			got := calc.Calculate(tt.model, tt.promptTokens, tt.completionTokens)

			if math.Abs(got-tt.wantCost) > 0.000001 {
				t.Errorf("Calculate() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestCalculateWithTier(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	standard := calc.CalculateWithTier("openai/gpt-4o", TierStandard, 1000, 500)
	batch := calc.CalculateWithTier("openai/gpt-4o", TierBatch, 1000, 500)

	if standard <= 0 {
		t.Fatalf("expected standard cost > 0, got %f", standard)
	}
	if batch <= 0 {
		t.Fatalf("expected batch cost > 0, got %f", batch)
	}
	if batch >= standard {
		t.Fatalf("expected batch cost < standard cost (batch=%f standard=%f)", batch, standard)
	}

	// Batch is not defined for gpt-3.5-turbo in our curated table; should fall back to standard.
	standard35 := calc.CalculateWithTier("openai/gpt-3.5-turbo", TierStandard, 1000, 500)
	batch35 := calc.CalculateWithTier("openai/gpt-3.5-turbo", TierBatch, 1000, 500)
	if math.Abs(standard35-batch35) > 0.000001 {
		t.Fatalf("expected tier fallback to match standard (standard=%f batch=%f)", standard35, batch35)
	}
}

func TestDefaultCalculate(t *testing.T) {
	t.Parallel()
	cost := Calculate("openai/gpt-4o", 1000, 500)
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

	cost := calc.Calculate("custom/custom-model", 1000, 500)
	expected := 0.020 // (1000 * 10 + 500 * 20) / 1M

	if math.Abs(cost-expected) > 0.000001 {
		t.Errorf("Calculate() = %f, want %f", cost, expected)
	}
}

func TestSetModelPricingForTier(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	calc.SetModelPricingForTier("custom-model", TierPriority, ModelPricing{PromptPrice: 1.0, CompletionPrice: 2.0})
	priority := calc.CalculateWithTier("custom-model", TierPriority, 1000, 500)
	standard := calc.CalculateWithTier("custom-model", TierStandard, 1000, 500)

	if priority == 0 {
		t.Fatalf("expected priority cost > 0")
	}
	if standard != 0 {
		t.Fatalf("expected standard cost to be 0 without standard pricing, got %f", standard)
	}
}

func TestHasPricing(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"known model", "openai/gpt-4o", true},
		{"known alias", "gpt-3.5-turbo", true},
		{"unknown model", "unknown-model-xyz", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := calc.HasPricing(tt.model)
			if got != tt.want {
				t.Errorf("HasPricing(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetPricingForTier(t *testing.T) {
	t.Parallel()
	calc := NewCalculator()

	pricing, ok := calc.GetPricingForTier("openai/gpt-4o", TierPriority)
	if !ok {
		t.Fatal("expected pricing for openai/gpt-4o priority")
	}
	if pricing.PromptPrice != 4.25 {
		t.Fatalf("PromptPrice = %f, want 4.25", pricing.PromptPrice)
	}
	if pricing.CompletionPrice != 17.0 {
		t.Fatalf("CompletionPrice = %f, want 17.0", pricing.CompletionPrice)
	}
}

func TestCalculatorConcurrency(t *testing.T) {
	calc := NewCalculator()

	done := make(chan bool)
	for range 10 {
		go func() {
			_ = calc.Calculate("openai/gpt-4o", 1000, 500)
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}
