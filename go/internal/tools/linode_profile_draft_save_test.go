package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	// saveDraftName is the conventional draft name reused across the
	// happy-path tests. Distinct from mutateDraftName so the goconst
	// linter sees two test files using their own constants.
	saveDraftName = "my-saved"
)

// minimalConfigYAML is the smallest valid config used as the
// starting point for save tests. The save handler reads from disk,
// merges, and writes back so the test temp file needs at least the
// server + environments scaffolding to round-trip cleanly.
const minimalConfigYAML = `server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`

// writableSaveConfig stages a minimal config file in a temp dir and
// returns its path. The save handler reads + writes through this
// path, so each test gets its own to avoid cross-test pollution.
func writableSaveConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(minimalConfigYAML), 0o600))

	return path
}

// staticConfigPath wraps a string as a ConfigPathProvider. Tests use
// this rather than the production config.GetConfigPath so the home
// dir + env-var lookup stays out of the picture.
func staticConfigPath(path string) tools.ConfigPathProvider {
	return func() string { return path }
}

// TestSaveRegistration locks in the static contract: tool name,
// description, CapMeta tag.
func TestSaveRegistration(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	tool, capability, handler := tools.NewLinodeProfileDraftSaveTool(
		reg,
		staticConfigPath("/dev/null"),
	)

	assert.Equal(t, "linode_profile_draft_save", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, profiles.CapMeta, capability)
	assert.NotNil(t, handler)
}

// TestSaveCreatesNewProfile is the happy path for a brand-new
// user-defined profile. The diff carries IsNew=true and the full
// AllowedTools as AddedTools.
func TestSaveCreatesNewProfile(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()
	draft, err := reg.Create(saveDraftName, nil)
	require.NoError(t, err)

	draft.Description = "saved via test"
	draft.AllowedTools = []string{toolHello, toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	require.Equal(t, saveDraftName, out[keyName])
	assert.Equal(t, true, out["is_new"])
	added, _ := out["added_tools"].([]any)
	assert.ElementsMatch(t, []any{toolHello, toolInstanceBoot}, added)
	assert.Empty(t, out["removed_tools"])

	// Disk side-effect: reload config and confirm the new profile
	// landed with the right contents.
	reloaded, err := config.Load(path)
	require.NoError(t, err)

	stored, ok := reloaded.Profiles[saveDraftName]
	require.True(t, ok, "saved profile must appear in cfg.Profiles after reload")
	assert.Equal(t, "saved via test", stored.Description)
	assert.ElementsMatch(t, []string{toolHello, toolInstanceBoot}, stored.AllowedTools)
}

// TestSaveUpdatesExistingProfile is the round-trip update case. The
// existing profile gets a new tool added and one removed; the diff
// reports both deltas and the prior state in ChangedFields.
func TestSaveUpdatesExistingProfile(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	// Stage an existing user-defined profile.
	priorCfg, err := config.Load(path)
	require.NoError(t, err)

	priorCfg.Profiles = map[string]config.UserProfileConfig{
		saveDraftName: {
			Description:  "prior",
			AllowedTools: []string{toolHello},
		},
	}
	require.NoError(t, config.WriteAtomic(path, priorCfg))

	reg := builder.NewRegistry()
	draft, err := reg.Create(saveDraftName, nil)
	require.NoError(t, err)

	draft.Description = "updated"
	draft.AllowedTools = []string{toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	assert.Equal(t, false, out["is_new"])
	added, _ := out["added_tools"].([]any)
	assert.Equal(t, []any{toolInstanceBoot}, added)

	removed, _ := out["removed_tools"].([]any)
	assert.Equal(t, []any{toolHello}, removed)

	changes, _ := out["changed_fields"].(map[string]any)
	require.Contains(t, changes, "description")
	descChange, _ := changes["description"].(map[string]any)
	assert.Equal(t, "prior", descChange["old"])
	assert.Equal(t, "updated", descChange["new"])
}

// TestSaveRefusesMissingConfirm guards the destructive operation
// contract. Without confirm=true the handler returns
// ErrConfirmRequired and writes nothing.
func TestSaveRefusesMissingConfirm(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)
	originalBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	reg := builder.NewRegistry()
	_, err = reg.Create(saveDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyName: saveDraftName}

	_, err = handler(t.Context(), req)
	require.ErrorIs(t, err, tools.ErrConfirmRequired)

	// File untouched.
	finalBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, originalBytes, finalBytes,
		"refused save must not write to disk")
}

// TestSaveRefusesBuiltinName covers the built-in-shadow guard. The
// user cannot save a draft over a built-in profile name.
func TestSaveRefusesBuiltinName(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()
	_, err := reg.Create(profiles.BuiltinComputeAdmin, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    profiles.BuiltinComputeAdmin,
		keyConfirm: true,
	}

	_, err = handler(t.Context(), req)
	require.ErrorIs(t, err, tools.ErrSaveBuiltinName)
}

// TestSaveRefusesUnknownDraft surfaces builder.ErrDraftNotFound when
// the draft isn't in the registry.
func TestSaveRefusesUnknownDraft(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)
	reg := builder.NewRegistry()

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    draftNonexistent,
		keyConfirm: true,
	}

	_, err := handler(t.Context(), req)
	require.ErrorIs(t, err, builder.ErrDraftNotFound)
}

// TestSaveRefusesMissingName covers the validation guard.
func TestSaveRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath("/dev/null"))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyConfirm: true}

	_, err := handler(t.Context(), req)
	require.ErrorIs(t, err, tools.ErrDraftNameMissing)
}

// TestSaveRefusesEmptyConfigPath surfaces ErrConfigPathUnknown when
// the provider returns "". This is the safety net for servers
// started without a known config path on disk.
func TestSaveRefusesEmptyConfigPath(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(saveDraftName, nil)
	require.NoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(""))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}

	_, err = handler(t.Context(), req)
	require.ErrorIs(t, err, tools.ErrConfigPathUnknown)
}

// TestSaveRespectsContextCancellation locks the cancellation
// contract.
func TestSaveRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath("/dev/null"))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := handler(ctx, mcp.CallToolRequest{})
	require.ErrorIs(t, err, context.Canceled)
}

// TestSaveResultIsValidJSON verifies the response decodes cleanly
// and carries the expected top-level fields. Lock the wire shape so
// Python and Go can compare against this fixture in cross-language
// parity tests later.
func TestSaveResultIsValidJSON(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()
	draft, err := reg.Create(saveDraftName, nil)
	require.NoError(t, err)

	draft.AllowedTools = []string{toolHello}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}

	result, err := handler(t.Context(), req)
	require.NoError(t, err)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &payload))

	assert.Contains(t, payload, "name")
	assert.Contains(t, payload, "is_new")
	assert.Contains(t, payload, "added_tools")
	assert.Contains(t, payload, "removed_tools")
	assert.Contains(t, payload, "changed_fields")
}
