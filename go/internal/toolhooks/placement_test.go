package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	argLinodes  = "linodes"
	argPGType   = "placement_group_type"
	argPGPolicy = "placement_group_policy"

	pgLabel  = "pg-test"
	pgRegion = "us-east"
	pgType   = "anti_affinity:local"
	pgPolicy = "strict"

	// pgPaddingLabel is a label that is only whitespace.
	pgPaddingLabel = "   "
)

func TestLinodePlacementGroupCreateNormalizeTrimsEveryTextArgument(t *testing.T) {
	t.Parallel()

	args := map[string]any{
		labelArg:    "  " + pgLabel + "  ",
		argRegion:   " " + pgRegion + " ",
		argPGType:   " " + pgType + " ",
		argPGPolicy: " " + pgPolicy + " ",
	}

	request := requestWith(args)
	toolhooks.LinodePlacementGroupCreateNormalize(&request)

	for name, want := range map[string]string{
		labelArg:    pgLabel,
		argRegion:   pgRegion,
		argPGType:   pgType,
		argPGPolicy: pgPolicy,
	} {
		if args[name] != want {
			t.Errorf("%s = %v, want %v", name, args[name], want)
		}
	}
}

// A value the trim cannot apply to is left for the check to refuse, so the
// sentence a caller gets names the type rather than a blank.
func TestLinodePlacementGroupCreateNormalizeLeavesNonTextAlone(t *testing.T) {
	t.Parallel()

	args := map[string]any{labelArg: 123}

	request := requestWith(args)
	toolhooks.LinodePlacementGroupCreateNormalize(&request)

	if args[labelArg] != 123 {
		t.Errorf("%s = %v, want %v", labelArg, args[labelArg], 123)
	}
}

// placementSideEffects reads the prose out of a preview, which carries it as
// JSON rather than as the sentence itself.
func placementSideEffects(t *testing.T, preview string) []string {
	t.Helper()

	var decoded struct {
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(preview), &decoded); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return decoded.SideEffects
}

func TestLinodePlacementGroupUnassignPreviewNamesEveryLinode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{keyGrantID: 7, keyLabel: "pg-test"}); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argGroupID: 7,
		argLinodes: []any{123, 456},
		keyDryRun:  true,
	})

	result, err := toolhooks.LinodePlacementGroupUnassignPreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost, "/placement/groups/7/unassign", nil,
	)
	if err != nil {
		t.Fatalf("LinodePlacementGroupUnassignPreview: %v", err)
	}

	got := placementSideEffects(t, resultText(t, result))
	for _, want := range []string{
		"Linode 123 will be removed from placement group 7.",
		"Linode 456 will be removed from placement group 7.",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("side effects = %v, want one reading %q", got, want)
		}
	}
}

func TestLinodePlacementGroupAssignPreviewNamesEveryLinode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"id": 7, "label": pgLabel}); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{
		argGroupID: 7,
		argLinodes: []any{123, 456},
		keyDryRun:  true,
	})

	result, err := toolhooks.LinodePlacementGroupAssignPreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost, "/placement/groups/7/assign", nil,
	)
	if err != nil {
		t.Fatalf("LinodePlacementGroupAssignPreview: %v", err)
	}

	got := placementSideEffects(t, resultText(t, result))
	for _, want := range []string{
		"Linode 123 will be assigned to placement group 7.",
		"Linode 456 will be assigned to placement group 7.",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("side effects = %v, want one reading %q", got, want)
		}
	}
}
