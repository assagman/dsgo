package ids

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func TestNewUUID(t *testing.T) {
	t.Run("generates valid UUID", func(t *testing.T) {
		uuid := NewUUID()

		// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		parts := strings.Split(uuid, "-")
		if len(parts) != 5 {
			t.Errorf("UUID has %d parts, want 5", len(parts))
		}

		if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
			t.Errorf("UUID format invalid: %s", uuid)
		}

		// Check total length (36 characters including dashes)
		if len(uuid) != 36 {
			t.Errorf("UUID length = %d, want 36", len(uuid))
		}
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			uuid := NewUUID()
			if seen[uuid] {
				t.Errorf("Generated duplicate UUID: %s", uuid)
			}
			seen[uuid] = true
		}
	})

	t.Run("version 4 UUID", func(t *testing.T) {
		uuid := NewUUID()
		parts := strings.Split(uuid, "-")

		// Version 4 UUIDs have '4' as the first character of the 3rd part
		if parts[2][0] != '4' {
			t.Errorf("UUID version = %c, want 4", parts[2][0])
		}
	})

	t.Run("variant bits are correct", func(t *testing.T) {
		uuid := NewUUID()
		parts := strings.Split(uuid, "-")

		// Variant bits: 4th part should start with 8, 9, a, or b
		firstChar := parts[3][0]
		if firstChar != '8' && firstChar != '9' && firstChar != 'a' && firstChar != 'b' {
			t.Errorf("UUID variant = %c, want 8, 9, a, or b", firstChar)
		}
	})
}

func TestNewShortID(t *testing.T) {
	t.Run("generates valid short ID", func(t *testing.T) {
		id := NewShortID()

		// Should be 8 hex characters
		if len(id) != 8 {
			t.Errorf("Short ID length = %d, want 8", len(id))
		}

		// Should only contain hex characters
		for _, c := range id {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				t.Errorf("Short ID contains non-hex character: %c", c)
			}
		}
	})

	t.Run("generates unique short IDs", func(t *testing.T) {
		seen := make(map[string]bool)
		collisions := 0
		for i := 0; i < 1000; i++ {
			id := NewShortID()
			if seen[id] {
				collisions++
			}
			seen[id] = true
		}

		// With 8 hex chars (32 bits), collisions are possible but should be rare
		// We expect very few collisions in 1000 iterations
		if collisions > 5 {
			t.Errorf("Too many collisions: %d in 1000 iterations", collisions)
		}
	})
}

// errorReader always returns an error when Read is called
type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("random reader error")
}

func TestNewUUID_ErrorPanics(t *testing.T) {
	// Save original reader
	originalReader := rand.Reader
	defer func() {
		// Restore original reader
		rand.Reader = originalReader
	}()

	// Replace with error reader
	rand.Reader = errorReader{}

	// Recover from panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewUUID did not panic on reader error")
		}
	}()

	// This should panic
	NewUUID()
}

func TestNewShortID_ErrorPanics(t *testing.T) {
	// Save original reader
	originalReader := rand.Reader
	defer func() {
		// Restore original reader
		rand.Reader = originalReader
	}()

	// Replace with error reader
	rand.Reader = errorReader{}

	// Recover from panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewShortID did not panic on reader error")
		}
	}()

	// This should panic
	NewShortID()
}
