package types

import "time"

// ChatAgent types
type ChatInput struct {
	Query string `dsgo:"input,desc=User's learning request about Snowflake"`
}

type ChatOutput struct {
	LearningObjectives string `dsgo:"output,desc=Identified learning objectives"`
	SkillLevel         string `dsgo:"output,desc=Detected skill level: beginner, intermediate, advanced"`
	EstimatedDuration  string `dsgo:"output,desc=Estimated training duration"`
	TopicScope         string `dsgo:"output,desc=Refined topic scope for research"`
}

// SupervisorAgent types
type SupervisorInput struct {
	Query              string `dsgo:"input,desc=Original learning request"`
	LearningObjectives string `dsgo:"input,desc=Identified learning objectives"`
	SkillLevel         string `dsgo:"input,desc=Target skill level"`
}

type SupervisorOutput struct {
	ResearchPlan  string `dsgo:"output,desc=Research plan mapping to learning modules"`
	SubTopics     string `dsgo:"output,desc=JSON array of research angles"`
	ModuleOutline string `dsgo:"output,desc=Preliminary training module structure"`
	Prerequisites string `dsgo:"output,desc=Required prerequisite knowledge"`
}

// WebResearchAgent types
type ResearchInput struct {
	SubTopic     string `dsgo:"input,desc=Specific research sub-topic"`
	LearningGoal string `dsgo:"input,desc=What learner should know after this module"`
	SkillLevel   string `dsgo:"input,desc=Target skill level for content depth"`
}

type ResearchOutput struct {
	CoreConcepts   string `dsgo:"output,desc=Key concepts to teach"`
	Explanations   string `dsgo:"output,desc=Clear explanations suitable for teaching"`
	Examples       string `dsgo:"output,desc=Concrete examples and use cases"`
	CodeSnippets   string `dsgo:"output,desc=Relevant code examples with explanations"`
	CommonMistakes string `dsgo:"output,desc=Common mistakes and misconceptions"`
	Sources        string `dsgo:"output,desc=Authoritative sources for further reading"`
	QuizMaterial   string `dsgo:"output,desc=Facts suitable for quiz questions"`
}

// CombinerAgent types
type CombinerInput struct {
	OriginalQuery string `dsgo:"input,desc=Original learning request"`
	Findings      string `dsgo:"input,desc=JSON array of research findings"`
	SkillLevel    string `dsgo:"input,desc=Target skill level"`
}

type CombinerOutput struct {
	UnifiedKnowledge   string `dsgo:"output,desc=Synthesized knowledge base"`
	KeyTakeaways       string `dsgo:"output,desc=Most important points to remember"`
	LearningPath       string `dsgo:"output,desc=Recommended order of topics"`
	DifficultyMapping  string `dsgo:"output,desc=Topics mapped by difficulty"`
	PracticalExercises string `dsgo:"output,desc=Hands-on exercise ideas"`
}

// CurriculumAgent types - SIMPLIFIED: flat string outputs
type CurriculumInput struct {
	UnifiedKnowledge   string `dsgo:"input,desc=Synthesized knowledge base"`
	LearningObjectives string `dsgo:"input,desc=Target learning objectives"`
	SkillLevel         string `dsgo:"input,desc=Target skill level"`
	EstimatedDuration  string `dsgo:"input,desc=Target training duration"`
}

type CurriculumOutput struct {
	Modules    string `dsgo:"output,desc=JSON array of learning modules"`
	Quizzes    string `dsgo:"output,desc=JSON array of quiz questions per module"`
	Exercises  string `dsgo:"output,desc=JSON array of hands-on exercises"`
	Challenges string `dsgo:"output,desc=JSON array of practical challenges"`
	Glossary   string `dsgo:"output,desc=Key terms and definitions"`
	Resources  string `dsgo:"output,desc=Additional learning resources"`
}

// SIMPLIFIED flat data structures - no deep nesting

// Module - flat structure with string content
type Module struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Duration           string   `json:"duration"`
	Difficulty         string   `json:"difficulty"`
	LearningObjectives []string `json:"learningObjectives"`
	Content            string   `json:"content"`   // Main content as markdown/text
	KeyPoints          []string `json:"keyPoints"` // Key takeaways
	Warnings           []string `json:"warnings"`  // Common mistakes
	CodeExamples       string   `json:"codeExamples,omitempty"`
}

// Quiz - flat structure
type Quiz struct {
	ModuleID  string   `json:"moduleId"`
	Title     string   `json:"title"`
	Questions []string `json:"questions"` // Simple string questions
}

// Exercise - flat structure
type Exercise struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	StarterCode  string `json:"starterCode,omitempty"`
	Solution     string `json:"solution,omitempty"`
	Difficulty   string `json:"difficulty"`
}

// Challenge - flat structure
type Challenge struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Difficulty  string   `json:"difficulty"`
	Duration    string   `json:"duration"`
	Goals       []string `json:"goals"`
}

// GlossaryEntry - simple key-value
type GlossaryEntry struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// Resource - simple link
type Resource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

// Research pipeline data structures
type ResearchFindings struct {
	SubTopics []ResearchOutput `json:"subTopics"`
	TotalCost float64          `json:"totalCost"`
	TotalTime time.Duration    `json:"totalTime"`
	Sources   []string         `json:"sources"`
	Unified   CombinerOutput   `json:"unified"`
}

type Curriculum struct {
	Modules         []Module        `json:"modules"`
	Quizzes         []Quiz          `json:"quizzes"`
	Exercises       []Exercise      `json:"exercises"`
	Challenges      []Challenge     `json:"challenges"`
	Glossary        []GlossaryEntry `json:"glossary"`
	Resources       []Resource      `json:"resources"`
	GeneratedAt     time.Time       `json:"generatedAt"`
	ResearchSources []string        `json:"researchSources"`
	Topic           string          `json:"topic"`
	SkillLevel      string          `json:"skillLevel"`
	EstimatedTime   string          `json:"estimatedTime"`
}

// Report configuration
type ReportConfig struct {
	Title            string          `json:"title"`
	Topic            string          `json:"topic"`
	SkillLevel       string          `json:"skillLevel"`
	EstimatedTime    string          `json:"estimatedTime"`
	Modules          []Module        `json:"modules"`
	Quizzes          []Quiz          `json:"quizzes"`
	Exercises        []Exercise      `json:"exercises"`
	Challenges       []Challenge     `json:"challenges"`
	Glossary         []GlossaryEntry `json:"glossary"`
	Resources        []Resource      `json:"resources"`
	GeneratedAt      time.Time       `json:"generatedAt"`
	ResearchSources  []string        `json:"researchSources"`
	ReportTitle      string          `json:"reportTitle"`
	ReportDate       string          `json:"reportDate"`
	Author           string          `json:"author"`
	ExecutiveSummary string          `json:"executiveSummary"`
	KeyTakeaways     []string        `json:"keyTakeaways"`
}
