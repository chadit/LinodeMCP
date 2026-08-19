package server_test

import (
	"encoding/json"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// callBuilderTool dispatches one profile-builder tool through the real server
// and returns its parsed answer. Going through HandleMessage is what proves the
// builder state reaches a handler: nothing else attaches it.
func callBuilderTool(
	t *testing.T, srv *server.Server, tool string, args map[string]any,
) map[string]any {
	t.Helper()

	isError, text := callServerTool(t, srv, tool, args)
	if isError {
		t.Fatalf("%s refused: %s", tool, text)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal %s response: %v", tool, err)
	}

	return out
}

// TestBuilderStateFollowsProfileReload covers the freshness a fixture cannot
// see: the seam carries readers rather than snapshots, so a profile swapped by
// ReloadProfile reaches the next call to an already-registered tool.
func TestBuilderStateFollowsProfileReload(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := map[string]any{
		"calls": []any{map[string]any{"tool": toolInstanceCreate}},
	}

	before := callBuilderTool(t, srv, "linode_profile_can_run", args)
	if before["active_profile"] != profiles.BuiltinDefault {
		t.Fatalf("active_profile = %v, want %v", before["active_profile"], profiles.BuiltinDefault)
	}

	if allowed := firstVerdict(t, before); allowed {
		t.Errorf("allowed = true under %s, want false", profiles.BuiltinDefault)
	}

	if err := srv.ReloadProfile(fullAccessConfig()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := callBuilderTool(t, srv, "linode_profile_can_run", args)
	if after["active_profile"] != profiles.BuiltinFullAccess {
		t.Errorf("active_profile = %v, want %v", after["active_profile"], profiles.BuiltinFullAccess)
	}

	if allowed := firstVerdict(t, after); !allowed {
		t.Errorf("allowed = false under %s, want true", profiles.BuiltinFullAccess)
	}
}

// firstVerdict reads the allowed flag off the first pre-check result row.
func firstVerdict(t *testing.T, body map[string]any) bool {
	t.Helper()

	results, isList := body["results"].([]any)
	if !isList || len(results) == 0 {
		t.Fatalf("no results in %v", body)
	}

	row, isObject := results[0].(map[string]any)
	if !isObject {
		t.Fatalf("unexpected result shape in %v", body)
	}

	allowed, _ := row["allowed"].(bool)

	return allowed
}

// TestDraftSurvivesTwoDispatches covers the pointer stability the design turns
// on from the server's side: the state is built once in New, so a draft
// started by one call is still there for the next.
func TestDraftSurvivesTwoDispatches(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const draftName = "seam-survival"

	created := callBuilderTool(t, srv, "linode_profile_draft_new", map[string]any{callNameKey: draftName})
	if created["name"] != draftName {
		t.Fatalf("name = %v, want %v", created["name"], draftName)
	}

	shown := callBuilderTool(t, srv, "linode_profile_draft_show", map[string]any{callNameKey: draftName})
	if shown["name"] != draftName {
		t.Errorf("name = %v, want %v", shown["name"], draftName)
	}
}

// TestDraftNewClonesThroughTheServerCatalog covers the dependency the collapse
// replaced: clone_from resolves against the running config and the catalog the
// seam carries, with no resolver injected into the tool.
func TestDraftNewClonesThroughTheServerCatalog(t *testing.T) {
	t.Parallel()

	cfg := fullAccessConfig()
	cfg.Profiles = map[string]config.UserProfileConfig{
		"clone-source": {
			Description:  "seam clone source",
			AllowedTools: []string{toolInstancesList},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	created := callBuilderTool(t, srv, "linode_profile_draft_new", map[string]any{
		callNameKey:  "cloned-draft",
		"clone_from": "clone-source",
	})

	if created["description"] != "seam clone source" {
		t.Errorf("description = %v, want %v", created["description"], "seam clone source")
	}

	allowed, isList := created["allowed_tools"].([]any)
	if !isList || len(allowed) != 1 || allowed[0] != toolInstancesList {
		t.Errorf("allowed_tools = %v, want [%s]", created["allowed_tools"], toolInstancesList)
	}
}
