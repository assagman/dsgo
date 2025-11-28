package fixtures

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/assagman/dsgo/core"
)

// SearchTool creates a mock search tool
func SearchTool() *core.Tool {
	return core.NewTool(
		"search",
		"Search for information",
		func(ctx context.Context, args map[string]any) (any, error) {
			query, _ := args["query"].(string)
			return "Search results for '" + query + "': Found 5 relevant sources", nil
		},
	).AddParameter("query", "string", "Search query", true)
}

// CalculatorTool creates a mock calculator tool
func CalculatorTool() *core.Tool {
	return core.NewTool(
		"calculate",
		"Perform mathematical calculations",
		func(ctx context.Context, args map[string]any) (any, error) {
			operation, _ := args["operation"].(string)
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)

			var result float64
			switch operation {
			case "add":
				result = a + b
			case "subtract":
				result = a - b
			case "multiply":
				result = a * b
			case "divide":
				if b != 0 {
					result = a / b
				}
			}
			return result, nil
		},
	).
		AddParameter("operation", "string", "Operation (add/subtract/multiply/divide)", true).
		AddParameter("a", "float", "First number", true).
		AddParameter("b", "float", "Second number", true)
}

// DatabaseQueryTool creates a mock database query tool
func DatabaseQueryTool() *core.Tool {
	return core.NewTool(
		"query_db",
		"Query the database",
		func(ctx context.Context, args map[string]any) (any, error) {
			table, _ := args["table"].(string)
			condition, _ := args["condition"].(string)
			return "Query result from " + table + " where " + condition + ": 42 rows returned", nil
		},
	).
		AddParameter("table", "string", "Table name", true).
		AddParameter("condition", "string", "Where condition", true)
}

// WeatherTool creates a mock weather tool
func WeatherTool() *core.Tool {
	return core.NewTool(
		"get_weather",
		"Get weather information",
		func(ctx context.Context, args map[string]any) (any, error) {
			location, _ := args["location"].(string)
			return "Weather in " + location + ": Sunny, 72°F, Light breeze", nil
		},
	).AddParameter("location", "string", "City or location", true)
}

// TimeTool creates a mock time tool
func TimeTool() *core.Tool {
	return core.NewTool(
		"get_time",
		"Get current time",
		func(ctx context.Context, args map[string]any) (any, error) {
			timezone, _ := args["timezone"].(string)
			if timezone == "" {
				timezone = "UTC"
			}
			return "Current time in " + timezone + ": 14:30:45", nil
		},
	).AddParameter("timezone", "string", "Timezone", false)
}

// CommonTools returns a slice of commonly used test tools
func CommonTools() []core.Tool {
	return []core.Tool{
		*SearchTool(),
		*CalculatorTool(),
		*DatabaseQueryTool(),
	}
}

// AllTools returns all available test tools
func AllTools() []core.Tool {
	return []core.Tool{
		*SearchTool(),
		*CalculatorTool(),
		*DatabaseQueryTool(),
		*WeatherTool(),
		*TimeTool(),
	}
}

// ============================================================================
// Advanced Tool Scenarios
// ============================================================================

// ComplexJSONTool creates a tool that returns complex nested JSON
func ComplexJSONTool() *core.Tool {
	return core.NewTool(
		"get_user_data",
		"Get comprehensive user data",
		func(ctx context.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			return map[string]any{
				"user": map[string]any{
					"id":       userID,
					"name":     "Test User",
					"email":    "test@example.com",
					"verified": true,
				},
				"profile": map[string]any{
					"bio":      "A test user for integration testing",
					"location": "San Francisco, CA",
					"avatar":   "https://example.com/avatar.png",
				},
				"stats": map[string]any{
					"posts":     42,
					"followers": 1000,
					"following": 500,
				},
				"preferences": map[string]any{
					"theme":         "dark",
					"notifications": true,
					"language":      "en",
				},
			}, nil
		},
	).AddParameter("user_id", "string", "User ID to fetch", true)
}

// DelayedTool creates a tool with configurable response delay
func DelayedTool(delay time.Duration) *core.Tool {
	return core.NewTool(
		"slow_operation",
		"A slow operation that takes time",
		func(ctx context.Context, args map[string]any) (any, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				return "Operation completed after delay", nil
			}
		},
	).AddParameter("operation", "string", "Operation to perform", true)
}

// IntermittentFailTool creates a tool that fails intermittently
func IntermittentFailTool(failRate float64) *core.Tool {
	var callCount int
	return core.NewTool(
		"flaky_service",
		"A service that sometimes fails",
		func(ctx context.Context, args map[string]any) (any, error) {
			callCount++
			if float64(callCount%10)/10.0 < failRate {
				return nil, errors.New("intermittent service failure")
			}
			return "Service call succeeded", nil
		},
	).AddParameter("request", "string", "Request data", true)
}

// StatefulTool creates a tool that maintains state across calls
func StatefulTool() *core.Tool {
	state := make(map[string]any)
	var mu sync.Mutex

	return core.NewTool(
		"state_manager",
		"Manage stateful data",
		func(ctx context.Context, args map[string]any) (any, error) {
			mu.Lock()
			defer mu.Unlock()

			action, _ := args["action"].(string)
			key, _ := args["key"].(string)

			switch action {
			case "set":
				value := args["value"]
				state[key] = value
				return "Value set successfully", nil
			case "get":
				if value, ok := state[key]; ok {
					return value, nil
				}
				return nil, errors.New("key not found")
			case "delete":
				delete(state, key)
				return "Key deleted", nil
			case "list":
				keys := make([]string, 0, len(state))
				for k := range state {
					keys = append(keys, k)
				}
				return keys, nil
			default:
				return nil, errors.New("unknown action")
			}
		},
	).
		AddParameter("action", "string", "Action: set, get, delete, list", true).
		AddParameter("key", "string", "Key name", false).
		AddParameter("value", "any", "Value to set", false)
}

// MultiStepTool creates a tool that requires multiple calls to complete
func MultiStepTool() *core.Tool {
	sessions := make(map[string]int)
	var mu sync.Mutex

	return core.NewTool(
		"multi_step_process",
		"A process requiring multiple steps",
		func(ctx context.Context, args map[string]any) (any, error) {
			mu.Lock()
			defer mu.Unlock()

			sessionID, _ := args["session_id"].(string)
			step := sessions[sessionID]
			step++
			sessions[sessionID] = step

			switch step {
			case 1:
				return map[string]any{
					"status":   "in_progress",
					"step":     1,
					"message":  "Step 1 complete. Call again to continue.",
					"complete": false,
				}, nil
			case 2:
				return map[string]any{
					"status":   "in_progress",
					"step":     2,
					"message":  "Step 2 complete. Call again to finish.",
					"complete": false,
				}, nil
			default:
				delete(sessions, sessionID)
				return map[string]any{
					"status":   "complete",
					"step":     step,
					"message":  "Process completed successfully!",
					"complete": true,
					"result":   "Final result data",
				}, nil
			}
		},
	).AddParameter("session_id", "string", "Session identifier", true)
}

// ============================================================================
// Tool Collections
// ============================================================================

// AdvancedTools returns tools for advanced testing scenarios
func AdvancedTools() []core.Tool {
	return []core.Tool{
		*ComplexJSONTool(),
		*StatefulTool(),
		*MultiStepTool(),
	}
}

// AllToolsWithAdvanced returns all tools including advanced ones
func AllToolsWithAdvanced() []core.Tool {
	basic := AllTools()
	advanced := AdvancedTools()
	return append(basic, advanced...)
}
