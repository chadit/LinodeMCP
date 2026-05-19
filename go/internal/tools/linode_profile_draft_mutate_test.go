package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/internal/tools"
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
		{Name: "linode_instance_shutdown", Capability: profiles.CapWrite},
		{Name: "linode_domain_get", Capability: profiles.CapRead},
		{Name: toolHello, Capability: profiles.CapMeta},
	}
}

// staticCatalog wraps a slice as a CatalogSnapshot. Tests that don't
// care about hot-reloads-mid-call (most of them) use this single-shot
// adapter.
func staticCatalog(catalog []profiles.ToolDescriptor) tools.CatalogSnapshot {
	return func() []profiles.ToolDescriptor { return catalog }
}

// callMutateHandler invokes the handler and returns the parsed
// response. Mirrors the helper in linode_profile_draft_test.go.
func callMutateHandler(
	t *testing.T,
	handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error),
	args map[string]any,
) map[string]any {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "result content must be TextContent")

	var out map[string]any

	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &out))

	return out
}

// TestDraftAddToolsRegistration locks in the CapMeta tag and tool
// name. CapMeta is what keeps the builder available under read-only
// profiles; a regression on the tag would silently break the
// conversation flow.
func TestDraftAddToolsRegistration(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	tool, capability, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	assert.Equal(t, "linode_profile_draft_add_tools", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, profiles.CapMeta, capability)
	assert.NotNil(t, handler)
}

// TestDraftAddToolsAddsLiterals exercises the no-wildcard path.
// Literal names match the catalog and land on the draft sorted.
func TestDraftAddToolsAddsLiterals(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolInstanceBoot, toolHello},
	})

	require.Equal(t, mutateDraftName, out[keyName])
	added, ok := out["added"].([]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []any{toolHello, toolInstanceBoot}, added)

	draft, _ := reg.Get(mutateDraftName)
	assert.ElementsMatch(
		t,
		[]string{toolHello, toolInstanceBoot},
		draft.AllowedTools,
	)
}

// TestDraftAddToolsExpandsWildcards verifies the wildcard path.
// `linode_instance_*` against the 3-tool fixture must add exactly
// boot + reboot + shutdown.
func TestDraftAddToolsExpandsWildcards(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{"linode_instance_*"},
	})

	added, _ := out["added"].([]any)
	assert.ElementsMatch(
		t,
		[]any{toolInstanceBoot, toolInstanceReboot, "linode_instance_shutdown"},
		added,
	)
}

// TestDraftAddToolsDedupesAgainstExisting confirms the no-duplicate
// contract. A second add of the same literal returns an empty added
// list since the draft already has it.
func TestDraftAddToolsDedupesAgainstExisting(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	// First add: toolHello lands.
	_ = callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	// Second add: toolHello is already there.
	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	added, _ := out["added"].([]any)
	assert.Empty(t, added, "second add of the same literal must report empty added list")

	draft, _ := reg.Get(mutateDraftName)
	assert.Equal(t, []string{toolHello}, draft.AllowedTools,
		"draft must contain the literal once, not twice")
}

// TestDraftAddToolsRefusesUnknownDraft surfaces ErrDraftNotFound
// when the draft isn't in the registry. Tool error keeps the user
// from accidentally constructing one through add-on-missing.
func TestDraftAddToolsRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:  envNonexistent,
		keyTools: []any{toolHello},
	}

	_, err := handler(t.Context(), req)
	require.ErrorIs(t, err, builder.ErrDraftNotFound)
}

// TestDraftAddToolsRefusesMissingName covers the validation guard.
func TestDraftAddToolsRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)

	_, err := handler(t.Context(), mcp.CallToolRequest{})
	require.ErrorIs(t, err, tools.ErrDraftNameMissing)
}

// TestDraftRemoveToolsRemovesLiterals is the happy path: literal
// names matched against the draft's existing AllowedTools come out.
func TestDraftRemoveToolsRemovesLiterals(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	draft, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	draft.AllowedTools = []string{toolInstanceBoot, toolInstanceReboot, toolHello}

	_, _, handler := tools.NewLinodeProfileDraftRemoveToolsTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{toolHello},
	})

	removed, _ := out["removed"].([]any)
	assert.Equal(t, []any{toolHello}, removed)

	updated, _ := reg.Get(mutateDraftName)
	assert.ElementsMatch(
		t,
		[]string{toolInstanceBoot, toolInstanceReboot},
		updated.AllowedTools,
	)
}

// TestDraftRemoveToolsExpandsWildcardsAgainstDraft confirms that
// wildcards match the draft's CURRENT state, not the live catalog.
// `linode_instance_*` removes exactly the instance tools the draft
// already had.
func TestDraftRemoveToolsExpandsWildcardsAgainstDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	draft, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	draft.AllowedTools = []string{toolInstanceBoot, toolInstanceReboot, toolHello}

	_, _, handler := tools.NewLinodeProfileDraftRemoveToolsTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{"linode_instance_*"},
	})

	removed, _ := out["removed"].([]any)
	assert.ElementsMatch(
		t,
		[]any{toolInstanceBoot, toolInstanceReboot},
		removed,
	)

	updated, _ := reg.Get(mutateDraftName)
	assert.Equal(t, []string{toolHello}, updated.AllowedTools)
}

// TestDraftRemoveToolsNoMatchIsBenign returns an empty removed list
// when no patterns match. No error, no side effects.
func TestDraftRemoveToolsNoMatchIsBenign(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	draft, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	draft.AllowedTools = []string{toolHello}

	_, _, handler := tools.NewLinodeProfileDraftRemoveToolsTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:  mutateDraftName,
		keyTools: []any{"nonexistent-tool"},
	})

	removed, _ := out["removed"].([]any)
	assert.Empty(t, removed)

	updated, _ := reg.Get(mutateDraftName)
	assert.Equal(t, []string{toolHello}, updated.AllowedTools)
}

// TestDraftRemoveToolsRefusesUnknownDraft surfaces ErrDraftNotFound.
func TestDraftRemoveToolsRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftRemoveToolsTool(reg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:  envNonexistent,
		keyTools: []any{toolHello},
	}

	_, err := handler(t.Context(), req)
	require.ErrorIs(t, err, builder.ErrDraftNotFound)
}

// TestDraftSetRegistersAndIsCapMeta covers the static contract.
func TestDraftSetRegistersAndIsCapMeta(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	tool, capability, handler := tools.NewLinodeProfileDraftSetTool(reg)

	assert.Equal(t, "linode_profile_draft_set", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, profiles.CapMeta, capability)
	assert.NotNil(t, handler)
}

// TestDraftSetEnvironmentsOnly verifies that the handler only
// touches the fields the caller actually provided. Missing fields
// stay unchanged.
func TestDraftSetEnvironmentsOnly(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	draft, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	draft.AllowedEnvironments = []string{"old-env"}
	draft.RequiredTokenScopes = []string{"scope:read"}
	draft.AllowYolo = true

	_, _, handler := tools.NewLinodeProfileDraftSetTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:                "my-draft",
		"allowed_environments": []any{envProd},
	})

	changes, _ := out["changes"].(map[string]any)
	require.Contains(t, changes, "allowed_environments")
	assert.NotContains(t, changes, "required_token_scopes",
		"unspecified fields must not appear in changes")
	assert.NotContains(t, changes, "allow_yolo")

	updated, _ := reg.Get(mutateDraftName)
	assert.Equal(t, []string{envProd}, updated.AllowedEnvironments)
	assert.Equal(t, []string{"scope:read"}, updated.RequiredTokenScopes,
		"untouched field must keep its prior value")
	assert.True(t, updated.AllowYolo,
		"untouched flag must keep its prior value")
}

// TestDraftSetAllowYoloFlipsCleanly covers the bool field.
// Setting allow_yolo=true on a draft that started false is a
// material policy change; the change should appear in the response.
func TestDraftSetAllowYoloFlipsCleanly(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSetTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:      mutateDraftName,
		keyAllowYolo: true,
	})

	changes, _ := out["changes"].(map[string]any)
	assert.Equal(t, true, changes["allow_yolo"])

	updated, _ := reg.Get(mutateDraftName)
	assert.True(t, updated.AllowYolo)
}

// TestDraftSetMultipleFieldsAtOnce confirms that a single call can
// update every settable field.
func TestDraftSetMultipleFieldsAtOnce(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSetTool(reg)

	out := callMutateHandler(t, handler, map[string]any{
		keyName:                 mutateDraftName,
		"allowed_environments":  []any{envProd, "dev"},
		"required_token_scopes": []any{"linodes:read_write"},
		keyAllowYolo:            true,
	})

	changes, _ := out["changes"].(map[string]any)
	assert.Len(t, changes, 3,
		"every supplied field must appear in changes")
}

// TestDraftSetEmptyCallNoOps covers the call-with-just-name path.
// The handler returns an empty changes map; no fields are written.
func TestDraftSetEmptyCallNoOps(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(mutateDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSetTool(reg)

	out := callMutateHandler(t, handler, map[string]any{keyName: mutateDraftName})

	changes, _ := out["changes"].(map[string]any)
	assert.Empty(t, changes)
}

// TestDraftSetRefusesUnknownDraft surfaces ErrDraftNotFound when any
// of the per-field setters runs.
func TestDraftSetRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftSetTool(reg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:      envNonexistent,
		keyAllowYolo: true,
	}

	_, err := handler(t.Context(), req)
	require.ErrorIs(t, err, builder.ErrDraftNotFound)
}

// TestDraftMutatorsRespectContextCancellation locks the cancellation
// contract across all three handlers.
func TestDraftMutatorsRespectContextCancellation(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, addHandler := tools.NewLinodeProfileDraftAddToolsTool(
		reg,
		staticCatalog(mutateFixtureCatalog()),
	)
	_, _, removeHandler := tools.NewLinodeProfileDraftRemoveToolsTool(reg)
	_, _, setHandler := tools.NewLinodeProfileDraftSetTool(reg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, errAdd := addHandler(ctx, mcp.CallToolRequest{})
	_, errRemove := removeHandler(ctx, mcp.CallToolRequest{})
	_, errSet := setHandler(ctx, mcp.CallToolRequest{})

	require.ErrorIs(t, errAdd, context.Canceled)
	require.ErrorIs(t, errRemove, context.Canceled)
	require.ErrorIs(t, errSet, context.Canceled)
}
