package dsgo

import "context"

// Module is the base interface for all DSGo modules
type Module interface {
	Forward(ctx context.Context, inputs map[string]any) (map[string]any, error)
	GetSignature() *Signature
}
