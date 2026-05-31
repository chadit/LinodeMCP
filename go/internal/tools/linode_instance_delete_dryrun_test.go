package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			ID: 123, Label: "web-prod-01", Status: "running", Type: "g6-standard-2",
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))

	assert.Equal(t, "linode_instance_delete", body["tool"])

	would, _ := body["would_execute"].(map[string]any)
	assert.Equal(t, "DELETE", would["method"])
	assert.Equal(t, "/linode/instances/123", would["path"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 3, "expected volume, public_ip, and firewall dependencies")

	kinds := make([]string, 0, len(deps))

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		require.True(t, ok, "dependency entry should be an object")

		kind, ok := dep["kind"].(string)
		require.True(t, ok, "dependency should have a string kind")

		kinds = append(kinds, kind)
	}

	assert.ElementsMatch(t, []string{"volume", "public_ip", "firewall"}, kinds)

	billing, _ := body["billing_delta"].(map[string]any)
	assert.Equal(t, "-20.00", billing["monthly_change_usd"])

	warnings, _ := body["warnings"].([]any)
	assert.NotEmpty(t, warnings, "a running instance should produce a warning")

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_instance_rebuild", body["tool"])

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "the disk is erased and recreated")

	warnings, _ := body["warnings"].([]any)
	require.NotEmpty(t, warnings)

	warning, ok := warnings[len(warnings)-1].(string)
	require.True(t, ok)
	assert.Contains(t, warning, "linode/debian12")

	assert.NotContains(t, *methods, http.MethodPost, "dry_run must not issue a POST")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_instance_password_reset", body["tool"])

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "power-down and reboot is the side effect")

	warnings, _ := body["warnings"].([]any)
	assert.NotEmpty(t, warnings, "a running instance should produce a downtime warning")

	assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_instance_rescue", body["tool"])

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "rescue-mode reboot is the side effect")

	assert.NotEmpty(t, body["warnings"], "a running instance should produce a downtime warning")
	assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_instance_backup_restore", body["tool"])

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "the overwrite restore is the side effect")

	effect, ok := sideEffects[0].(string)
	require.True(t, ok)
	assert.Contains(t, effect, "999", "side effect should name the target instance")

	assert.NotEmpty(t, body["warnings"], "overwrite=true should produce a data-loss warning")
	assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
}
