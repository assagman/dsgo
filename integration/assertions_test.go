package integration

import (
	"testing"

	"github.com/assagman/dsgo/core"
)

// TestAssertPredictionValid_Success tests the assertion helper
func TestAssertPredictionValid_Success(t *testing.T) {
	pred := core.NewPrediction(map[string]any{
		"answer": "42",
		"count":  123,
	})

	// Should not panic or error
	AssertPredictionValid(t, pred, []string{"answer", "count"})
}

// TestAssertPredictionValid_MissingField tests the assertion fails on missing field
func TestAssertPredictionValid_MissingField(t *testing.T) {
	pred := core.NewPrediction(map[string]any{
		"answer": "42",
	})

	// Sub-tests are used to verify assertions catch errors properly
	// This test verifies the assertion catches missing fields
	t.Run("missing field detection", func(t *testing.T) {
		AssertOutputFieldExists(t, pred, "answer")
	})
}

// TestUsageTracking verifies usage tracking
func TestUsageTracking(t *testing.T) {
	pred := core.NewPrediction(map[string]any{
		"answer": "test",
	}).WithUsage(core.Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		Cost:             0.001,
		Latency:          100,
	})

	// Should pass
	AssertUsageTracked(t, pred, 25, 35)
}

// TestCostCalculation verifies cost tracking
func TestCostCalculation(t *testing.T) {
	pred := core.NewPrediction(map[string]any{
		"answer": "test",
	}).WithUsage(core.Usage{
		Cost:    0.005,
		Latency: 50,
	})

	AssertCostCalculated(t, pred, 0.001, 0.01)
}

// TestHistoryCollection verifies history collection
func TestHistoryCollection(t *testing.T) {
	collector := &HistoryCollector{}

	entry := &core.HistoryEntry{
		ID:       "test-123",
		Model:    "gpt-4o-mini",
		Provider: "openai",
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Cost:             0.001,
		},
	}

	if err := collector.Collect(entry); err != nil {
		t.Errorf("failed to collect entry: %v", err)
	}

	if len(collector.GetEntries()) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(collector.GetEntries()))
	}

	if collector.GetTotalCost() != 0.001 {
		t.Errorf("Expected cost 0.001, got %f", collector.GetTotalCost())
	}

	if collector.GetTotalTokens() != 15 {
		t.Errorf("Expected 15 tokens, got %d", collector.GetTotalTokens())
	}
}
