package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	dsgo "github.com/assagman/dsgo"
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

func TestModelCatalog_SyncedWithModelsDev(t *testing.T) {
	if os.Getenv("DSGO_SKIP_MODELS_DEV_SYNC") == "1" {
		t.Skip("DSGO_SKIP_MODELS_DEV_SYNC=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsDevAPIURL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetch %s: %v", modelsDevAPIURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close models.dev response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch %s: unexpected status %s", modelsDevAPIURL, resp.Status)
	}

	var root map[string]modelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		t.Fatalf("decode models.dev api: %v", err)
	}

	openaiProvider, ok := root["openai"]
	if !ok {
		t.Fatalf("models.dev response missing provider 'openai'")
	}
	openrouterProvider, ok := root["openrouter"]
	if !ok {
		t.Fatalf("models.dev response missing provider 'openrouter'")
	}

	verifyProvider(t, "openai", openaiProvider.Models, true)
	verifyProvider(t, "openrouter", openrouterProvider.Models, false)
}

func verifyProvider(t *testing.T, provider string, remote map[string]modelsDevModel, expectAlias bool) {
	t.Helper()

	catalogModels := dsgo.ListModelsByProvider(provider)
	catalog := make(map[string]dsgo.Model, len(catalogModels))
	for _, m := range catalogModels {
		catalog[m.ID] = m
	}

	remoteIDs := make(map[string]modelsDevModel, len(remote))
	for key, entry := range remote {
		id := entry.ID
		if id == "" {
			id = key
		}
		remoteIDs[provider+"/"+id] = entry
	}

	var missing []string
	for id := range remoteIDs {
		if _, ok := catalog[id]; !ok {
			missing = append(missing, id)
		}
	}

	var extra []string
	for id := range catalog {
		if _, ok := remoteIDs[id]; !ok {
			extra = append(extra, id)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 || len(extra) > 0 {
		msg := fmt.Sprintf("provider=%s: catalog out of sync with models.dev", provider)
		if len(missing) > 0 {
			msg += fmt.Sprintf("\nmissing (%d): %s", len(missing), strings.Join(limitList(missing, 25), ", "))
		}
		if len(extra) > 0 {
			msg += fmt.Sprintf("\nextra (%d): %s", len(extra), strings.Join(limitList(extra, 25), ", "))
		}
		t.Fatal(msg)
	}

	for id, remoteModel := range remoteIDs {
		local := catalog[id]

		if expectAlias {
			if len(local.Aliases) == 0 {
				t.Fatalf("%s: expected at least one alias", id)
			}
			// For OpenAI we register the raw model id as alias.
			if local.Aliases[0] != strings.TrimPrefix(id, provider+"/") {
				t.Fatalf("%s: alias[0]=%q want %q", id, local.Aliases[0], strings.TrimPrefix(id, provider+"/"))
			}
		} else if len(local.Aliases) != 0 {
			t.Fatalf("%s: expected no aliases, got %v", id, local.Aliases)
		}

		// Modalities are intentionally forced to text-only for now.
		if !stringSliceEqual(local.Modalities.Input, []string{"text"}) {
			t.Fatalf("%s: Modalities.Input=%v want [text]", id, local.Modalities.Input)
		}
		if !stringSliceEqual(local.Modalities.Output, []string{"text"}) {
			t.Fatalf("%s: Modalities.Output=%v want [text]", id, local.Modalities.Output)
		}

		assertFloatClose(t, id+" pricing.prompt", local.Pricing.PromptPrice, remoteModel.Cost.Input)
		assertFloatClose(t, id+" pricing.completion", local.Pricing.CompletionPrice, remoteModel.Cost.Output)
		assertFloatClose(t, id+" pricing.cache_read", local.Pricing.CacheReadPrice, remoteModel.Cost.CacheRead)

		if local.Limits.ContextTokens != remoteModel.Limit.Context {
			t.Fatalf("%s: Limits.ContextTokens=%d want %d", id, local.Limits.ContextTokens, remoteModel.Limit.Context)
		}
		if local.Limits.OutputTokens != remoteModel.Limit.Output {
			t.Fatalf("%s: Limits.OutputTokens=%d want %d", id, local.Limits.OutputTokens, remoteModel.Limit.Output)
		}

		wantCaps := dsgo.Capabilities{
			Attachment:       remoteModel.Attachment,
			Reasoning:        remoteModel.Reasoning,
			ToolCall:         remoteModel.ToolCall,
			StructuredOutput: remoteModel.StructuredOutput,
			Temperature:      remoteModel.Temperature,
		}
		if local.Capabilities != wantCaps {
			t.Fatalf("%s: Capabilities=%+v want %+v", id, local.Capabilities, wantCaps)
		}

		wantMeta := dsgo.Metadata{
			Name:        remoteModel.Name,
			Family:      remoteModel.Family,
			Knowledge:   remoteModel.Knowledge,
			ReleaseDate: remoteModel.ReleaseDate,
			LastUpdated: remoteModel.LastUpdated,
			OpenWeights: remoteModel.OpenWeights,
		}
		if local.Metadata != wantMeta {
			t.Fatalf("%s: Metadata=%+v want %+v", id, local.Metadata, wantMeta)
		}
	}
}

func assertFloatClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsNaN(want) {
		t.Fatalf("%s: NaN not allowed (got=%v want=%v)", label, got, want)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s: got=%v want=%v", label, got, want)
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func limitList(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	out := append([]string(nil), in[:n]...)
	out = append(out, fmt.Sprintf("... and %d more", len(in)-n))
	return out
}
