package yamlprogram

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/core"
)

func builtinTool(name string) (*core.Tool, error) {
	switch name {
	case "current_datetime":
		return core.NewTool("current_datetime", "Get the current date and time", func(ctx context.Context, args map[string]any) (any, error) {
			return map[string]any{
				"datetime": time.Now().Format(time.RFC3339),
			}, nil
		}).AddParameter("format", "string", "Optional time format (RFC3339 by default)", false), nil

	case "calculate":
		return core.NewTool("calculate", "Evaluate a simple arithmetic expression", func(ctx context.Context, args map[string]any) (any, error) {
			expr, _ := args["expression"].(string)
			expr = strings.TrimSpace(expr)
			if expr == "" {
				return nil, fmt.Errorf("expression is required")
			}
			// Very small evaluator: supports + - * / with floats using Go's parser would require deps.
			// We implement a minimal two-operand parser for demo purposes.
			fields := strings.Fields(expr)
			if len(fields) != 3 {
				return nil, fmt.Errorf("unsupported expression %q: expected 'a op b'", expr)
			}
			a, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid lhs: %w", err)
			}
			op := fields[1]
			b, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid rhs: %w", err)
			}
			var res float64
			switch op {
			case "+":
				res = a + b
			case "-":
				res = a - b
			case "*":
				res = a * b
			case "/":
				if b == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				res = a / b
			default:
				return nil, fmt.Errorf("unsupported operator %q", op)
			}
			return map[string]any{"result": res}, nil
		}).AddParameter("expression", "string", "Expression like '2 + 2'", true), nil

	case "random_number":
		return core.NewTool("random_number", "Generate a random number in range", func(ctx context.Context, args map[string]any) (any, error) {
			min := int64(0)
			max := int64(100)
			if v, ok := args["min"].(float64); ok {
				min = int64(v)
			}
			if v, ok := args["max"].(float64); ok {
				max = int64(v)
			}
			if max < min {
				return nil, fmt.Errorf("max must be >= min")
			}
			span := big.NewInt(max - min + 1)
			n, err := rand.Int(rand.Reader, span)
			if err != nil {
				return nil, fmt.Errorf("rand: %w", err)
			}
			return map[string]any{"number": min + n.Int64()}, nil
		}).AddParameter("min", "number", "Minimum (inclusive)", false).
			AddParameter("max", "number", "Maximum (inclusive)", false), nil

	case "string_length":
		return core.NewTool("string_length", "Get length of a string", func(ctx context.Context, args map[string]any) (any, error) {
			text, _ := args["text"].(string)
			return map[string]any{"length": len(text)}, nil
		}).AddParameter("text", "string", "Text to measure", true), nil

	case "word_count":
		return core.NewTool("word_count", "Count words in text", func(ctx context.Context, args map[string]any) (any, error) {
			text, _ := args["text"].(string)
			words := strings.Fields(text)
			return map[string]any{"words": len(words)}, nil
		}).AddParameter("text", "string", "Text to count", true), nil

	case "environment_info":
		return core.NewTool("environment_info", "Return environment/runtime info", func(ctx context.Context, args map[string]any) (any, error) {
			info := map[string]any{
				"goos":   runtime.GOOS,
				"goarch": runtime.GOARCH,
				"pwd":    mustGetwd(),
				"env": map[string]any{
					"DSGO_LOG": os.Getenv("DSGO_LOG"),
				},
			}
			b, _ := json.Marshal(info)
			var out map[string]any
			_ = json.Unmarshal(b, &out)
			return out, nil
		}), nil
	default:
		return nil, fmt.Errorf("unknown builtin tool %q", name)
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}
