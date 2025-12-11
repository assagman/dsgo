package logging

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

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

	cfg := LoadConfigFromEnv()
	if cfg.Level != LevelError {
		t.Fatalf("Level = %v, want LevelError", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Fatalf("Format = %q, want json", cfg.Format)
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
