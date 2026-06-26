package audit_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
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
		if dup {
			t.Error("dup = true, want false")
		}

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

	if !reflect.DeepEqual(redacted["linode_id"], 12345) {
		t.Errorf("got %v, want %v", redacted["linode_id"], 12345)
	}

	if !reflect.DeepEqual(redacted[argKeyLabel], "my-instance") {
		t.Errorf("redacted[argKeyLabel] = %v, want %v", redacted[argKeyLabel], "my-instance")
	}

	if !audit.IsRedacted(redacted[argRootPass]) {
		t.Error("audit.IsRedacted(redacted[argRootPass]) = false, want true")
	}

	if !audit.IsRedacted(redacted[argKeyToken]) {
		t.Error("audit.IsRedacted(redacted[argKeyToken]) = false, want true")
	}
	{
		gotEls := slices.Clone(keys)
		wantEls := slices.Clone([]string{argRootPass, argKeyToken})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", keys, []string{argRootPass, argKeyToken})
		}
	}
}

func TestRedactAccountUserUpdateSensitiveFields(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		tcPasswordCreated: "2024-01-02T03:04:05",
		tcSSHKeys:         []any{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest"},
	}

	redacted, keys := audit.Redact(args)

	if !audit.IsRedacted(redacted[tcPasswordCreated]) {
		t.Error("expected condition to be true")
	}

	if !audit.IsRedacted(redacted[tcSSHKeys]) {
		t.Error("expected condition to be true")
	}
	{
		gotEls := slices.Clone(keys)
		wantEls := slices.Clone([]string{tcPasswordCreated, tcSSHKeys})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", keys, []string{tcPasswordCreated, tcSSHKeys})
		}
	}
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
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !audit.IsRedacted(nested["api_key"]) {
		t.Error("expected condition to be true")
	}

	if !reflect.DeepEqual(nested[keyRegion], valUSEast) {
		t.Errorf("nested[keyRegion] = %v, want %v", nested[keyRegion], valUSEast)
	}

	if !slices.Contains(keys, "api_key") {
		t.Errorf("keys does not contain %v", "api_key")
	}
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

	if !reflect.DeepEqual(redacted["cluster_root_pass"], "should-pass-through-because-variant") {
		t.Errorf("got %v, want %v", redacted["cluster_root_pass"], "should-pass-through-because-variant")
	}

	if !reflect.DeepEqual(redacted["new_root_pass"], "also-variant") {
		t.Errorf("got %v, want %v", redacted["new_root_pass"], "also-variant")
	}

	if !reflect.DeepEqual(redacted["Root_Pass"], "different-case") {
		t.Errorf("got %v, want %v", redacted["Root_Pass"], "different-case")
	}

	if len(keys) != 0 {
		t.Errorf("keys = %v, want empty", keys)
	}
}

// TestRedactNilArgsProducesEmptyResult covers the empty-input path.
// The walker must not panic on nil args; it returns nil for the map
// and an empty slice for the keys list.
func TestRedactNilArgsProducesEmptyResult(t *testing.T) {
	t.Parallel()

	redacted, keys := audit.Redact(nil)

	if redacted != nil {
		t.Errorf("redacted = %v, want nil", redacted)
	}

	if len(keys) != 0 {
		t.Errorf("keys = %v, want empty", keys)
	}
}

// TestRedactReturnsCopyNotMutation guards the no-mutation contract.
// The redact walker copies values into a new map; the caller's
// original args remain untouched even when sensitive keys are
// present.
func TestRedactReturnsCopyNotMutation(t *testing.T) {
	t.Parallel()

	args := map[string]any{argRootPass: "secret"}

	_, _ = audit.Redact(args)

	if !reflect.DeepEqual(args[argRootPass], "secret") {
		t.Errorf("args[argRootPass] = %v, want %v", args[argRootPass], "secret")
	}
}

// TestRedactionFieldSetMatchesList confirms the set helper builds a
// set with the same membership as the list. A drift between the two
// would cause the runtime walker to miss redactions.
func TestRedactionFieldSetMatchesList(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFields()
	set := audit.RedactionFieldSet()

	if len(set) != len(fields) {
		t.Errorf("len(set) = %d, want %d", len(set), len(fields))
	}

	for _, name := range fields {
		_, present := set[name]
		if !present {
			t.Error("present = false, want true")
		}
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
		"phone",
		"phone_number",
		"state",
		"tax_id",
		"zip",
	}

	{
		gotEls := slices.Clone(audit.RedactionFieldsPII())
		wantEls := slices.Clone(expected)

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", audit.RedactionFieldsPII(), expected)
		}
	}
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
		if dup {
			t.Error("dup = true, want false")
		}

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
		if overlap {
			t.Error("overlap = true, want false")
		}
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
		argContactName:  "Jane Doe",         // not in PII list, must pass through
		argContactEmail: "jane@example.org", // not in PII list, must pass through
		"country":       "us",               // not in PII list, must pass through
	}

	redacted, keys := audit.RedactWithPII(args)

	if !reflect.DeepEqual(redacted[argLinodeID], 42) {
		t.Errorf("redacted[argLinodeID] = %v, want %v", redacted[argLinodeID], 42)
	}

	if !reflect.DeepEqual(redacted[argKeyLabel], "primary") {
		t.Errorf("redacted[argKeyLabel] = %v, want %v", redacted[argKeyLabel], "primary")
	}

	if !reflect.DeepEqual(redacted["country"], "us") {
		t.Errorf("got %v, want %v", redacted["country"], "us")
	}

	if !audit.IsRedacted(redacted[argKeyToken]) {
		t.Error("audit.IsRedacted(redacted[argKeyToken]) = false, want true")
	}

	if !audit.IsRedacted(redacted[argTaxID]) {
		t.Error("audit.IsRedacted(redacted[argTaxID]) = false, want true")
	}

	if !audit.IsRedacted(redacted[argPhone]) {
		t.Error("audit.IsRedacted(redacted[argPhone]) = false, want true")
	}

	if !audit.IsRedacted(redacted[argAddress1]) {
		t.Error("audit.IsRedacted(redacted[argAddress1]) = false, want true")
	}

	if !audit.IsRedacted(redacted[argCity]) {
		t.Error("audit.IsRedacted(redacted[argCity]) = false, want true")
	}

	if !reflect.DeepEqual(redacted[argContactName], "Jane Doe") {
		t.Errorf("redacted[argContactName] = %v, want %v", redacted[argContactName], "Jane Doe")
	}

	if !reflect.DeepEqual(redacted[argContactEmail], "jane@example.org") {
		t.Errorf("redacted[argContactEmail] = %v, want %v", redacted[argContactEmail], "jane@example.org")
	}
	{
		gotEls := slices.Clone(keys)
		wantEls := slices.Clone([]string{argKeyToken, argTaxID, argPhone, argAddress1, argCity})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", keys, []string{argKeyToken, argTaxID, argPhone, argAddress1, argCity})
		}
	}
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

	if !audit.IsRedacted(redacted[argKeyToken]) {
		t.Error("audit.IsRedacted(redacted[argKeyToken]) = false, want true")
	}

	if !reflect.DeepEqual(redacted[argTaxID], "TX-99") {
		t.Errorf("redacted[argTaxID] = %v, want %v", redacted[argTaxID], "TX-99")
	}

	if !reflect.DeepEqual(redacted[argPhone], "+1-555-0100") {
		t.Errorf("redacted[argPhone] = %v, want %v", redacted[argPhone], "+1-555-0100")
	}

	if !reflect.DeepEqual(redacted[argAddress1], "123 Main St") {
		t.Errorf("redacted[argAddress1] = %v, want %v", redacted[argAddress1], "123 Main St")
	}

	if !reflect.DeepEqual(keys, []string{argKeyToken}) {
		t.Errorf("keys = %v, want %v", keys, []string{argKeyToken})
	}
}

// TestRedactionFieldSetPIIMatchesList: same drift guard as
// TestRedactionFieldSetMatchesList, applied to the PII helper.
func TestRedactionFieldSetPIIMatchesList(t *testing.T) {
	t.Parallel()

	fields := audit.RedactionFieldsPII()
	set := audit.RedactionFieldSetPII()

	if len(set) != len(fields) {
		t.Errorf("len(set) = %d, want %d", len(set), len(fields))
	}

	for _, name := range fields {
		_, present := set[name]
		if !present {
			t.Error("present = false, want true")
		}
	}
}
