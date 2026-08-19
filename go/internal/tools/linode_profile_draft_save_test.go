package tools_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

// configPathEnv is what config.Path reads before falling back to the home
// directory. The save handler reads the live path on every call, so setting it
// is how a case aims the save somewhere writable. Setting an environment
// variable forbids t.Parallel, which is why these cases do not run in parallel.
const configPathEnv = "LINODEMCP_CONFIG_PATH"

// TestSaveRegistration locks in the static contract: tool name,
// description, CapMeta tag.
func TestSaveRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeProfileDraftSaveTool(&config.Config{})

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
	path := writableSaveConfig(t)

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.Description = "saved via test"
	draft.AllowedTools = []string{toolHello, toolInstanceBoot}

	t.Setenv(configPathEnv, path)

	out := callSaveAnswer(t, draftState(reg), map[string]any{
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
	if writeErr := config.WriteAtomic(path, priorCfg); writeErr != nil {
		t.Errorf("unexpected error: %v", writeErr)
	}

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// All three value shapes the diff can carry: a string, a string list and
	// a bool. Each renders through its own arm of the conversion, so changing
	// only the description would leave two of them unexercised.
	draft.Description = "updated"
	draft.AllowedTools = []string{toolInstanceBoot}
	draft.AllowedEnvironments = []string{envProd}
	draft.AllowYolo = true

	t.Setenv(configPathEnv, path)

	out := callSaveAnswer(t, draftState(reg), map[string]any{
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

	// The list arm: an empty prior list and the one the draft carries.
	envChange, _ := changes[tcAllowedEnvironments].(map[string]any)
	if !reflect.DeepEqual(envChange["old"], []any{}) {
		t.Errorf("envChange[old] = %v, want []", envChange["old"])
	}

	if !reflect.DeepEqual(envChange["new"], []any{envProd}) {
		t.Errorf("envChange[new] = %v, want %v", envChange["new"], []any{envProd})
	}

	// The bool arm.
	yoloChange, _ := changes[keyAllowYolo].(map[string]any)
	if !reflect.DeepEqual(yoloChange["old"], false) {
		t.Errorf("yoloChange[old] = %v, want false", yoloChange["old"])
	}

	if !reflect.DeepEqual(yoloChange["new"], true) {
		t.Errorf("yoloChange[new] = %v, want true", yoloChange["new"])
	}
}

// TestSaveRefusesMissingConfirm guards the destructive operation
// contract. Without confirm=true the handler returns
// ErrConfirmRequired and writes nothing.
func TestSaveRefusesMissingConfirm(t *testing.T) {
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

	t.Setenv(configPathEnv, path)

	// The confirm gate is the generated handler's now, so it is driven
	// through the emitted factory rather than through the answer.
	_, _, handler := gentools.NewLinodeProfileDraftSaveTool(&config.Config{})

	wantRefusal(t, draftState(reg), handler, map[string]any{keyName: saveDraftName},
		"confirm=true is required for draft save")

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
	path := writableSaveConfig(t)

	reg := builder.NewRegistry()

	_, err := reg.Create(profiles.BuiltinComputeAdmin, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	t.Setenv(configPathEnv, path)

	wantAnswerRefusal(t, draftState(reg), tools.ProfileDraftSaveAnswer, nil, map[string]any{
		keyName:    profiles.BuiltinComputeAdmin,
		keyConfirm: true,
	}, "cannot save over built-in profile name: compute-admin")
}

// TestSaveRefusesUnknownDraft surfaces builder.ErrDraftNotFound when
// the draft isn't in the registry.
func TestSaveRefusesUnknownDraft(t *testing.T) {
	path := writableSaveConfig(t)
	reg := builder.NewRegistry()

	t.Setenv(configPathEnv, path)

	wantAnswerRefusal(t, draftState(reg), tools.ProfileDraftSaveAnswer, nil, map[string]any{
		keyName:    draftNonexistent,
		keyConfirm: true,
	}, "draft not found: nonexistent-draft")
}

// TestSaveRefusesMissingName covers the validation guard.
func TestSaveRefusesMissingName(t *testing.T) {
	t.Parallel()

	wantAnswerRefusal(t, draftState(builder.NewRegistry()), tools.ProfileDraftSaveAnswer, nil,
		map[string]any{keyConfirm: true}, wantDraftNameMissing)
}

// TestSaveRespectsContextCancellation locks the cancellation
// contract.
func TestSaveRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeProfileDraftSaveTool(&config.Config{})

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
	path := writableSaveConfig(t)

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolHello}

	t.Setenv(configPathEnv, path)

	payload := callSaveAnswer(t, draftState(reg), map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	})

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

// TestSaveReportsLoadFailure covers the branch where the config path names no
// readable file. The draft is real and confirmed by then, so a swallowed load
// error would report a save that never had a config to merge into.
func TestSaveReportsLoadFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.yml")
	t.Setenv(configPathEnv, missing)

	reg := builder.NewRegistry()

	if _, err := reg.Create(saveDraftName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	refusal := refusalText(t, callAnswer(t, draftState(reg), tools.ProfileDraftSaveAnswer, nil, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}))

	if !strings.HasPrefix(refusal, "failed to load config from ") {
		t.Errorf("refusal = %q, want the load failure reported", refusal)
	}

	if !strings.Contains(refusal, missing) {
		t.Errorf("refusal = %q, want the unreadable path named in it", refusal)
	}
}

// readOnlyConfigDir stages a minimal config in a directory the process cannot
// write to, and returns the config path. WriteAtomic lands its temp file beside
// the target, so a read-only directory is what a config on a read-only mount
// looks like from the handler's side.
func readOnlyConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if err := os.WriteFile(path, []byte(minimalConfigYAML), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Restore before TempDir's own cleanup, which needs the write bit back to
	// remove the directory. Cleanups run last-registered-first.
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	probe, probeErr := os.CreateTemp(dir, "probe.*")
	if probeErr == nil {
		if err := probe.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		t.Skip("this process writes through a read-only directory (running as root?)")
	}

	return path
}

// TestSaveReportsWriteFailure covers the branch where the config loads but
// cannot be written back. The draft has already been merged into the in-memory
// config by then, so a swallowed write error would report a saved profile that
// only exists in this process and vanishes on restart.
func TestSaveReportsWriteFailure(t *testing.T) {
	path := readOnlyConfigDir(t)

	reg := builder.NewRegistry()

	draft, err := reg.Create(saveDraftName, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolHello}

	t.Setenv(configPathEnv, path)

	refusal := refusalText(t, callAnswer(t, draftState(reg), tools.ProfileDraftSaveAnswer, nil, map[string]any{
		keyName:    saveDraftName,
		keyConfirm: true,
	}))

	if !strings.HasPrefix(refusal, "failed to write config to ") {
		t.Errorf("refusal = %q, want the write failure reported", refusal)
	}

	if !strings.Contains(refusal, fs.ErrPermission.Error()) {
		t.Errorf("refusal = %q, want the refused write named in it", refusal)
	}

	reloaded, loadErr := config.Load(path)
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}

	if _, saved := reloaded.Profiles[saveDraftName]; saved {
		t.Error("the profile reached disk despite the write failure")
	}
}
