package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// coercionDraftName is the draft the wrongly-typed-argument cases mutate.
const coercionDraftName = "coercion-draft"

// This file pins the reading Go gives a wrongly typed argument, which is the
// reference the Python readers in linodemcp/tools/argreader.py copy. It came
// from mcp-go's typed getters rather than from anything this repo owns, so a
// dependency bump could change it and take the two languages apart again with
// every gate still green.

// TestDraftNewRefusesANonStringName proves a non-string name refuses rather
// than keying a draft on the value.
func TestDraftNewRefusesANonStringName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	wantRefusal(t, draftState(reg), generatedBuilder(gentools.NewLinodeProfileDraftNewTool, &config.Config{}),
		map[string]any{keyName: 123}, wantDraftNameMissing)

	if names := reg.List(); len(names) != 0 {
		t.Errorf("reg.List() = %v, want none", names)
	}
}

// TestDraftSetReadsTheStringFalseAsFalse proves allow_yolo: "false" clears the
// flag, where plain truthiness would set it.
func TestDraftSetReadsTheStringFalseAsFalse(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	if _, err := reg.Create(coercionDraftName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	callBuilder(t, draftState(reg),
		generatedBuilder(gentools.NewLinodeProfileDraftSetTool, &config.Config{}), map[string]any{
			keyName: coercionDraftName, keyAllowYolo: caseFalse,
		})

	draft, found := reg.Get(coercionDraftName)
	if !found {
		t.Fatal("found = false, want true")
	}

	if draft.AllowYolo {
		t.Error("draft.AllowYolo = true, want false")
	}
}

// TestDraftSetDropsANonStringListEntry proves a wrongly typed list entry is
// dropped, so no value the caller never named reaches the draft.
func TestDraftSetDropsANonStringListEntry(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	if _, err := reg.Create(coercionDraftName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	callBuilder(t, draftState(reg),
		generatedBuilder(gentools.NewLinodeProfileDraftSetTool, &config.Config{}), map[string]any{
			keyName: coercionDraftName, "allowed_environments": []any{canRunEnvProd, 42},
		})

	draft, found := reg.Get(coercionDraftName)
	if !found {
		t.Fatal("found = false, want true")
	}

	if len(draft.AllowedEnvironments) != 1 || draft.AllowedEnvironments[0] != "prod" {
		t.Errorf("draft.AllowedEnvironments = %v, want [prod]", draft.AllowedEnvironments)
	}
}

// TestAuditRecentDefaultsAnUnparseableLimit proves limit: "abc" answers a
// window on the default rather than reporting a parse failure.
func TestAuditRecentDefaultsAnUnparseableLimit(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	_, _, handler := gentools.NewLinodeAuditRecentTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		"limit": "abc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("result.IsError = true, want false (%s)", behaviorText(t, result))
	}
}

// TestAuditExportAcceptsANumericStringMaxRecords proves max_records: "1" caps
// the export, where a type check on int alone would fall to the default.
func TestAuditExportAcceptsANumericStringMaxRecords(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	cfg := unopenableStoreConfig(t, stateHome)
	cfg.Audit.SQLite.Enabled = false

	_, _, handler := gentools.NewLinodeAuditExportTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		"format": "json", "max_records": "1",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded := decodeFallbackResult(t, result); decoded.RecordCount != 1 {
		t.Errorf("decoded.RecordCount = %d, want %d", decoded.RecordCount, 1)
	}
}

// TestBuilderNameRuleRefusesBeforeTheBody proves the name rule the *Input
// messages declare answers the same sentence the bodies do, through the
// generated handler, for both an absent name and one no string can hold.
func TestBuilderNameRuleRefusesBeforeTheBody(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		"absent name":     {},
		"non-string name": {keyName: 123},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := gentools.NewLinodeProfileDraftShowTool(&config.Config{})

			result, err := handler(
				tools.WithBuilderState(t.Context(), draftState(builder.NewRegistry())),
				createRequestWithArgs(t, args),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatalf("result.IsError = false, want true (%s)", behaviorText(t, result))
			}

			if got := behaviorText(t, result); got != wantDraftNameMissing {
				t.Errorf("refusal = %q, want %q", got, wantDraftNameMissing)
			}
		})
	}
}
