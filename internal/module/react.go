package module

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/jsonutil"
	"github.com/assagman/dsgo/internal/logging"
)

const (
	MaxReActIterations = 10

	defaultReActMaxToolResultBytes = 16 * 1024
	// A conservative prompt budget used for trajectory rendering.
	// ReAct will still detect provider context overflow errors and shrink further.
	defaultReActMaxPromptBytes = 256 * 1024
)

// ReAct implements the Reasoning and Acting pattern.
//
// Key properties:
// - Uses an explicit trajectory object rather than mutating a shared []Message.
// - Encodes tool observations as a bounded JSON envelope.
// - Detects context overflow errors and truncates oldest trajectory steps.
// - Supports a prompted planning mode for LMs that don't support native tool calling.
// - Prefers a signature-valid in-loop final candidate; otherwise falls back to an extractor stage.
//
// ReAct instances are not safe for concurrent use. For parallel execution,
// call Clone() per concurrent worker and configure each clone before use.
//
// NOTE: This module is intentionally breaking-change tolerant, but dsgo.NewReAct
// remains available as the primary constructor.
type ReAct struct {
	Signature *core.Signature
	LM        core.LM
	Tools     []core.Tool
	Options   *core.GenerateOptions
	Adapter   core.Adapter
	History   *core.History  // Optional conversation history
	Demos     []core.Example // Optional few-shot examples

	MaxIterations int
	Verbose       bool

	// MaxToolResultBytes controls deterministic truncation of tool results
	// before they are added to the prompt.
	MaxToolResultBytes int

	// MaxPromptBytes is the soft prompt budget for trajectory rendering.
	// If 0, a conservative default is used.
	MaxPromptBytes int
}

// NewReAct creates a new ReAct module.
//
// Panics if signature or lm is nil to fail fast on invalid configuration.
func NewReAct(signature *core.Signature, lm core.LM, tools []core.Tool) *ReAct {
	if signature == nil {
		panic("NewReAct: signature cannot be nil")
	}
	if lm == nil {
		panic("NewReAct: LM cannot be nil")
	}
	if tools == nil {
		tools = []core.Tool{}
	}

	clonedTools := make([]core.Tool, len(tools))
	copy(clonedTools, tools)

	r := &ReAct{
		Signature:          signature,
		LM:                 lm,
		Tools:              clonedTools,
		Options:            core.DefaultGenerateOptions(),
		Adapter:            core.NewFallbackAdapter(),
		MaxIterations:      MaxReActIterations,
		Verbose:            false,
		MaxToolResultBytes: defaultReActMaxToolResultBytes,
		MaxPromptBytes:     defaultReActMaxPromptBytes,
	}

	// Allow env override for prompt budget in tests / tuning.
	if v := strings.TrimSpace(os.Getenv("DSGO_REACT_MAX_PROMPT_BYTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			r.MaxPromptBytes = n
		}
	}

	// AUTO-INJECT finish tool if not present.
	// finish is a termination signal; the final answer still goes through extraction.
	if r.findTool("finish") == nil {
		finishTool := buildFinishTool(signature)
		r.Tools = append(r.Tools, *finishTool)
	}

	return r
}

// WithOptions sets custom generation options.
// If nil is passed, defaults are used.
func (r *ReAct) WithOptions(options *core.GenerateOptions) *ReAct {
	if options == nil {
		r.Options = core.DefaultGenerateOptions()
	} else {
		r.Options = options
	}
	return r
}

// WithAdapter sets a custom adapter.
func (r *ReAct) WithAdapter(adapter core.Adapter) *ReAct {
	r.Adapter = adapter
	return r
}

// WithHistory sets conversation history for multi-turn interactions.
func (r *ReAct) WithHistory(history *core.History) *ReAct {
	r.History = history
	return r
}

// WithDemos sets few-shot examples for in-context learning.
func (r *ReAct) WithDemos(demos []core.Example) *ReAct {
	r.Demos = demos
	return r
}

// WithMaxIterations sets the maximum number of ReAct iterations.
func (r *ReAct) WithMaxIterations(max int) *ReAct {
	r.MaxIterations = max
	return r
}

// WithVerbose enables verbose logging.
func (r *ReAct) WithVerbose(verbose bool) *ReAct {
	r.Verbose = verbose
	return r
}

// GetSignature returns the module's signature.
func (r *ReAct) GetSignature() *core.Signature {
	return r.Signature
}

// Forward executes the ReAct loop.
func (r *ReAct) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleReAct, r.Signature.Description)
	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleReAct, time.Since(startTime), predErr)
	}()

	if err := r.Signature.ValidateInputs(inputs); err != nil {
		predErr = fmt.Errorf("input validation failed: %w", err)
		return nil, predErr
	}

	newMessages, err := r.Adapter.Format(r.Signature, inputs, r.Demos)
	if err != nil {
		predErr = fmt.Errorf("failed to format messages: %w", err)
		return nil, predErr
	}

	// Build base trajectory messages.
	var base []core.Message
	if systemPrompt := r.buildSystemPrompt(); systemPrompt != "" {
		base = append(base, core.Message{Role: "system", Content: systemPrompt})
	}
	if r.History != nil && !r.History.IsEmpty() {
		base = append(base, r.Adapter.FormatHistory(r.History)...)
	}
	base = append(base, newMessages...)

	traj := newReActTrajectory(base)
	term := newReActTermination()

	// Aggregate usage across all LM calls (loop + extraction).
	totalUsage := core.Usage{}

	iterationsUsed := 0
	extractionUsed := false
	for i := 0; i < r.MaxIterations; i++ {
		iterationsUsed = i + 1
		if err := ctx.Err(); err != nil {
			predErr = fmt.Errorf("context canceled before iteration %d: %w", i+1, err)
			return nil, predErr
		}

		if r.LM.SupportsTools() {
			usage, done, stepErr := r.iterationNativeTools(ctx, traj, term, i)
			totalUsage = addUsage(totalUsage, usage)
			if stepErr != nil {
				predErr = stepErr
				return nil, predErr
			}
			if done {
				break
			}
			continue
		}

		if r.hasRealTools() {
			// Prompted planning mode for non-tool LMs.
			usage, done, stepErr := r.iterationPrompted(ctx, traj, term, i)
			totalUsage = addUsage(totalUsage, usage)
			if stepErr != nil {
				predErr = stepErr
				return nil, predErr
			}
			if done {
				break
			}
			continue
		}

		// No usable tools and no native tool support: skip loop and rely on extractor.
		break
	}

	if err := ctx.Err(); err != nil {
		predErr = fmt.Errorf("context canceled before extraction: %w", err)
		return nil, predErr
	}

	// Prefer returning a signature-valid candidate captured in-loop.
	if pred, ok := r.tryFinalizeFromCandidate(inputs, newMessages, totalUsage, term); ok {
		pred.AddMetadata("react_iterations_used", iterationsUsed)
		pred.AddMetadata("react_max_iterations", r.MaxIterations)
		pred.AddMetadata("react_termination_reason", string(term.Reason()))
		pred.AddMetadata("react_extraction_used", false)
		return pred, nil
	}

	// Fall back to extractor stage over the rendered trajectory.
	extractionUsed = true
	pred, err := r.runExtractWithContextRetry(ctx, traj, inputs, newMessages, totalUsage)
	if err != nil {
		predErr = err
		return nil, predErr
	}

	pred.AddMetadata("react_iterations_used", iterationsUsed)
	pred.AddMetadata("react_max_iterations", r.MaxIterations)
	pred.AddMetadata("react_termination_reason", string(term.Reason()))
	pred.AddMetadata("react_extraction_used", extractionUsed)
	return pred, nil
}

func (r *ReAct) iterationNativeTools(ctx context.Context, traj *reactTrajectory, term *reactTermination, iteration int) (core.Usage, bool, error) {
	options := r.Options.Copy()
	options.ResponseFormat = ""
	options.ResponseSchema = nil
	options.Tools = r.Tools
	options.ToolChoice = "auto"

	result, err := r.generateWithContextRetry(ctx, traj, options, nil)
	if err != nil {
		return core.Usage{}, false, fmt.Errorf("LM generation failed at iteration %d: %w", iteration+1, err)
	}

	step := traj.AddStep(result.Content, result.ToolCalls)

	if r.Verbose {
		fmt.Printf("\n=== ReAct Iteration %d (native tools) ===\n", iteration+1)
		fmt.Printf("Thought: %s\n", core.StripMarkers(result.Content))
	}

	if len(result.ToolCalls) == 0 {
		// Implicit finish: model chose to respond directly.
		term.SetFinalContent(result.Content)
		term.MarkDone(terminationNoToolCalls)
		return result.Usage, true, nil
	}

	// Track finish but still execute ALL tool calls.
	// Provider APIs (OpenAI/OpenRouter) require a tool message for every tool_call_id.
	hasFinish := false
	for _, tc := range result.ToolCalls {
		if strings.EqualFold(tc.Name, "finish") {
			term.SetFinalToolArgs(tc.Arguments)
			hasFinish = true
		}
	}

	// Execute every tool call and append a tool message for each.
	// Even if termination conditions are triggered mid-iteration, we must respond to all tool calls
	// from this assistant message.
	for _, tc := range result.ToolCalls {
		term.ObserveToolCall(tc)

		tool := r.findTool(tc.Name)
		var toolOut any
		var toolErr error
		if tool == nil {
			toolErr = fmt.Errorf("tool '%s' not found", tc.Name)
		} else {
			toolOut, toolErr = tool.Execute(ctx, tc.Arguments)
		}

		env, truncated, obsHash := encodeToolResult(tc.Name, tc.ID, toolOut, toolErr, r.MaxToolResultBytes)
		step.AddToolResult(reactToolResult{ToolCallID: tc.ID, ToolName: tc.Name, Content: env, Truncated: truncated, Err: toolErr})
		term.ObserveToolResult(tc, obsHash, toolErr)

		if r.Verbose {
			fmt.Printf("Action: %s(%v)\n", tc.Name, tc.Arguments)
			fmt.Printf("Observation: %s\n", env)
		}
	}

	if hasFinish {
		term.MarkDone(terminationFinishTool)
		return result.Usage, true, nil
	}

	return result.Usage, term.ShouldStop(), nil
}

func (r *ReAct) iterationPrompted(ctx context.Context, traj *reactTrajectory, term *reactTermination, iteration int) (core.Usage, bool, error) {
	planningPrompt := r.buildPlanningPrompt()
	options := r.Options.Copy()
	options.Tools = nil
	options.ToolChoice = ""
	options.ResponseSchema = nil
	if r.LM.SupportsJSON() {
		options.ResponseFormat = "json"
	} else {
		options.ResponseFormat = ""
	}

	result, err := r.generateWithContextRetry(ctx, traj, options, []core.Message{{Role: "user", Content: planningPrompt}})
	if err != nil {
		return core.Usage{}, false, fmt.Errorf("planning generation failed at iteration %d: %w", iteration+1, err)
	}

	plan, parseErr := parsePlanningResult(result.Content)
	if parseErr != nil {
		// Record the assistant content as a step for debugging, then terminate into extraction.
		traj.AddStep(result.Content, nil)
		term.ObserveError(parseErr)
		term.MarkDone(terminationPlanningParseError)
		return result.Usage, true, nil
	}

	if r.Verbose {
		fmt.Printf("\n=== ReAct Iteration %d (prompted) ===\n", iteration+1)
		fmt.Printf("Plan: %s\n", strings.TrimSpace(result.Content))
	}

	if plan.Done || plan.NextToolName == "" || strings.EqualFold(plan.NextToolName, "finish") {
		traj.AddStep(result.Content, nil)
		term.MarkDone(terminationPlanningDone)
		return result.Usage, true, nil
	}

	// Synthesize a deterministic tool call ID for prompted mode.
	toolCallID := fmt.Sprintf("prompted_%d", iteration+1)
	toolCall := core.ToolCall{ID: toolCallID, Name: plan.NextToolName, Arguments: plan.NextToolArgs}
	step := traj.AddStep(result.Content, []core.ToolCall{toolCall})

	term.ObserveToolCall(toolCall)
	if term.ShouldStop() {
		term.MarkDone(terminationStagnation)
		return result.Usage, true, nil
	}

	tool := r.findTool(plan.NextToolName)
	var toolOut any
	var toolErr error
	if tool == nil {
		toolErr = fmt.Errorf("tool '%s' not found", plan.NextToolName)
	} else {
		toolOut, toolErr = tool.Execute(ctx, plan.NextToolArgs)
	}

	env, truncated, obsHash := encodeToolResult(plan.NextToolName, toolCallID, toolOut, toolErr, r.MaxToolResultBytes)
	step.AddToolResult(reactToolResult{ToolCallID: toolCallID, ToolName: plan.NextToolName, Content: env, Truncated: truncated, Err: toolErr})
	term.ObserveToolResult(toolCall, obsHash, toolErr)

	return result.Usage, term.ShouldStop(), nil
}

type planningResult struct {
	NextToolName string
	NextToolArgs map[string]any
	Done         bool
}

func parsePlanningResult(content string) (planningResult, error) {
	cleaned := stripToJSON(content)
	repaired := jsonutil.RepairJSON(cleaned)

	var raw map[string]any
	if err := json.Unmarshal([]byte(repaired), &raw); err != nil {
		return planningResult{}, fmt.Errorf("parse planning json: %w", err)
	}

	// Be tolerant to naming variations.
	name := ""
	if v, ok := raw["next_tool_name"]; ok {
		name, _ = v.(string)
	}
	if name == "" {
		if v, ok := raw["tool_name"]; ok {
			name, _ = v.(string)
		}
	}
	if name == "" {
		if v, ok := raw["tool"]; ok {
			name, _ = v.(string)
		}
	}

	args := map[string]any{}
	if v, ok := raw["next_tool_args"]; ok {
		if m, ok := v.(map[string]any); ok {
			args = m
		}
	}
	if len(args) == 0 {
		if v, ok := raw["tool_args"]; ok {
			if m, ok := v.(map[string]any); ok {
				args = m
			}
		}
	}
	if len(args) == 0 {
		// Some models return args as a JSON string.
		if v, ok := raw["args"]; ok {
			switch vv := v.(type) {
			case map[string]any:
				args = vv
			case string:
				_ = json.Unmarshal([]byte(jsonutil.RepairJSON(vv)), &args)
			}
		}
	}

	done := false
	if v, ok := raw["done"]; ok {
		if b, ok := v.(bool); ok {
			done = b
		}
	}
	if v, ok := raw["final"]; ok {
		if b, ok := v.(bool); ok {
			done = done || b
		}
	}

	return planningResult{NextToolName: strings.TrimSpace(name), NextToolArgs: args, Done: done}, nil
}

func (r *ReAct) buildSystemPrompt() string {
	if !r.hasRealTools() {
		return ""
	}

	var b strings.Builder
	b.WriteString("You are a helpful AI assistant. Use tools when needed.\n\n")
	if r.LM.SupportsTools() {
		b.WriteString("When you need external information, call tools using the native tool calling mechanism.\n")
		b.WriteString("When you are done, you may either respond directly or call the 'finish' tool.\n")
		b.WriteString("Do not write textual tool call syntax; use the tool calling API.\n")
	} else {
		b.WriteString("This model does not support native tool calling.\n")
		b.WriteString("You will be asked to output a JSON plan indicating the next tool to run.\n")
		b.WriteString("When you are done, set next_tool_name to \"finish\" and done=true.\n")
	}
	return b.String()
}

func (r *ReAct) buildPlanningPrompt() string {
	var b strings.Builder
	b.WriteString("Decide the next action.\n\n")
	b.WriteString("Return ONLY a JSON object with: \n")
	b.WriteString("- next_tool_name: string (tool name, or \"finish\" if done)\n")
	b.WriteString("- next_tool_args: object (arguments for that tool)\n")
	b.WriteString("- done: boolean\n\n")
	b.WriteString("Available tools:\n")
	for _, t := range r.Tools {
		if strings.EqualFold(t.Name, "finish") {
			continue
		}
		b.WriteString("- ")
		b.WriteString(t.Name)
		if t.Description != "" {
			b.WriteString(": ")
			b.WriteString(t.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nIf no tool is needed, set next_tool_name=\"finish\" and done=true.\n")
	return b.String()
}

func (r *ReAct) buildExtractionPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Based on the conversation above (including tool observations), synthesize the final answer.\n")
	prompt.WriteString("Return ONLY a JSON object with the required fields.\n\n")
	return prompt.String()
}

func (r *ReAct) findTool(name string) *core.Tool {
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			return &r.Tools[i]
		}
	}
	return nil
}

// hasRealTools returns true if there are tools beyond the auto-injected "finish" tool.
func (r *ReAct) hasRealTools() bool {
	for _, t := range r.Tools {
		if !strings.EqualFold(t.Name, "finish") {
			return true
		}
	}
	return false
}

// buildFinishTool creates a synthetic "finish" tool that allows models to explicitly
// conclude the ReAct loop by providing final outputs matching the signature.
func buildFinishTool(signature *core.Signature) *core.Tool {
	tool := core.NewTool(
		"finish",
		"Call this tool when you are ready to conclude the reasoning/tool loop.",
		func(ctx context.Context, args map[string]any) (any, error) {
			return "finish", nil
		},
	)

	for _, field := range signature.OutputFields {
		description := field.Description
		if description == "" {
			description = fmt.Sprintf("The %s field of the final answer", field.Name)
		}

		paramType := "string"
		switch field.Type {
		case core.FieldTypeInt:
			paramType = "number"
		case core.FieldTypeBool:
			paramType = "boolean"
		}
		if field.Type == core.FieldTypeClass && len(field.Classes) > 0 {
			description = fmt.Sprintf("%s (one of: %s)", description, strings.Join(field.Classes, ", "))
		}
		tool.AddParameter(field.Name, paramType, description, !field.Optional)
	}

	return tool
}

// Clone creates an independent copy of ReAct module.
func (r *ReAct) Clone() core.Module {
	cloned := &ReAct{
		Signature:          r.Signature,
		LM:                 r.LM,
		Tools:              make([]core.Tool, len(r.Tools)),
		Options:            r.Options,
		Adapter:            r.Adapter,
		History:            nil,
		Demos:              make([]core.Example, len(r.Demos)),
		MaxIterations:      r.MaxIterations,
		Verbose:            r.Verbose,
		MaxToolResultBytes: r.MaxToolResultBytes,
		MaxPromptBytes:     r.MaxPromptBytes,
	}

	copy(cloned.Demos, r.Demos)
	copy(cloned.Tools, r.Tools)

	if r.History != nil {
		cloned.History = r.History.Clone()
	}

	return cloned
}

func (r *ReAct) tryFinalizeFromCandidate(inputs map[string]any, newMessages []core.Message, usage core.Usage, term *reactTermination) (*core.Prediction, bool) {
	// 1) Finish tool args (already structured).
	if args := term.FinalToolArgs(); args != nil {
		outputs := cloneMap(args)
		outputs = coerceBasicTypes(r.Signature, outputs)
		outputs = core.NormalizeOutputKeys(r.Signature, outputs)

		// Extract rationale/reasoning if present and not in signature.
		rationale := ""
		if val, ok := outputs["rationale"]; ok {
			rationale = fmt.Sprintf("%v", val)
			if r.Signature.GetOutputField("rationale") == nil {
				delete(outputs, "rationale")
			}
		}
		if rationale == "" {
			if val, ok := outputs["reasoning"]; ok {
				rationale = fmt.Sprintf("%v", val)
				if r.Signature.GetOutputField("reasoning") == nil {
					delete(outputs, "reasoning")
				}
			}
		}

		if err := r.Signature.ValidateOutputs(outputs); err == nil {
			if r.History != nil {
				for _, msg := range newMessages {
					if msg.Role == "user" {
						r.History.Add(msg)
					}
				}
				contentBytes, _ := json.Marshal(outputs)
				r.History.Add(core.Message{Role: "assistant", Content: string(contentBytes)})
			}

			pred := core.NewPrediction(outputs).
				WithUsage(usage).
				WithModuleName(logging.ModuleReAct).
				WithInputs(inputs)
			if rationale != "" {
				pred = pred.WithRationale(rationale)
			}
			return pred, true
		}
	}

	// 2) Direct answer content (parse + validate).
	content := strings.TrimSpace(term.FinalContent())
	if content != "" {
		parsed, err := r.Adapter.Parse(r.Signature, content)
		if err != nil {
			cleaned := stripToJSON(content)
			if cleaned != content {
				parsed, err = r.Adapter.Parse(r.Signature, cleaned)
			}
		}
		if err == nil {
			parsed = coerceBasicTypes(r.Signature, parsed)
			parsed = core.NormalizeOutputKeys(r.Signature, parsed)
			rationale := ""
			if val, ok := parsed["rationale"]; ok {
				rationale = fmt.Sprintf("%v", val)
				if r.Signature.GetOutputField("rationale") == nil {
					delete(parsed, "rationale")
				}
			}
			if rationale == "" {
				if val, ok := parsed["reasoning"]; ok {
					rationale = fmt.Sprintf("%v", val)
					if r.Signature.GetOutputField("reasoning") == nil {
						delete(parsed, "reasoning")
					}
				}
			}

			if err := r.Signature.ValidateOutputs(parsed); err == nil {
				adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(parsed)

				if r.History != nil {
					for _, msg := range newMessages {
						if msg.Role == "user" {
							r.History.Add(msg)
						}
					}
					r.History.Add(core.Message{Role: "assistant", Content: content})
				}

				pred := core.NewPrediction(parsed).
					WithUsage(usage).
					WithModuleName(logging.ModuleReAct).
					WithInputs(inputs)
				if rationale != "" {
					pred = pred.WithRationale(rationale)
				}
				if adapterUsed != "" {
					pred.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
				}
				return pred, true
			}
		}
	}

	return nil, false
}

// --- Extraction helpers (always uses extractor) ---

func (r *ReAct) runExtractWithContextRetry(
	ctx context.Context,
	traj *reactTrajectory,
	inputs map[string]any,
	newMessages []core.Message,
	priorUsage core.Usage,
) (*core.Prediction, error) {
	// Retry extraction a few times on context overflow, truncating oldest steps.
	for attempt := 0; attempt < 3; attempt++ {
		pred, err := r.runExtract(ctx, traj.Render(r.maxPromptBytes()), inputs, newMessages, priorUsage, false)
		if err == nil {
			return pred, nil
		}
		if !isContextOverflowError(err) {
			return nil, err
		}
		if traj.DropOldestSteps(1) == 0 {
			return nil, err
		}
	}
	// Final attempt without further truncation.
	return r.runExtract(ctx, traj.Render(r.maxPromptBytes()), inputs, newMessages, priorUsage, false)
}

func (r *ReAct) maxPromptBytes() int {
	if r.MaxPromptBytes > 0 {
		return r.MaxPromptBytes
	}
	return defaultReActMaxPromptBytes
}

// runExtract performs post-loop extraction to synthesize a final answer
// from the accumulated message history (trajectory).
func (r *ReAct) runExtract(ctx context.Context, messages []core.Message, inputs map[string]any, newMessages []core.Message, priorUsage core.Usage, historyUpdated bool) (*core.Prediction, error) {
	settings := core.GetSettings()
	useStructuredMode := settings.StructuredOutput.Enabled
	if useStructuredMode {
		return r.runExtractStructured(ctx, messages, inputs, newMessages, priorUsage, historyUpdated)
	}
	return r.runExtractLegacy(ctx, messages, inputs, newMessages, priorUsage, historyUpdated)
}

func (r *ReAct) runExtractStructured(ctx context.Context, messages []core.Message, inputs map[string]any, newMessages []core.Message, priorUsage core.Usage, historyUpdated bool) (*core.Prediction, error) {
	settings := core.GetSettings()

	extractMessages := make([]core.Message, len(messages))
	copy(extractMessages, messages)
	extractMessages = append(extractMessages, core.Message{Role: "user", Content: r.buildExtractionPrompt()})

	wrappedAdapter := &reactExtractAdapter{
		base:     core.NewSchemaFirstAdapter(r.LM.SupportsJSON()).WithReasoning(true),
		messages: extractMessages,
	}

	extractOptions := r.Options.Copy()
	if r.LM.SupportsTools() {
		extractOptions.Tools = r.Tools
		extractOptions.ToolChoice = "none"
	} else {
		extractOptions.Tools = nil
		extractOptions.ToolChoice = ""
	}

	result, err := core.GenerateStructured(
		ctx,
		r.LM,
		r.Signature,
		inputs,
		[]core.Example{},
		core.GenerateStructuredOptions{
			Adapter:        wrappedAdapter,
			BaseOptions:    extractOptions,
			MaxAttempts:    settings.StructuredOutput.MaxAttempts,
			Temperature:    settings.StructuredOutput.Temperature,
			UseJSONFormat:  r.LM.SupportsJSON(),
			StreamCallback: r.Options.StreamCallback,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("extraction generation failed: %w", err)
	}

	outputs := result.Outputs

	// Extract rationale/reasoning when not part of signature.
	rationale := ""
	if val, ok := outputs["rationale"]; ok {
		rationale, _ = val.(string)
		if r.Signature.GetOutputField("rationale") == nil {
			delete(outputs, "rationale")
		}
	}
	if rationale == "" {
		if val, ok := outputs["reasoning"]; ok {
			rationale, _ = val.(string)
			if r.Signature.GetOutputField("reasoning") == nil {
				delete(outputs, "reasoning")
			}
		}
	}

	totalUsage := addUsage(priorUsage, result.Usage)

	if r.History != nil && !historyUpdated {
		for _, msg := range newMessages {
			if msg.Role == "user" {
				r.History.Add(msg)
			}
		}
		contentBytes, _ := json.Marshal(outputs)
		r.History.Add(core.Message{Role: "assistant", Content: string(contentBytes)})
	}

	pred := core.NewPrediction(outputs).
		WithRationale(rationale).
		WithUsage(totalUsage).
		WithModuleName(logging.ModuleReAct).
		WithInputs(inputs)

	if result.Diagnostics != nil {
		pred.WithParseDiagnostics(result.Diagnostics)
	}
	if result.AdapterUsed != "" {
		pred.WithAdapterMetrics(result.AdapterUsed, result.ParseAttempts, result.FallbackUsed)
	}

	return pred, nil
}

type reactExtractAdapter struct {
	base     core.Adapter
	messages []core.Message
}

func (rea *reactExtractAdapter) Format(sig *core.Signature, inputs map[string]any, demos []core.Example) ([]core.Message, error) {
	return rea.messages, nil
}

func (rea *reactExtractAdapter) Parse(sig *core.Signature, content string) (map[string]any, error) {
	return rea.base.Parse(sig, content)
}

func (rea *reactExtractAdapter) FormatHistory(history *core.History) []core.Message {
	return rea.base.FormatHistory(history)
}

func (r *ReAct) runExtractLegacy(ctx context.Context, messages []core.Message, inputs map[string]any, newMessages []core.Message, priorUsage core.Usage, historyUpdated bool) (*core.Prediction, error) {
	extractMessages := make([]core.Message, len(messages))
	copy(extractMessages, messages)
	extractMessages = append(extractMessages, core.Message{Role: "user", Content: r.buildExtractionPrompt()})

	options := r.Options.Copy()
	if r.LM.SupportsTools() {
		options.Tools = r.Tools
		options.ToolChoice = "none"
	} else {
		options.Tools = nil
		options.ToolChoice = ""
	}

	if r.LM.SupportsJSON() {
		options.ResponseFormat = "json"
		if options.ResponseSchema == nil {
			if r.LM.IsOpenAI() {
				options.ResponseSchema = r.Signature.SignatureToOpenAIJSONSchema()
			} else {
				options.ResponseSchema = r.Signature.SignatureToJSONSchema()
			}
		}
	}

	result, err := r.LM.Generate(ctx, extractMessages, options)
	if err != nil {
		return nil, fmt.Errorf("extraction generation failed: %w", err)
	}

	cleanedContent := stripToJSON(result.Content)

	extractAdapter := core.NewFallbackAdapter().WithReasoning(true)
	outputs, err := extractAdapter.Parse(r.Signature, cleanedContent)
	if err != nil {
		outputs = make(map[string]any)
		if jsonErr := json.Unmarshal([]byte(cleanedContent), &outputs); jsonErr != nil {
			outputs = r.extractTextOutputs(cleanedContent, extractMessages)
			if len(outputs) == 0 {
				return nil, fmt.Errorf("extraction failed to parse output: %w (JSON error: %v)", err, jsonErr)
			}
		}
	}

	var rationale string
	if val, ok := outputs["rationale"]; ok {
		rationale, _ = val.(string)
		if r.Signature.GetOutputField("rationale") == nil {
			delete(outputs, "rationale")
		}
	}
	if rationale == "" {
		if val, ok := outputs["reasoning"]; ok {
			rationale, _ = val.(string)
			if r.Signature.GetOutputField("reasoning") == nil {
				delete(outputs, "reasoning")
			}
		}
	}

	outputs = coerceBasicTypes(r.Signature, outputs)
	outputs = core.NormalizeOutputKeys(r.Signature, outputs)
	diagnostics := r.Signature.ValidateOutputsPartial(outputs)

	adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

	totalUsage := addUsage(priorUsage, result.Usage)

	if r.History != nil && !historyUpdated {
		for _, msg := range newMessages {
			if msg.Role == "user" {
				r.History.Add(msg)
			}
		}
		contentBytes, _ := json.Marshal(outputs)
		r.History.Add(core.Message{Role: "assistant", Content: string(contentBytes)})
	}

	pred := core.NewPrediction(outputs).
		WithUsage(totalUsage).
		WithModuleName(logging.ModuleReAct).
		WithInputs(inputs).
		WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed).
		WithParseDiagnostics(diagnostics)

	if rationale != "" {
		pred = pred.WithRationale(rationale)
	}

	return pred, nil
}

// extractTextOutputs attempts to extract output fields from raw text when structured parsing fails.
func (r *ReAct) extractTextOutputs(content string, messages []core.Message) map[string]any {
	outputs := make(map[string]any)
	content = strings.TrimSpace(content)

	// If content is empty/very short, synthesize from tool observations.
	if len(content) < 10 {
		content = r.synthesizeAnswerFromHistory(messages)
	}

	var stringFields []core.Field
	for _, field := range r.Signature.OutputFields {
		if field.Type == core.FieldTypeString {
			stringFields = append(stringFields, field)
		}
	}
	if len(stringFields) == 0 {
		return nil
	}

	if len(stringFields) == 1 && stringFields[0].Name == "answer" {
		outputs["answer"] = content
		return outputs
	}

	primaryField := ""
	if r.Signature.GetOutputField("answer") != nil {
		primaryField = "answer"
	} else {
		primaryField = stringFields[0].Name
	}
	outputs[primaryField] = content

	for _, field := range stringFields {
		if field.Name != primaryField && !field.Optional {
			outputs[field.Name] = content
		}
	}

	return outputs
}

// synthesizeAnswerFromHistory extracts recent tool observations from history.
// Used as a fallback when the model produces empty content in extraction.
func (r *ReAct) synthesizeAnswerFromHistory(messages []core.Message) string {
	var observations []string
	for _, msg := range messages {
		if msg.Role == "tool" && strings.TrimSpace(msg.Content) != "" {
			if strings.HasPrefix(strings.TrimSpace(msg.Content), "Error:") {
				continue
			}
			observations = append(observations, strings.TrimSpace(msg.Content))
		}
	}
	if len(observations) == 0 {
		return "No information available from tools"
	}

	seen := make(map[string]bool)
	unique := make([]string, 0, 3)
	for i := len(observations) - 1; i >= 0 && len(unique) < 3; i-- {
		obs := observations[i]
		if seen[obs] {
			continue
		}
		if len(obs) <= 20 {
			continue
		}
		seen[obs] = true
		unique = append([]string{obs}, unique...)
	}
	if len(unique) > 0 {
		return strings.Join(unique, " ")
	}
	return observations[len(observations)-1]
}

// stripToJSON removes common LLM artifacts from JSON output.
func stripToJSON(content string) string {
	content = strings.TrimSpace(content)
	re := regexp.MustCompile("(?s)```(?:json)?\\n?(.*?)\\n?```")
	if matches := re.FindStringSubmatch(content); len(matches) > 1 {
		content = strings.TrimSpace(matches[1])
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start != -1 && end != -1 && end > start {
		content = content[start : end+1]
	}
	return strings.TrimSpace(content)
}

// coerceBasicTypes handles basic type mismatches in parsed outputs.
func coerceBasicTypes(signature *core.Signature, outputs map[string]any) map[string]any {
	coerced := make(map[string]any)
	for key, value := range outputs {
		field := signature.GetOutputField(key)
		if field == nil {
			coerced[key] = value
			continue
		}

		switch field.Type {
		case core.FieldTypeInt:
			if strVal, ok := value.(string); ok {
				re := regexp.MustCompile(`-?\d+`)
				if match := re.FindString(strVal); match != "" {
					if intVal, err := strconv.Atoi(match); err == nil {
						coerced[key] = intVal
						continue
					}
				}
			}
			if floatVal, ok := value.(float64); ok {
				coerced[key] = int(floatVal)
				continue
			}
			coerced[key] = value

		case core.FieldTypeBool:
			if strVal, ok := value.(string); ok {
				strVal = strings.ToLower(strings.TrimSpace(strVal))
				if strVal == "true" || strVal == "yes" || strVal == "1" {
					coerced[key] = true
					continue
				}
				if strVal == "false" || strVal == "no" || strVal == "0" {
					coerced[key] = false
					continue
				}
			}
			coerced[key] = value

		case core.FieldTypeString:
			if value != nil {
				coerced[key] = fmt.Sprintf("%v", value)
			} else {
				coerced[key] = value
			}

		default:
			coerced[key] = value
		}
	}
	return coerced
}

func addUsage(a, b core.Usage) core.Usage {
	a.PromptTokens += b.PromptTokens
	a.CompletionTokens += b.CompletionTokens
	a.TotalTokens += b.TotalTokens
	a.Cost += b.Cost
	a.Latency += b.Latency
	return a
}

// For safety, keep output map independent when using tool args.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}
