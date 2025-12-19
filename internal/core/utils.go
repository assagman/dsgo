package core

// =============================================================================
// Utilities
// =============================================================================
//
// This file contains small, dependency-free helper functions used across the
// core layer and (where appropriate) by internal providers.
//
// Keep utilities:
//   - deterministic (no hidden global state)
//   - side-effect free
//   - narrowly scoped (prefer a single-purpose helper)
// =============================================================================

// -----------------------------------------------------------------------------
// String / slice utilities
// -----------------------------------------------------------------------------

// DedupeStringsPreserveOrder returns a new slice containing the first occurrence
// of each string in the input, preserving the original order.
func DedupeStringsPreserveOrder(in []string) []string {
	if len(in) == 0 {
		return []string{} // Return empty slice instead of nil for safe JSON serialization
	}

	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
