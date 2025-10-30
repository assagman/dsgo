package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/assagman/dsgo"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// Level represents the log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger interface for logging and tracing
type Logger interface {
	Debug(ctx context.Context, msg string, fields map[string]any)
	Info(ctx context.Context, msg string, fields map[string]any)
	Warn(ctx context.Context, msg string, fields map[string]any)
	Error(ctx context.Context, msg string, fields map[string]any)
}

// DefaultLogger is a simple logger that writes to stdout
type DefaultLogger struct {
	level  Level
	prefix string
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(level Level) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		prefix: "[DSGo]",
	}
}

func (l *DefaultLogger) log(ctx context.Context, level Level, msg string, fields map[string]any) {
	if level < l.level {
		return
	}

	levelStr := ""
	switch level {
	case LevelDebug:
		levelStr = "DEBUG"
	case LevelInfo:
		levelStr = "INFO"
	case LevelWarn:
		levelStr = "WARN"
	case LevelError:
		levelStr = "ERROR"
	}

	requestID := GetRequestID(ctx)
	if requestID == "" {
		requestID = "-"
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logMsg := fmt.Sprintf("%s %s [%s] [%s] %s", l.prefix, timestamp, levelStr, requestID, msg)

	if len(fields) > 0 {
		logMsg += " |"
		for k, v := range fields {
			logMsg += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	fmt.Println(logMsg)
}

func (l *DefaultLogger) Debug(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, LevelDebug, msg, fields)
}

func (l *DefaultLogger) Info(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, LevelInfo, msg, fields)
}

func (l *DefaultLogger) Warn(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, LevelWarn, msg, fields)
}

func (l *DefaultLogger) Error(ctx context.Context, msg string, fields map[string]any) {
	l.log(ctx, LevelError, msg, fields)
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

func (n *NoOpLogger) Debug(ctx context.Context, msg string, fields map[string]any) {}
func (n *NoOpLogger) Info(ctx context.Context, msg string, fields map[string]any)  {}
func (n *NoOpLogger) Warn(ctx context.Context, msg string, fields map[string]any)  {}
func (n *NoOpLogger) Error(ctx context.Context, msg string, fields map[string]any) {}

// Global logger instance
var globalLogger Logger = &NoOpLogger{}

// SetLogger sets the global logger
func SetLogger(logger Logger) {
	if logger == nil {
		globalLogger = &NoOpLogger{}
	} else {
		globalLogger = logger
	}
}

// GetLogger returns the global logger
func GetLogger() Logger {
	return globalLogger
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// LogAPIRequest logs the start of an API request
func LogAPIRequest(ctx context.Context, model string, promptLength int) {
	globalLogger.Info(ctx, "API request started", map[string]any{
		"model":         model,
		"prompt_length": promptLength,
	})
}

// LogAPIResponse logs the end of an API request
func LogAPIResponse(ctx context.Context, model string, statusCode int, duration time.Duration, usage dsgo.Usage) {
	globalLogger.Info(ctx, "API request completed", map[string]any{
		"model":             model,
		"status_code":       statusCode,
		"duration_ms":       duration.Milliseconds(),
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
	})
}

// LogAPIError logs an API error
func LogAPIError(ctx context.Context, model string, err error) {
	globalLogger.Error(ctx, "API request failed", map[string]any{
		"model": model,
		"error": err.Error(),
	})
}

// LogPredictionStart logs the start of a prediction
func LogPredictionStart(ctx context.Context, moduleName string, signature string) {
	globalLogger.Debug(ctx, "Prediction started", map[string]any{
		"module":    moduleName,
		"signature": signature,
	})
}

// LogPredictionEnd logs the end of a prediction
func LogPredictionEnd(ctx context.Context, moduleName string, duration time.Duration, err error) {
	fields := map[string]any{
		"module":      moduleName,
		"duration_ms": duration.Milliseconds(),
	}
	if err != nil {
		fields["error"] = err.Error()
		globalLogger.Error(ctx, "Prediction failed", fields)
	} else {
		globalLogger.Debug(ctx, "Prediction completed", fields)
	}
}
