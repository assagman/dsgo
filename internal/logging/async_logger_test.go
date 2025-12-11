package logging

import (
	"context"

	"sync"
	"testing"
	"time"
)

func TestDefaultLogger_BasicLogging(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.FlushInterval = 10 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	ctx := WithRequestID(context.Background(), "req-123")
	dl.Info(ctx, "test message", map[string]any{"key": "value"})

	time.Sleep(50 * time.Millisecond)

	stats := dl.GetStats()
	if stats["buffer_length"].(int) != 0 {
		t.Errorf("Expected empty buffer, got %d", stats["buffer_length"])
	}
	if stats["info_logged"].(int64) != 1 {
		t.Errorf("Expected 1 info log, got %d", stats["info_logged"])
	}
}

func TestDefaultLogger_BatchProcessing(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.BatchSize = 3
	config.FlushInterval = 100 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	// Send multiple messages
	for i := 0; i < 5; i++ {
		dl.Info(context.Background(), "message", map[string]any{"index": i})
	}

	// Wait for batch processing
	time.Sleep(150 * time.Millisecond)

	stats := dl.GetStats()
	if stats["info_logged"].(int64) != 5 {
		t.Errorf("Expected 5 info logs, got %d", stats["info_logged"])
	}
	if stats["buffer_length"].(int) != 0 {
		t.Errorf("Expected empty buffer, got %d", stats["buffer_length"])
	}
}

func TestDefaultLogger_GracefulShutdown(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.FlushTimeout = 1 * time.Second
	config.FlushInterval = 10 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)

	// Send some messages
	for i := 0; i < 3; i++ {
		dl.Info(context.Background(), "message", map[string]any{"index": i})
	}

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Stop should flush all
	dl.Stop()

	stats := dl.GetStats()
	if stats["info_logged"].(int64) != 3 {
		t.Errorf("Expected 3 info logs after shutdown, got %d", stats["info_logged"])
	}
}

func TestDefaultLogger_DropWhenFull(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.BufferSize = 2
	config.DropWhenFull = true
	config.BatchSize = 1
	config.FlushInterval = 10 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	// Send more than buffer
	for i := 0; i < 10; i++ {
		dl.Info(context.Background(), "message", map[string]any{"index": i})
	}

	time.Sleep(200 * time.Millisecond)

	stats := dl.GetStats()
	if dropped := stats["dropped_count"].(int64); dropped == 0 {
		t.Error("Expected drops")
	}
	if logged := stats["info_logged"].(int64); logged == 0 {
		t.Error("Expected some logged")
	}
}

func TestDefaultLogger_BlockWhenFull(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.BufferSize = 2
	config.DropWhenFull = false
	config.FlushInterval = 10 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	done := make(chan bool, 1)
	go func() {
		for i := 0; i < 5; i++ {
			dl.Info(context.Background(), "message", map[string]any{"index": i})
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Blocking too long")
	}

	time.Sleep(200 * time.Millisecond)

	stats := dl.GetStats()
	if dropped := stats["dropped_count"].(int64); dropped > 0 {
		t.Errorf("No drops expected, got %d", dropped)
	}
	if stats["info_logged"].(int64) != 5 {
		t.Errorf("Expected 5 logs, got %d", stats["info_logged"])
	}
}

func TestDefaultLogger_ConcurrentLogging(t *testing.T) {
	config := DefaultConfig()
	config.Level = LevelDebug
	config.FlushInterval = 10 * time.Millisecond

	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				dl.Info(context.Background(), "message", map[string]any{
					"goroutine": goroutineID,
					"message":   j,
				})
			}
		}(i)
	}

	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	stats := dl.GetStats()
	expected := int64(numGoroutines * messagesPerGoroutine)
	if stats["info_logged"].(int64) != expected {
		t.Errorf("Expected %d logs, got %d", expected, stats["info_logged"])
	}
	if stats["dropped_count"].(int64) != 0 {
		t.Errorf("Expected no drops, got %d", stats["dropped_count"])
	}
}

func TestDefaultLogger_LevelFiltering(t *testing.T) {
	config := DefaultConfig()
	config.FlushInterval = 10 * time.Millisecond
	config.BatchSize = 1

	// Debug should not log at Info level
	config.Level = LevelInfo
	dl := NewDefaultLoggerWithConfig(config)
	defer dl.Stop()

	dl.Debug(context.Background(), "should not log", nil)
	time.Sleep(50 * time.Millisecond)
	stats := dl.GetStats()
	if stats["debug_logged"].(int64) != 0 {
		t.Error("Debug should not log at Info level")
	}

	// Info should log
	dl.Info(context.Background(), "should log", nil)
	time.Sleep(50 * time.Millisecond)
	stats = dl.GetStats()
	if stats["info_logged"].(int64) != 1 {
		t.Error("Info should log")
	}
}

func TestDefaultLogger_MultipleStartStop(t *testing.T) {
	config := DefaultConfig()
	dl := NewDefaultLoggerWithConfig(config)

	// Multiple starts safe
	// dl.Start() // no Start method, auto-starts in constructor
	// No Start method, auto start.

	// Multiple stops safe
	dl.Stop()
	dl.Stop()

	stats := dl.GetStats()
	if !stats["stopped"].(bool) {
		t.Error("Should be stopped")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BufferSize != 1000 {
		t.Errorf("BufferSize = %d, want 1000", config.BufferSize)
	}
	if config.FlushInterval != 100*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 100ms", config.FlushInterval)
	}
	if config.FlushTimeout != 5*time.Second {
		t.Errorf("FlushTimeout = %v, want 5s", config.FlushTimeout)
	}
	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if config.DropWhenFull != false {
		t.Errorf("DropWhenFull = %v, want false", config.DropWhenFull)
	}
	if config.MaxMemoryUsage != 10*1024*1024 {
		t.Errorf("MaxMemoryUsage = %d, want 10MB", config.MaxMemoryUsage)
	}
}
