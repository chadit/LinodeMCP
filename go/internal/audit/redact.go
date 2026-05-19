package audit

import (
	"sort"
)

// RedactedValue is the placeholder string written into the audit
// event in place of a sensitive arg value. Exposed so tests can
// match it without re-defining the constant.
const RedactedValue = "[REDACTED]"

// RedactionFields is the canonical list of arg names whose values
// get replaced with RedactedValue before the audit event is written.
// Match semantics are EXACT field name (per the spec): no substring,
// no suffix, no case-fold. A variant like `cluster_root_pass` needs
// its own entry in this list.
//
// The cross-language parity test in event_test.go asserts that the
// Python equivalent at `python/src/linodemcp/audit/redact.py` carries
// the same set of names.
func RedactionFields() []string {
	// Returned by function rather than a package-level slice so the
	// global-state linter has nothing to flag. The list is small and
	// the audit middleware (Phase 1b) caches the lookup set so the
	// allocation per call doesn't matter.
	return []string{
		"api_key",
		"apiKey",
		"authorized_keys",
		"data",
		"kubeconfig",
		"pass",
		"password",
		"root_pass",
		"secret",
		"service_token",
		"token",
	}
}

// RedactionFieldSet returns the redaction list as a set for O(1)
// lookups inside the recursive walker. Equivalent to
// slices.Index(RedactionFields(), key) >= 0 but cheaper when called
// per-key over a deep args tree.
func RedactionFieldSet() map[string]struct{} {
	fields := RedactionFields()
	out := make(map[string]struct{}, len(fields))

	for _, name := range fields {
		out[name] = struct{}{}
	}

	return out
}

// Redact walks the args map and replaces sensitive values with
// RedactedValue. Returns the redacted copy and a sorted list of
// every key that was redacted (deduped across the recursive walk).
//
// The original args map is NOT mutated; callers that need the
// unredacted values can keep their own copy.
//
// The walk recurses into nested maps but does NOT recurse into
// slices of maps. That's a deliberate simplification: every
// sensitive arg in the current tool surface lives at the top level
// or inside a nested object literal, never inside an array element.
// If a future tool needs array-element redaction, the walker grows
// to handle it then.
func Redact(args map[string]any) (map[string]any, []string) {
	fields := RedactionFieldSet()
	redactedKeys := make(map[string]struct{})

	out := redactMap(args, fields, redactedKeys)
	keys := make([]string, 0, len(redactedKeys))

	for name := range redactedKeys {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	return out, keys
}

// redactMap is the recursive worker. Walks the map, copying values
// into a new map; sensitive values are replaced.
func redactMap(
	source map[string]any,
	fields map[string]struct{},
	redactedKeys map[string]struct{},
) map[string]any {
	if source == nil {
		return nil
	}

	out := make(map[string]any, len(source))

	for key, value := range source {
		if _, sensitive := fields[key]; sensitive {
			out[key] = RedactedValue
			redactedKeys[key] = struct{}{}

			continue
		}

		nested, ok := value.(map[string]any)
		if !ok {
			out[key] = value

			continue
		}

		out[key] = redactMap(nested, fields, redactedKeys)
	}

	return out
}

// IsRedacted reports whether a value position holds the redaction
// placeholder. Used by tests to assert that a specific arg key was
// scrubbed without re-implementing the equality check.
func IsRedacted(value any) bool {
	asString, ok := value.(string)
	if !ok {
		return false
	}

	return asString == RedactedValue
}
