package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
)

// TestRedactionListNoDuplicates guards against an accidental
// duplicate entry. A duplicate doesn't break behavior but would
// surface as a flaky parity test on the Python side.
func TestRedactionListNoDuplicates(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFields()
	seen := make(map[string]struct{}, len(fields))

	for _, name := range fields {
		_, dup := seen[name]
		assert.False(t, dup, "redaction list must not contain duplicate %q", name)

		seen[name] = struct{}{}
	}
}

// TestRedactReplacesSensitiveTopLevelKeys is the smoke test for the
// happy path. Each top-level sensitive key gets replaced; non-sensitive
// keys pass through unchanged; the redacted-key list reports each
// scrubbed name once.
func TestRedactReplacesSensitiveTopLevelKeys(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		argLinodeID: 12345,
		argRootPass: "super-secret",
		argKeyLabel: "my-instance",
		argKeyToken: "abc123",
	}

	redacted, keys := audit.Redact(args)

	assert.Equal(t, 12345, redacted["linode_id"], "non-sensitive value must pass through")
	assert.Equal(t, "my-instance", redacted[argKeyLabel])
	assert.True(t, audit.IsRedacted(redacted[argRootPass]), "root_pass must be redacted")
	assert.True(t, audit.IsRedacted(redacted[argKeyToken]), "token must be redacted")
	assert.ElementsMatch(t, []string{argRootPass, argKeyToken}, keys,
		"redacted-key list must report each scrubbed name")
}

// TestRedactRecursesIntoNestedMaps verifies the spec's "match by
// name, not by depth" rule. A sensitive field name nested inside an
// object literal still gets redacted.
func TestRedactRecursesIntoNestedMaps(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		argKeyLabel: "test",
		"meta": map[string]any{
			"api_key": "sk-leaked",
			"region":  "us-east",
		},
	}

	redacted, keys := audit.Redact(args)

	nested, ok := redacted["meta"].(map[string]any)
	require.True(t, ok, "nested object must remain a map")
	assert.True(t, audit.IsRedacted(nested["api_key"]),
		"nested api_key must be redacted")
	assert.Equal(t, "us-east", nested["region"],
		"nested non-sensitive value passes through")
	assert.Contains(t, keys, "api_key", "nested key reported in keys list")
}

// TestRedactExactNameMatch locks the spec's exact-match rule:
// variant names like `cluster_root_pass` do NOT match the `root_pass`
// entry. The Risks section calls out that variants need explicit
// list entries.
func TestRedactExactNameMatch(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"cluster_root_pass": "should-pass-through-because-variant",
		"new_root_pass":     "also-variant",
		"Root_Pass":         "different-case",
	}

	redacted, keys := audit.Redact(args)

	assert.Equal(t, "should-pass-through-because-variant", redacted["cluster_root_pass"],
		"variant cluster_root_pass must not match exact rule for root_pass")
	assert.Equal(t, "also-variant", redacted["new_root_pass"])
	assert.Equal(t, "different-case", redacted["Root_Pass"],
		"case-folded variant must not match exact rule")
	assert.Empty(t, keys, "no sensitive keys hit the exact-match rule")
}

// TestRedactNilArgsProducesEmptyResult covers the empty-input path.
// The walker must not panic on nil args; it returns nil for the map
// and an empty slice for the keys list.
func TestRedactNilArgsProducesEmptyResult(t *testing.T) {
	t.Parallel()

	redacted, keys := audit.Redact(nil)

	assert.Nil(t, redacted, "nil args produce nil result")
	assert.Empty(t, keys)
}

// TestRedactReturnsCopyNotMutation guards the no-mutation contract.
// The redact walker copies values into a new map; the caller's
// original args remain untouched even when sensitive keys are
// present.
func TestRedactReturnsCopyNotMutation(t *testing.T) {
	t.Parallel()

	args := map[string]any{argRootPass: "secret"}

	_, _ = audit.Redact(args)

	assert.Equal(t, "secret", args[argRootPass],
		"original args map must not be mutated")
}

// TestRedactionFieldSetMatchesList confirms the set helper builds a
// set with the same membership as the list. A drift between the two
// would cause the runtime walker to miss redactions.
func TestRedactionFieldSetMatchesList(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFields()
	set := audit.RedactionFieldSet()

	assert.Len(t, set, len(fields), "set must have one entry per list field")

	for _, name := range fields {
		_, present := set[name]
		assert.True(t, present, "field %q must appear in the set", name)
	}
}
