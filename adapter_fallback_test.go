package dsgo

import (
	"strings"
	"testing"
)

// TestFallbackAdapter_DefaultChain tests the default adapter chain
func TestFallbackAdapter_DefaultChain(t *testing.T) {
	adapter := NewFallbackAdapter()

	// Default should be ChatAdapter → JSONAdapter
	if len(adapter.adapters) != 2 {
		t.Errorf("Expected 2 adapters in default chain, got %d", len(adapter.adapters))
	}

	// Verify types
	if _, ok := adapter.adapters[0].(*ChatAdapter); !ok {
		t.Errorf("Expected first adapter to be ChatAdapter, got %T", adapter.adapters[0])
	}
	if _, ok := adapter.adapters[1].(*JSONAdapter); !ok {
		t.Errorf("Expected second adapter to be JSONAdapter, got %T", adapter.adapters[1])
	}
}

// TestFallbackAdapter_ParseChatSuccess tests successful parsing with ChatAdapter
func TestFallbackAdapter_ParseChatSuccess(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response with field markers (ChatAdapter format)
	content := "[[ ## answer ## ]]\n42"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if answer, ok := outputs["answer"].(string); !ok || answer != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used first adapter (ChatAdapter)
	if adapter.GetLastUsedAdapter() != 0 {
		t.Errorf("Expected adapter 0 to be used, got %d", adapter.GetLastUsedAdapter())
	}

	// Check metadata
	if outputs["__parse_attempts"] != 1 {
		t.Errorf("Expected 1 parse attempt, got %v", outputs["__parse_attempts"])
	}
	if outputs["__fallback_used"] != false {
		t.Errorf("Expected fallback_used=false, got %v", outputs["__fallback_used"])
	}
}

// TestFallbackAdapter_ParseFallbackToJSON tests fallback to JSONAdapter
func TestFallbackAdapter_ParseFallbackToJSON(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response in JSON format (no field markers, ChatAdapter will fail)
	content := `{"answer": "42"}`

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if answer, ok := outputs["answer"].(string); !ok || answer != "42" {
		t.Errorf("Expected answer='42', got %v", outputs["answer"])
	}

	// Should have used second adapter (JSONAdapter)
	if adapter.GetLastUsedAdapter() != 1 {
		t.Errorf("Expected adapter 1 to be used, got %d", adapter.GetLastUsedAdapter())
	}

	// Check metadata - fallback was used
	if outputs["__parse_attempts"] != 2 {
		t.Errorf("Expected 2 parse attempts, got %v", outputs["__parse_attempts"])
	}
	if outputs["__fallback_used"] != true {
		t.Errorf("Expected fallback_used=true, got %v", outputs["__fallback_used"])
	}
}

// TestFallbackAdapter_ParseAllFail tests when all adapters fail
func TestFallbackAdapter_ParseAllFail(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "")

	// Response that neither adapter can parse
	content := "This is just plain text with no structure"

	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Fatal("Expected parse to fail when all adapters fail")
	}

	// Error should mention all adapters
	if !strings.Contains(err.Error(), "all adapters failed") {
		t.Errorf("Expected error about all adapters failing, got: %v", err)
	}
}

// TestFallbackAdapter_WithReasoning tests reasoning propagation to all adapters
func TestFallbackAdapter_WithReasoning(t *testing.T) {
	adapter := NewFallbackAdapter().WithReasoning(true)

	// Verify all adapters have reasoning enabled
	for i, a := range adapter.adapters {
		switch typed := a.(type) {
		case *ChatAdapter:
			if !typed.IncludeReasoning {
				t.Errorf("Adapter %d (ChatAdapter) should have reasoning enabled", i)
			}
		case *JSONAdapter:
			if !typed.IncludeReasoning {
				t.Errorf("Adapter %d (JSONAdapter) should have reasoning enabled", i)
			}
		}
	}
}

// TestFallbackAdapter_ParseSuccessRate tests parse success across diverse inputs
// Target: >95% success rate
func TestFallbackAdapter_ParseSuccessRate(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("answer", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	testCases := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "Clean field markers",
			content: "[[ ## answer ## ]]\nyes\n[[ ## confidence ## ]]\n0.9",
			wantErr: false,
		},
		{
			name:    "Clean JSON",
			content: `{"answer": "yes", "confidence": 0.9}`,
			wantErr: false,
		},
		{
			name:    "JSON in markdown",
			content: "```json\n{\"answer\": \"yes\", \"confidence\": 0.9}\n```",
			wantErr: false,
		},
		{
			name:    "Field markers with extra text",
			content: "Let me think...\n[[ ## answer ## ]]\nyes\n\n[[ ## confidence ## ]]\n0.9",
			wantErr: false,
		},
		{
			name:    "Loose field markers",
			content: "[[## answer ##]]\nyes\n[[## confidence ##]]\n0.9",
			wantErr: false,
		},
		{
			name:    "JSON with reasoning",
			content: "Let me analyze... ```json\n{\"answer\": \"yes\", \"confidence\": 0.9}\n```",
			wantErr: false,
		},
	}

	successCount := 0
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tc.content)
			if (err != nil) != tc.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				successCount++

				// Verify outputs exist
				if _, ok := outputs["answer"]; !ok {
					t.Error("Missing 'answer' in outputs")
				}
				if _, ok := outputs["confidence"]; !ok {
					t.Error("Missing 'confidence' in outputs")
				}
			}
		})
	}

	// Calculate success rate
	successRate := float64(successCount) / float64(len(testCases)) * 100
	t.Logf("Parse success rate: %.1f%% (%d/%d)", successRate, successCount, len(testCases))

	// Target: >95% success rate
	if successRate < 95.0 {
		t.Errorf("Parse success rate %.1f%% is below target 95%%", successRate)
	}
}

// TestFallbackAdapter_TypeCoercionConsistency tests type coercion across adapters
func TestFallbackAdapter_TypeCoercionConsistency(t *testing.T) {
	adapter := NewFallbackAdapter()
	sig := NewSignature("test").
		AddOutput("count", FieldTypeInt, "").
		AddOutput("score", FieldTypeFloat, "")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "ChatAdapter format",
			content: "[[ ## count ## ]]\n42\n\n[[ ## score ## ]]\n0.95",
		},
		{
			name:    "JSONAdapter format",
			content: `{"count": "42", "score": "0.95"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Both should coerce to proper types
			if count, ok := outputs["count"].(int); !ok || count != 42 {
				t.Errorf("Expected count=42 (int), got %v (%T)", outputs["count"], outputs["count"])
			}
			if score, ok := outputs["score"].(float64); !ok || score != 0.95 {
				t.Errorf("Expected score=0.95 (float64), got %v (%T)", outputs["score"], outputs["score"])
			}
		})
	}
}
