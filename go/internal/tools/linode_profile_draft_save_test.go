package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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
	expectNoError(t, os.WriteFile(path, []byte(minimalConfigYAML), 0o600))

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

	checkEqual(t, "linode_profile_draft_save", tool.Name)
	expectNotEmpty(t, tool.Description)
	checkEqual(t, profiles.CapMeta, capability)
	expectNotNil(t, handler)
}

// TestSaveCreatesNewProfile is the happy path for a brand-new
// user-defined profile. The diff carries IsNew=true and the full
// AllowedTools as AddedTools.
func TestSaveCreatesNewProfile(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()
	draft, err := reg.Create(saveDraftName, nil)
	expectNoError(t, err)

	draft.Description = "saved via test"
	draft.AllowedTools = []string{toolHello, toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	expectEqual(t, saveDraftName, out[keyName])
	checkEqual(t, true, out["is_new"])
	added, _ := out["added_tools"].([]any)
	{
		expectedElements := []any{toolHello, toolInstanceBoot}
		actualElements := added
		expectLen(t, actualElements, len(expectedElements))

		for _, expectedElement := range expectedElements {
			expectContains(t, actualElements, expectedElement)
		}
	}

	checkEmpty(t, out["removed_tools"])

	// Disk side-effect: reload config and confirm the new profile
	// landed with the right contents.
	reloaded, err := config.Load(path)
	expectNoError(t, err)

	stored, ok := reloaded.Profiles[saveDraftName]
	expectTrue(t, ok, "saved profile must appear in cfg.Profiles after reload")
	checkEqual(t, "saved via test", stored.Description)
	{
		expectedElements := []string{toolHello, toolInstanceBoot}
		actualElements := stored.AllowedTools
		expectLen(t, actualElements, len(expectedElements))

		for _, expectedElement := range expectedElements {
			expectContains(t, actualElements, expectedElement)
		}
	}
}

// TestSaveUpdatesExistingProfile is the round-trip update case. The
// existing profile gets a new tool added and one removed; the diff
// reports both deltas and the prior state in ChangedFields.
func TestSaveUpdatesExistingProfile(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	// Stage an existing user-defined profile.
	priorCfg, err := config.Load(path)
	expectNoError(t, err)

	priorCfg.Profiles = map[string]config.UserProfileConfig{
		saveDraftName: {
			Description:  "prior",
			AllowedTools: []string{toolHello},
		},
	}
	expectNoError(t, config.WriteAtomic(path, priorCfg))

	reg := builder.NewRegistry()
	draft, err := reg.Create(saveDraftName, nil)
	expectNoError(t, err)

	draft.Description = "updated"
	draft.AllowedTools = []string{toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	checkEqual(t, false, out["is_new"])
	added, _ := out["added_tools"].([]any)
	checkEqual(t, []any{toolInstanceBoot}, added)

	removed, _ := out["removed_tools"].([]any)
	checkEqual(t, []any{toolHello}, removed)

	changes, _ := out["changed_fields"].(map[string]any)
	expectContains(t, changes, "description")
	descChange, _ := changes["description"].(map[string]any)
	checkEqual(t, "prior", descChange["old"])
	checkEqual(t, "updated", descChange["new"])
}

// TestSaveRefusesMissingConfirm guards the destructive operation
// contract. Without confirm=true the handler returns
// ErrConfirmRequired and writes nothing.
func TestSaveRefusesMissingConfirm(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)
	originalBytes, err := os.ReadFile(path)
	expectNoError(t, err)

	reg := builder.NewRegistry()
	_, err = reg.Create(saveDraftName, nil)
	expectNoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyName: saveDraftName}

	_, err = handler(t.Context(), req)
	if !errors.Is(err, tools.ErrConfirmRequired) {
		t.Fatalf("expected error %v, got %v", tools.ErrConfirmRequired, err)
	}

	// File untouched.
	finalBytes, err := os.ReadFile(path)
	expectNoError(t, err)
	checkEqual(t, originalBytes, finalBytes, "refused save must not write to disk")
}

// TestSaveRefusesBuiltinName covers the built-in-shadow guard. The
// user cannot save a draft over a built-in profile name.
func TestSaveRefusesBuiltinName(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()
	_, err := reg.Create(profiles.BuiltinComputeAdmin, nil)
	expectNoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    profiles.BuiltinComputeAdmin,
		keyConfirm: true,
	}

	_, err = handler(t.Context(), req)
	if !errors.Is(err, tools.ErrSaveBuiltinName) {
		t.Fatalf("expected error %v, got %v", tools.ErrSaveBuiltinName, err)
	}
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
	if !errors.Is(err, builder.ErrDraftNotFound) {
		t.Fatalf("expected error %v, got %v", builder.ErrDraftNotFound, err)
	}
}

// TestSaveRefusesMissingName covers the validation guard.
func TestSaveRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath("/dev/null"))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyConfirm: true}

	_, err := handler(t.Context(), req)
	if !errors.Is(err, tools.ErrDraftNameMissing) {
		t.Fatalf("expected error %v, got %v", tools.ErrDraftNameMissing, err)
	}
}

// TestSaveRefusesEmptyConfigPath surfaces ErrConfigPathUnknown when
// the provider returns "". This is the safety net for servers
// started without a known config path on disk.
func TestSaveRefusesEmptyConfigPath(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create(saveDraftName, nil)
	expectNoError(t, err)

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(""))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}

	_, err = handler(t.Context(), req)
	if !errors.Is(err, tools.ErrConfigPathUnknown) {
		t.Fatalf("expected error %v, got %v", tools.ErrConfigPathUnknown, err)
	}
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
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, err)
	}
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
	expectNoError(t, err)

	draft.AllowedTools = []string{toolHello}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}

	result, err := handler(t.Context(), req)
	expectNoError(t, err)

	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok)

	var payload map[string]any
	expectNoError(t, json.Unmarshal([]byte(textContent.Text), &payload))

	expectContainsWithMode(t, false, payload, "name")
	expectContainsWithMode(t, false, payload, "is_new")
	expectContainsWithMode(t, false, payload, "added_tools")
	expectContainsWithMode(t, false, payload, "removed_tools")
	expectContainsWithMode(t, false, payload, "changed_fields")
}
