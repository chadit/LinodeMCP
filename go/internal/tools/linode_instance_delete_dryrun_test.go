package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestLinodeInstanceDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// dependency walk: the preview must surface attached volumes (detached),
// public IPs (released), and firewall attachments (removed), estimate the
// monthly billing change from the instance type, warn when the instance is
// running, and never issue a DELETE.
func TestLinodeInstanceDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/linode/instances/123": linode.Instance{
			ID: 123, Label: "web-prod-01", Status: statusRunning, Type: "g6-standard-2",
		},
		"/linode/instances/123/volumes": linode.PaginatedResponse[linode.Volume]{
			Data: []linode.Volume{{ID: 6789, Label: "data-vol", Size: 50}},
		},
		"/linode/instances/123/ips": linode.InstanceIPAddresses{
			IPv4: &linode.InstanceIPv4{Public: []linode.IPAddress{{Address: "198.51.100.10"}}},
		},
		"/linode/instances/123/firewalls": linode.PaginatedResponse[linode.Firewall]{
			Data: []linode.Firewall{{ID: 42, Label: "web-fw"}},
		},
		"/linode/types/g6-standard-2": linode.InstanceType{
			ID: "g6-standard-2", Price: linode.Price{Monthly: 20.0},
		},
	})

	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyDryRun:     true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], canRunDestroyTool) {
		t.Errorf("got %v, want %v", body["tool"], canRunDestroyTool)
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], instanceGetPath) {
		t.Errorf("got %v, want %v", would["path"], instanceGetPath)
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 3 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 3)
	}

	kinds := make([]string, 0, len(deps))

	for _, entry := range deps {
		dep, depOK := entry.(map[string]any)
		if !depOK {
			t.Fatal("ok = false, want true")
		}

		kind, ok := dep[tcKind].(string)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		kinds = append(kinds, kind)
	}

	{
		gotEls := slices.Clone(kinds)
		wantEls := slices.Clone([]string{"volume", "public_ip", "firewall"})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", kinds, []string{"volume", "public_ip", "firewall"})
		}
	}

	billing, _ := body["billing_delta"].(map[string]any)
	if !reflect.DeepEqual(billing["monthly_change_usd"], "-20.00") {
		t.Errorf("got %v, want %v", billing["monthly_change_usd"], "-20.00")
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}

// TestLinodeInstanceRebuildToolDryRunSideEffects exercises the Phase 2 Tier A
// side-effects walk: a rebuild erases each disk (reported as a side effect) and
// replaces the current image (named in a warning); it never issues a POST.
func TestLinodeInstanceRebuildToolDryRunSideEffects(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/linode/instances/123": linode.Instance{
			ID: 123, Label: "rebuild-target", Status: statusRunning, Image: "linode/debian12",
		},
		"/linode/instances/123/disks": linode.PaginatedResponse[linode.InstanceDisk]{
			Data: []linode.InstanceDisk{
				{ID: 1, Label: "boot", Size: 25600, Filesystem: "ext4"},
			},
		},
	})

	_, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyImage:    imageIDUbuntu2404,
		keyRootPass: rootPassStrong,
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_rebuild") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_rebuild")
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("warnings is empty")
	}

	warning, ok := warnings[len(warnings)-1].(string)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(warning, "linode/debian12") {
		t.Errorf("warning does not contain %v", "linode/debian12")
	}

	if slices.Contains(*methods, http.MethodPost) {
		t.Errorf("*methods should not contain %v", http.MethodPost)
	}
}

// TestLinodeInstancePasswordResetToolDryRunSideEffects exercises the Phase 2
// Tier A side-effects walk: the reset powers the instance down and reboots it,
// reported as a side effect, with a downtime warning when it is running. The
// walk reads only the FetchState result, so the preview issues a single GET.
func TestLinodeInstancePasswordResetToolDryRunSideEffects(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/linode/instances/123", linode.Instance{
		ID: 123, Label: "pw-reset-host", Status: statusRunning,
	})

	_, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyRootPass: rootPassStrong,
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_password_reset") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_password_reset")
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}

// TestLinodeInstanceRescueToolDryRunSideEffects exercises the Phase 2 Tier A
// side-effects walk wired through RunDryRunPreviewDetailed: rescue reboots the
// instance into a recovery environment (a side effect) with a downtime warning
// when it is running. The walk reads only the FetchState result (single GET).
func TestLinodeInstanceRescueToolDryRunSideEffects(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/linode/instances/123", linode.Instance{
		ID: 123, Label: "rescue-host", Status: statusRunning,
	})

	_, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_rescue") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_rescue")
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}

// TestLinodeInstanceBackupRestoreToolDryRunSideEffects exercises the Phase 2
// Tier A side-effects walk: with overwrite=true the target instance's disks and
// configs are destroyed and replaced, reported as a side effect plus a
// data-loss warning. The walk reads only the FetchState result (single GET).
func TestLinodeInstanceBackupRestoreToolDryRunSideEffects(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, "/linode/instances/123/backups/456", linode.InstanceBackup{ID: 456})

	_, _, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID:       float64(123),
		keyBackupID:       float64(456),
		keyTargetLinodeID: float64(999),
		"overwrite":       true,
		keyDryRun:         true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_backup_restore") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_backup_restore")
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, ok := sideEffects[0].(string)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(effect, "999") {
		t.Errorf("effect does not contain %v", "999")
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}
