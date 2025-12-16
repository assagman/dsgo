package cost

import (
	"fmt"
	"math"
	"sync"
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

func TestCalculate_UnknownModel(t *testing.T) {
	t.Parallel()

	calc := NewCalculator()
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		wantCost         float64
	}{
		{"completely unknown model", "unknown/model", 1000, 500, 0.0},
		{"empty model name", "", 1000, 500, 0.0},
		{"whitespace model name", "   ", 1000, 500, 0.0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := calc.Calculate(tt.model, tt.promptTokens, tt.completionTokens)
			if got != tt.wantCost {
				t.Errorf("Calculate() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestCalculate_EdgeCases(t *testing.T) {
	t.Parallel()

	calc := NewCalculator()

	// Negative tokens should not panic
	t.Run("negative tokens", func(t *testing.T) {
		_ = calc.Calculate("openai/gpt-4o", -1000, -500)
	})

	// Very large tokens
	t.Run("very large tokens", func(t *testing.T) {
		cost := calc.Calculate("openai/gpt-4o", 1_000_000_000, 0)
		if cost < 2000.0 {
			t.Errorf("Calculate() = %f, want at least 2000.0", cost)
		}
	})
}

func TestCalculator_NilMaps(t *testing.T) {
	t.Parallel()

	calc := &Calculator{pricing: nil}

	cost := calc.Calculate("openai/gpt-4o", 1000, 500)
	if cost != 0 {
		t.Errorf("Calculate with nil pricing = %f, want 0", cost)
	}

	ok := calc.HasPricing("openai/gpt-4o")
	if ok {
		t.Error("HasPricing with nil pricing = true, want false")
	}
}

func TestConcurrentSetAndGet(t *testing.T) {
	t.Parallel()

	calc := NewCalculator()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			calc.SetModelPricing(fmt.Sprintf("concurrent/model-%d", i), ModelPricing{
				PromptPrice:     float64(i),
				CompletionPrice: float64(i * 2),
			})
		}()
	}

	// Concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = calc.Calculate("openai/gpt-4o", 1000, 500)
			_ = calc.HasPricing("openai/gpt-4o")
			_, _ = calc.GetPricing("openai/gpt-4o")
		}()
	}

	wg.Wait()
}
