package agents

import (
	"context"

	"github.com/assagman/dsgo"
)

type CurriculumAgent struct {
	module dsgo.Module
}

func NewCurriculumAgent(lm dsgo.LM) *CurriculumAgent {
	sig := dsgo.NewSignature("Transform research findings into a structured training curriculum. "+
		"CRITICAL: Generate RICH, DETAILED content for each field - do not leave content empty. "+
		"Each module must have substantial content (at least 3-4 paragraphs), multiple key points, and real code examples. "+
		"Output flat, simple JSON structures that are easy to parse.").
		AddInput("unifiedKnowledge", dsgo.FieldTypeString, "Synthesized knowledge base overview").
		AddInput("researchFindings", dsgo.FieldTypeJSON, "JSON array of research findings").
		AddInput("keyTakeaways", dsgo.FieldTypeString, "Most important points").
		AddInput("learningPath", dsgo.FieldTypeString, "Recommended topic order").
		AddInput("practicalExercises", dsgo.FieldTypeString, "Exercise ideas").
		AddInput("learningObjectives", dsgo.FieldTypeString, "Target learning objectives").
		AddInput("skillLevel", dsgo.FieldTypeString, "Target skill level").
		AddInput("estimatedDuration", dsgo.FieldTypeString, "Target duration").
		AddOutput("modules", dsgo.FieldTypeJSON,
			"JSON array of modules. Each module MUST have: id (string), title (string), duration (string), "+
				"difficulty (string), learningObjectives (array of 3+ strings), content (string with 3-4 paragraphs of detailed explanation), "+
				"keyPoints (array of 4+ strings), warnings (array of 2+ strings), codeExamples (string with real SQL code)").
		AddOutput("quizzes", dsgo.FieldTypeJSON,
			"JSON array of quizzes. Each quiz MUST have: moduleId (string), title (string), questions (array of 3+ detailed question strings)").
		AddOutput("exercises", dsgo.FieldTypeJSON,
			"JSON array of exercises. Each MUST have: id, title, instructions (detailed multi-step), starterCode (real SQL), solution (complete SQL), difficulty (all strings)").
		AddOutput("challenges", dsgo.FieldTypeJSON,
			"JSON array of challenges. Each MUST have: id, title, description (detailed scenario), difficulty, duration (strings), goals (array of 3+ strings)").
		AddOutput("glossary", dsgo.FieldTypeJSON,
			"JSON array of glossary entries. Each MUST have: term (string), definition (string with 2+ sentences)").
		AddOutput("resources", dsgo.FieldTypeJSON,
			"JSON array of resources. Each MUST have: title (string), url (string - real Snowflake docs URL), type (string)")

	// Few-shot example showing the expected content depth
	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{
				"unifiedKnowledge":   "Snowflake Tasks enable automated SQL scheduling using CRON or interval-based schedules.",
				"researchFindings":   `[{"coreConcepts":"Tasks are scheduled SQL units","explanations":"Tasks run SQL statements or stored procedures on a schedule. They can be chained using AFTER to form DAGs.","examples":"Daily ETL, hourly aggregation","codeSnippets":"CREATE TASK my_task...","commonMistakes":"Forgetting to RESUME"}]`,
				"keyTakeaways":       "Tasks automate SQL; AFTER creates dependencies; monitor with TASK_HISTORY",
				"learningPath":       "1) Task basics 2) Scheduling 3) DAGs 4) Monitoring",
				"practicalExercises": "Create a daily task; chain tasks with AFTER",
				"learningObjectives": "Master Snowflake Tasks",
				"skillLevel":         "intermediate",
				"estimatedDuration":  "2 hours",
			},
			map[string]any{
				"modules": []map[string]any{
					{
						"id":         "mod1",
						"title":      "Understanding Snowflake Tasks",
						"duration":   "45 min",
						"difficulty": "intermediate",
						"learningObjectives": []string{
							"Understand what Tasks are and when to use them",
							"Learn the difference between scheduled and dependent tasks",
							"Master CRON and interval scheduling syntax",
						},
						"content": "Snowflake Tasks are first-class database objects that allow you to schedule SQL statements or stored procedure calls. Unlike traditional cron jobs that run on external servers, Tasks are fully managed by Snowflake and benefit from the platform's scalability and reliability.\n\nTasks can run on a dedicated virtual warehouse or use Snowflake's serverless compute (available on Enterprise Edition and above). Serverless tasks automatically provision and scale compute resources, eliminating the need to manage warehouse sizing for scheduled workloads.\n\nThere are two types of task scheduling: time-based (using CRON expressions or minute intervals) and dependency-based (using the AFTER clause). Time-based tasks are ideal for regular batch jobs, while dependency-based tasks create DAGs (Directed Acyclic Graphs) for complex data pipelines where execution order matters.\n\nA key architectural consideration is that tasks must be explicitly resumed after creation or modification. This safety feature prevents accidental execution of new tasks and gives you control over when automated processes begin.",
						"keyPoints": []string{
							"Tasks are managed database objects for scheduling SQL execution",
							"Serverless tasks eliminate warehouse management overhead",
							"CRON syntax follows standard cron format: minute hour day month weekday",
							"AFTER clause creates task dependencies forming a DAG",
							"Tasks must be resumed with ALTER TASK ... RESUME to start execution",
						},
						"warnings": []string{
							"Forgetting to RESUME a task is the #1 mistake - newly created tasks are SUSPENDED by default",
							"Serverless tasks can have cold start latency on first execution after idle periods",
						},
						"codeExamples": "-- Create a daily ETL task at 2 AM UTC\nCREATE OR REPLACE TASK daily_etl_task\n  WAREHOUSE = compute_wh\n  SCHEDULE = 'USING CRON 0 2 * * * UTC'\nAS\n  CALL run_daily_etl();\n\n-- Create a dependent task that runs after the ETL\nCREATE OR REPLACE TASK validation_task\n  WAREHOUSE = compute_wh\n  AFTER daily_etl_task\nAS\n  CALL validate_etl_results();\n\n-- Resume both tasks to start the schedule\nALTER TASK validation_task RESUME;\nALTER TASK daily_etl_task RESUME;",
					},
				},
				"quizzes": []map[string]any{
					{
						"moduleId": "mod1",
						"title":    "Module 1 Quiz: Snowflake Tasks",
						"questions": []string{
							"What is the default state of a newly created task in Snowflake? (SUSPENDED - tasks must be explicitly resumed)",
							"Which Snowflake edition is required to use serverless tasks? (Enterprise Edition or higher)",
							"In a task DAG, if Task B has 'AFTER Task A', which task runs first? (Task A runs first, then Task B)",
						},
					},
				},
				"exercises": []map[string]any{
					{
						"id":           "ex1",
						"title":        "Create Your First Scheduled Task",
						"instructions": "1. Create a task that runs every 5 minutes to insert a timestamp into a log table\n2. Create the log table if it doesn't exist\n3. Resume the task and verify it runs\n4. Check TASK_HISTORY to see execution records\n5. Suspend the task when done",
						"starterCode":  "-- Create a log table\nCREATE TABLE IF NOT EXISTS task_log (\n  id INT AUTOINCREMENT,\n  run_time TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP()\n);\n\n-- Create your task here\nCREATE OR REPLACE TASK ???\n  WAREHOUSE = ???\n  SCHEDULE = ???\nAS\n  ???;",
						"solution":     "-- Create a log table\nCREATE TABLE IF NOT EXISTS task_log (\n  id INT AUTOINCREMENT,\n  run_time TIMESTAMP_NTZ DEFAULT CURRENT_TIMESTAMP()\n);\n\n-- Create the task\nCREATE OR REPLACE TASK log_timestamp_task\n  WAREHOUSE = compute_wh\n  SCHEDULE = '5 MINUTE'\nAS\n  INSERT INTO task_log (run_time) VALUES (CURRENT_TIMESTAMP());\n\n-- Resume the task\nALTER TASK log_timestamp_task RESUME;\n\n-- Check execution history\nSELECT * FROM TABLE(INFORMATION_SCHEMA.TASK_HISTORY()) WHERE NAME = 'LOG_TIMESTAMP_TASK';",
						"difficulty":   "beginner",
					},
				},
				"challenges": []map[string]any{
					{
						"id":          "ch1",
						"title":       "Build an ETL Pipeline with Task DAG",
						"description": "Your company needs a nightly ETL pipeline that: extracts data from a staging table, transforms it with aggregations, loads it to a reporting table, and sends a notification on completion. Design and implement this as a task DAG with proper error handling.",
						"difficulty":  "advanced",
						"duration":    "2 hours",
						"goals": []string{
							"Create a root task for the extract phase running at midnight",
							"Create transform and load tasks as dependent tasks using AFTER",
							"Implement a notification task that runs after load completes",
							"Use TASK_HISTORY to monitor the pipeline health",
						},
					},
				},
				"glossary": []map[string]any{
					{
						"term":       "Task DAG",
						"definition": "A Directed Acyclic Graph of tasks where child tasks are triggered by the completion of parent tasks using the AFTER clause. DAGs ensure proper execution order for complex data pipelines.",
					},
					{
						"term":       "Serverless Tasks",
						"definition": "Tasks that use Snowflake-managed compute instead of a user-specified warehouse. Serverless tasks automatically scale and are billed per-second of compute time, ideal for variable workloads.",
					},
				},
				"resources": []map[string]any{
					{
						"title": "Introduction to Tasks - Snowflake Documentation",
						"url":   "https://docs.snowflake.com/en/user-guide/tasks-intro",
						"type":  "documentation",
					},
					{
						"title": "CREATE TASK - SQL Reference",
						"url":   "https://docs.snowflake.com/en/sql-reference/sql/create-task",
						"type":  "reference",
					},
				},
			},
		),
	}

	cot := dsgo.NewChainOfThought(sig, lm).WithDemos(demos)
	options := dsgo.DefaultGenerateOptions().Copy()
	options.MaxTokens = 32000
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
