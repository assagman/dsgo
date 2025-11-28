package integration

import (
	"testing"

	"github.com/assagman/dsgo"
)

// AssertPredictionValid validates that a prediction contains all expected fields
func AssertPredictionValid(t *testing.T, pred *dsgo.Prediction, expectedFields []string) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	if pred.Outputs == nil {
		t.Fatal("Prediction.Outputs is nil")
	}

	for _, field := range expectedFields {
		if _, exists := pred.Outputs[field]; !exists {
			t.Errorf("Expected field %q not found in outputs", field)
		}
	}
}

// AssertPredictionError validates that an error occurred with an expected message
func AssertPredictionError(t *testing.T, err error, expectedSubstring string) {
	t.Helper()

	if err == nil {
		t.Errorf("Expected error containing %q, got nil", expectedSubstring)
		return
	}

	errMsg := err.Error()
	if !stringContains(errMsg, expectedSubstring) {
		t.Errorf("Expected error to contain %q, got: %s", expectedSubstring, errMsg)
	}
}

// AssertOutputsMatch validates that prediction outputs match expected values
func AssertOutputsMatch(t *testing.T, pred *dsgo.Prediction, expected map[string]interface{}) {
	t.Helper()

	AssertPredictionValid(t, pred, getKeys(expected))

	for key, expectedVal := range expected {
		actualVal, exists := pred.Outputs[key]
		if !exists {
			t.Errorf("Field %q not found in outputs", key)
			continue
		}

		if actualVal != expectedVal {
			t.Errorf("Field %q: expected %v, got %v", key, expectedVal, actualVal)
		}
	}
}

// AssertUsageTracked validates that usage information was captured
func AssertUsageTracked(t *testing.T, pred *dsgo.Prediction, minTokens, maxTokens int) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	totalTokens := pred.Usage.TotalTokens
	if totalTokens < minTokens {
		t.Errorf("TotalTokens %d is less than minimum %d", totalTokens, minTokens)
	}

	if totalTokens > maxTokens {
		t.Errorf("TotalTokens %d exceeds maximum %d", totalTokens, maxTokens)
	}

	if pred.Usage.Cost <= 0 {
		t.Errorf("Cost is not calculated or is negative (%.6f)", pred.Usage.Cost)
	}

	if pred.Usage.Latency <= 0 {
		t.Errorf("Latency is not recorded or is negative (%d ms)", pred.Usage.Latency)
	}
}

// AssertCostCalculated validates that cost was properly calculated
func AssertCostCalculated(t *testing.T, pred *dsgo.Prediction, minCost, maxCost float64) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	cost := pred.Usage.Cost
	if cost < minCost || cost > maxCost {
		t.Errorf("Cost %f is outside range [%f, %f]", cost, minCost, maxCost)
	}

	if cost <= 0 {
		t.Errorf("Cost should be positive, got %f", cost)
	}
}

// AssertHistoryCollected validates that history was collected
func AssertHistoryCollected(t *testing.T, entries []*dsgo.HistoryEntry, expectedCount int) {
	t.Helper()

	if len(entries) != expectedCount {
		t.Errorf("Expected %d history entries, got %d", expectedCount, len(entries))
	}
}

// AssertHistoryComplete validates that a history entry has complete metadata
func AssertHistoryComplete(t *testing.T, entry *dsgo.HistoryEntry, model, provider string) {
	t.Helper()

	if entry == nil {
		t.Fatal("HistoryEntry is nil")
	}

	if entry.Model != model {
		t.Errorf("Model: expected %q, got %q", model, entry.Model)
	}

	if entry.Provider != provider {
		t.Errorf("Provider: expected %q, got %q", provider, entry.Provider)
	}

	if entry.Usage.PromptTokens <= 0 {
		t.Error("PromptTokens should be positive")
	}

	if entry.Usage.CompletionTokens <= 0 {
		t.Error("CompletionTokens should be positive")
	}

	if entry.Usage.Cost <= 0 {
		t.Error("Cost should be positive")
	}

	if entry.ID == "" {
		t.Error("ID is empty")
	}

	if entry.Timestamp.IsZero() {
		t.Error("Timestamp is not set")
	}
}

// AssertCompositionCost validates that composition cost is within expected range
func AssertCompositionCost(t *testing.T, results []*dsgo.Prediction, maxTotalCost float64) {
	t.Helper()

	totalCost := 0.0
	for _, r := range results {
		if r == nil {
			t.Fatal("Prediction is nil")
		}
		totalCost += r.Usage.Cost
	}

	if totalCost > maxTotalCost {
		t.Errorf("Total cost %f exceeds maximum %f", totalCost, maxTotalCost)
	}
}

// AssertAllModulesExecuted validates that all expected modules executed
func AssertAllModulesExecuted(t *testing.T, entries []*dsgo.HistoryEntry, expectedModuleCount int) {
	t.Helper()

	if len(entries) != expectedModuleCount {
		t.Errorf("Expected %d module executions, got %d", expectedModuleCount, len(entries))
	}

	for i, entry := range entries {
		if entry.Model == "" {
			t.Errorf("Entry %d: Model is empty", i)
		}
		if entry.Provider == "" {
			t.Errorf("Entry %d: Provider is empty", i)
		}
		if entry.Usage.TotalTokens <= 0 {
			t.Errorf("Entry %d: Usage tokens not tracked", i)
		}
	}
}

// AssertParseError validates parsing-related errors
func AssertParseError(t *testing.T, pred *dsgo.Prediction, shouldHaveErrors bool) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	hasDiagnostics := pred.ParseDiagnostics != nil &&
		(len(pred.ParseDiagnostics.MissingFields) > 0 ||
			len(pred.ParseDiagnostics.TypeErrors) > 0)

	if hasDiagnostics != shouldHaveErrors {
		t.Errorf("ParseDiagnostics: expected errors=%v, got %v", shouldHaveErrors, hasDiagnostics)
	}
}

// AssertAdapterUsed validates which adapter was used
func AssertAdapterUsed(t *testing.T, pred *dsgo.Prediction, expectedAdapter string) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	if pred.AdapterUsed != expectedAdapter {
		t.Errorf("AdapterUsed: expected %q, got %q", expectedAdapter, pred.AdapterUsed)
	}
}

// AssertFallbackUsed validates that fallback adapter was used
func AssertFallbackUsed(t *testing.T, pred *dsgo.Prediction, shouldUseFallback bool) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	if pred.FallbackUsed != shouldUseFallback {
		t.Errorf("FallbackUsed: expected %v, got %v", shouldUseFallback, pred.FallbackUsed)
	}
}

// AssertFieldValue safely retrieves and validates a field value
func AssertFieldValue(t *testing.T, pred *dsgo.Prediction, fieldName string, expectedValue interface{}) {
	t.Helper()

	if pred == nil || pred.Outputs == nil {
		t.Fatal("Prediction or Outputs is nil")
	}

	actualValue, exists := pred.Outputs[fieldName]
	if !exists {
		t.Errorf("Field %q not found in outputs", fieldName)
		return
	}

	if actualValue != expectedValue {
		t.Errorf("Field %q: expected %v (%T), got %v (%T)", fieldName, expectedValue, expectedValue, actualValue, actualValue)
	}
}

// AssertOutputFieldExists validates that a field exists in outputs
func AssertOutputFieldExists(t *testing.T, pred *dsgo.Prediction, fieldName string) {
	t.Helper()

	if pred == nil || pred.Outputs == nil {
		t.Fatal("Prediction or Outputs is nil")
	}

	if _, exists := pred.Outputs[fieldName]; !exists {
		availableFields := make([]string, 0, len(pred.Outputs))
		for k := range pred.Outputs {
			availableFields = append(availableFields, k)
		}
		t.Errorf("Field %q not found. Available fields: %v", fieldName, availableFields)
	}
}

// AssertModuleName validates the module name in prediction
func AssertModuleName(t *testing.T, pred *dsgo.Prediction, expectedModuleName string) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	if pred.ModuleName != expectedModuleName {
		t.Errorf("ModuleName: expected %q, got %q", expectedModuleName, pred.ModuleName)
	}
}

// AssertRationale validates that reasoning trace exists
func AssertRationale(t *testing.T, pred *dsgo.Prediction, shouldExist bool) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	hasRationale := pred.Rationale != ""
	if hasRationale != shouldExist {
		t.Errorf("Rationale: expected to exist=%v, got %v", shouldExist, hasRationale)
	}
}

// AssertCompletions validates alternative completions
func AssertCompletions(t *testing.T, pred *dsgo.Prediction, expectedCount int) {
	t.Helper()

	if pred == nil {
		t.Fatal("Prediction is nil")
	}

	if len(pred.Completions) != expectedCount {
		t.Errorf("Completions: expected %d, got %d", expectedCount, len(pred.Completions))
	}
}

// Helper functions

// getKeys returns all keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// stringContains checks if a string contains a substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
