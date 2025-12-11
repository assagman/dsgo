package logging

// Canonical module names for structured logging and module-level overrides.
// These strings are used for per-module logging (DSGO_LOG_MODULE_LEVELS),
// LogPredictionStart/End, and Prediction.ModuleName provenance.
const (
	ModulePredict              = "module.Predict"
	ModuleChainOfThought       = "module.ChainOfThought"
	ModuleReAct                = "module.ReAct"
	ModuleRefine               = "module.Refine"
	ModuleBestOfN              = "module.BestOfN"
	ModuleProgram              = "module.Program"
	ModuleParallel             = "module.Parallel"
	ModuleProgramOfThought     = "module.ProgramOfThought"
	ModuleMultiChainComparison = "module.MultiChainComparison"
)
