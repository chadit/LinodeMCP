package tools_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	// mutateDraftName is the conventional draft name reused across the
	// happy-path tests for add/remove/set.
	mutateDraftName = "my-draft"
)

// mutateFixtureCatalog returns the static tool catalog the
// _draft_add_tools tests expand patterns against. Includes a compute
// trio (boot + reboot + shutdown) so wildcard expansion can be
// verified without being dominated by a single literal.
func mutateFixtureCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: toolInstanceBoot, Capability: profiles.CapWrite},
		{Name: toolInstanceReboot, Capability: profiles.CapWrite},
		{Name: tcLinodeInstanceShutdown, Capability: profiles.CapWrite},
		{Name: tcLinodeDomainGet, Capability: profiles.CapRead},
		{Name: toolHello, Capability: profiles.CapMeta},
	}
}

// mutateState carries the registry plus the catalog _draft_add_tools expands
// patterns against.
func mutateState(reg *builder.Registry) *tools.BuilderState {
	return builderState(reg, mutateFixtureCatalog(), noProfile, emptyConfig())
}

// mutateHandler is the generated handler behind one draft mutator. The three
// declare their answers now, so every case here drives the handler, which is
// where the whole body lives: the reads, the operation and the sentences.
func mutateHandler(factory builderFactory) builderHandler {
	return generatedBuilder(factory, &config.Config{})
}

// callMutateAnswer runs one mutator with the state attached and returns the
// parsed response. None of the three reads the configuration.
func callMutateAnswer(
	t *testing.T,
	state *tools.BuilderState,
	factory builderFactory,
	args map[string]any,
) map[string]any {
	t.Helper()

	return builderBody(t, callBuilder(t, state, mutateHandler(factory), args))
}

// wantMutateRefusal asserts one mutator refused with exactly the sentence.
func wantMutateRefusal(
	t *testing.T, state *tools.BuilderState, factory builderFactory,
	args map[string]any, want string,
) {
	t.Helper()

	wantRefusal(t, state, mutateHandler(factory), args, want)
}

// TestDraftAddToolsRegistration locks in the CapMeta tag and tool
// name. CapMeta is what keeps the builder available under read-only
// profiles; a regression on the tag would silently break the
// conversation flow.
func TestDraftAddToolsRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeProfileDraftAddToolsTool(&config.Config{})

	if tool.Name != "linode_profile_draft_add_tools" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_draft_add_tools")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestDraftAddToolsAddsLiterals exercises the no-wildcard path.
// Literal names match the catalog and land on the draft sorted.
func TestDraftAddToolsAddsLiterals(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolInstanceBoot, toolHello},
	})

	if !reflect.DeepEqual(out[keyName], mutateDraftName) {
		t.Errorf("out[keyName] = %v, want %v", out[keyName], mutateDraftName)
	}

	added, ok := out["added"].([]any)
	if !ok {
		t.Error("ok = false, want true")
	}

	gotElems1 := make([]string, len(added))
	for i, v := range added {
		gotElems1[i], _ = v.(string)
	}

	wantElems1 := slices.Clone([]string{toolHello, toolInstanceBoot})

	slices.Sort(gotElems1)
	slices.Sort(wantElems1)

	if !slices.Equal(gotElems1, wantElems1) {
		t.Errorf("elements = %v, want %v (any order)", gotElems1, []string{toolHello, toolInstanceBoot})
	}

	draft, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	gotElems2 := slices.Clone(draft.AllowedTools)
	wantElems2 := slices.Clone([]string{toolHello, toolInstanceBoot})

	slices.Sort(gotElems2)
	slices.Sort(wantElems2)

	if !slices.Equal(gotElems2, wantElems2) {
		t.Errorf("elements = %v, want %v (any order)", gotElems2, []string{toolHello, toolInstanceBoot})
	}
}

// TestDraftAddToolsExpandsWildcards verifies the wildcard path.
// `linode_instance_*` against the 3-tool fixture must add exactly
// boot + reboot + shutdown.
func TestDraftAddToolsExpandsWildcards(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolInstanceGlob},
	})

	added, _ := out["added"].([]any)

	gotElems3 := make([]string, len(added))
	for i, v := range added {
		gotElems3[i], _ = v.(string)
	}

	wantElems3 := slices.Clone([]string{toolInstanceBoot, toolInstanceReboot, tcLinodeInstanceShutdown})

	slices.Sort(gotElems3)
	slices.Sort(wantElems3)

	if !slices.Equal(gotElems3, wantElems3) {
		t.Errorf("elements = %v, want %v (any order)", gotElems3, []string{toolInstanceBoot, toolInstanceReboot, tcLinodeInstanceShutdown})
	}
}

// TestDraftAddToolsDedupesAgainstExisting confirms the no-duplicate
// contract. A second add of the same literal returns an empty added
// list since the draft already has it.
func TestDraftAddToolsDedupesAgainstExisting(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// First add: toolHello lands.
	_ = callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	// Second add: toolHello is already there.
	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	added, _ := out["added"].([]any)
	if len(added) != 0 {
		t.Errorf("added = %v, want empty", added)
	}

	draft, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	if !reflect.DeepEqual(draft.AllowedTools, []string{toolHello}) {
		t.Errorf("draft.AllowedTools = %v, want %v", draft.AllowedTools, []string{toolHello})
	}
}

// TestDraftAddToolsRefusesUnknownDraft surfaces ErrDraftNotFound
// when the draft isn't in the registry. Tool error keeps the user
// from accidentally constructing one through add-on-missing.
func TestDraftAddToolsRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, map[string]any{
		keyName:  envNonexistent,
		keyTools: []any{toolHello},
	}, "draft not found: nonexistent")
}

// TestDraftAddToolsRefusesMissingName covers the validation guard.
func TestDraftAddToolsRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftAddToolsTool, nil, wantDraftNameMissing)
}

// TestDraftRemoveToolsRefusesMissingName mirrors the add-tools guard.
func TestDraftRemoveToolsRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftRemoveToolsTool, map[string]any{keyTools: []any{toolHello}},
		wantDraftNameMissing)
}

// TestDraftRemoveToolsRemovesLiterals is the happy path: literal
// names matched against the draft's existing AllowedTools come out.
func TestDraftRemoveToolsRemovesLiterals(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolInstanceBoot, toolInstanceReboot, toolHello}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftRemoveToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	removed, _ := out["removed"].([]any)
	if !reflect.DeepEqual(removed, []any{toolHello}) {
		t.Errorf("removed = %v, want %v", removed, []any{toolHello})
	}

	updated, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	gotElems4 := slices.Clone(updated.AllowedTools)
	wantElems4 := slices.Clone([]string{toolInstanceBoot, toolInstanceReboot})

	slices.Sort(gotElems4)
	slices.Sort(wantElems4)

	if !slices.Equal(gotElems4, wantElems4) {
		t.Errorf("elements = %v, want %v (any order)", gotElems4, []string{toolInstanceBoot, toolInstanceReboot})
	}
}

// TestDraftRemoveToolsExpandsWildcardsAgainstDraft confirms that
// wildcards match the draft's CURRENT state, not the live catalog.
// `linode_instance_*` removes exactly the instance tools the draft
// already had.
func TestDraftRemoveToolsExpandsWildcardsAgainstDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolInstanceBoot, toolInstanceReboot, toolHello}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftRemoveToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolInstanceGlob},
	})

	removed, _ := out["removed"].([]any)

	gotElems5 := make([]string, len(removed))
	for i, v := range removed {
		gotElems5[i], _ = v.(string)
	}

	wantElems5 := slices.Clone([]string{toolInstanceBoot, toolInstanceReboot})

	slices.Sort(gotElems5)
	slices.Sort(wantElems5)

	if !slices.Equal(gotElems5, wantElems5) {
		t.Errorf("elements = %v, want %v (any order)", gotElems5, []string{toolInstanceBoot, toolInstanceReboot})
	}

	updated, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	if !reflect.DeepEqual(updated.AllowedTools, []string{toolHello}) {
		t.Errorf("updated.AllowedTools = %v, want %v", updated.AllowedTools, []string{toolHello})
	}
}

// TestDraftRemoveToolsNoMatchIsBenign returns an empty removed list
// when no patterns match. No error, no side effects.
func TestDraftRemoveToolsNoMatchIsBenign(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolHello}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftRemoveToolsTool, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{"nonexistent-tool"},
	})

	removed, _ := out["removed"].([]any)
	if len(removed) != 0 {
		t.Errorf("removed = %v, want empty", removed)
	}

	updated, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	if !reflect.DeepEqual(updated.AllowedTools, []string{toolHello}) {
		t.Errorf("updated.AllowedTools = %v, want %v", updated.AllowedTools, []string{toolHello})
	}
}

// TestDraftRemoveToolsRefusesUnknownDraft surfaces ErrDraftNotFound.
func TestDraftRemoveToolsRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftRemoveToolsTool, map[string]any{
		keyName:  envNonexistent,
		keyTools: []any{toolHello},
	}, "draft not found: nonexistent")
}

// TestDraftSetRegistersAndIsCapMeta covers the static contract.
func TestDraftSetRegistersAndIsCapMeta(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeProfileDraftSetTool(&config.Config{})

	if tool.Name != "linode_profile_draft_set" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_draft_set")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestDraftSetEnvironmentsOnly verifies that the handler only
// touches the fields the caller actually provided. Missing fields
// stay unchanged.
func TestDraftSetEnvironmentsOnly(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedEnvironments = []string{"old-env"}
	draft.RequiredTokenScopes = []string{tcScopeRead}
	draft.AllowYolo = true

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{
		keyName:               "my-draft",
		tcAllowedEnvironments: []any{envProd},
	})

	changes, _ := out["changes"].(map[string]any)
	if _, ok := changes[tcAllowedEnvironments]; !ok {
		t.Errorf("changes missing key %v", tcAllowedEnvironments)
	}

	if _, ok := changes[tcRequiredTokenScopes]; ok {
		t.Errorf("unspecified fields must not appear in changes: changes unexpectedly has key %q", tcRequiredTokenScopes)
	}

	if _, ok := changes["allow_yolo"]; ok {
		t.Errorf("changes unexpectedly has key %q", "allow_yolo")
	}

	updated, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	if !reflect.DeepEqual(updated.AllowedEnvironments, []string{envProd}) {
		t.Errorf("updated.AllowedEnvironments = %v, want %v", updated.AllowedEnvironments, []string{envProd})
	}

	if !reflect.DeepEqual(updated.RequiredTokenScopes, []string{tcScopeRead}) {
		t.Errorf("updated.RequiredTokenScopes = %v, want %v", updated.RequiredTokenScopes, []string{tcScopeRead})
	}

	if !updated.AllowYolo {
		t.Error("updated.AllowYolo = false, want true")
	}
}

// TestDraftSetAllowYoloFlipsCleanly covers the bool field.
// Setting allow_yolo=true on a draft that started false is a
// material policy change; the change should appear in the response.
func TestDraftSetAllowYoloFlipsCleanly(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{
		keyName:      mutateDraftName,
		keyAllowYolo: true,
	})

	changes, _ := out["changes"].(map[string]any)
	if !reflect.DeepEqual(changes["allow_yolo"], true) {
		t.Errorf("got %v, want %v", changes["allow_yolo"], true)
	}

	updated, found := reg.Get(mutateDraftName)
	if !found {
		t.Error("found = false, want true")
	}

	if !updated.AllowYolo {
		t.Error("updated.AllowYolo = false, want true")
	}
}

// TestDraftSetMultipleFieldsAtOnce confirms that a single call can
// update every settable field.
func TestDraftSetMultipleFieldsAtOnce(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{
		keyName:               mutateDraftName,
		tcAllowedEnvironments: []any{envProd, "dev"},
		tcRequiredTokenScopes: []any{scopeLinodesReadWrite},
		keyAllowYolo:          true,
	})

	changes, _ := out["changes"].(map[string]any)
	if len(changes) != 3 {
		t.Errorf("len(changes) = %d, want %d", len(changes), 3)
	}
}

// TestDraftSetEmptyCallNoOps covers the call-with-just-name path.
// The handler returns an empty changes map; no fields are written.
func TestDraftSetEmptyCallNoOps(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(mutateDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callMutateAnswer(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{keyName: mutateDraftName})

	changes, _ := out["changes"].(map[string]any)
	if len(changes) != 0 {
		t.Errorf("changes = %v, want empty", changes)
	}
}

// TestDraftSetRefusesUnknownDraft surfaces ErrDraftNotFound when any
// of the per-field setters runs.
func TestDraftSetRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{
		keyName:      envNonexistent,
		keyAllowYolo: true,
	}, "draft not found: nonexistent")
}

// TestDraftSetRefusesMissingName mirrors the add- and remove-tools guards.
func TestDraftSetRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, map[string]any{keyAllowYolo: true},
		wantDraftNameMissing)
}

// TestDraftSetRefusesUnknownDraftPerField covers each settable field's own
// refusal: the handler sets them one at a time, so a field whose miss was
// never exercised could report something else entirely.
func TestDraftSetRefusesUnknownDraftPerField(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		tcAllowedEnvironments: {keyName: envNonexistent, tcAllowedEnvironments: []any{envProd}},
		tcRequiredTokenScopes: {keyName: envNonexistent, tcRequiredTokenScopes: []any{scopeLinodesReadWrite}},
	}

	for field, args := range cases {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			reg := builder.NewRegistry()

			wantMutateRefusal(t, mutateState(reg), gentools.NewLinodeProfileDraftSetTool, args,
				"draft not found: nonexistent")
		})
	}
}

// TestDraftMutatorsRespectContextCancellation locks the cancellation
// contract across all three handlers.
func TestDraftMutatorsRespectContextCancellation(t *testing.T) {
	t.Parallel()

	_, _, addHandler := gentools.NewLinodeProfileDraftAddToolsTool(&config.Config{})
	_, _, removeHandler := gentools.NewLinodeProfileDraftRemoveToolsTool(&config.Config{})
	_, _, setHandler := gentools.NewLinodeProfileDraftSetTool(&config.Config{})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, errAdd := addHandler(ctx, mcp.CallToolRequest{})
	_, errRemove := removeHandler(ctx, mcp.CallToolRequest{})
	_, errSet := setHandler(ctx, mcp.CallToolRequest{})

	if !errors.Is(errAdd, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errAdd)
	}

	if !errors.Is(errRemove, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errRemove)
	}

	if !errors.Is(errSet, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errSet)
	}
}
