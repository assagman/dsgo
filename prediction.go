package dsgo

// Prediction wraps module outputs with metadata and provenance
type Prediction struct {
	// Core output
	Outputs map[string]any

	// Metadata
	Rationale   string           // Reasoning trace (for CoT, etc.)
	Score       float64          // Confidence/quality score
	Completions []map[string]any // Alternative completions (for BestOfN)
	Usage       Usage            // Token usage statistics

	// Provenance
	ModuleName string         // Name of module that generated this
	Inputs     map[string]any // Original inputs

	// Adapter metrics (for diagnostics and monitoring)
	AdapterUsed   string // Name of the adapter that successfully parsed the response
	ParseSuccess  bool   // Whether parsing succeeded on first attempt
	ParseAttempts int    // Number of parse attempts (for fallback adapters)
	FallbackUsed  bool   // Whether fallback to another adapter was needed
}

// NewPrediction creates a new prediction from outputs
func NewPrediction(outputs map[string]any) *Prediction {
	return &Prediction{
		Outputs:     outputs,
		Completions: []map[string]any{},
	}
}

// WithRationale adds reasoning trace to the prediction
func (p *Prediction) WithRationale(rationale string) *Prediction {
	p.Rationale = rationale
	return p
}

// WithScore adds a confidence/quality score
func (p *Prediction) WithScore(score float64) *Prediction {
	p.Score = score
	return p
}

// WithCompletions adds alternative completions
func (p *Prediction) WithCompletions(completions []map[string]any) *Prediction {
	p.Completions = completions
	return p
}

// WithUsage adds token usage statistics
func (p *Prediction) WithUsage(usage Usage) *Prediction {
	p.Usage = usage
	return p
}

// WithModuleName records which module generated this prediction
func (p *Prediction) WithModuleName(name string) *Prediction {
	p.ModuleName = name
	return p
}

// WithInputs records the original inputs
func (p *Prediction) WithInputs(inputs map[string]any) *Prediction {
	p.Inputs = inputs
	return p
}

// Get retrieves a value from outputs
func (p *Prediction) Get(key string) (any, bool) {
	val, ok := p.Outputs[key]
	return val, ok
}

// GetString retrieves a string value from outputs
func (p *Prediction) GetString(key string) (string, bool) {
	val, ok := p.Outputs[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetFloat retrieves a float value from outputs
func (p *Prediction) GetFloat(key string) (float64, bool) {
	val, ok := p.Outputs[key]
	if !ok {
		return 0, false
	}
	f, ok := val.(float64)
	return f, ok
}

// GetInt retrieves an int value from outputs
func (p *Prediction) GetInt(key string) (int, bool) {
	val, ok := p.Outputs[key]
	if !ok {
		return 0, false
	}

	// Handle both int and float64 (from JSON)
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool retrieves a bool value from outputs
func (p *Prediction) GetBool(key string) (bool, bool) {
	val, ok := p.Outputs[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// HasRationale returns true if prediction includes reasoning
func (p *Prediction) HasRationale() bool {
	return p.Rationale != ""
}

// HasCompletions returns true if there are alternative completions
func (p *Prediction) HasCompletions() bool {
	return len(p.Completions) > 0
}

// WithAdapterMetrics records adapter usage information
func (p *Prediction) WithAdapterMetrics(adapterName string, attempts int, fallbackUsed bool) *Prediction {
	p.AdapterUsed = adapterName
	p.ParseAttempts = attempts
	p.ParseSuccess = attempts == 1
	p.FallbackUsed = fallbackUsed
	return p
}

// ExtractAdapterMetadata extracts and removes adapter metadata from outputs map
// Returns (adapterUsed, parseAttempts, fallbackUsed)
func ExtractAdapterMetadata(outputs map[string]any) (string, int, bool) {
	adapterUsed, _ := outputs["__adapter_used"].(string)
	parseAttempts, _ := outputs["__parse_attempts"].(int)
	fallbackUsed, _ := outputs["__fallback_used"].(bool)

	// Remove metadata from outputs (internal only)
	delete(outputs, "__adapter_used")
	delete(outputs, "__parse_attempts")
	delete(outputs, "__fallback_used")

	return adapterUsed, parseAttempts, fallbackUsed
}
