package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type ChatAgent struct {
	module dsgo.Module
}

func NewChatAgent(lm dsgo.LM) *ChatAgent {
	sig := dsgo.NewSignature("Parse user's learning request and extract key information").
		AddInput("query", dsgo.FieldTypeString, "User's learning request about Snowflake").
		AddOutput("learningObjectives", dsgo.FieldTypeString, "Identified learning objectives").
		AddOutput("skillLevel", dsgo.FieldTypeString, "Detected skill level: beginner, intermediate, advanced").
		AddOutput("estimatedDuration", dsgo.FieldTypeString, "Estimated training duration").
		AddOutput("topicScope", dsgo.FieldTypeString, "Refined topic scope for research")

	return &ChatAgent{
		module: dsgo.NewPredict(sig, lm),
	}
}

func (a *ChatAgent) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return a.module.Forward(ctx, inputs)
}

func (a *ChatAgent) GetSignature() *dsgo.Signature {
	return a.module.GetSignature()
}

func (a *ChatAgent) Clone() dsgo.Module {
	return &ChatAgent{
		module: a.module.Clone(),
	}
}
