package trainer

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/assagman/dsgo/examples/snowflake_trainer/types"
)

//go:embed templates/*.md templates/*.html.tmpl
var templateFS embed.FS

type ReportGenerator struct {
	templates *template.Template
}

func NewReportGenerator() *ReportGenerator {
	// Parse templates with custom functions
	funcMap := template.FuncMap{
		"toCodeBlock":      toMarkdownCodeBlock,
		"toTable":          toMarkdownTable,
		"toList":           toMarkdownList,
		"escapeMarkdown":   escapeMarkdown,
		"formatQuiz":       formatQuizQuestion,
		"formatExercise":   formatExercise,
		"formatModuleQuiz": formatModuleQuiz,
		"countQuestions":   countQuestions,
		"formatDate":       formatDate,
		"toLower":          strings.ToLower,
		"replace":          strings.Replace,
		"add":              func(a, b int) int { return a + b },
	}

	tmpl := template.Must(template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/*.md"))

	return &ReportGenerator{
		templates: tmpl,
	}
}

func (g *ReportGenerator) Generate(config types.ReportConfig) (string, error) {
	// Prepare template data
	data := map[string]any{
		"Title":            config.Title,
		"Topic":            config.Topic,
		"SkillLevel":       config.SkillLevel,
		"EstimatedTime":    config.EstimatedTime,
		"Modules":          config.Modules,
		"Quizzes":          config.Quizzes,
		"Exercises":        config.Exercises,
		"Challenges":       config.Challenges,
		"Glossary":         config.Glossary,
		"Resources":        config.Resources,
		"GeneratedAt":      config.GeneratedAt,
		"ResearchSources":  config.ResearchSources,
		"ReportTitle":      config.ReportTitle,
		"ReportDate":       config.ReportDate,
		"Author":           config.Author,
		"ExecutiveSummary": config.ExecutiveSummary,
		"KeyTakeaways":     config.KeyTakeaways,
	}

	// Execute main template
	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "report.md", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// Markdown formatting helpers

func toMarkdownCodeBlock(language, code string) string {
	if code == "" {
		return ""
	}
	return fmt.Sprintf("```%s\n%s\n```", language, strings.TrimSpace(code))
}

func toMarkdownTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	var buf bytes.Buffer

	// Header row
	buf.WriteString("| " + strings.Join(headers, " | ") + " |\n")

	// Separator row
	separator := make([]string, len(headers))
	for i := range headers {
		separator[i] = "---"
	}
	buf.WriteString("| " + strings.Join(separator, " | ") + " |\n")

	// Data rows
	for _, row := range rows {
		// Ensure row has same length as headers
		paddedRow := make([]string, len(headers))
		copy(paddedRow, row)
		for i := len(row); i < len(headers); i++ {
			paddedRow[i] = ""
		}
		buf.WriteString("| " + strings.Join(paddedRow, " | ") + " |\n")
	}

	return buf.String()
}

func toMarkdownList(items []string) string {
	if len(items) == 0 {
		return ""
	}

	var result []string
	for _, item := range items {
		result = append(result, fmt.Sprintf("- %s", item))
	}
	return strings.Join(result, "\n")
}

func escapeMarkdown(text string) string {
	// Escape markdown special characters
	specialChars := []string{"*", "_", "`", "[", "]", "#", "+", "-", ".", "!", "|", "{", "}", "(", ")"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}

func formatQuizQuestion(q types.QuizQuestion) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("### %s\n\n", q.Question))

	switch q.Type {
	case "multiple_choice":
		buf.WriteString("**Options:**\n\n")
		for i, option := range q.Options {
			buf.WriteString(fmt.Sprintf("%c) %s\n", 'A'+i, option))
		}
		buf.WriteString("\n")

		// Convert correct answer index to letter
		if correctIdx, ok := q.Correct.(float64); ok {
			correctLetter := string(rune('A' + int(correctIdx)))
			buf.WriteString(fmt.Sprintf("**Correct Answer:** %s\n\n", correctLetter))
		}

	case "true_false":
		buf.WriteString("**Options:**\n\n")
		buf.WriteString("A) True\n")
		buf.WriteString("B) False\n\n")

		if correct, ok := q.Correct.(bool); ok {
			if correct {
				buf.WriteString("**Correct Answer:** A) True\n\n")
			} else {
				buf.WriteString("**Correct Answer:** B) False\n\n")
			}
		}

	case "fill_blank", "code_completion":
		if q.Template != "" {
			buf.WriteString(fmt.Sprintf("**Template:**\n\n%s\n\n", toMarkdownCodeBlock("sql", q.Template)))
		}
		if answer, ok := q.Correct.(string); ok {
			buf.WriteString(fmt.Sprintf("**Answer:** %s\n\n", answer))
		}
	}

	if q.Explanation != "" {
		buf.WriteString(fmt.Sprintf("**Explanation:** %s\n\n", q.Explanation))
	}

	if len(q.Hints) > 0 {
		buf.WriteString("**Hints:**\n\n")
		for _, hint := range q.Hints {
			buf.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		buf.WriteString("\n")
	}

	if q.LearnMore != "" {
		buf.WriteString(fmt.Sprintf("**Learn More:** %s\n\n", q.LearnMore))
	}

	return buf.String()
}

func formatModuleQuiz(moduleID string, quiz types.Quiz) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("## Module Quiz: %s\n\n", moduleID))
	buf.WriteString(fmt.Sprintf("**Number of Questions:** %d\n\n", len(quiz.Questions)))

	for i, question := range quiz.Questions {
		buf.WriteString(fmt.Sprintf("### Question %d\n\n", i+1))
		buf.WriteString(formatQuizQuestion(question))
		buf.WriteString("\n---\n\n")
	}

	return buf.String()
}

func formatExercise(e types.PracticalExercise) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("### %s\n\n", e.Title))
	buf.WriteString(fmt.Sprintf("**Type:** %s\n\n", e.Type))
	buf.WriteString(fmt.Sprintf("**Instructions:** %s\n\n", e.Instructions))

	if e.Scenario != "" {
		buf.WriteString(fmt.Sprintf("**Scenario:** %s\n\n", e.Scenario))
	}

	if e.StarterCode != "" {
		buf.WriteString(fmt.Sprintf("**Starter Code:**\n\n%s\n\n", toMarkdownCodeBlock("sql", e.StarterCode)))
	}

	if len(e.Requirements) > 0 {
		buf.WriteString("**Requirements:**\n\n")
		for _, req := range e.Requirements {
			buf.WriteString(fmt.Sprintf("- %s\n", req))
		}
		buf.WriteString("\n")
	}

	if e.Solution != "" {
		buf.WriteString(fmt.Sprintf("**Solution:**\n\n%s\n\n", toMarkdownCodeBlock("sql", e.Solution)))
	}

	if e.Explanation != "" {
		buf.WriteString(fmt.Sprintf("**Explanation:** %s\n\n", e.Explanation))
	}

	if len(e.Hints) > 0 {
		buf.WriteString("**Hints:**\n\n")
		for _, hint := range e.Hints {
			buf.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		buf.WriteString("\n")
	}

	if e.Validation != "" {
		buf.WriteString(fmt.Sprintf("**Validation:** %s\n\n", e.Validation))
	}

	return buf.String()
}

func countQuestions(quizzes []types.Quiz) int {
	total := 0
	for _, quiz := range quizzes {
		total += len(quiz.Questions)
	}
	return total
}

func formatDate(t time.Time) string {
	return t.Format("January 2, 2006")
}
