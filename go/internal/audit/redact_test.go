package audit_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/audit"
)

const (
	argContactEmail = "contact_email"
	argContactName  = "contact_name"
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
		checkFalse(t, dup, "redaction list must not contain duplicate %q", name)

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
		argKeyToken: valFakeToken,
	}

	redacted, keys := audit.Redact(args)

	checkEqual(t, 12345, redacted["linode_id"], "non-sensitive value must pass through")
	checkEqual(t, "my-instance", redacted[argKeyLabel])
	checkTrue(t, audit.IsRedacted(redacted[argRootPass]), "root_pass must be redacted")
	checkTrue(t, audit.IsRedacted(redacted[argKeyToken]), "token must be redacted")
	checkElementsMatch(t, []string{argRootPass, argKeyToken}, keys,
		"redacted-key list must report each scrubbed name")
}

func TestRedactAccountUserUpdateSensitiveFields(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		"password_created": "2024-01-02T03:04:05",
		"ssh_keys":         []any{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest"},
	}

	redacted, keys := audit.Redact(args)

	checkTrue(t, audit.IsRedacted(redacted["password_created"]), "password_created must be redacted")
	checkTrue(t, audit.IsRedacted(redacted["ssh_keys"]), "ssh_keys must be redacted")
	checkElementsMatch(t, []string{"password_created", "ssh_keys"}, keys)
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
			keyRegion: valUSEast,
		},
	}

	redacted, keys := audit.Redact(args)

	nested, ok := redacted["meta"].(map[string]any)
	mustTrue(t, ok, "nested object must remain a map")
	checkTrue(t, audit.IsRedacted(nested["api_key"]),
		"nested api_key must be redacted")
	checkEqual(t, valUSEast, nested[keyRegion],
		"nested non-sensitive value passes through")
	checkContains(t, keys, "api_key", "nested key reported in keys list")
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

	checkEqual(t, "should-pass-through-because-variant", redacted["cluster_root_pass"],
		"variant cluster_root_pass must not match exact rule for root_pass")
	checkEqual(t, "also-variant", redacted["new_root_pass"])
	checkEqual(t, "different-case", redacted["Root_Pass"],
		"case-folded variant must not match exact rule")
	checkEmpty(t, keys, "no sensitive keys hit the exact-match rule")
}

// TestRedactNilArgsProducesEmptyResult covers the empty-input path.
// The walker must not panic on nil args; it returns nil for the map
// and an empty slice for the keys list.
func TestRedactNilArgsProducesEmptyResult(t *testing.T) {
	t.Parallel()

	redacted, keys := audit.Redact(nil)

	checkNil(t, redacted, "nil args produce nil result")
	checkEmpty(t, keys)
}

// TestRedactReturnsCopyNotMutation guards the no-mutation contract.
// The redact walker copies values into a new map; the caller's
// original args remain untouched even when sensitive keys are
// present.
func TestRedactReturnsCopyNotMutation(t *testing.T) {
	t.Parallel()

	args := map[string]any{argRootPass: "secret"}

	_, _ = audit.Redact(args)

	checkEqual(t, "secret", args[argRootPass],
		"original args map must not be mutated")
}

// TestRedactionFieldSetMatchesList confirms the set helper builds a
// set with the same membership as the list. A drift between the two
// would cause the runtime walker to miss redactions.
func TestRedactionFieldSetMatchesList(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFields()
	set := audit.RedactionFieldSet()

	checkLen(t, set, len(fields), "set must have one entry per list field")

	for _, name := range fields {
		_, present := set[name]
		checkTrue(t, present, "field %q must appear in the set", name)
	}
}

// TestRedactionFieldsPIIList locks the source-verified PII list.
// Each name was confirmed against the Go and Python tool schemas to
// appear only in account-update or profile-verification contexts; a
// drift here means the wrong field is being redacted (or missed).
func TestRedactionFieldsPIIList(t *testing.T) {
	t.Parallel()

	expected := []string{
		"address_1",
		"address_2",
		"city",
		argContactEmail,
		argContactName,
		"phone",
		"phone_number",
		"phone_primary",
		"phone_secondary",
		"state",
		"tax_id",
		"zip",
	}

	checkElementsMatch(t, expected, audit.RedactionFieldsPII(),
		"PII list must exactly match the source-verified conservative scope")
}

// TestRedactionFieldsPIINoDuplicates guards against an accidental
// duplicate. Same reasoning as the credential-list dup check: a
// duplicate doesn't break behavior but signals copy-paste drift.
func TestRedactionFieldsPIINoDuplicates(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFieldsPII()
	seen := make(map[string]struct{}, len(fields))

	for _, name := range fields {
		_, dup := seen[name]
		checkFalse(t, dup, "PII list must not contain duplicate %q", name)

		seen[name] = struct{}{}
	}
}

// TestRedactionListsDisjoint asserts the credential and PII lists
// share no entries. The combined-set helper assumes disjoint sets so
// it can merge without dedup; a shared entry would still work today
// (set semantics) but signals taxonomy drift worth catching now.
func TestRedactionListsDisjoint(t *testing.T) {
	t.Parallel()

	credSet := audit.RedactionFieldSet()

	for _, pii := range audit.RedactionFieldsPII() {
		_, overlap := credSet[pii]
		checkFalse(t, overlap,
			"PII name %q must not also appear in the credential list", pii)
	}
}

// TestRedactWithPIIScrubsPIIFields verifies that the PII-aware entry
// point redacts both credential and PII names in one walk. This is
// the path the audit middleware will take when audit.redact_pii=true.
func TestRedactWithPIIScrubsPIIFields(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		argLinodeID:     42,
		argKeyLabel:     "primary",
		argKeyToken:     valFakeToken,
		argTaxID:        "TX-99",
		argPhone:        "+1-555-0100",
		argAddress1:     "123 Main St",
		argCity:         "Springfield",
		argContactName:  "Jane Doe",
		argContactEmail: "jane@example.org",
		"country":       "us", // not in PII list, must pass through
	}

	redacted, keys := audit.RedactWithPII(args)

	checkEqual(t, 42, redacted[argLinodeID], "non-sensitive value must pass through")
	checkEqual(t, "primary", redacted[argKeyLabel])
	checkEqual(t, "us", redacted["country"],
		"country is a region filter, NOT redacted")
	checkTrue(t, audit.IsRedacted(redacted[argKeyToken]), "credential still redacted")
	checkTrue(t, audit.IsRedacted(redacted[argTaxID]))
	checkTrue(t, audit.IsRedacted(redacted[argPhone]))
	checkTrue(t, audit.IsRedacted(redacted[argAddress1]))
	checkTrue(t, audit.IsRedacted(redacted[argCity]))
	checkTrue(t, audit.IsRedacted(redacted[argContactName]))
	checkTrue(t, audit.IsRedacted(redacted[argContactEmail]))
	checkElementsMatch(t,
		[]string{argKeyToken, argTaxID, argPhone, argAddress1, argCity, argContactName, argContactEmail},
		keys,
		"redacted-key list must report each scrubbed name once")
}

// TestRedactLeavesPIIWhenFlagOff is the inverse contract: when the
// operator opts out of PII redaction (audit.redact_pii=false), the
// middleware uses Redact (credentials-only). PII passes through in
// cleartext while credentials stay scrubbed.
func TestRedactLeavesPIIWhenFlagOff(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		argKeyToken: valFakeToken,
		argTaxID:    "TX-99",
		argPhone:    "+1-555-0100",
		argAddress1: "123 Main St",
	}

	redacted, keys := audit.Redact(args)

	checkTrue(t, audit.IsRedacted(redacted[argKeyToken]),
		"credential must always be redacted")
	checkEqual(t, "TX-99", redacted[argTaxID],
		"PII passes through when caller uses Redact (flag off path)")
	checkEqual(t, "+1-555-0100", redacted[argPhone])
	checkEqual(t, "123 Main St", redacted[argAddress1])
	checkEqual(t, []string{argKeyToken}, keys,
		"only the credential should appear in the redacted-key list")
}

// TestRedactionFieldSetPIIMatchesList: same drift guard as
// TestRedactionFieldSetMatchesList, applied to the PII helper.
func TestRedactionFieldSetPIIMatchesList(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFieldsPII()
	set := audit.RedactionFieldSetPII()

	checkLen(t, set, len(fields), "PII set must have one entry per list field")

	for _, name := range fields {
		_, present := set[name]
		checkTrue(t, present, "PII field %q must appear in the set", name)
	}
}
