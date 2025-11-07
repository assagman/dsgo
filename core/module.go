package core

import "context"

// Module is the base interface for all DSGo modules
type Module interface {
	Forward(ctx context.Context, inputs map[string]any) (*Prediction, error)
	GetSignature() *Signature
}
