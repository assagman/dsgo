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
	funcMap := template.FuncMap{
		"toCodeBlock":    toMarkdownCodeBlock,
		"toTable":        toMarkdownTable,
		"toList":         toMarkdownList,
		"escapeMarkdown": escapeMarkdown,
		"countQuestions": countQuestions,
		"formatDate":     formatDate,
		"toLower":        strings.ToLower,
		"replace":        strings.Replace,
		"add":            func(a, b int) int { return a + b },
	}

	tmpl := template.Must(template.New("report").Funcs(funcMap).ParseFS(templateFS, "templates/*.md"))

	return &ReportGenerator{
		templates: tmpl,
	}
}

func (g *ReportGenerator) Generate(config types.ReportConfig) (string, error) {
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

	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "report.md", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

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

	buf.WriteString("| " + strings.Join(headers, " | ") + " |\n")

	separator := make([]string, len(headers))
	for i := range headers {
		separator[i] = "---"
	}
	buf.WriteString("| " + strings.Join(separator, " | ") + " |\n")

	for _, row := range rows {
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
	specialChars := []string{"*", "_", "`", "[", "]", "#", "+", "-", ".", "!", "|", "{", "}", "(", ")"}
	result := text
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
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
