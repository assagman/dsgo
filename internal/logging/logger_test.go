package logging

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
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

	// Test default (DefaultLogger with LevelWarn)
	defaultLogger, ok := GetLogger().(*DefaultLogger)
	if !ok {
		t.Error("Expected default logger to be DefaultLogger")
	}
	if defaultLogger.level != LevelWarn {
		t.Errorf("Expected default logger level to be WARN, got %v", defaultLogger.level)
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
	LogAPIRequest(ctx, "test-provider", "gpt-4", 100)
}

func TestLogAPIResponse(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	LogAPIResponse(ctx, "test-provider", "gpt-4", 200, 500*time.Millisecond, usage)
}

func TestLogAPIError(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	logger := NewDefaultLogger(LevelDebug)
	SetLogger(logger)

	ctx := WithRequestID(context.Background(), "test-123")
	err := context.DeadlineExceeded
	LogAPIError(ctx, "test-provider", "gpt-4", err)
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
	ctx := context.Background()
	id := GetRequestID(ctx)
	if id != "" {
		t.Errorf("Expected empty string for nil context, got %s", id)
	}
}

type captureLogger struct {
	mu         sync.Mutex
	lastLevel  Level
	lastMsg    string
	lastFields map[string]any
	calls      int
}

func (c *captureLogger) store(level Level, msg string, fields map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastLevel = level
	c.lastMsg = msg
	c.calls++
	if fields == nil {
		c.lastFields = nil
		return
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	c.lastFields = copied
}

func (c *captureLogger) Debug(ctx context.Context, msg string, fields map[string]any) {
	c.store(LevelDebug, msg, fields)
}
func (c *captureLogger) Info(ctx context.Context, msg string, fields map[string]any) {
	c.store(LevelInfo, msg, fields)
}
func (c *captureLogger) Warn(ctx context.Context, msg string, fields map[string]any) {
	c.store(LevelWarn, msg, fields)
}
func (c *captureLogger) Error(ctx context.Context, msg string, fields map[string]any) {
	c.store(LevelError, msg, fields)
}
func (c *captureLogger) Fatal(ctx context.Context, msg string, fields map[string]any) {
	c.store(LevelFatal, msg, fields)
}

func TestParseHelpers(t *testing.T) {
	if got := parseInt("10"); got != 10 {
		t.Fatalf("parseInt = %d, want 10", got)
	}
	if got := parseInt("bad"); got != 0 {
		t.Fatalf("parseInt invalid = %d, want 0 fallback", got)
	}
	if got := parseInt64("20"); got != 20 {
		t.Fatalf("parseInt64 = %d, want 20", got)
	}
	if got := parseInt64("bad"); got != 0 {
		t.Fatalf("parseInt64 invalid = %d, want 0 fallback", got)
	}

	if got := parseDuration("150ms"); got != 150*time.Millisecond {
		t.Fatalf("parseDuration = %v, want 150ms", got)
	}
	if got := parseDuration("not-a-duration"); got != 0 {
		t.Fatalf("parseDuration invalid = %v, want 0", got)
	}

	cases := map[string]Level{
		"debug": LevelDebug,
		"INFO":  LevelInfo,
		"warn":  LevelWarn,
		"ERROR": LevelError,
		"fatal": LevelFatal,
		"bogus": -1,
	}
	for input, want := range cases {
		if got := parseLevel(input); got != want {
			t.Fatalf("parseLevel(%q) = %v, want %v", input, got, want)
		}
	}

	levels := parseModuleLevels("modA=debug, modB=INFO, badpair, missing=, =oops")
	want := map[string]Level{"modA": LevelDebug, "modB": LevelInfo}
	if !reflect.DeepEqual(levels, want) {
		t.Fatalf("parseModuleLevels = %#v, want %#v", levels, want)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("DSGO_LOG_LEVEL", "error")
	t.Setenv("DSGO_LOG_FORMAT", "json")
	t.Setenv("DSGO_LOG_BUFFER_SIZE", "42")
	t.Setenv("DSGO_LOG_FLUSH_INTERVAL", "250ms")
	t.Setenv("DSGO_LOG_FLUSH_TIMEOUT", "3s")
	t.Setenv("DSGO_LOG_BATCH_SIZE", "5")
	t.Setenv("DSGO_LOG_DROP_WHEN_FULL", "1")
	t.Setenv("DSGO_LOG_MAX_MEMORY", "1024")
	t.Setenv("DSGO_LOG_MODULE_LEVELS", "m1=debug,m2=ERROR")
	t.Setenv("DSGO_LOG_COLOR", "never")

	cfg := LoadConfigFromEnv()
	if cfg.Level != LevelError {
		t.Fatalf("Level = %v, want LevelError", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Fatalf("Format = %q, want json", cfg.Format)
	}
	if cfg.Color != colorModeNever {
		t.Fatalf("Color = %q, want %q", cfg.Color, colorModeNever)
	}
	if cfg.BufferSize != 42 || cfg.BatchSize != 5 {
		t.Fatalf("BufferSize=%d BatchSize=%d", cfg.BufferSize, cfg.BatchSize)
	}
	if cfg.FlushInterval != 250*time.Millisecond {
		t.Fatalf("FlushInterval = %v", cfg.FlushInterval)
	}
	if cfg.FlushTimeout != 3*time.Second {
		t.Fatalf("FlushTimeout = %v", cfg.FlushTimeout)
	}
	if !cfg.DropWhenFull {
		t.Fatal("DropWhenFull should be true")
	}
	if cfg.MaxMemoryUsage != 1024 {
		t.Fatalf("MaxMemoryUsage = %d", cfg.MaxMemoryUsage)
	}
	wantLevels := map[string]Level{"m1": LevelDebug, "m2": LevelError}
	if !reflect.DeepEqual(cfg.ModuleLevels, wantLevels) {
		t.Fatalf("ModuleLevels = %#v, want %#v", cfg.ModuleLevels, wantLevels)
	}
}

func TestConfigureLoggerFromEnv(t *testing.T) {
	original := GetLogger()
	t.Cleanup(func() { SetLogger(original) })

	t.Setenv("DSGO_LOG_LEVEL", "info")
	t.Setenv("DSGO_LOG_FORMAT", "text")
	ConfigureLoggerFromEnv()

	dl, ok := GetLogger().(*DefaultLogger)
	if !ok {
		t.Fatal("Expected DefaultLogger after ConfigureLoggerFromEnv")
	}
	if dl.level != LevelInfo {
		t.Fatalf("Level = %v, want LevelInfo", dl.level)
	}
	dl.Stop()
}

func TestInjectIDsAndCorrelation(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithCorrelationID(ctx, "corr-1")

	fields := injectIDs(ctx, map[string]any{"request_id": "keep"})
	if fields["request_id"].(string) != "keep" {
		t.Fatalf("request_id should not be overwritten")
	}
	if fields["correlation_id"].(string) != "corr-1" {
		t.Fatalf("correlation_id should be injected")
	}

	// Nil fields should not panic
	fields = injectIDs(context.TODO(), nil)
	if len(fields) != 0 {
		t.Fatalf("expected empty fields, got %#v", fields)
	}
}

func TestEnsureCorrelationID(t *testing.T) {
	ctx := context.Background()
	ctx = EnsureCorrelationID(ctx)
	first := GetCorrelationID(ctx)
	if first == "" {
		t.Fatal("correlation id should be set")
	}

	ctx = WithCorrelationID(ctx, "existing")
	ctx = EnsureCorrelationID(ctx)
	if got := GetCorrelationID(ctx); got != "existing" {
		t.Fatalf("existing correlation id should be preserved, got %s", got)
	}
}

func TestModuleLevelOverrides(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.ModuleLevels = map[string]Level{"moduleX": LevelError}
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.BatchSize = 1

	dl := NewDefaultLoggerWithConfig(cfg)
	defer dl.Stop()

	dl.Info(context.Background(), "should-drop", map[string]any{"module": "moduleX"})
	dl.Error(context.Background(), "should-log", map[string]any{"module": "moduleX"})

	time.Sleep(20 * time.Millisecond)

	stats := dl.GetStats()
	if stats["info_logged"].(int64) != 0 {
		t.Fatalf("info_logged = %d, want 0", stats["info_logged"].(int64))
	}
	if stats["error_logged"].(int64) != 1 {
		t.Fatalf("error_logged = %d, want 1", stats["error_logged"].(int64))
	}
}

func TestLogJSONFallbackOnMarshalError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelInfo
	cfg.Format = "json"
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.BatchSize = 1

	dl := NewDefaultLoggerWithConfig(cfg)
	defer dl.Stop()

	dl.Info(context.Background(), "bad json", map[string]any{"cannot": func() {}})
	time.Sleep(20 * time.Millisecond)

	if stats := dl.GetStats()["info_logged"].(int64); stats != 1 {
		t.Fatalf("info_logged = %d, want 1", stats)
	}
}

func TestLogPredictionEndLongDuration(t *testing.T) {
	original := GetLogger()
	t.Cleanup(func() { SetLogger(original) })

	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.BatchSize = 1
	dl := NewDefaultLoggerWithConfig(cfg)
	SetLogger(dl)

	LogPredictionEnd(context.Background(), "Predict", 150*time.Millisecond, nil)
	time.Sleep(20 * time.Millisecond)

	if stats := dl.GetStats()["info_logged"].(int64); stats != 1 {
		t.Fatalf("info_logged = %d, want 1", stats)
	}
	dl.Stop()
}

func TestAPIHelpersCapture(t *testing.T) {
	original := GetLogger()
	t.Cleanup(func() { SetLogger(original) })

	cap := &captureLogger{}
	SetLogger(cap)

	ctx := context.Background()
	LogAPIRequest(ctx, "", "modelA", 10)
	cap.mu.Lock()
	if cap.lastFields["module"] != "provider.API" {
		t.Fatalf("expected default provider module, got %v", cap.lastFields["module"])
	}
	cap.mu.Unlock()

	usage := Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3, Latency: 123}
	LogAPIResponse(ctx, "prov", "m", 200, 500*time.Millisecond, usage)
	cap.mu.Lock()
	if cap.lastFields["latency_ms"].(int64) != 123 {
		t.Fatalf("latency_ms = %v, want 123", cap.lastFields["latency_ms"])
	}
	cap.mu.Unlock()

	err := context.DeadlineExceeded
	LogAPIError(ctx, "prov", "m", err)
	cap.mu.Lock()
	if cap.lastLevel != LevelError {
		t.Fatalf("LogAPIError should log error level, got %v", cap.lastLevel)
	}
	cap.mu.Unlock()
}

func TestLogJSONSuccessfulFormatting(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.Format = "json"
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.BatchSize = 1

	dl := NewDefaultLoggerWithConfig(cfg)
	defer dl.Stop()

	// Test with valid JSON-serializable data
	ctx := WithRequestID(context.Background(), "test-req-123")
	fields := map[string]any{
		"module":     "TestModule",
		"request_id": "test-req-123",
		"count":      42,
		"enabled":    true,
		"tags":       []string{"tag1", "tag2"},
		"metadata":   map[string]any{"key": "value"},
	}

	dl.Info(ctx, "test message with valid data", fields)
	time.Sleep(20 * time.Millisecond)

	stats := dl.GetStats()
	if stats["info_logged"].(int64) != 1 {
		t.Fatalf("info_logged = %d, want 1", stats["info_logged"].(int64))
	}

	// Test with different log levels
	dl.Debug(ctx, "debug message", fields)
	dl.Warn(ctx, "warning message", fields)
	dl.Error(ctx, "error message", fields)
	time.Sleep(20 * time.Millisecond)

	stats = dl.GetStats()
	if stats["debug_logged"].(int64) != 1 {
		t.Fatalf("debug_logged = %d, want 1", stats["debug_logged"].(int64))
	}
	if stats["warn_logged"].(int64) != 1 {
		t.Fatalf("warn_logged = %d, want 1", stats["warn_logged"].(int64))
	}
	if stats["error_logged"].(int64) != 1 {
		t.Fatalf("error_logged = %d, want 1", stats["error_logged"].(int64))
	}
}

func TestFatalExitFunctions(t *testing.T) {
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	var mu sync.Mutex
	called := false
	code := 0
	exitFunc = func(c int) {
		mu.Lock()
		defer mu.Unlock()
		called = true
		code = c
	}

	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushInterval = 5 * time.Millisecond
	cfg.BatchSize = 1
	dl := NewDefaultLoggerWithConfig(cfg)

	dl.Fatal(context.Background(), "fatal message", nil)

	mu.Lock()
	if !called || code != 1 {
		t.Fatalf("exitFunc not called correctly: called=%v code=%d", called, code)
	}
	mu.Unlock()

	if stats := dl.GetStats()["fatal_logged"].(int64); stats != 1 {
		t.Fatalf("fatal_logged = %d, want 1", stats)
	}

	dl.Stop()

	// NoOpLogger should also call exitFunc
	called = false
	code = 0
	noop := &NoOpLogger{}
	noop.Fatal(context.Background(), "fatal", nil)
	mu.Lock()
	if !called || code != 1 {
		t.Fatalf("NoOpLogger exitFunc not called: called=%v code=%d", called, code)
	}
	mu.Unlock()
}

func TestStop_NoGoroutineLeak(t *testing.T) {
	// This test verifies that Stop() doesn't leave hanging goroutines
	// when timeout occurs
	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushTimeout = 100 * time.Millisecond
	cfg.FlushInterval = 1 * time.Second // Long interval to avoid flush before timeout

	dl := NewDefaultLoggerWithConfig(cfg)

	// Log some entries but don't wait for flush
	dl.Info(context.Background(), "test", nil)

	// Calling Stop() with short timeout should not leave hanging goroutines
	dl.Stop()

	// Give a brief moment to ensure goroutine cleanup
	time.Sleep(50 * time.Millisecond)

	// Verify stopped flag is set
	stats := dl.GetStats()
	if !stats["stopped"].(bool) {
		t.Error("Logger should be marked as stopped")
	}
}

func TestStop_IdempotentCalls(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushInterval = 5 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(cfg)

	// Multiple Stop calls should be safe
	dl.Stop()
	dl.Stop()
	dl.Stop()

	stats := dl.GetStats()
	if !stats["stopped"].(bool) {
		t.Error("Logger should be marked as stopped")
	}
}

func TestSetLogger_StopsOldLogger(t *testing.T) {
	// Save original logger
	original := GetLogger()
	defer SetLogger(original)

	// Create a new logger
	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushInterval = 5 * time.Millisecond
	oldLogger := NewDefaultLoggerWithConfig(cfg)

	// Set it as the global logger
	SetLogger(oldLogger)
	if GetLogger() != oldLogger {
		t.Fatal("SetLogger did not set the logger")
	}

	// Create another logger and set it, which should stop the old one
	newLogger := NewDefaultLoggerWithConfig(cfg)
	SetLogger(newLogger)

	// Give a moment for the old logger's Stop to complete
	time.Sleep(20 * time.Millisecond)

	// The old logger should be stopped now
	if !oldLogger.GetStats()["stopped"].(bool) {
		t.Error("Old logger should have been stopped by SetLogger")
	}

	// Clean up
	newLogger.Stop()
}

func TestSetLogger_WithNilLogger(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	SetLogger(nil)
	if _, ok := GetLogger().(*NoOpLogger); !ok {
		t.Fatal("SetLogger(nil) should set NoOpLogger")
	}
}

func TestSetLogger_ReplaceWithNoOp(t *testing.T) {
	original := GetLogger()
	defer SetLogger(original)

	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.FlushInterval = 5 * time.Millisecond
	dl := NewDefaultLoggerWithConfig(cfg)

	SetLogger(dl)
	if GetLogger() != dl {
		t.Fatal("SetLogger did not set logger")
	}

	// Replace with NoOp - should stop the DefaultLogger
	SetLogger(&NoOpLogger{})

	time.Sleep(20 * time.Millisecond)

	if !dl.GetStats()["stopped"].(bool) {
		t.Error("DefaultLogger should be stopped when replaced with NoOpLogger")
	}
}

func TestFormatTextLine_ColorEnabled(t *testing.T) {
	ts := time.Date(2025, 12, 15, 1, 2, 3, 0, time.UTC)

	tests := []struct {
		name      string
		level     Level
		wantColor string
	}{
		{"debug is cyan", LevelDebug, ansiCyan},
		{"info is green", LevelInfo, ansiGreen},
		{"warn is yellow", LevelWarn, ansiYellow},
		{"error is red", LevelError, ansiRed},
		{"fatal is red", LevelFatal, ansiRed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTextLine(tt.level, "boom", "mod", map[string]any{"k": "v"}, true, ts)
			if !strings.Contains(got, tt.wantColor) {
				t.Fatalf("expected color %q in output: %q", tt.wantColor, got)
			}
			if !strings.Contains(got, ansiReset) {
				t.Fatalf("expected reset %q in output: %q", ansiReset, got)
			}
			if !strings.Contains(got, levelString(tt.level)) {
				t.Fatalf("expected level %q in output: %q", levelString(tt.level), got)
			}
		})
	}
}

func TestFormatTextLine_ColorDisabled(t *testing.T) {
	ts := time.Date(2025, 12, 15, 1, 2, 3, 0, time.UTC)
	got := formatTextLine(LevelError, "boom", "", map[string]any{"k": "v"}, false, ts)
	if strings.Contains(got, ansiRed) || strings.Contains(got, ansiReset) {
		t.Fatalf("did not expect ANSI codes in output: %q", got)
	}
	if !strings.Contains(got, "ERROR") {
		t.Fatalf("expected ERROR in output: %q", got)
	}
}

func BenchmarkFormatTextLine_ColorEnabled(b *testing.B) {
	ts := time.Date(2025, 12, 15, 1, 2, 3, 0, time.UTC)
	fields := map[string]any{"k": "v", "n": 123}
	for i := 0; i < b.N; i++ {
		_ = formatTextLine(LevelError, "boom", "mod", fields, true, ts)
	}
}

func TestParseColorModeEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ALWAYS", colorModeAlways},
		{" never ", colorModeNever},
		{"invalid", colorModeAuto},
		{"", colorModeAuto},
		{"AUTO", colorModeAuto},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseColorMode(tt.input)
			if result != tt.expected {
				t.Errorf("parseColorMode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldEnableColor(t *testing.T) {
	// Save original TERM env var and restore after test
	originalTerm := os.Getenv("TERM")
	defer func() {
		_ = os.Setenv("TERM", originalTerm)
	}()

	tests := []struct {
		name        string
		format      string
		colorMode   string
		termEnv     string
		want        bool
		description string
	}{
		{
			name:        "json format always disabled",
			format:      "json",
			colorMode:   colorModeAlways,
			termEnv:     "xterm",
			want:        false,
			description: "format=json + Color=always => false",
		},
		{
			name:        "text format never disabled",
			format:      "text",
			colorMode:   colorModeNever,
			termEnv:     "xterm",
			want:        false,
			description: "format=text + Color=never => false",
		},
		{
			name:        "text format always enabled",
			format:      "text",
			colorMode:   colorModeAlways,
			termEnv:     "xterm",
			want:        true,
			description: "format=text + Color=always => true",
		},
		{
			name:        "term dumb disables auto",
			format:      "text",
			colorMode:   colorModeAuto,
			termEnv:     "dumb",
			want:        false,
			description: "TERM=dumb => false (in auto mode)",
		},
		{
			name:        "term dumb disables always",
			format:      "text",
			colorMode:   colorModeAlways,
			termEnv:     "dumb",
			want:        true,
			description: "TERM=dumb + Color=always => true (always overrides dumb)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set TERM environment variable
			_ = os.Setenv("TERM", tt.termEnv)

			result := shouldEnableColor(tt.format, tt.colorMode)
			if result != tt.want {
				t.Errorf("shouldEnableColor(%q, %q) with TERM=%q = %v, want %v (%s)",
					tt.format, tt.colorMode, tt.termEnv, result, tt.want, tt.description)
			}
		})
	}
}

func BenchmarkFormatTextLine_ColorDisabled(b *testing.B) {
	ts := time.Date(2025, 12, 15, 1, 2, 3, 0, time.UTC)
	fields := map[string]any{"k": "v", "n": 123}
	for i := 0; i < b.N; i++ {
		_ = formatTextLine(LevelError, "boom", "mod", fields, false, ts)
	}
}
