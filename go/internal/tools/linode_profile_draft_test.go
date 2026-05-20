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
	// draftFixtureName is the conventional name reused across happy-path
	// tests for new/show/discard. Hoisted as a constant per goconst.
	draftFixtureName = "dns-readall"
	// cloneSourceName is the resolver-fixture profile callers can clone
	// from in the clone_from path tests.
	cloneSourceName = "compute-admin"
)

// fixtureSourceProfile is the canonical Profile the test resolver
// returns when callers ask for cloneSourceName. Distinct field values
// per slice so test assertions can spot field-level mistakes.
func fixtureSourceProfile() profiles.Profile {
	return profiles.Profile{
		Name:                cloneSourceName,
		Description:         "Compute admin clone source",
		AllowedTools:        []string{"linode_instance_boot", "linode_instance_list"},
		AllowedEnvironments: []string{envProd},
		RequiredTokenScopes: []string{"linodes:read_write"},
		AllowYolo:           false,
	}
}

// fixtureResolver returns a Phase 8.3 ProfileResolver that knows about
// exactly cloneSourceName. Anything else returns (zero, false). This
// is the minimal contract _draft_new depends on; the production
// resolver (Server.LookupProfile) consults the live config.
func fixtureResolver() tools.ProfileResolver {
	src := fixtureSourceProfile()

	return func(name string) (profiles.Profile, bool) {
		if name == cloneSourceName {
			return src, true
		}

		return profiles.Profile{}, false
	}
}

// callDraftHandler invokes the given handler with the arg map and
// returns the parsed JSON object. Cuts the boilerplate the
// parameterized tests would otherwise repeat per case.
func callDraftHandler(
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

// TestDraftNewRegistration locks in the static contract: the tool's
// name, description presence, and CapMeta tag. CapMeta is what makes
// the builder tools always-available regardless of the active profile.
func TestDraftNewRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeProfileDraftNewTool(
		builder.NewRegistry(),
		fixtureResolver(),
	)

	assert.Equal(t, "linode_profile_draft_new", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, profiles.CapMeta, capability)
	assert.NotNil(t, handler)
}

// TestDraftNewCreatesEmptyDraft is the no-clone-from happy path.
// Resulting draft carries the requested name and empty slices/zero
// flag for everything else.
func TestDraftNewCreatesEmptyDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftNewTool(reg, fixtureResolver())

	out := callDraftHandler(t, handler, map[string]any{keyName: draftFixtureName})

	assert.Equal(t, draftFixtureName, out[keyName])
	assert.Empty(t, out["description"])
	assert.Equal(t, []any{}, out["allowed_tools"])
	assert.Equal(t, []any{}, out["allowed_environments"])
	assert.Equal(t, []any{}, out["required_token_scopes"])
	assert.Equal(t, false, out["allow_yolo"])

	// Registry side-effect: the draft is now retrievable.
	_, ok := reg.Get(draftFixtureName)
	assert.True(t, ok, "draft must be registered after _new returns")
}

// TestDraftNewClonesFromSource covers the clone_from path: every
// field on the source profile lands on the new draft.
func TestDraftNewClonesFromSource(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftNewTool(reg, fixtureResolver())

	out := callDraftHandler(t, handler, map[string]any{
		keyName:      draftFixtureName,
		"clone_from": cloneSourceName,
	})

	src := fixtureSourceProfile()

	assert.Equal(t, draftFixtureName, out[keyName])
	assert.Equal(t, src.Description, out["description"])
	assert.Equal(t, anySlice(src.AllowedTools), out["allowed_tools"])
	assert.Equal(t, anySlice(src.AllowedEnvironments), out["allowed_environments"])
	assert.Equal(t, anySlice(src.RequiredTokenScopes), out["required_token_scopes"])
	assert.Equal(t, src.AllowYolo, out["allow_yolo"])
}

// TestDraftNewRefusesMissingName covers the validation guard. The
// schema marks name as required so MCP should reject before the
// handler runs, but we belt-and-suspenders inside the handler too.
func TestDraftNewRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftNewTool(reg, fixtureResolver())

	_, err := handler(t.Context(), mcp.CallToolRequest{})

	require.ErrorIs(t, err, tools.ErrDraftNameMissing)
}

// TestDraftNewRefusesUnknownCloneSource covers the unknown-source path.
// The user typo'd a profile name; surface the error rather than
// silently producing an empty draft.
func TestDraftNewRefusesUnknownCloneSource(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftNewTool(reg, fixtureResolver())

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:      draftFixtureName,
		"clone_from": "nonexistent-profile",
	}

	_, err := handler(t.Context(), req)

	require.ErrorIs(t, err, tools.ErrCloneSourceMissing)

	_, exists := reg.Get(draftFixtureName)
	assert.False(t, exists, "failed _new must not leave a draft behind")
}

// TestDraftNewRefusesDuplicateName surfaces the underlying
// builder.ErrDraftExists. The user must discard first or pick a
// different name; no silent overwrite.
func TestDraftNewRefusesDuplicateName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftNewTool(reg, fixtureResolver())

	_ = callDraftHandler(t, handler, map[string]any{keyName: draftFixtureName})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyName: draftFixtureName}

	_, err := handler(t.Context(), req)

	require.ErrorIs(t, err, builder.ErrDraftExists)
}

// TestDraftShowReturnsLiveDraftState reads the draft back. Mirrors
// the conversation flow where the model creates a draft, mutates it
// (Phase 8.4), then re-reads to confirm.
func TestDraftShowReturnsLiveDraftState(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	src := fixtureSourceProfile()
	_, err := reg.Create(draftFixtureName, &src)
	require.NoError(t, err)

	_, _, showHandler := tools.NewLinodeProfileDraftShowTool(reg)

	out := callDraftHandler(t, showHandler, map[string]any{keyName: draftFixtureName})

	assert.Equal(t, draftFixtureName, out[keyName])
	assert.Equal(t, src.Description, out["description"])
	assert.Equal(t, anySlice(src.AllowedTools), out["allowed_tools"])
}

// TestDraftShowRefusesUnknown covers the typo / expired-session path.
// The handler returns builder.ErrDraftNotFound so callers can match
// without parsing the message.
func TestDraftShowRefusesUnknown(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, showHandler := tools.NewLinodeProfileDraftShowTool(reg)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyName: draftNonexistent}

	_, err := showHandler(t.Context(), req)

	require.ErrorIs(t, err, builder.ErrDraftNotFound)
}

// TestDraftShowRefusesMissingName mirrors the _new validation guard.
func TestDraftShowRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, showHandler := tools.NewLinodeProfileDraftShowTool(reg)

	_, err := showHandler(t.Context(), mcp.CallToolRequest{})

	require.ErrorIs(t, err, tools.ErrDraftNameMissing)
}

// TestDraftDiscardRemovesDraft is the happy path. The discarded
// response carries the boolean and the name for human-readable logs;
// the registry no longer holds the draft afterward.
func TestDraftDiscardRemovesDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(draftFixtureName, nil)
	require.NoError(t, err)

	_, _, discardHandler := tools.NewLinodeProfileDraftDiscardTool(reg)

	out := callDraftHandler(t, discardHandler, map[string]any{keyName: draftFixtureName})

	assert.Equal(t, draftFixtureName, out[keyName])
	assert.Equal(t, true, out["discarded"])

	_, exists := reg.Get(draftFixtureName)
	assert.False(t, exists, "discard must remove the draft from the registry")
}

// TestDraftDiscardIdempotent covers the unknown-name path. Discard
// against a name that was never created returns discarded=false, not
// an error. Tool handlers should be safe to call on cleanup paths.
func TestDraftDiscardIdempotent(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, discardHandler := tools.NewLinodeProfileDraftDiscardTool(reg)

	out := callDraftHandler(t, discardHandler, map[string]any{keyName: draftNonexistent})

	assert.Equal(t, draftNonexistent, out[keyName])
	assert.Equal(t, false, out["discarded"])
}

// TestDraftDiscardRefusesMissingName mirrors _new and _show.
func TestDraftDiscardRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, discardHandler := tools.NewLinodeProfileDraftDiscardTool(reg)

	_, err := discardHandler(t.Context(), mcp.CallToolRequest{})

	require.ErrorIs(t, err, tools.ErrDraftNameMissing)
}

// TestDraftToolsRespectContextCancellation locks the cancellation
// contract across all three handlers. A canceled context surfaces
// ctx.Err and produces no result. Test exists to catch a refactor
// that drops the select gate.
func TestDraftToolsRespectContextCancellation(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	resolver := fixtureResolver()

	_, _, newHandler := tools.NewLinodeProfileDraftNewTool(reg, resolver)
	_, _, showHandler := tools.NewLinodeProfileDraftShowTool(reg)
	_, _, discardHandler := tools.NewLinodeProfileDraftDiscardTool(reg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, errNew := newHandler(ctx, mcp.CallToolRequest{})
	_, errShow := showHandler(ctx, mcp.CallToolRequest{})
	_, errDiscard := discardHandler(ctx, mcp.CallToolRequest{})

	require.ErrorIs(t, errNew, context.Canceled)
	require.ErrorIs(t, errShow, context.Canceled)
	require.ErrorIs(t, errDiscard, context.Canceled)
}

// anySlice converts a []string to the []any shape json.Unmarshal
// produces for arrays. Equality assertions in this file compare
// against the unmarshaled wire shape, so the conversion lives here
// rather than at every call site.
func anySlice(in []string) []any {
	out := make([]any, len(in))
	for i, item := range in {
		out[i] = item
	}

	return out
}
