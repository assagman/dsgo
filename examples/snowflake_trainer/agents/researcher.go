package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type WebResearchAgent struct {
	module dsgo.Module
}

func NewWebResearchAgent(lm dsgo.LM, tools []dsgo.Tool) *WebResearchAgent {
	sig := dsgo.NewSignature("Execute focused web research on Snowflake platform using authoritative sources. CRITICAL: Only search snowflake.com, docs.snowflake.com, and community.snowflake.com domains.").
		AddInput("subTopic", dsgo.FieldTypeString, "Specific Snowflake sub-topic to research").
		AddInput("learningGoal", dsgo.FieldTypeString, "What learner should know about this Snowflake topic").
		AddInput("skillLevel", dsgo.FieldTypeString, "Target skill level for Snowflake content depth").
		AddInput("platform", dsgo.FieldTypeString, "Target platform - must be 'Snowflake Data Cloud Platform'").
		AddInput("domains", dsgo.FieldTypeString, "Comma-separated list of allowed domains: snowflake.com,docs.snowflake.com,community.snowflake.com").
		AddOutput("coreConcepts", dsgo.FieldTypeString, "Key Snowflake concepts to teach").
		AddOutput("explanations", dsgo.FieldTypeString, "Clear explanations of Snowflake features suitable for teaching").
		AddOutput("examples", dsgo.FieldTypeString, "Concrete Snowflake examples and use cases").
		AddOutput("codeSnippets", dsgo.FieldTypeString, "Relevant Snowflake SQL/code examples with explanations").
		AddOutput("commonMistakes", dsgo.FieldTypeString, "Common Snowflake mistakes and misconceptions").
		AddOutput("sources", dsgo.FieldTypeString, "Authoritative Snowflake sources for further reading (docs.snowflake.com, community.snowflake.com, etc.)").
		AddOutput("quizMaterial", dsgo.FieldTypeString, "Snowflake facts suitable for quiz questions")

	return &WebResearchAgent{
		module: dsgo.NewReAct(sig, lm, tools),
	}
}

func (a *WebResearchAgent) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return a.module.Forward(ctx, inputs)
}

func (a *WebResearchAgent) GetSignature() *dsgo.Signature {
	return a.module.GetSignature()
}

func (a *WebResearchAgent) Clone() dsgo.Module {
	return &WebResearchAgent{
		module: a.module.Clone(),
	}
}
