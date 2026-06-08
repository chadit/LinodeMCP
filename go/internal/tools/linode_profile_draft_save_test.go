package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	if err := os.WriteFile(path, []byte(minimalConfigYAML), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return path
}

// staticConfigPath wraps a string as a ConfigPathProvider. Tests use
// this rather than the production config.Path so the home
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

	if tool.Name != "linode_profile_draft_save" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_draft_save")
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

// TestSaveCreatesNewProfile is the happy path for a brand-new
// user-defined profile. The diff carries IsNew=true and the full
// AllowedTools as AddedTools.
func TestSaveCreatesNewProfile(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.Description = "saved via test"
	draft.AllowedTools = []string{toolHello, toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	if !reflect.DeepEqual(out[keyName], saveDraftName) {
		t.Errorf("out[keyName] = %v, want %v", out[keyName], saveDraftName)
	}

	if !reflect.DeepEqual(out["is_new"], true) {
		t.Errorf("got %v, want %v", out["is_new"], true)
	}

	added, _ := out["added_tools"].([]any)

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

	if v, ok := out["removed_tools"].([]any); ok && len(v) != 0 {
		t.Errorf("value = %v, want empty", out["removed_tools"])
	}

	// Disk side-effect: reload config and confirm the new profile
	// landed with the right contents.
	reloaded, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stored, ok := reloaded.Profiles[saveDraftName]
	if !ok {
		t.Error("ok = false, want true")
	}

	if stored.Description != "saved via test" {
		t.Errorf("stored.Description = %v, want %v", stored.Description, "saved via test")
	}

	gotElems2 := slices.Clone(stored.AllowedTools)
	wantElems2 := slices.Clone([]string{toolHello, toolInstanceBoot})

	slices.Sort(gotElems2)
	slices.Sort(wantElems2)

	if !slices.Equal(gotElems2, wantElems2) {
		t.Errorf("elements = %v, want %v (any order)", gotElems2, []string{toolHello, toolInstanceBoot})
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	priorCfg.Profiles = map[string]config.UserProfileConfig{
		saveDraftName: {
			Description:  "prior",
			AllowedTools: []string{toolHello},
		},
	}
	if err := config.WriteAtomic(path, priorCfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.Description = "updated"
	draft.AllowedTools = []string{toolInstanceBoot}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	out := callMutateHandler(t, handler, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

	if !reflect.DeepEqual(out["is_new"], false) {
		t.Errorf("got %v, want %v", out["is_new"], false)
	}

	added, _ := out["added_tools"].([]any)
	if !reflect.DeepEqual(added, []any{toolInstanceBoot}) {
		t.Errorf("added = %v, want %v", added, []any{toolInstanceBoot})
	}

	removed, _ := out["removed_tools"].([]any)
	if !reflect.DeepEqual(removed, []any{toolHello}) {
		t.Errorf("removed = %v, want %v", removed, []any{toolHello})
	}

	changes, _ := out["changed_fields"].(map[string]any)
	if _, ok := changes[keyDescription]; !ok {
		t.Errorf("changes missing key %v", keyDescription)
	}

	descChange, _ := changes[keyDescription].(map[string]any)
	if !reflect.DeepEqual(descChange["old"], "prior") {
		t.Errorf("got %v, want %v", descChange["old"], "prior")
	}

	if !reflect.DeepEqual(descChange["new"], "updated") {
		t.Errorf("got %v, want %v", descChange["new"], "updated")
	}
}

// TestSaveRefusesMissingConfirm guards the destructive operation
// contract. Without confirm=true the handler returns
// ErrConfirmRequired and writes nothing.
func TestSaveRefusesMissingConfirm(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	originalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	reg := builder.NewRegistry()

	_, err = reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{keyName: saveDraftName}

	_, err = handler(t.Context(), req)
	if !errors.Is(err, tools.ErrConfirmRequired) {
		t.Fatalf("expected error %v, got %v", tools.ErrConfirmRequired, err)
	}

	// File untouched.
	finalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(finalBytes, originalBytes) {
		t.Errorf("finalBytes = %v, want %v", finalBytes, originalBytes)
	}
}

// TestSaveRefusesBuiltinName covers the built-in-shadow guard. The
// user cannot save a draft over a built-in profile name.
func TestSaveRefusesBuiltinName(t *testing.T) {
	t.Parallel()

	path := writableSaveConfig(t)

	reg := builder.NewRegistry()

	_, err := reg.Create(profiles.BuiltinComputeAdmin, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolHello}

	_, _, handler := tools.NewLinodeProfileDraftSaveTool(reg, staticConfigPath(path))

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &payload); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, ok := payload["name"]; !ok {
		t.Errorf("payload missing key %v", "name")
	}

	if _, ok := payload["is_new"]; !ok {
		t.Errorf("payload missing key %v", "is_new")
	}

	if _, ok := payload["added_tools"]; !ok {
		t.Errorf("payload missing key %v", "added_tools")
	}

	if _, ok := payload["removed_tools"]; !ok {
		t.Errorf("payload missing key %v", "removed_tools")
	}

	if _, ok := payload["changed_fields"]; !ok {
		t.Errorf("payload missing key %v", "changed_fields")
	}
}
