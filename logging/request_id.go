package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// EnsureRequestID ensures the context has a request ID, creating one if necessary
func EnsureRequestID(ctx context.Context) context.Context {
	if GetRequestID(ctx) != "" {
		return ctx
	}
	return WithRequestID(ctx, GenerateRequestID())
}
