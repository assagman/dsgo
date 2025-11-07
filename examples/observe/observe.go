package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type SpanKind string

const (
	SpanKindRun       SpanKind = "run"
	SpanKindModule    SpanKind = "module"
	SpanKindAdapter   SpanKind = "adapter"
	SpanKindCache     SpanKind = "cache"
	SpanKindTool      SpanKind = "tool"
	SpanKindStream    SpanKind = "stream"
	SpanKindProgram   SpanKind = "program"
	SpanKindReActStep SpanKind = "react_step"
	SpanKindBestOfN   SpanKind = "bestofn"
	SpanKindRefine    SpanKind = "refine"
	SpanKindFewShot   SpanKind = "fewshot"
)

type Event struct {
	Timestamp time.Time              `json:"ts"`
	Level     string                 `json:"level"`
	SpanID    string                 `json:"span_id"`
	ParentID  string                 `json:"parent_id,omitempty"`
	RunID     string                 `json:"run_id,omitempty"`
	Kind      SpanKind               `json:"kind"`
	Operation string                 `json:"operation"`
	Module    string                 `json:"module,omitempty"`
	Adapter   string                 `json:"adapter,omitempty"`
	Model     string                 `json:"model,omitempty"`
	LatencyMs int64                  `json:"latency_ms,omitempty"`
	Tokens    *TokenUsage            `json:"tokens,omitempty"`
	Cache     *CacheInfo             `json:"cache,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

type CacheInfo struct {
	Status string `json:"status"` // hit, miss, store
}

type Span struct {
	id        string
	parentID  string
	runID     string
	kind      SpanKind
	operation string
	startTime time.Time
	fields    map[string]interface{}
	children  []*Span
	mu        sync.Mutex
}

type contextKey int

const (
	spanContextKey contextKey = iota
	runIDContextKey
)

var (
	defaultLogger *Logger
	once          sync.Once
)

type Logger struct {
	writer     io.Writer
	jsonWriter io.Writer
	mode       LogMode
	minLevel   LogLevel
	mu         sync.Mutex
}

type LogMode int

const (
	LogModeOff LogMode = iota
	LogModeEvents
	LogModePretty
	LogModeBoth
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func init() {
	once.Do(func() {
		mode := parseLogMode(os.Getenv("DSGO_LOG"))
		defaultLogger = &Logger{
			writer:     os.Stdout,
			jsonWriter: os.Stdout,
			mode:       mode,
			minLevel:   LogLevelInfo,
		}
	})
}

func parseLogMode(s string) LogMode {
	switch strings.ToLower(s) {
	case "off", "false", "0":
		return LogModeOff
	case "events", "json":
		return LogModeEvents
	case "pretty", "tree":
		return LogModePretty
	case "both", "all":
		return LogModeBoth
	default:
		return LogModePretty
	}
}

func SetLogger(l *Logger) {
	defaultLogger = l
}

func NewLogger(mode LogMode, w io.Writer) *Logger {
	return &Logger{
		writer:     w,
		jsonWriter: w,
		mode:       mode,
		minLevel:   LogLevelInfo,
	}
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func Start(ctx context.Context, kind SpanKind, operation string, fields map[string]interface{}) (context.Context, *Span) {
	parent := SpanFromContext(ctx)
	runID := RunIDFromContext(ctx)
	if runID == "" {
		runID = generateID()
		ctx = context.WithValue(ctx, runIDContextKey, runID)
	}

	parentID := ""
	if parent != nil {
		parentID = parent.id
	}

	span := &Span{
		id:        generateID(),
		parentID:  parentID,
		runID:     runID,
		kind:      kind,
		operation: operation,
		startTime: time.Now(),
		fields:    fields,
	}

	if parent != nil {
		parent.mu.Lock()
		parent.children = append(parent.children, span)
		parent.mu.Unlock()
	}

	defaultLogger.logEvent(Event{
		Timestamp: span.startTime,
		Level:     "INFO",
		SpanID:    span.id,
		ParentID:  parentID,
		RunID:     runID,
		Kind:      kind,
		Operation: operation + ".start",
		Fields:    fields,
	}, parent)

	return context.WithValue(ctx, spanContextKey, span), span
}

func (s *Span) Event(operation string, fields map[string]interface{}) {
	defaultLogger.logEvent(Event{
		Timestamp: time.Now(),
		Level:     "INFO",
		SpanID:    s.id,
		ParentID:  s.parentID,
		RunID:     s.runID,
		Kind:      s.kind,
		Operation: operation,
		Fields:    fields,
	}, s)
}

func (s *Span) End(err error) {
	latency := time.Since(s.startTime).Milliseconds()

	evt := Event{
		Timestamp: time.Now(),
		Level:     "INFO",
		SpanID:    s.id,
		ParentID:  s.parentID,
		RunID:     s.runID,
		Kind:      s.kind,
		Operation: s.operation + ".end",
		LatencyMs: latency,
		Fields:    s.fields,
	}

	if err != nil {
		evt.Level = "ERROR"
		evt.Error = err.Error()
	}

	defaultLogger.logEvent(evt, s)
}

func (l *Logger) logEvent(evt Event, span *Span) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.mode == LogModeOff {
		return
	}

	if l.mode == LogModeEvents || l.mode == LogModeBoth {
		data, _ := json.Marshal(evt)
		_, _ = fmt.Fprintln(l.jsonWriter, string(data))
	}

	if l.mode == LogModePretty || l.mode == LogModeBoth {
		l.printPretty(evt, span)
	}
}

func (l *Logger) printPretty(evt Event, span *Span) {
	depth := 0
	if span != nil {
		depth = l.getDepth(span)
	}

	indent := strings.Repeat("  ", depth)
	symbol := l.getSymbol(evt.Operation, evt.Level)

	var parts []string
	parts = append(parts, fmt.Sprintf("%s%s %s", indent, symbol, evt.Operation))

	if evt.Module != "" {
		parts = append(parts, fmt.Sprintf("[%s]", evt.Module))
	}
	if evt.Adapter != "" {
		parts = append(parts, fmt.Sprintf("[%s]", evt.Adapter))
	}
	if evt.Model != "" {
		parts = append(parts, fmt.Sprintf("model=%s", evt.Model))
	}
	if evt.LatencyMs > 0 {
		parts = append(parts, fmt.Sprintf("%dms", evt.LatencyMs))
	}
	if evt.Tokens != nil && evt.Tokens.Total > 0 {
		parts = append(parts, fmt.Sprintf("tokens=%d", evt.Tokens.Total))
	}
	if evt.Cache != nil {
		parts = append(parts, fmt.Sprintf("cache=%s", evt.Cache.Status))
	}
	if evt.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", evt.Error))
	}

	for k, v := range evt.Fields {
		if k != "depth" {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
	}

	_, _ = fmt.Fprintln(l.writer, strings.Join(parts, " "))
}

func (l *Logger) getSymbol(op, level string) string {
	if strings.HasSuffix(op, ".start") {
		return "▶"
	}
	if strings.HasSuffix(op, ".end") {
		if level == "ERROR" {
			return "✗"
		}
		return "✓"
	}
	return "•"
}

func (l *Logger) getDepth(span *Span) int {
	depth := 0
	parentID := span.parentID

	// Simple depth calculation based on parent
	for parentID != "" {
		depth++
		// In a real implementation, track parent spans in a map
		break
	}

	return depth
}

func SpanFromContext(ctx context.Context) *Span {
	span, _ := ctx.Value(spanContextKey).(*Span)
	return span
}

func RunIDFromContext(ctx context.Context) string {
	runID, _ := ctx.Value(runIDContextKey).(string)
	return runID
}

func Info(ctx context.Context, operation string, fields map[string]interface{}) {
	span := SpanFromContext(ctx)
	if span != nil {
		span.Event(operation, fields)
	}
}
