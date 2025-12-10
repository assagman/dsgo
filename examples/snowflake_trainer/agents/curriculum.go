package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type CurriculumAgent struct {
	module dsgo.Module
}

func NewCurriculumAgent(lm dsgo.LM) *CurriculumAgent {
	sig := dsgo.NewSignature("Transform research findings into a rich, content-filled training curriculum. "+
		"CRITICAL: You MUST populate each module's sections with ACTUAL CONTENT from the research findings. "+
		"Each section's 'content' field should contain detailed explanations, examples, and code snippets. "+
		"DO NOT leave content empty or use placeholders. Use the research data directly. "+
		"Module Section Types: 'concept' (explain core concepts), 'example' (practical examples with code), "+
		"'practice' (interactive exercise), 'warning' (common mistakes to avoid). "+
		"Each section MUST have: title, type, content (FULL detailed text), keyPoints (array of takeaways).").
		AddInput("unifiedKnowledge", dsgo.FieldTypeString, "Synthesized knowledge base overview").
		AddInput("researchFindings", dsgo.FieldTypeJSON, "JSON array of detailed research findings with coreConcepts, explanations, examples, codeSnippets, commonMistakes, quizMaterial for each sub-topic").
		AddInput("keyTakeaways", dsgo.FieldTypeString, "Most important points to remember").
		AddInput("learningPath", dsgo.FieldTypeString, "Recommended order of topics").
		AddInput("practicalExercises", dsgo.FieldTypeString, "Hands-on exercise ideas from research").
		AddInput("learningObjectives", dsgo.FieldTypeString, "Target learning objectives").
		AddInput("skillLevel", dsgo.FieldTypeString, "Target skill level").
		AddInput("estimatedDuration", dsgo.FieldTypeString, "Target training duration").
		AddOutput("modules", dsgo.FieldTypeJSON, "JSON array of learning modules. Each module MUST include: "+
			"id (string), title (string), duration (string like '30 min'), difficulty (beginner/intermediate/advanced), "+
			"learningObjectives (array of strings), sections (array of section objects with type, title, content containing "+
			"FULL paragraphs of explanation and code examples from researchFindings, keyPoints array), "+
			"quiz (object with questions array), practicalExercise (object with type, title, instructions, starterCode, solution)").
		AddOutput("quizzes", dsgo.FieldTypeJSON, "JSON array of quiz objects with questions derived from quizMaterial in research").
		AddOutput("exercises", dsgo.FieldTypeJSON, "JSON array of hands-on exercises based on codeSnippets from research").
		AddOutput("challenges", dsgo.FieldTypeJSON, "JSON array of practical challenges").
		AddOutput("glossary", dsgo.FieldTypeJSON, "Key terms and definitions from coreConcepts").
		AddOutput("resources", dsgo.FieldTypeJSON, "Additional learning resources from sources")

	// Create ChainOfThought with increased token limit for comprehensive curriculum generation
	cot := dsgo.NewChainOfThought(sig, lm)
	options := dsgo.DefaultGenerateOptions().Copy()
	options.MaxTokens = 32000 // Increase from default 10000 to 32000 for comprehensive curriculum
	cot = cot.WithOptions(options)

	return &CurriculumAgent{
		module: cot,
	}
}

func (a *CurriculumAgent) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return a.module.Forward(ctx, inputs)
}

func (a *CurriculumAgent) GetSignature() *dsgo.Signature {
	return a.module.GetSignature()
}

func (a *CurriculumAgent) Clone() dsgo.Module {
	return &CurriculumAgent{
		module: a.module.Clone(),
	}
}
