package trainer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/assagman/dsgo/examples/snowflake_trainer/types"
)

//go:embed templates/*.html.tmpl
var htmlTemplateFS embed.FS

type HTMLReportGenerator struct {
	templates *template.Template
}

func NewHTMLReportGenerator() *HTMLReportGenerator {
	funcMap := template.FuncMap{
		"formatDate":     formatDate,
		"add":            func(a, b int) int { return a + b },
		"toLower":        strings.ToLower,
		"countQuestions": countQuestions,
	}

	tmpl := template.Must(
		template.New("html_report").
			Funcs(funcMap).
			ParseFS(htmlTemplateFS, "templates/*.html.tmpl"),
	)

	return &HTMLReportGenerator{templates: tmpl}
}

func (g *HTMLReportGenerator) Generate(config types.ReportConfig) (string, error) {
	var buf bytes.Buffer
	if err := g.templates.ExecuteTemplate(&buf, "report.html.tmpl", config); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}
	return buf.String(), nil
}
