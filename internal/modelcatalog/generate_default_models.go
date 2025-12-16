//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const modelsDevAPIURL = "https://models.dev/api.json"

type modelsDevProvider struct {
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Family           string `json:"family"`
	Attachment       bool   `json:"attachment"`
	Reasoning        bool   `json:"reasoning"`
	ToolCall         bool   `json:"tool_call"`
	StructuredOutput bool   `json:"structured_output"`
	Temperature      bool   `json:"temperature"`
	Knowledge        string `json:"knowledge"`
	ReleaseDate      string `json:"release_date"`
	LastUpdated      string `json:"last_updated"`
	OpenWeights      bool   `json:"open_weights"`
	Cost             struct {
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
		CacheRead float64 `json:"cache_read"`
	} `json:"cost"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}

type model struct {
	ID           string
	Aliases      []string
	Pricing      pricing
	Limits       limits
	Capabilities capabilities
	Modalities   modalities
	Metadata     metadata
}

type pricing struct {
	PromptPrice     float64
	CompletionPrice float64
	CacheReadPrice  float64
	CacheWritePrice float64
}

type limits struct {
	ContextTokens int
	OutputTokens  int
}

type capabilities struct {
	Attachment       bool
	Reasoning        bool
	ToolCall         bool
	StructuredOutput bool
	Temperature      bool
}

type modalities struct {
	Input  []string
	Output []string
}

type metadata struct {
	Name        string
	Family      string
	Knowledge   string
	ReleaseDate string
	LastUpdated string
	OpenWeights bool
}

func main() {
	root, err := fetchModelsDev(modelsDevAPIURL)
	if err != nil {
		fatal(err)
	}

	models := make([]model, 0, 512)
	models = append(models, modelsFromProvider(root, "openai", true)...)
	models = append(models, modelsFromProvider(root, "openrouter", false)...)

	// Keep stable ordering.
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })

	// Append mock models (not part of models.dev).
	models = append(models, model{
		ID:         "mock/gpt-4o-mini",
		Pricing:    pricing{PromptPrice: 0.15, CompletionPrice: 0.6},
		Limits:     limits{ContextTokens: 128000, OutputTokens: 16384},
		Modalities: modalities{Input: []string{"text"}, Output: []string{"text"}},
		Metadata:   metadata{Name: "Mock GPT-4o mini", Family: "mock"},
	})
	models = append(models, model{
		ID:         "mock/gpt-4o",
		Pricing:    pricing{PromptPrice: 2.5, CompletionPrice: 10},
		Limits:     limits{ContextTokens: 128000, OutputTokens: 16384},
		Modalities: modalities{Input: []string{"text"}, Output: []string{"text"}},
		Metadata:   metadata{Name: "Mock GPT-4o", Family: "mock"},
	})

	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })

	src, err := render(models)
	if err != nil {
		fatal(err)
	}

	if err := os.WriteFile("default_models.go", src, 0o644); err != nil {
		fatal(err)
	}
}

func fetchModelsDev(url string) (map[string]modelsDevProvider, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("fetch %s: unexpected status %s (%s)", url, resp.Status, strings.TrimSpace(string(b)))
	}

	var root map[string]modelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode models.dev api: %w", err)
	}
	return root, nil
}

func modelsFromProvider(root map[string]modelsDevProvider, provider string, addAlias bool) []model {
	p, ok := root[provider]
	if !ok {
		fatal(fmt.Errorf("models.dev response missing provider %q", provider))
	}

	out := make([]model, 0, len(p.Models))
	for key, entry := range p.Models {
		id := entry.ID
		if id == "" {
			id = key
		}
		canonical := provider + "/" + id

		aliases := []string(nil)
		if addAlias {
			aliases = []string{id}
		}

		out = append(out, model{
			ID:      canonical,
			Aliases: aliases,
			Pricing: pricing{
				PromptPrice:     entry.Cost.Input,
				CompletionPrice: entry.Cost.Output,
				CacheReadPrice:  entry.Cost.CacheRead,
			},
			Limits: limits{
				ContextTokens: entry.Limit.Context,
				OutputTokens:  entry.Limit.Output,
			},
			Capabilities: capabilities{
				Attachment:       entry.Attachment,
				Reasoning:        entry.Reasoning,
				ToolCall:         entry.ToolCall,
				StructuredOutput: entry.StructuredOutput,
				Temperature:      entry.Temperature,
			},
			// Note: modalities are intentionally forced to text-only for now.
			Modalities: modalities{Input: []string{"text"}, Output: []string{"text"}},
			Metadata: metadata{
				Name:        entry.Name,
				Family:      entry.Family,
				Knowledge:   entry.Knowledge,
				ReleaseDate: entry.ReleaseDate,
				LastUpdated: entry.LastUpdated,
				OpenWeights: entry.OpenWeights,
			},
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func render(models []model) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("package modelcatalog\n\n")
	b.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	b.WriteString("//\n")
	b.WriteString("// Default supported model list.\n")
	b.WriteString("//\n")
	b.WriteString("// This catalog is authoritative for core.NewLM/dsgo.NewLM.\n")
	b.WriteString("//\n")
	b.WriteString("// Source of truth for OpenAI/OpenRouter models: https://models.dev/api.json\n")
	b.WriteString("// Note: Modalities are forced to text-only for now.\n")
	b.WriteString("//go:generate go run ./generate_default_models.go\n\n")
	b.WriteString("func init() {\n")
	b.WriteString("\tdefaults := []Model{\n")

	for _, m := range models {
		fmt.Fprintf(&b, "\t\t{\n")
		fmt.Fprintf(&b, "\t\t\tID: %q,\n", m.ID)
		if len(m.Aliases) > 0 {
			fmt.Fprintf(&b, "\t\t\tAliases: []string{%s},\n", quoteList(m.Aliases))
		}
		fmt.Fprintf(&b, "\t\t\tPricing: Pricing{PromptPrice: %v, CompletionPrice: %v, CacheReadPrice: %v, CacheWritePrice: %v},\n",
			m.Pricing.PromptPrice, m.Pricing.CompletionPrice, m.Pricing.CacheReadPrice, m.Pricing.CacheWritePrice)
		fmt.Fprintf(&b, "\t\t\tLimits: Limits{ContextTokens: %d, OutputTokens: %d},\n", m.Limits.ContextTokens, m.Limits.OutputTokens)
		fmt.Fprintf(&b, "\t\t\tCapabilities: Capabilities{Attachment: %v, Reasoning: %v, ToolCall: %v, StructuredOutput: %v, Temperature: %v},\n",
			m.Capabilities.Attachment, m.Capabilities.Reasoning, m.Capabilities.ToolCall, m.Capabilities.StructuredOutput, m.Capabilities.Temperature)
		fmt.Fprintf(&b, "\t\t\tModalities: Modalities{Input: []string{%s}, Output: []string{%s}},\n",
			quoteList(m.Modalities.Input), quoteList(m.Modalities.Output))
		fmt.Fprintf(&b, "\t\t\tMetadata: Metadata{Name: %q, Family: %q, Knowledge: %q, ReleaseDate: %q, LastUpdated: %q, OpenWeights: %v},\n",
			m.Metadata.Name, m.Metadata.Family, m.Metadata.Knowledge, m.Metadata.ReleaseDate, m.Metadata.LastUpdated, m.Metadata.OpenWeights)
		fmt.Fprintf(&b, "\t\t},\n")
	}

	b.WriteString("\t}\n\n")
	b.WriteString("\tfor _, m := range defaults {\n")
	b.WriteString("\t\t// Ignore errors in init: duplicates are a programmer error.\n")
	b.WriteString("\t\t_ = RegisterModel(m)\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return format.Source(b.Bytes())
}

func quoteList(in []string) string {
	if len(in) == 0 {
		return ""
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return strings.Join(out, ", ")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
