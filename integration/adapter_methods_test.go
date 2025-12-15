package integration

import (
	"context"
	"testing"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/internal/providers/mock"
)

// newScriptedMockLM creates a mock LM with a specific transport script.
// It ensures the transport is reset after the test.
func newScriptedMockLM(t *testing.T, steps ...mock.HTTPResponseStep) dsgo.LM {
	t.Helper()
	reset := mock.SetHTTPTransport(mock.NewScriptedTransport(steps...))
	t.Cleanup(reset)
	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o-mini")
	if err != nil {
		t.Fatalf("Failed to create mock LM: %v", err)
	}
	return lm
}

func TestAdapters_FormatAndParse_Smoke(t *testing.T) {
	// Use multi-field signature for fallback test to prevent JSONAdapter's single-field raw text fallback
	multiFieldSig := fixtures.ChainOfThoughtSig() // Has "reasoning" and "answer" fields

	tests := []struct {
		name          string
		sig           *dsgo.Signature
		inputs        map[string]any
		adapter       dsgo.Adapter
		response      string
		expectAnswer  string
		expectMetrics map[string]any // Optional expectation for __* metrics in parsed output
	}{
		{
			name:         "JSONAdapter",
			sig:          fixtures.SimplePredictSig(),
			inputs:       map[string]any{"question": "test"},
			adapter:      dsgo.NewJSONAdapter(),
			response:     `{"answer": "ok"}`,
			expectAnswer: "ok",
		},
		{
			name:         "ChatAdapter",
			sig:          fixtures.SimplePredictSig(),
			inputs:       map[string]any{"question": "test"},
			adapter:      dsgo.NewChatAdapter(),
			response:     "[[ ## answer ## ]]\nok",
			expectAnswer: "ok",
		},
		{
			name:         "FallbackAdapter",
			sig:          multiFieldSig,                                                        // Multi-field so JSONAdapter can't use raw text fallback
			inputs:       map[string]any{"problem": "test"},                                    // ChainOfThoughtSig uses "problem" as input
			adapter:      dsgo.NewFallbackAdapter(),                                            // Defaults to JSON -> Chat
			response:     "[[ ## reasoning ## ]]\nthinking\n\n[[ ## answer ## ]]\nfallback ok", // JSON fails, Chat succeeds
			expectAnswer: "fallback ok",
			expectMetrics: map[string]any{
				"__adapter_used":   "adapter.ChatAdapter",
				"__parse_attempts": 2, // 1st failed, 2nd succeeded
				"__fallback_used":  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Test Format
			msgs, err := tt.adapter.Format(tt.sig, tt.inputs, nil)
			if err != nil {
				t.Fatalf("Format failed: %v", err)
			}
			if len(msgs) == 0 {
				t.Error("Format returned no messages")
			}

			// 2. Test Parse
			outputs, err := tt.adapter.Parse(tt.sig, tt.response)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if got := outputs["answer"]; got != tt.expectAnswer {
				t.Errorf("Expected answer %q, got %q", tt.expectAnswer, got)
			}

			// 3. Check metrics if expected (only present at adapter.Parse level)
			if tt.expectMetrics != nil {
				for k, v := range tt.expectMetrics {
					if got := outputs[k]; got != v {
						t.Errorf("Expected %s = %v, got %v", k, v, got)
					}
				}
			}
		})
	}
}

func TestPredict_RecordsAdapterMetrics_NotInOutputs(t *testing.T) {
	// This test verifies that Predict module extracts adapter metrics from outputs
	// and puts them into the Prediction struct, removing them from Outputs map.
	ctx := context.Background()
	// Use multi-field signature to prevent JSONAdapter's single-field raw text fallback
	sig := fixtures.ChainOfThoughtSig() // Has "reasoning" and "answer" fields

	// Setup a response that requires fallback (Chat format, but default chain starts with JSONAdapter)
	// The response uses Chat markers, so JSON fails and Chat succeeds
	lm := newScriptedMockLM(t, mock.HTTPResponseStep{
		Body: fixtures.OpenAIChatCompletionJSON("[[ ## reasoning ## ]]\nthinking\n\n[[ ## answer ## ]]\nfallback success", "stop", 10, 5),
	})

	// Use FallbackAdapter explicitly
	pred := dsgo.NewPredict(sig, lm).WithAdapter(dsgo.NewFallbackAdapter())

	result, err := pred.Forward(ctx, map[string]any{"problem": "test"})
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// 1. Check metrics on Prediction struct
	if result.AdapterUsed != "adapter.ChatAdapter" {
		t.Errorf("Expected AdapterUsed='adapter.ChatAdapter', got %q", result.AdapterUsed)
	}
	if !result.FallbackUsed {
		t.Error("Expected FallbackUsed=true")
	}
	if result.ParseAttempts != 2 {
		t.Errorf("Expected ParseAttempts=2, got %d", result.ParseAttempts)
	}

	// 2. Check metrics are NOT in Outputs
	forbiddenKeys := []string{"__adapter_used", "__fallback_used", "__parse_attempts"}
	for _, k := range forbiddenKeys {
		if _, exists := result.Outputs[k]; exists {
			t.Errorf("Key %q should have been removed from Outputs", k)
		}
	}

	// 3. Check value
	if got, _ := result.GetString("answer"); got != "fallback success" {
		t.Errorf("Expected answer='fallback success', got %q", got)
	}
}

func TestJSONAdapter_SingleStringOutput_FallbackToRawContent(t *testing.T) {
	// Tests that JSONAdapter falls back to raw content if JSON extraction fails
	// BUT ONLY IF the signature has exactly one output field of type string.
	sig := fixtures.SimplePredictSig() // 1 output: "answer" (string)
	adapter := dsgo.NewJSONAdapter()

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "Raw string fallback",
			content: "just plain text",
			want:    "just plain text",
			wantErr: false,
		},
		{
			name:    "Empty string",
			content: "",
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs, err := adapter.Parse(sig, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if got := outputs["answer"]; got != tt.want {
					t.Errorf("Expected answer %q, got %q", tt.want, got)
				}
			}
		})
	}
}

func TestJSONAdapter_NoFallback_MultiOutput_ErrorsOnNoJSON(t *testing.T) {
	// Use a signature with multiple outputs, so single-string fallback doesn't apply.
	sig := fixtures.ComplexOutputSig() // has result, confidence, count, valid
	adapter := dsgo.NewJSONAdapter()

	content := "just plain text, no json here"
	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Fatal("Expected error when parsing non-JSON for multi-output signature, got nil")
	}
}

func TestChatAdapter_RequiresMarker_WhenNoHeuristicMatch(t *testing.T) {
	sig := fixtures.SimplePredictSig()
	adapter := dsgo.NewChatAdapter()

	// "hello" contains no "answer:" prefix (heuristic) and no markers.
	content := "hello"
	_, err := adapter.Parse(sig, content)
	if err == nil {
		t.Fatal("Expected error when parsing content without markers or heuristics, got nil")
	}
}
