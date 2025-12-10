package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type SupervisorAgent struct {
	module dsgo.Module
}

func NewSupervisorAgent(lm dsgo.LM) *SupervisorAgent {
	sig := dsgo.NewSignature("Decompose learning request into research sub-topics that map to training modules").
		AddInput("query", dsgo.FieldTypeString, "Original learning request").
		AddInput("learningObjectives", dsgo.FieldTypeString, "Identified learning objectives").
		AddInput("skillLevel", dsgo.FieldTypeString, "Target skill level").
		AddOutput("researchPlan", dsgo.FieldTypeString, "Research plan mapping to learning modules").
		AddOutput("subTopics", dsgo.FieldTypeJSON, "JSON array of research angles").
		AddOutput("moduleOutline", dsgo.FieldTypeString, "Preliminary training module structure").
		AddOutput("prerequisites", dsgo.FieldTypeString, "Required prerequisite knowledge")

	return &SupervisorAgent{
		module: dsgo.NewChainOfThought(sig, lm),
	}
}

func (a *SupervisorAgent) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return a.module.Forward(ctx, inputs)
}

func (a *SupervisorAgent) GetSignature() *dsgo.Signature {
	return a.module.GetSignature()
}

func (a *SupervisorAgent) Clone() dsgo.Module {
	return &SupervisorAgent{
		module: a.module.Clone(),
	}
}
