package util

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
)

func TestFilterChatCompletionProviderParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    map[string]any{},
			expected: nil,
		},
		{
			name: "all blocked keys",
			input: map[string]any{
				"model":                 "gpt-4",
				"messages":              []any{},
				"temperature":           0.7,
				"top_p":                 0.9,
				"stop":                  []string{"END"},
				"max_tokens":            1000,
				"max_completion_tokens": 500,
				"response_format":       map[string]any{"type": "json"},
				"frequency_penalty":     0.5,
				"presence_penalty":      0.3,
				"tools":                 []any{},
				"tool_choice":           "auto",
				"n":                     5,
				"stream":                true,
				"stream_options":        map[string]any{},
				"logprobs":              true,
				"top_logprobs":          5,
			},
			expected: map[string]any{},
		},
		{
			name: "all allowed keys",
			input: map[string]any{
				"seed":     42,
				"top_k":    50,
				"metadata": map[string]any{"user": "test"},
				"reasoning": map[string]any{
					"effort": "high",
				},
			},
			expected: map[string]any{
				"seed":     42,
				"top_k":    50,
				"metadata": map[string]any{"user": "test"},
				"reasoning": map[string]any{
					"effort": "high",
				},
			},
		},
		{
			name: "mixed blocked and allowed keys",
			input: map[string]any{
				"temperature": 0.9,
				"model":       "evil-model",
				"n":           10,
				"stream":      true,
				"seed":        123,
				"top_k":       40,
				"reasoning": map[string]any{
					"effort": "medium",
				},
			},
			expected: map[string]any{
				"seed":  123,
				"top_k": 40,
				"reasoning": map[string]any{
					"effort": "medium",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := filterChatCompletionProviderParams(tt.input)

			if tt.expected == nil {
				if len(result) > 0 {
					t.Errorf("expected nil or empty, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for key, expectedVal := range tt.expected {
				if result[key] == nil {
					t.Errorf("expected key %q to be present", key)
					continue
				}
				switch ev := expectedVal.(type) {
				case int:
					if result[key] != ev {
						t.Errorf("key %q: expected %v, got %v", key, ev, result[key])
					}
				case map[string]any:
					rv, ok := result[key].(map[string]any)
					if !ok {
						t.Errorf("key %q: expected map, got %T", key, result[key])
						continue
					}
					for k, v := range ev {
						if rv[k] != v {
							t.Errorf("key %q.%s: expected %v, got %v", key, k, v, rv[k])
						}
					}
				}
			}
		})
	}
}

func TestApplyChatCompletionProviderParams(t *testing.T) {
	t.Parallel()

	t.Run("applies allowed params", func(t *testing.T) {
		t.Parallel()
		params := openai.ChatCompletionNewParams{}

		ApplyChatCompletionProviderParams(&params, map[string]any{
			"seed":  42,
			"top_k": 50,
		})

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)

		if seed, ok := m["seed"].(float64); !ok || int(seed) != 42 {
			t.Errorf("expected seed 42, got %v", m["seed"])
		}
		if topK, ok := m["top_k"].(float64); !ok || int(topK) != 50 {
			t.Errorf("expected top_k 50, got %v", m["top_k"])
		}
	})

	t.Run("does not apply blocked params", func(t *testing.T) {
		t.Parallel()
		params := openai.ChatCompletionNewParams{}

		ApplyChatCompletionProviderParams(&params, map[string]any{
			"temperature": 0.9,
			"n":           5,
		})

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)

		if _, ok := m["temperature"]; ok {
			t.Errorf("expected temperature to be blocked, got %v", m["temperature"])
		}
		if _, ok := m["n"]; ok {
			t.Errorf("expected n to be blocked, got %v", m["n"])
		}
	})

	t.Run("empty params is no-op", func(t *testing.T) {
		t.Parallel()
		params := openai.ChatCompletionNewParams{}

		ApplyChatCompletionProviderParams(&params, nil)

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)

		if m["seed"] != nil || m["top_k"] != nil {
			t.Errorf("expected no extra fields to be set")
		}
	})
}
