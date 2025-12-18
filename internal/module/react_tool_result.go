package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

type reactToolResult struct {
	ToolCallID string
	ToolName   string
	Content    string
	Truncated  bool
	Err        error
}

type toolResultEnvelope struct {
	Tool         string `json:"tool"`
	ToolCallID   string `json:"tool_call_id,omitempty"`
	OK           bool   `json:"ok"`
	Result       any    `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	OriginalSize int    `json:"original_bytes,omitempty"`
}

// encodeToolResult encodes tool outputs as a JSON envelope and truncates deterministically
// to maxBytes, ensuring the resulting string is valid JSON.
//
// It also returns a stable hash of the observation content that excludes per-call identifiers
// (like tool_call_id), so termination policies can detect repeated observations.
func encodeToolResult(toolName, toolCallID string, value any, toolErr error, maxBytes int) (content string, truncated bool, stableHash string) {
	if maxBytes <= 0 {
		maxBytes = defaultReActMaxToolResultBytes
	}

	env := toolResultEnvelope{
		Tool:       toolName,
		ToolCallID: toolCallID,
		OK:         toolErr == nil,
	}
	if toolErr != nil {
		env.Error = toolErr.Error()
	} else {
		env.Result = value
	}

	data, err := json.Marshal(env)
	if err != nil {
		fallback := toolResultEnvelope{Tool: toolName, ToolCallID: toolCallID, OK: false, Error: fmt.Sprintf("marshal error: %v", err)}
		fallbackBytes, _ := json.Marshal(fallback)
		stable := stableToolEnvelope(toolName, toolErr == nil, nil, fallback.Error, false)
		return string(fallbackBytes), false, stableHashJSON(stable)
	}
	if len(data) <= maxBytes {
		stable := stableToolEnvelope(toolName, toolErr == nil, value, env.Error, false)
		return string(data), false, stableHashJSON(stable)
	}

	origSize := len(data)

	// Truncation strategy: represent the tool result as a JSON string excerpt.
	var excerptBytes []byte
	if toolErr != nil {
		excerptBytes = []byte(toolErr.Error())
	} else {
		if b, err := json.Marshal(value); err == nil {
			excerptBytes = b
		} else {
			excerptBytes = []byte(fmt.Sprintf("%v", value))
		}
	}

	trunc := toolResultEnvelope{
		Tool:         toolName,
		ToolCallID:   toolCallID,
		OK:           toolErr == nil,
		Truncated:    true,
		OriginalSize: origSize,
	}
	if toolErr != nil {
		trunc.Error = toolErr.Error()
	}

	// Binary search the maximum excerpt prefix that still fits.
	low, high := 0, len(excerptBytes)
	best := 0
	for low <= high {
		mid := (low + high) / 2
		trunc.Result = truncateUTF8Bytes(excerptBytes, mid)
		b, _ := json.Marshal(trunc)
		if len(b) <= maxBytes {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}

	trunc.Result = truncateUTF8Bytes(excerptBytes, best)
	b, _ := json.Marshal(trunc)
	if len(b) <= maxBytes {
		stable := stableToolEnvelope(toolName, toolErr == nil, trunc.Result, trunc.Error, true)
		return string(b), true, stableHashJSON(stable)
	}

	// If still too large (extreme maxBytes), fall back to a minimal envelope.
	minimal := toolResultEnvelope{
		Tool:       toolName,
		ToolCallID: toolCallID,
		OK:         toolErr == nil,
		Truncated:  true,
	}
	minimalBytes, _ := json.Marshal(minimal)
	if len(minimalBytes) <= maxBytes {
		stable := stableToolEnvelope(toolName, toolErr == nil, nil, "", true)
		return string(minimalBytes), true, stableHashJSON(stable)
	}

	// Last resort: produce valid JSON even if it exceeds budget.
	stable := stableToolEnvelope(toolName, toolErr == nil, nil, "", true)
	return string(minimalBytes), true, stableHashJSON(stable)
}

type stableToolResultEnvelope struct {
	Tool      string `json:"tool"`
	OK        bool   `json:"ok"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

func stableToolEnvelope(tool string, ok bool, result any, errMsg string, truncated bool) stableToolResultEnvelope {
	env := stableToolResultEnvelope{Tool: tool, OK: ok, Truncated: truncated}
	if !ok {
		env.Error = errMsg
	} else {
		env.Result = result
	}
	return env
}

func stableHashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Fall back to hashing the error string to remain stable.
		h := sha256.Sum256([]byte(fmt.Sprintf("marshal error: %v", err)))
		return hex.EncodeToString(h[:])
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func truncateUTF8Bytes(b []byte, n int) string {
	if n <= 0 {
		return ""
	}
	if n >= len(b) {
		return string(b)
	}
	cut := b[:n]
	for len(cut) > 0 && !utf8.Valid(cut) {
		cut = cut[:len(cut)-1]
	}
	return string(cut)
}
