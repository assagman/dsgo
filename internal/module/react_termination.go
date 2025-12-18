package module

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/assagman/dsgo/internal/core"
)

type terminationReason string

const (
	terminationNone               terminationReason = ""
	terminationNoToolCalls        terminationReason = "no_tool_calls"
	terminationFinishTool         terminationReason = "finish_tool"
	terminationPlanningDone       terminationReason = "planning_done"
	terminationPlanningParseError terminationReason = "planning_parse_error"
	terminationRepeatedToolCall   terminationReason = "repeated_tool_call"
	terminationRepeatedErrors     terminationReason = "repeated_errors"
	terminationStagnation         terminationReason = "stagnation"
)

// reactTermination implements loop termination policies:
// - repeated tool+args fingerprint
// - repeated tool errors
// - repeated identical observations (stagnation)
//
// It does not inject prompts; it only signals that the loop should stop.
type reactTermination struct {
	done   bool
	reason terminationReason

	// Optional final candidates captured from the loop.
	finalToolArgs map[string]any
	finalContent  string

	lastToolFingerprint string
	repeatToolCalls     int

	lastObservationHash string
	repeatObservations  int

	consecutiveErrors int
}

func newReActTermination() *reactTermination {
	return &reactTermination{}
}

func (t *reactTermination) MarkDone(reason terminationReason) {
	if t.done {
		return
	}
	t.done = true
	t.reason = reason
}

func (t *reactTermination) ShouldStop() bool {
	return t.done
}

func (t *reactTermination) Reason() terminationReason {
	return t.reason
}

func (t *reactTermination) SetFinalToolArgs(args map[string]interface{}) {
	if args == nil {
		return
	}
	copied := make(map[string]any, len(args))
	for k, v := range args {
		copied[k] = v
	}
	t.finalToolArgs = copied
}

func (t *reactTermination) SetFinalContent(content string) {
	t.finalContent = content
}

func (t *reactTermination) FinalToolArgs() map[string]any {
	return t.finalToolArgs
}

func (t *reactTermination) FinalContent() string {
	return t.finalContent
}

func (t *reactTermination) ObserveToolCall(tc core.ToolCall) {
	fp := toolFingerprint(tc)
	if fp == "" {
		return
	}
	if fp == t.lastToolFingerprint {
		t.repeatToolCalls++
		if t.repeatToolCalls >= 2 {
			t.MarkDone(terminationRepeatedToolCall)
		}
		return
	}
	t.lastToolFingerprint = fp
	t.repeatToolCalls = 0
}

func (t *reactTermination) ObserveToolResult(tc core.ToolCall, observationHash string, err error) {
	if err != nil {
		t.consecutiveErrors++
		if t.consecutiveErrors >= 2 {
			t.MarkDone(terminationRepeatedErrors)
		}
	} else {
		t.consecutiveErrors = 0
	}

	hash := observationHash
	if hash == "" {
		h := sha256.Sum256([]byte(tc.Name))
		hash = hex.EncodeToString(h[:])
	}
	if hash == t.lastObservationHash {
		t.repeatObservations++
		if t.repeatObservations >= 2 {
			t.MarkDone(terminationStagnation)
		}
		return
	}
	t.lastObservationHash = hash
	t.repeatObservations = 0
}

func (t *reactTermination) ObserveError(err error) {
	if err == nil {
		return
	}
	t.consecutiveErrors++
	if t.consecutiveErrors >= 2 {
		t.MarkDone(terminationRepeatedErrors)
	}
}

func toolFingerprint(tc core.ToolCall) string {
	argsJSON, _ := json.Marshal(tc.Arguments)
	h := sha256.Sum256(argsJSON)
	return tc.Name + ":" + hex.EncodeToString(h[:])
}
