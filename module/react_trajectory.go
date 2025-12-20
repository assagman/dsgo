package module

import (
	"encoding/json"

	"github.com/assagman/dsgo/core"
)

// reactTrajectory stores ReAct loop state as structured steps and can render them
// into a provider-friendly []core.Message under a soft budget.
//
// Budgeting is approximate (byte-based) and intended for deterministic truncation
// and overflow recovery.
type reactTrajectory struct {
	base  []core.Message
	steps []*reactStep
}

type reactStep struct {
	Thought     string
	ToolCalls   []core.ToolCall
	ToolResults []reactToolResult
	Errors      []string
}

func newReActTrajectory(base []core.Message) *reactTrajectory {
	cloned := make([]core.Message, len(base))
	copy(cloned, base)
	return &reactTrajectory{base: cloned, steps: []*reactStep{}}
}

func (t *reactTrajectory) AddStep(thought string, toolCalls []core.ToolCall) *reactStep {
	step := &reactStep{Thought: thought}
	if toolCalls != nil {
		step.ToolCalls = make([]core.ToolCall, len(toolCalls))
		copy(step.ToolCalls, toolCalls)
	}
	t.steps = append(t.steps, step)
	return step
}

func (s *reactStep) AddToolResult(res reactToolResult) {
	s.ToolResults = append(s.ToolResults, res)
}

func (t *reactTrajectory) DropOldestSteps(n int) int {
	if n <= 0 || len(t.steps) == 0 {
		return 0
	}
	if n >= len(t.steps) {
		dropped := len(t.steps)
		t.steps = nil
		return dropped
	}
	t.steps = t.steps[n:]
	return n
}

func (t *reactTrajectory) HasToolContent() bool {
	for _, s := range t.steps {
		if len(s.ToolCalls) > 0 || len(s.ToolResults) > 0 {
			return true
		}
	}
	return false
}

func (t *reactTrajectory) Render(budgetBytes int) []core.Message {
	if budgetBytes <= 0 {
		budgetBytes = defaultReActMaxPromptBytes
	}

	base := make([]core.Message, len(t.base))
	copy(base, t.base)
	if len(t.steps) == 0 {
		return base
	}

	baseBytes := approxMessagesBytes(base)
	remaining := budgetBytes - baseBytes
	if remaining <= 0 {
		return base
	}

	// Select suffix of steps that fits within remaining.
	includeFrom := len(t.steps) - 1
	used := 0
	for i := len(t.steps) - 1; i >= 0; i-- {
		stepBytes := t.steps[i].approxBytes()
		// Always include the newest step even if it exceeds the budget.
		if used+stepBytes > remaining && i != len(t.steps)-1 {
			break
		}
		includeFrom = i
		used += stepBytes
	}

	msgs := append([]core.Message{}, base...)
	for _, s := range t.steps[includeFrom:] {
		msgs = append(msgs, s.toMessages()...)
	}
	return msgs
}

func (s *reactStep) toMessages() []core.Message {
	msgs := []core.Message{}
	if s.Thought != "" || len(s.ToolCalls) > 0 {
		msgs = append(msgs, core.Message{Role: "assistant", Content: s.Thought, ToolCalls: s.ToolCalls})
	}
	for _, tr := range s.ToolResults {
		msgs = append(msgs, core.Message{Role: "tool", Content: tr.Content, ToolID: tr.ToolCallID})
	}
	for _, e := range s.Errors {
		msgs = append(msgs, core.Message{Role: "system", Content: e})
	}
	return msgs
}

func (s *reactStep) approxBytes() int {
	b := len(s.Thought)
	if len(s.ToolCalls) > 0 {
		if data, err := json.Marshal(s.ToolCalls); err == nil {
			b += len(data)
		}
	}
	for _, tr := range s.ToolResults {
		b += len(tr.Content)
	}
	for _, e := range s.Errors {
		b += len(e)
	}
	return b
}

func approxMessagesBytes(msgs []core.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
		if len(m.ToolCalls) > 0 {
			if data, err := json.Marshal(m.ToolCalls); err == nil {
				total += len(data)
			}
		}
	}
	return total
}
