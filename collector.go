package dsgo

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// MemoryCollector stores history entries in a ring buffer (in-memory)
type MemoryCollector struct {
	entries []*HistoryEntry
	size    int
	head    int
	count   int64
	mu      sync.RWMutex
}

// NewMemoryCollector creates a new in-memory ring buffer collector
func NewMemoryCollector(size int) *MemoryCollector {
	if size <= 0 {
		size = 100 // Default size
	}
	return &MemoryCollector{
		entries: make([]*HistoryEntry, size),
		size:    size,
		head:    0,
		count:   0,
	}
}

// Collect adds a history entry to the ring buffer
func (c *MemoryCollector) Collect(entry *HistoryEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.head] = entry
	c.head = (c.head + 1) % c.size
	c.count++

	return nil
}

// Close is a no-op for memory collector
func (c *MemoryCollector) Close() error {
	return nil
}

// GetAll returns all entries in the buffer (oldest first)
func (c *MemoryCollector) GetAll() []*HistoryEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.count < int64(c.size) {
		// Buffer not full yet - return from start to head
		result := make([]*HistoryEntry, 0, c.head)
		for i := 0; i < c.head; i++ {
			if c.entries[i] != nil {
				result = append(result, c.entries[i])
			}
		}
		return result
	}

	// Buffer full - return from head (oldest) to end, then start to head
	result := make([]*HistoryEntry, 0, c.size)
	for i := c.head; i < c.size; i++ {
		if c.entries[i] != nil {
			result = append(result, c.entries[i])
		}
	}
	for i := 0; i < c.head; i++ {
		if c.entries[i] != nil {
			result = append(result, c.entries[i])
		}
	}
	return result
}

// GetLast returns the last N entries (most recent first)
func (c *MemoryCollector) GetLast(n int) []*HistoryEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if n <= 0 {
		return []*HistoryEntry{}
	}

	all := c.getAllUnsafe()
	if n >= len(all) {
		// Reverse the entire slice
		result := make([]*HistoryEntry, len(all))
		for i, entry := range all {
			result[len(all)-1-i] = entry
		}
		return result
	}

	// Get last n entries in reverse order
	result := make([]*HistoryEntry, n)
	for i := 0; i < n; i++ {
		result[i] = all[len(all)-1-i]
	}
	return result
}

// getAllUnsafe returns all entries without locking (helper method)
func (c *MemoryCollector) getAllUnsafe() []*HistoryEntry {
	if c.count < int64(c.size) {
		result := make([]*HistoryEntry, 0, c.head)
		for i := 0; i < c.head; i++ {
			if c.entries[i] != nil {
				result = append(result, c.entries[i])
			}
		}
		return result
	}

	result := make([]*HistoryEntry, 0, c.size)
	for i := c.head; i < c.size; i++ {
		if c.entries[i] != nil {
			result = append(result, c.entries[i])
		}
	}
	for i := 0; i < c.head; i++ {
		if c.entries[i] != nil {
			result = append(result, c.entries[i])
		}
	}
	return result
}

// Count returns the total number of entries collected
func (c *MemoryCollector) Count() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}

// Len returns the current number of entries in the buffer
func (c *MemoryCollector) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.count < int64(c.size) {
		return c.head
	}
	return c.size
}

// Clear removes all entries from the buffer
func (c *MemoryCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make([]*HistoryEntry, c.size)
	c.head = 0
	c.count = 0
}

// JSONLCollector writes history entries to a JSONL file
type JSONLCollector struct {
	file  *os.File
	mu    sync.Mutex
	path  string
	count int64
}

// NewJSONLCollector creates a new JSONL collector
func NewJSONLCollector(path string) (*JSONLCollector, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}

	return &JSONLCollector{
		file:  file,
		path:  path,
		count: 0,
	}, nil
}

// Collect writes a history entry to the JSONL file
func (c *JSONLCollector) Collect(entry *HistoryEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal history entry: %w", err)
	}

	if _, err := c.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to JSONL file: %w", err)
	}

	c.count++
	return nil
}

// Close closes the JSONL file
func (c *JSONLCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// Count returns the number of entries written
func (c *JSONLCollector) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// Path returns the file path
func (c *JSONLCollector) Path() string {
	return c.path
}

// CompositeCollector sends history entries to multiple collectors
type CompositeCollector struct {
	collectors []Collector
}

// NewCompositeCollector creates a new composite collector
func NewCompositeCollector(collectors ...Collector) *CompositeCollector {
	return &CompositeCollector{
		collectors: collectors,
	}
}

// Collect sends the entry to all collectors
func (c *CompositeCollector) Collect(entry *HistoryEntry) error {
	var firstError error
	for _, collector := range c.collectors {
		if err := collector.Collect(entry); err != nil && firstError == nil {
			firstError = err
		}
	}
	return firstError
}

// Close closes all collectors
func (c *CompositeCollector) Close() error {
	var errs []error
	for _, collector := range c.collectors {
		if err := collector.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d collector(s): %v", len(errs), errs)
	}
	return nil
}

// Add adds a collector to the composite
func (c *CompositeCollector) Add(collector Collector) {
	c.collectors = append(c.collectors, collector)
}

// Len returns the number of collectors
func (c *CompositeCollector) Len() int {
	return len(c.collectors)
}
