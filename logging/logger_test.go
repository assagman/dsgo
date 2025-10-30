package logging

import (
	"context"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

func TestRequestIDGeneration(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID returned empty string")
	}
	if id1 == id2 {
		t.Error("GenerateRequestID generated duplicate IDs")
	}
	if len(id1) != 16 {
		t.Errorf("Expected request ID length 16, got %d", len(id1))
	}
}

func TestRequestIDContext(t *testing.T) {
	ctx := context.Background()

	// Initially no request ID
	if got := GetRequestID(ctx); got != "" {
		t.Errorf("Expected empty request ID, got %s", got)
	}

	// Add request ID
	requestID := "test-request-123"
	ctx = WithRequestID(ctx, requestID)

	// Retrieve request ID
	if got := GetRequestID(ctx); got != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, got)
	}
}

func TestEnsureRequestID(t *testing.T) {
	// Context without request ID
	ctx := context.Background()
	ctx = EnsureRequestID(ctx)

	id1 := GetRequestID(ctx)
	if id1 == "" {
		t.Error("EnsureRequestID should create a request ID")
	}

	// Context with existing request ID
	ctx = EnsureRequestID(ctx)
	id2 := GetRequestID(ctx)

	if id1 != id2 {
		t.Error("EnsureRequestID should not replace existing request ID")
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger(LevelDebug)
	if logger == nil {
		t.Fatal("NewDefaultLogger returned nil")
	}

	ctx := WithRequestID(context.Background(), "test-123")

	// Test all log levels
	logger.Debug(ctx, "debug message", map[string]any{"key": "value"})
	logger.Info(ctx, "info message", map[string]any{"count": 42})
	logger.Warn(ctx, "warn message", nil)
	logger.Error(ctx, "error message", map[string]any{"error": "test"})
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name       string
		logLevel   Level
		shouldLog  bool
		logMessage func(logger *DefaultLogger, ctx context.Context)
	}{
		{
			name:      "debug at debug level",
			logLevel:  LevelDebug,
			shouldLog: true,
			logMessage: func(l *DefaultLogger, ctx context.Context) {
				l.Debug(ctx, "test", nil)
			},
		},
		{
			name:      "debug at info level",
			logLevel:  LevelInfo,
			shouldLog: false,
			logMessage: func(l *DefaultLogger, ctx context.Context) {
				l.Debug(ctx, "test", nil)
			},
		},
		{
			name:      "info at debug level",
			logLevel:  LevelDebug,
			shouldLog: true,
			logMessage: func(l *DefaultLogger, ctx context.Context) {
				l.Info(ctx, "test", nil)
			},
		},
		{
			name:      "error at warn level",
			logLevel:  LevelWarn,
			shouldLog: true,
			logMessage: func(l *DefaultLogger, ctx context.Context) {
				l.Error(ctx, "test", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewDefaultLogger(tt.logLevel)
			ctx := context.Background()
			tt.logMessage(logger, ctx)
		})
	}
}

func TestNoOpLogger(t *testing.T) {
	logger := &NoOpLogger{}
	ctx := context.Background()

	// Should not panic
	logger.Debug(ctx, "test", nil)
	logger.Info(ctx, "test", nil)
	logger.Warn(ctx, "test", nil)
	logger.Error(ctx, "test", nil)
}

func TestGlobalLogger(t *testing.T) {
	// Save original logger
	original := GetLogger()
	defer SetLogger(original)

	// Test default (NoOpLogger)
	if _, ok := GetLogger().(*NoOpLogger); !ok {
		t.Error("Expected default logger to be NoOpLogger")
	}

	// Set custom logger
	customLogger := NewDefaultLogger(LevelInfo)
	SetLogger(customLogger)

	if got := GetLogger(); got != customLogger {
		t.Error("SetLogger did not set the global logger")
	}

	// Set nil logger (should revert to NoOpLogger)
	SetLogger(nil)
	if _, ok := GetLogger().(*NoOpLogger); !ok {
		t.Error("Setting nil logger should revert to NoOpLogger")
	}
}

func TestLogAPIRequest(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	LogAPIRequest(ctx, "gpt-4", 100)
}

func TestLogAPIResponse(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	usage := dsgo.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	LogAPIResponse(ctx, "gpt-4", 200, 500*time.Millisecond, usage)
}

func TestLogAPIError(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	err := context.DeadlineExceeded
	LogAPIError(ctx, "gpt-4", err)
}

func TestLogPredictionStart(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	LogPredictionStart(ctx, "Predict", "test signature")
}

func TestLogPredictionEnd(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")

	// Test success case
	LogPredictionEnd(ctx, "Predict", 100*time.Millisecond, nil)

	// Test error case
	LogPredictionEnd(ctx, "Predict", 100*time.Millisecond, context.Canceled)
}

func TestLoggerWithNilContext(t *testing.T) {
	// Should not panic with nil context
	id := GetRequestID(nil)
	if id != "" {
		t.Errorf("Expected empty string for nil context, got %s", id)
	}
}
