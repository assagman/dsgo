package module

import (
	"context"
	"errors"
	"strings"

	"github.com/assagman/dsgo/internal/core"
)

const reactContextOverflowMaxRetries = 3

// contextLengthSentinel is an internal error marker.
// It is not used by providers directly but allows tests to simulate overflow.
type contextLengthSentinel struct{}

func (contextLengthSentinel) Error() string { return "context length exceeded" }

// generateWithContextRetry calls the LM and retries on context overflow errors by
// truncating the oldest trajectory steps (DSPy-style) and retrying.
func (r *ReAct) generateWithContextRetry(ctx context.Context, traj *reactTrajectory, options *core.GenerateOptions, extra []core.Message) (*core.GenerateResult, error) {
	var lastErr error
	for attempt := 0; attempt < reactContextOverflowMaxRetries; attempt++ {
		messages := traj.Render(r.maxPromptBytes())
		if len(extra) > 0 {
			messages = append(messages, extra...)
		}

		result, err := r.LM.Generate(ctx, messages, options)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isContextOverflowError(err) {
			return nil, err
		}
		if traj.DropOldestSteps(1) == 0 {
			return nil, err
		}
	}

	if lastErr == nil {
		lastErr = errors.New("context overflow retry exhausted")
	}
	return nil, lastErr
}

// isContextOverflowError attempts to detect provider context window exceeded errors.
// This is heuristic by design because different providers surface different error types.
func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, contextLengthSentinel{}) {
		return true
	}

	msg := strings.ToLower(err.Error())
	patterns := []string{
		"context_length_exceeded",
		"maximum context length",
		"max context length",
		"maximum context window",
		"context window",
		"too many tokens",
		"exceeded the context",
		"please reduce the length of the messages",
		"prompt is too long",
		"tokens exceeded",
		"input is too long",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
