package yamlprogram

import (
	"fmt"
	"strings"
)

// Validate performs semantic validation of the spec.
//
// Assumes Normalize() has been applied.
func Validate(s *Spec) error {
	if s == nil {
		return fmt.Errorf("spec is nil")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(s.Signatures) == 0 {
		return fmt.Errorf("at least one signature must be defined")
	}
	if len(s.Modules) == 0 {
		return fmt.Errorf("at least one module must be defined")
	}
	if len(s.Pipeline) == 0 {
		return fmt.Errorf("pipeline must not be empty")
	}

	for sigName, sig := range s.Signatures {
		if err := validateSignature(sigName, sig); err != nil {
			return err
		}
	}

	if err := validateToolSources(s.ToolSources); err != nil {
		return err
	}

	for histName, hs := range s.Histories {
		if err := validateHistory(histName, hs); err != nil {
			return err
		}
	}

	for modName, mod := range s.Modules {
		if err := validateModule(s, modName, mod); err != nil {
			return err
		}
	}

	for i, step := range s.Pipeline {
		if _, ok := s.Modules[step]; !ok {
			return fmt.Errorf("pipeline step %d references unknown module %q", i+1, step)
		}
	}

	// Cycle detection across module references (Parallel/Program/BestOfN factory).
	if err := validateNoCycles(s); err != nil {
		return err
	}

	// Validate that MCC steps have a completion-producing predecessor.
	if err := validateMCCPreconditions(s, s.Pipeline); err != nil {
		return err
	}

	return nil
}

func validateHistory(name string, h HistorySpec) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("history name must not be empty")
	}
	if h.Limit != nil && *h.Limit < 0 {
		return fmt.Errorf("history %q: limit must be >= 0", name)
	}
	return nil
}

func validateSignature(name string, sig SignatureSpec) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("signature name must not be empty")
	}
	if strings.TrimSpace(sig.Desc) == "" {
		return fmt.Errorf("signature %q: desc is required", name)
	}
	if len(sig.In) == 0 {
		return fmt.Errorf("signature %q: in must not be empty", name)
	}
	if len(sig.Out) == 0 {
		return fmt.Errorf("signature %q: out must not be empty", name)
	}

	for fieldName, f := range sig.In {
		if err := validateFieldType(name, "in", fieldName, f); err != nil {
			return err
		}
	}
	for fieldName, f := range sig.Out {
		if err := validateFieldType(name, "out", fieldName, f); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldType(sigName, dir, fieldName string, f FieldSpec) error {
	if strings.TrimSpace(fieldName) == "" {
		return fmt.Errorf("signature %q: %s field name must not be empty", sigName, dir)
	}
	valid := map[string]bool{
		"string":   true,
		"int":      true,
		"float":    true,
		"bool":     true,
		"json":     true,
		"image":    true,
		"datetime": true,
		"enum":     true,
		"array":    true,
	}
	if !valid[f.Type] {
		return fmt.Errorf("signature %q: %s.%s has invalid type %q", sigName, dir, fieldName, f.Type)
	}
	if f.Type == "enum" && len(f.Values) == 0 {
		return fmt.Errorf("signature %q: %s.%s is enum but values is empty", sigName, dir, fieldName)
	}
	if f.Type == "array" && f.Items != "" {
		validItems := map[string]bool{
			"string": true,
			"int":    true,
			"float":  true,
			"bool":   true,
			"json":   true,
		}
		if !validItems[f.Items] {
			return fmt.Errorf("signature %q: %s.%s has invalid array items type %q", sigName, dir, fieldName, f.Items)
		}
	}
	return nil
}

var validMCPTypes = map[string]bool{
	"exa":        true,
	"jina":       true,
	"tavily":     true,
	"filesystem": true,
	"shell":      true,
	"custom":     true,
}

func validateToolSources(ts ToolSources) error {
	for name, src := range ts.MCP {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("tool_sources.mcp: key must not be empty")
		}
		if !validMCPTypes[name] {
			return fmt.Errorf("tool_sources.mcp.%s: invalid MCP type (valid: exa, jina, tavily, filesystem, shell, custom)", name)
		}
		if name == "custom" && strings.TrimSpace(src.URL) == "" {
			return fmt.Errorf("tool_sources.mcp.custom: url is required")
		}
		if name == "filesystem" && len(src.AllowedDirs) == 0 {
			return fmt.Errorf("tool_sources.mcp.filesystem: allowed_dirs is required")
		}
	}

	return nil
}

// isValidToolSource checks if a source name is defined in ToolSources.
// Valid sources are: "builtin" (if builtin tools are defined) or any MCP type key.
func isValidToolSource(ts ToolSources, source string) bool {
	if source == "builtin" {
		return len(ts.Builtin) > 0
	}
	_, ok := ts.MCP[source]
	return ok
}

func validateModule(s *Spec, name string, mod ModuleSpec) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("modules: key must not be empty")
	}

	validKinds := map[string]bool{
		"predict":                true,
		"chain_of_thought":       true,
		"react":                  true,
		"refine":                 true,
		"best_of_n":              true,
		"program_of_thought":     true,
		"program":                true,
		"parallel":               true,
		"multi_chain_comparison": true,
	}
	if !validKinds[mod.Kind] {
		return fmt.Errorf("module %q: invalid kind %q", name, mod.Kind)
	}

	requiresSig := map[string]bool{
		"predict":                true,
		"chain_of_thought":       true,
		"react":                  true,
		"refine":                 true,
		"program_of_thought":     true,
		"multi_chain_comparison": true,
	}
	if requiresSig[mod.Kind] {
		if strings.TrimSpace(mod.Sig) == "" {
			return fmt.Errorf("module %q (%s): sig is required", name, mod.Kind)
		}
		if _, ok := s.Signatures[mod.Sig]; !ok {
			return fmt.Errorf("module %q: references unknown signature %q", name, mod.Sig)
		}
	}

	if mod.History != "" {
		if _, ok := s.Histories[mod.History]; !ok {
			return fmt.Errorf("module %q: references unknown history %q", name, mod.History)
		}
	}

	switch mod.Kind {
	case "react":
		if len(mod.ReAct.Tools) == 0 {
			return fmt.Errorf("module %q (react): react.tools must not be empty", name)
		}
		for _, sel := range mod.ReAct.Tools {
			if sel.Source == "" {
				return fmt.Errorf("module %q (react): tool selection missing source", name)
			}
			if !isValidToolSource(s.ToolSources, sel.Source) {
				return fmt.Errorf("module %q (react): unknown tool source %q", name, sel.Source)
			}
			if len(sel.Include) == 0 {
				return fmt.Errorf("module %q (react): tool selection %q include must not be empty", name, sel.Source)
			}
		}

	case "refine":
		// No additional hard requirements.

	case "best_of_n":
		if strings.TrimSpace(mod.BestOfN.Of) == "" {
			return fmt.Errorf("module %q (best_of_n): best_of_n.of is required", name)
		}
		if _, ok := s.Modules[mod.BestOfN.Of]; !ok {
			return fmt.Errorf("module %q (best_of_n): best_of_n.of references unknown module %q", name, mod.BestOfN.Of)
		}
		if mod.BestOfN.N <= 0 {
			return fmt.Errorf("module %q (best_of_n): n must be positive", name)
		}
		if mod.BestOfN.Scorer.Kind == "" {
			return fmt.Errorf("module %q (best_of_n): scorer.kind is required", name)
		}
		if mod.BestOfN.Scorer.Kind == "confidence" && strings.TrimSpace(mod.BestOfN.Scorer.Field) == "" {
			return fmt.Errorf("module %q (best_of_n): scorer.field is required for confidence scorer", name)
		}

	case "program_of_thought":
		if strings.TrimSpace(mod.ProgramOfThought.Language) == "" {
			return fmt.Errorf("module %q (program_of_thought): language is required", name)
		}

	case "program":
		if len(mod.Program.Steps) == 0 {
			return fmt.Errorf("module %q (program): program.steps must not be empty", name)
		}
		for i, step := range mod.Program.Steps {
			if _, ok := s.Modules[step]; !ok {
				return fmt.Errorf("module %q (program): steps[%d] references unknown module %q", name, i, step)
			}
		}
		if err := validateMCCPreconditions(s, mod.Program.Steps); err != nil {
			return fmt.Errorf("module %q (program): %w", name, err)
		}

	case "parallel":
		mode := mod.Parallel.Mode
		if mode == "" {
			mode = "clone"
		}
		switch mode {
		case "clone":
			if strings.TrimSpace(mod.Parallel.Module) == "" {
				return fmt.Errorf("module %q (parallel): parallel.module is required for clone mode", name)
			}
			if _, ok := s.Modules[mod.Parallel.Module]; !ok {
				return fmt.Errorf("module %q (parallel): parallel.module references unknown module %q", name, mod.Parallel.Module)
			}
		case "instances":
			if len(mod.Parallel.Instances) == 0 {
				return fmt.Errorf("module %q (parallel): parallel.instances is required for instances mode", name)
			}
			for _, inst := range mod.Parallel.Instances {
				if _, ok := s.Modules[inst]; !ok {
					return fmt.Errorf("module %q (parallel): parallel.instances references unknown module %q", name, inst)
				}
			}
		case "factory":
			if len(mod.Parallel.Factory) == 0 {
				return fmt.Errorf("module %q (parallel): parallel.factory is required for factory mode", name)
			}
			for i, use := range mod.Parallel.Factory {
				if strings.TrimSpace(use) == "" {
					return fmt.Errorf("module %q (parallel): factory[%d] must not be empty", name, i)
				}
				if _, ok := s.Modules[use]; !ok {
					return fmt.Errorf("module %q (parallel): factory[%d] references unknown module %q", name, i, use)
				}
			}
		default:
			return fmt.Errorf("module %q (parallel): invalid mode %q", name, mode)
		}

	case "multi_chain_comparison":
		if mod.MultiChainComparison.Attempts <= 0 {
			return fmt.Errorf("module %q (multi_chain_comparison): attempts must be positive", name)
		}
	}

	return nil
}

func validateNoCycles(s *Spec) error {
	// Build adjacency list.
	adj := make(map[string][]string, len(s.Modules))
	for name, mod := range s.Modules {
		adj[name] = referencedModules(mod)
	}

	// DFS colors.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(adj))
	stack := make([]string, 0)

	var visit func(n string) error
	visit = func(n string) error {
		color[n] = gray
		stack = append(stack, n)
		for _, m := range adj[n] {
			if _, ok := adj[m]; !ok {
				continue
			}
			switch color[m] {
			case white:
				if err := visit(m); err != nil {
					return err
				}
			case gray:
				// cycle: stack ... m
				cycle := []string{}
				for i := len(stack) - 1; i >= 0; i-- {
					cycle = append([]string{stack[i]}, cycle...)
					if stack[i] == m {
						break
					}
				}
				cycle = append(cycle, m)
				return fmt.Errorf("module graph has cycle: %s", strings.Join(cycle, " -> "))
			case black:
				// ok
			}
		}
		color[n] = black
		stack = stack[:len(stack)-1]
		return nil
	}

	for n := range adj {
		if color[n] == white {
			if err := visit(n); err != nil {
				return err
			}
		}
	}
	return nil
}

func referencedModules(mod ModuleSpec) []string {
	var refs []string
	switch mod.Kind {
	case "best_of_n":
		if mod.BestOfN.Of != "" {
			refs = append(refs, mod.BestOfN.Of)
		}
	case "program":
		refs = append(refs, mod.Program.Steps...)
	case "parallel":
		mode := mod.Parallel.Mode
		if mode == "" {
			mode = "clone"
		}
		switch mode {
		case "clone":
			if mod.Parallel.Module != "" {
				refs = append(refs, mod.Parallel.Module)
			}
		case "instances":
			refs = append(refs, mod.Parallel.Instances...)
		case "factory":
			refs = append(refs, mod.Parallel.Factory...)
		}
	}
	return refs
}

func validateMCCPreconditions(s *Spec, steps []string) error {
	for i, stepName := range steps {
		mod, ok := s.Modules[stepName]
		if !ok {
			continue
		}
		if mod.Kind != "multi_chain_comparison" {
			continue
		}
		if i == 0 {
			return fmt.Errorf("multi_chain_comparison step %q must not be first; it requires completions from a previous step", stepName)
		}
		prevName := steps[i-1]
		prev, ok := s.Modules[prevName]
		if !ok {
			return fmt.Errorf("multi_chain_comparison step %q previous step %q not found", stepName, prevName)
		}
		if !producesCompletions(prev) {
			return fmt.Errorf("multi_chain_comparison step %q requires previous step %q to produce completions (Parallel with return_all=true or BestOfN with return_all=true)", stepName, prevName)
		}
	}
	return nil
}

func producesCompletions(mod ModuleSpec) bool {
	switch mod.Kind {
	case "parallel":
		// DSGo Parallel produces completions if return_all=true.
		if mod.Parallel.ReturnAll != nil {
			return *mod.Parallel.ReturnAll
		}
		// DSGo Parallel default return_all=true
		return true
	case "best_of_n":
		if mod.BestOfN.ReturnAll != nil {
			return *mod.BestOfN.ReturnAll
		}
		return false
	default:
		return false
	}
}
