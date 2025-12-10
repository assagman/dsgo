package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type CombinerAgent struct {
	module dsgo.Module
}

func NewCombinerAgent(lm dsgo.LM) *CombinerAgent {
	sig := dsgo.NewSignature("Synthesize research findings into a coherent Snowflake knowledge base. CRITICAL: Filter out any information not related to the Snowflake Data Cloud Platform. ALWAYS produce ALL required output fields after reasoning. Use the exact field markers shown; do not omit or rename fields. Keep outputs Snowflake-specific.").
		AddInput("originalQuery", dsgo.FieldTypeString, "Original learning request").
		AddInput("findings", dsgo.FieldTypeJSON, "JSON array of research findings").
		AddInput("skillLevel", dsgo.FieldTypeString, "Target skill level").
		AddOutput("unifiedKnowledge", dsgo.FieldTypeString, "Synthesized knowledge base (Snowflake focused only). Provide a cohesive narrative using only Snowflake concepts.").
		AddOutput("keyTakeaways", dsgo.FieldTypeString, "Most important Snowflake concepts to remember. Provide bullet-style points.").
		AddOutput("learningPath", dsgo.FieldTypeString, "Recommended order of Snowflake topics. Provide a clear sequence.").
		AddOutput("difficultyMapping", dsgo.FieldTypeString, "Topics mapped by difficulty. Map items to beginner/intermediate/advanced.").
		AddOutput("practicalExercises", dsgo.FieldTypeString, "Hands-on Snowflake exercise ideas. Provide concrete tasks.")

	// Few-shot example to reinforce required outputs and format
	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{
				"originalQuery": "Snowflake tasks overview",
				"findings":      `[{"coreConcepts":"Tasks automate SQL in Snowflake","explanations":"Tasks use CRON/N MINUTE schedules and DAGs via AFTER","examples":"Daily ETL task; downstream child task for QA","codeSnippets":"CREATE TASK etl_task WAREHOUSE=WH1 SCHEDULE='USING CRON 0 2 * * * UTC' AS CALL run_etl();","commonMistakes":"Forgetting to RESUME tasks; missing warehouse for non-serverless"}]`,
				"skillLevel":    "intermediate",
			},
			map[string]any{
				"unifiedKnowledge":   "Snowflake Tasks are scheduled units that run SQL or stored procedures. They can be chained with AFTER to form DAGs and can run serverless or on a warehouse. Schedules support CRON or N MINUTE syntax.",
				"keyTakeaways":       "Tasks automate SQL; use AFTER for dependencies; monitor via TASK_HISTORY; RESUME/SUSPEND controls; serverless available",
				"learningPath":       "1) Task basics 2) Scheduling 3) DAGs with AFTER 4) Monitoring & TASK_HISTORY 5) Serverless vs warehouse",
				"difficultyMapping":  "Beginner: create/resume task; Intermediate: DAGs with AFTER; Advanced: serverless + monitoring",
				"practicalExercises": "Create a daily task; add a child task with AFTER; query TASK_HISTORY; switch a task to serverless",
			},
		),
	}

	cot := dsgo.NewChainOfThought(sig, lm).WithDemos(demos)

	return &CombinerAgent{
		module: cot,
	}
}

func (a *CombinerAgent) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return a.module.Forward(ctx, inputs)
}

func (a *CombinerAgent) GetSignature() *dsgo.Signature {
	return a.module.GetSignature()
}

func (a *CombinerAgent) Clone() dsgo.Module {
	return &CombinerAgent{
		module: a.module.Clone(),
	}
}
