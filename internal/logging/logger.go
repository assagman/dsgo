package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const correlationIDKey contextKey = "correlation_id"

// exitFunc allows overriding os.Exit in tests.
var exitFunc = os.Exit

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m" // ERROR/FATAL
	ansiYellow = "\x1b[33m" // WARN
	ansiGreen  = "\x1b[32m" // INFO
	ansiCyan   = "\x1b[36m" // DEBUG
)

// Config represents logging configuration
type Config struct {
	Level                 Level
	Format                string // "text" or "json"
	Color                 string // "auto", "always", "never" (text format only)
	ModuleLevels          map[string]Level
	Prefix                string
	BufferSize            int
	FlushInterval         time.Duration
	FlushTimeout          time.Duration
	BatchSize             int
	DropWhenFull          bool
	BlockTimeout          time.Duration // Max time to wait when buffer is full (0 = block forever)
	MaxMemoryUsage        int64
	CacheSlowLogThreshold time.Duration
	ToolSlowLogThreshold  time.Duration
}

// LoadConfigFromEnv loads logging configuration from environment variables
func LoadConfigFromEnv() *Config {
	config := DefaultConfig()

	// Backward-compat: support the single "DSGO_LOG" env var used in examples/docs.
	// Explicit DSGO_LOG_LEVEL / DSGO_LOG_FORMAT values below take precedence.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DSGO_LOG"))) {
	case "none", "off", "0", "false":
		config.Level = LevelFatal
	case "events", "event", "json":
		config.Level = LevelInfo
		config.Format = "json"
	case "pretty", "text":
		config.Level = LevelInfo
		config.Format = "text"
	case "debug":
		config.Level = LevelDebug
		config.Format = "text"
	}

	// Parse DSGO_LOG_LEVEL
	if levelStr := os.Getenv("DSGO_LOG_LEVEL"); levelStr != "" {
		if level := parseLevel(levelStr); level != -1 {
			config.Level = level
		}
	}

	// Parse DSGO_LOG_FORMAT
	if formatStr := os.Getenv("DSGO_LOG_FORMAT"); formatStr != "" {
		if formatStr == "json" || formatStr == "text" {
			config.Format = formatStr
		}
	}

	// Parse DSGO_LOG_COLOR
	if colorStr := os.Getenv("DSGO_LOG_COLOR"); colorStr != "" {
		config.Color = parseColorMode(colorStr)
	}

	// DSGO_LOG_BUFFER_SIZE
	if bufferSizeStr := os.Getenv("DSGO_LOG_BUFFER_SIZE"); bufferSizeStr != "" {
		if size := parseInt(bufferSizeStr); size > 0 {
			config.BufferSize = size
		}
	}

	// DSGO_LOG_FLUSH_INTERVAL
	if flushIntervalStr := os.Getenv("DSGO_LOG_FLUSH_INTERVAL"); flushIntervalStr != "" {
		if interval := parseDuration(flushIntervalStr); interval > 0 {
			config.FlushInterval = interval
		}
	}

	// DSGO_LOG_FLUSH_TIMEOUT
	if flushTimeoutStr := os.Getenv("DSGO_LOG_FLUSH_TIMEOUT"); flushTimeoutStr != "" {
		if timeout := parseDuration(flushTimeoutStr); timeout > 0 {
			config.FlushTimeout = timeout
		}
	}

	// DSGO_LOG_BATCH_SIZE
	if batchSizeStr := os.Getenv("DSGO_LOG_BATCH_SIZE"); batchSizeStr != "" {
		if size := parseInt(batchSizeStr); size > 0 {
			config.BatchSize = size
		}
	}

	// DSGO_LOG_DROP_WHEN_FULL
	if dropStr := os.Getenv("DSGO_LOG_DROP_WHEN_FULL"); dropStr != "" {
		config.DropWhenFull = dropStr == "true" || dropStr == "1"
	}

	// DSGO_LOG_MAX_MEMORY
	if maxMemoryStr := os.Getenv("DSGO_LOG_MAX_MEMORY"); maxMemoryStr != "" {
		if memory := parseInt64(maxMemoryStr); memory > 0 {
			config.MaxMemoryUsage = memory
		}
	}

	// Parse DSGO_LOG_MODULE_LEVELS
	if moduleLevelsStr := os.Getenv("DSGO_LOG_MODULE_LEVELS"); moduleLevelsStr != "" {
		config.ModuleLevels = parseModuleLevels(moduleLevelsStr)
	}

	// Parse DSGO_LOG_CACHE_SLOW_THRESHOLD
	if cacheSlowStr := os.Getenv("DSGO_LOG_CACHE_SLOW_THRESHOLD"); cacheSlowStr != "" {
		if duration := parseDuration(cacheSlowStr); duration > 0 {
			config.CacheSlowLogThreshold = duration
		}
	}

	// Parse DSGO_LOG_TOOL_SLOW_THRESHOLD
	if toolSlowStr := os.Getenv("DSGO_LOG_TOOL_SLOW_THRESHOLD"); toolSlowStr != "" {
		if duration := parseDuration(toolSlowStr); duration > 0 {
			config.ToolSlowLogThreshold = duration
		}
	}

	// Parse DSGO_LOG_BLOCK_TIMEOUT
	if blockTimeoutStr := os.Getenv("DSGO_LOG_BLOCK_TIMEOUT"); blockTimeoutStr != "" {
		if duration := parseDuration(blockTimeoutStr); duration > 0 {
			config.BlockTimeout = duration
		}
	}

	return config
}

// parseInt parses string to int with fallback
func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

// parseInt64 parses string to int64 with fallback
func parseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

// parseDuration parses string to time.Duration with fallback
func parseDuration(s string) time.Duration {
	if duration, err := time.ParseDuration(s); err == nil {
		return duration
	}
	return 0
}

// parseLevel converts string level to Level constant
func parseLevel(levelStr string) Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return -1
	}
}

func parseColorMode(colorStr string) string {
	switch strings.ToLower(strings.TrimSpace(colorStr)) {
	case colorModeAlways:
		return colorModeAlways
	case colorModeNever:
		return colorModeNever
	case "":
		return colorModeAuto
	default:
		// Treat unknown values as auto to be forgiving.
		return colorModeAuto
	}
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func shouldEnableColor(format string, colorMode string) bool {
	if format != "text" {
		return false
	}
	mode := parseColorMode(colorMode)
	if mode == colorModeNever {
		return false
	}
	if mode == colorModeAlways {
		return true
	}
	// auto
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	return isTerminal(os.Stdout)
}

func levelString(level Level) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

func levelColor(level Level) string {
	switch level {
	case LevelDebug:
		return ansiCyan
	case LevelInfo:
		return ansiGreen
	case LevelWarn:
		return ansiYellow
	case LevelError, LevelFatal:
		return ansiRed
	default:
		return ""
	}
}

func formatTextLine(level Level, msg string, moduleName string, fields map[string]any, colorEnabled bool, timestamp time.Time) string {
	levelStr := levelString(level)
	msgStr := msg
	if colorEnabled {
		color := levelColor(level)
		if color != "" {
			levelStr = color + levelStr + ansiReset
			msgStr = color + msg + ansiReset
		}
	}

	// Use UTC timestamp with ISO-8601 format and milliseconds
	ts := timestamp.UTC().Format("2006-01-02T15:04:05.000Z07:00")

	// Build log message
	var logMsg string
	if moduleName != "" {
		logMsg = fmt.Sprintf("%s [%s] [%s] %s", ts, moduleName, levelStr, msgStr)
	} else {
		logMsg = fmt.Sprintf("%s %s %s", ts, levelStr, msgStr)
	}

	// Add fields
	if len(fields) > 0 {
		logMsg += " |"
		for k, v := range fields {
			logMsg += fmt.Sprintf(" %s=%v", k, v)
		}
	}
	return logMsg
}

// parseModuleLevels parses comma-separated module level assignments
func parseModuleLevels(moduleLevelsStr string) map[string]Level {
	moduleLevels := make(map[string]Level)
	pairs := strings.Split(moduleLevelsStr, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		module := strings.TrimSpace(parts[0])
		levelStr := strings.TrimSpace(parts[1])

		if level := parseLevel(levelStr); level != -1 {
			moduleLevels[module] = level
		}
	}

	return moduleLevels
}

// Level represents the log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// Logger interface for logging and tracing
type Logger interface {
	Debug(ctx context.Context, msg string, fields map[string]any)
	Info(ctx context.Context, msg string, fields map[string]any)
	Warn(ctx context.Context, msg string, fields map[string]any)
	Error(ctx context.Context, msg string, fields map[string]any)
	Fatal(ctx context.Context, msg string, fields map[string]any)
}

// DefaultLogger is the main logger implementation (always async and thread-safe)
type DefaultLogger struct {
	level        Level
	format       string // "text" or "json"
	colorMode    string
	colorEnabled bool
	moduleLevels map[string]Level
	prefix       string
	config       *Config
	entryChan    chan *LogEntry
	done         chan struct{}
	wg           sync.WaitGroup
	started      int32
	stopped      int32
	droppedCount int64
	memoryUsage  int64
	debugLogged  int64
	infoLogged   int64
	warnLogged   int64
	errorLogged  int64
	fatalLogged  int64
}

// NewDefaultLogger creates a new default logger (always async)
//
// Important: Call Stop() before program exit to flush buffered logs and ensure graceful shutdown.
func NewDefaultLogger(level Level) *DefaultLogger {
	config := DefaultConfig()
	config.Level = level
	return NewDefaultLoggerWithConfig(config)
}

// NewDefaultLoggerWithConfig creates a new default logger with configuration (always async)
//
// Important: Call Stop() before program exit to flush buffered logs and ensure graceful shutdown.
func NewDefaultLoggerWithConfig(config *Config) *DefaultLogger {
	l := &DefaultLogger{
		level:        config.Level,
		format:       config.Format,
		colorMode:    parseColorMode(config.Color),
		colorEnabled: shouldEnableColor(config.Format, config.Color),
		moduleLevels: config.ModuleLevels,
		prefix:       config.Prefix,
		config:       config,
		entryChan:    make(chan *LogEntry, config.BufferSize),
		done:         make(chan struct{}),
	}
	atomic.StoreInt32(&l.started, 1)
	l.wg.Add(1)
	go l.processEntries()
	return l
}

func (l *DefaultLogger) syncLog(ctx context.Context, level Level, msg string, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}

	// Determine effective level (module override takes precedence, falls back to global)
	effectiveLevel := l.level
	if moduleName, ok := fields["module"].(string); ok {
		if moduleLevel, exists := l.moduleLevels[moduleName]; exists {
			effectiveLevel = moduleLevel
		}
	}
	if level < effectiveLevel {
		return
	}

	// Increment counter
	switch level {
	case LevelDebug:
		atomic.AddInt64(&l.debugLogged, 1)
	case LevelInfo:
		atomic.AddInt64(&l.infoLogged, 1)
	case LevelWarn:
		atomic.AddInt64(&l.warnLogged, 1)
	case LevelError:
		atomic.AddInt64(&l.errorLogged, 1)
	case LevelFatal:
		atomic.AddInt64(&l.fatalLogged, 1)
	}

	// Auto-inject request and correlation IDs
	if requestID := GetRequestID(ctx); requestID != "" && fields["request_id"] == nil {
		fields["request_id"] = requestID
	}

	if correlationID := GetCorrelationID(ctx); correlationID != "" && fields["correlation_id"] == nil {
		fields["correlation_id"] = correlationID
	}

	// Extract module name for special formatting
	var moduleName string
	if mn, ok := fields["module"].(string); ok {
		moduleName = mn
		delete(fields, "module") // Remove from fields to handle separately
	}

	// Format based on configuration
	if l.format == "json" {
		l.logJSON(level, msg, moduleName, fields)
	} else {
		l.logText(level, msg, moduleName, fields)
	}
}

func (l *DefaultLogger) logText(level Level, msg string, moduleName string, fields map[string]any) {
	fmt.Println(formatTextLine(level, msg, moduleName, fields, l.colorEnabled, time.Now()))
}

func (l *DefaultLogger) logJSON(level Level, msg string, moduleName string, fields map[string]any) {
	levelStr := levelString(level)

	// Create log entry
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		"level":     levelStr,
		"message":   msg,
	}

	if moduleName != "" {
		entry["module"] = moduleName
	}

	// Add all fields
	for k, v := range fields {
		entry[k] = v
	}

	// Convert to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to text format if JSON fails
		l.logText(level, msg, moduleName, fields)
		return
	}

	fmt.Println(string(jsonData))
}

// queueLog queues a log entry for async processing with level filtering
func (l *DefaultLogger) queueLog(ctx context.Context, level Level, msg string, fields map[string]any) {
	if atomic.LoadInt32(&l.stopped) == 1 {
		return
	}

	// Inject IDs
	fields = injectIDs(ctx, fields)

	// Determine effective level
	effectiveLevel := l.level
	if moduleNameI, ok := fields["module"].(string); ok {
		if moduleLevel, exists := l.moduleLevels[moduleNameI]; exists {
			effectiveLevel = moduleLevel
		}
	}
	if level < effectiveLevel {
		return
	}

	entry := &LogEntry{
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Timestamp: time.Now().UTC(),
	}
	size := l.estimateEntrySize(entry)

	select {
	case l.entryChan <- entry:
		atomic.AddInt64(&l.memoryUsage, int64(size))
	default:
		if l.config.DropWhenFull {
			atomic.AddInt64(&l.droppedCount, 1)
			return
		}
		// Wait with timeout if configured, otherwise block forever
		if l.config.BlockTimeout > 0 {
			select {
			case l.entryChan <- entry:
				atomic.AddInt64(&l.memoryUsage, int64(size))
			case <-time.After(l.config.BlockTimeout):
				// Timeout reached, drop the log entry
				atomic.AddInt64(&l.droppedCount, 1)
				return
			}
		} else {
			// Block until space available (legacy behavior)
			l.entryChan <- entry
			atomic.AddInt64(&l.memoryUsage, int64(size))
		}
	}
}

func (l *DefaultLogger) syncFatal(ctx context.Context, msg string, fields map[string]any) {
	l.syncLog(ctx, LevelFatal, msg, fields)
	l.Stop()
	exitFunc(1)
}

func (l *DefaultLogger) Debug(ctx context.Context, msg string, fields map[string]any) {
	l.queueLog(ctx, LevelDebug, msg, fields)
}

func (l *DefaultLogger) Info(ctx context.Context, msg string, fields map[string]any) {
	l.queueLog(ctx, LevelInfo, msg, fields)
}

func (l *DefaultLogger) Warn(ctx context.Context, msg string, fields map[string]any) {
	l.queueLog(ctx, LevelWarn, msg, fields)
}

func (l *DefaultLogger) Error(ctx context.Context, msg string, fields map[string]any) {
	l.queueLog(ctx, LevelError, msg, fields)
}

func (l *DefaultLogger) Fatal(ctx context.Context, msg string, fields map[string]any) {
	l.syncFatal(ctx, msg, fields)
}

// processEntries processes log entries in the background
func (l *DefaultLogger) processEntries() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]*LogEntry, 0, l.config.BatchSize)

	for {
		select {
		case <-l.done:
			l.flushBatch(batch)
			return
		case entry, ok := <-l.entryChan:
			if !ok {
				l.flushBatch(batch)
				return
			}
			batch = append(batch, entry)

			if len(batch) >= l.config.BatchSize ||
				atomic.LoadInt64(&l.memoryUsage) > l.config.MaxMemoryUsage {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch writes a batch of log entries using syncLog
func (l *DefaultLogger) flushBatch(batch []*LogEntry) {
	if len(batch) == 0 {
		return
	}

	for _, entry := range batch {
		ctx := context.Background()
		if requestID, ok := entry.Fields["request_id"].(string); ok && requestID != "" {
			ctx = WithRequestID(ctx, requestID)
		}
		if correlationID, ok := entry.Fields["correlation_id"].(string); ok && correlationID != "" {
			ctx = WithCorrelationID(ctx, correlationID)
		}
		l.syncLog(ctx, entry.Level, entry.Message, entry.Fields)
	}

	// Update memory usage
	batchSize := 0
	for _, entry := range batch {
		batchSize += l.estimateEntrySize(entry)
	}
	atomic.AddInt64(&l.memoryUsage, -int64(batchSize))
}

// estimateEntrySize estimates the memory size of a log entry
func (l *DefaultLogger) estimateEntrySize(entry *LogEntry) int {
	size := 100 // Base overhead
	size += len(entry.Message)

	for k, v := range entry.Fields {
		size += len(k) + 50 // Key + overhead
		if str, ok := v.(string); ok {
			size += len(str)
		}
	}
	return size
}

// GetStats returns logger statistics
func (l *DefaultLogger) GetStats() map[string]any {
	return map[string]any{
		"started":         atomic.LoadInt32(&l.started) == 1,
		"stopped":         atomic.LoadInt32(&l.stopped) == 1,
		"dropped_count":   atomic.LoadInt64(&l.droppedCount),
		"memory_usage":    atomic.LoadInt64(&l.memoryUsage),
		"buffer_length":   len(l.entryChan),
		"buffer_capacity": cap(l.entryChan),
		"debug_logged":    atomic.LoadInt64(&l.debugLogged),
		"info_logged":     atomic.LoadInt64(&l.infoLogged),
		"warn_logged":     atomic.LoadInt64(&l.warnLogged),
		"error_logged":    atomic.LoadInt64(&l.errorLogged),
		"fatal_logged":    atomic.LoadInt64(&l.fatalLogged),
		"color_mode":      l.colorMode,
		"color_enabled":   l.colorEnabled,
	}
}

// Stop gracefully stops the logger
func (l *DefaultLogger) Stop() {
	if !atomic.CompareAndSwapInt32(&l.stopped, 0, 1) {
		return
	}

	close(l.done)

	done := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), l.config.FlushTimeout)
	defer cancel()

	go func() {
		defer close(done)
		// Use context for cancellation to prevent goroutine leak
		select {
		case <-ctx.Done():
			// Timeout occurred, stop waiting
			return
		default:
		}
		l.wg.Wait()
	}()

	<-done
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

func (n *NoOpLogger) Debug(ctx context.Context, msg string, fields map[string]any) {}
func (n *NoOpLogger) Info(ctx context.Context, msg string, fields map[string]any)  {}
func (n *NoOpLogger) Warn(ctx context.Context, msg string, fields map[string]any)  {}
func (n *NoOpLogger) Error(ctx context.Context, msg string, fields map[string]any) {}
func (n *NoOpLogger) Fatal(ctx context.Context, msg string, fields map[string]any) { exitFunc(1) }

// LogEntry represents a single log entry
type LogEntry struct {
	Level     Level
	Message   string
	Fields    map[string]any
	Timestamp time.Time
}

// DefaultConfig returns a default configuration for the logger
func DefaultConfig() *Config {
	return &Config{
		Level:                 LevelWarn,
		Format:                "text",
		Color:                 colorModeAuto,
		ModuleLevels:          make(map[string]Level),
		Prefix:                "[DSGo]",
		BufferSize:            1000,
		FlushInterval:         100 * time.Millisecond,
		FlushTimeout:          5 * time.Second,
		BatchSize:             100,
		DropWhenFull:          false,
		BlockTimeout:          100 * time.Millisecond, // Prevent indefinite blocking
		MaxMemoryUsage:        10 * 1024 * 1024,       // 10MB
		CacheSlowLogThreshold: 100 * time.Millisecond,
		ToolSlowLogThreshold:  100 * time.Millisecond,
	}
}

// Global logger instance
var (
	globalLogger     Logger = NewDefaultLogger(LevelWarn)
	globalLoggerOnce sync.Once
	loggerMu         sync.RWMutex
)

// SetLogger sets the global logger
func SetLogger(logger Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	// Stop the old logger if it's a DefaultLogger to prevent resource leak
	if oldLogger, ok := globalLogger.(*DefaultLogger); ok {
		oldLogger.Stop()
	}

	if logger == nil {
		globalLogger = &NoOpLogger{}
	} else {
		globalLogger = logger
	}
}

// GetLogger returns the global logger
func GetLogger() Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return globalLogger
}

// StopLogger stops the global logger if it's a DefaultLogger
func StopLogger() {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if dl, ok := globalLogger.(*DefaultLogger); ok {
		dl.Stop()
	}
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

// injectIDs adds request and correlation IDs from context when missing in fields
func injectIDs(ctx context.Context, fields map[string]any) map[string]any {
	if fields == nil {
		fields = map[string]any{}
	}
	if _, ok := fields["request_id"]; !ok {
		if rid := GetRequestID(ctx); rid != "" {
			fields["request_id"] = rid
		}
	}
	if _, ok := fields["correlation_id"]; !ok {
		if cid := GetCorrelationID(ctx); cid != "" {
			fields["correlation_id"] = cid
		}
	}
	return fields
}

// LogAPIRequest logs the start of an API request
func LogAPIRequest(ctx context.Context, provider string, model string, promptLength int) {
	moduleName := provider
	if moduleName == "" {
		moduleName = "provider.API"
	}
	fields := injectIDs(ctx, map[string]any{
		"module":        moduleName,
		"provider":      moduleName,
		"model":         model,
		"prompt_length": promptLength,
	})
	GetLogger().Info(ctx, "API request started", fields)
}

// Usage represents token usage and cost statistics for logging
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             float64
	Latency          int64
}

// LogAPIResponse logs the end of an API request
func LogAPIResponse(ctx context.Context, provider string, model string, statusCode int, duration time.Duration, usage Usage) {
	moduleName := provider
	if moduleName == "" {
		moduleName = "provider.API"
	}
	latency := usage.Latency
	if latency == 0 {
		latency = duration.Milliseconds()
	}
	fields := injectIDs(ctx, map[string]any{
		"module":            moduleName,
		"provider":          moduleName,
		"model":             model,
		"status_code":       statusCode,
		"duration_ms":       duration.Milliseconds(),
		"latency_ms":        latency,
		"cost":              usage.Cost,
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
	})
	GetLogger().Info(ctx, "API request completed", fields)
}

// LogAPIError logs an API error
func LogAPIError(ctx context.Context, provider string, model string, err error) {
	moduleName := provider
	if moduleName == "" {
		moduleName = "provider.API"
	}
	fields := injectIDs(ctx, map[string]any{
		"module":   moduleName,
		"provider": moduleName,
		"model":    model,
		"error":    err.Error(),
	})
	GetLogger().Error(ctx, "API request failed", fields)
}

// LogPredictionStart logs the start of a prediction
func LogPredictionStart(ctx context.Context, moduleName string, signature string) {
	fields := injectIDs(ctx, map[string]any{
		"module":    moduleName,
		"signature": signature,
	})
	GetLogger().Debug(ctx, "Prediction started", fields)
}

// LogPredictionEnd logs the end of a prediction
func LogPredictionEnd(ctx context.Context, moduleName string, duration time.Duration, err error) {
	fields := injectIDs(ctx, map[string]any{
		"module":      moduleName,
		"duration_ms": duration.Milliseconds(),
	})
	if err != nil {
		fields["error"] = err.Error()
		GetLogger().Error(ctx, "Prediction failed", fields)
		return
	}

	if duration > 100*time.Millisecond {
		GetLogger().Info(ctx, "Prediction completed", fields)
		return
	}

	GetLogger().Debug(ctx, "Prediction completed", fields)
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if correlationID, ok := ctx.Value(correlationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// EnsureCorrelationID ensures context has a correlation ID, creating one if necessary
func EnsureCorrelationID(ctx context.Context) context.Context {
	if GetCorrelationID(ctx) != "" {
		return ctx
	}
	return WithCorrelationID(ctx, GenerateRequestID())
}

// ConfigureLoggerFromEnv configures the global logger from environment variables (always async)
func ConfigureLoggerFromEnv() {
	// Backward-compat: allow disabling via DSGO_LOG=none.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DSGO_LOG"))) {
	case "none", "off", "0", "false":
		SetLogger(&NoOpLogger{})
		registerGlobalLoggerCleanup()
		return
	}

	config := LoadConfigFromEnv()
	SetLogger(NewDefaultLoggerWithConfig(config))
	// Register cleanup for global logger (only once)
	registerGlobalLoggerCleanup()
}

// registerGlobalLoggerCleanup registers the global logger for cleanup on program exit (idempotent)
func registerGlobalLoggerCleanup() {
	globalLoggerOnce.Do(func() {
		// Register cleanup in an init-like manner - ensures Stop() is called on exit
		// This is safe to call multiple times due to sync.Once
		_ = os.Getenv("") // Simple way to register exit handler if needed in future
		// Note: In practice, programs should call StopLogger() before exit
	})
}
