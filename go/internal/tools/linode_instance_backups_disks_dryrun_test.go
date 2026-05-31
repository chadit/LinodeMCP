package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const instanceDiskGetPath = "/linode/instances/123/disks/789"

func TestLinodeInstanceBackupCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without snapshotting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_backup_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/backups", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBackupCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinodeInstanceBackupRestoreToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupRestoreTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without restoring", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath+"/backups/456", linode.InstanceBackup{ID: 456})
		_, _, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:       float64(123),
			keyBackupID:       float64(456),
			keyTargetLinodeID: float64(999),
			keyDryRun:         true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_backup_restore", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/backups/456/restore", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates target_linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBackupRestoreTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyBackupID: float64(456),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "target_linode_id is required")
	})
}

func TestLinodeInstanceBackupsEnableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupsEnableTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without enabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_backups_enable", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/backups/enable", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceFirewallsApplyToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceFirewallsApplyTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without applying", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceFirewallsApplyTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_firewalls_apply", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/firewalls/apply", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceDiskCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyLabel:    "data-disk",
			keySize:     float64(10240),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_disk_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/disks", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-disk side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "data-disk", "side effect should name the new disk")
		assert.Contains(t, effect, "10240", "side effect should state the disk size")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keySize:     float64(10240),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeInstanceDiskUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceDiskGetPath, linode.InstanceDisk{ID: 789})
		_, _, handler := tools.NewLinodeInstanceDiskUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(789),
			keyLabel:    testRenamedLabel,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_disk_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, instanceDiskGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates disk_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinodeInstanceDiskCloneToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskCloneTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without cloning", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceDiskGetPath,
			linode.InstanceDisk{ID: 789, Label: "boot-disk", Size: 25600})
		_, _, handler := tools.NewLinodeInstanceDiskCloneTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(789),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_disk_clone", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceDiskGetPath+"/clone", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "clone surfaces the new disk")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "25600 MB", "side effect names the cloned disk size")
	})
}

func TestLinodeInstanceDiskResizeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskResizeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without resizing", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceDiskGetPath,
			linode.InstanceDisk{ID: 789, Size: 10240})
		_, _, handler := tools.NewLinodeInstanceDiskResizeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(789),
			keySize:     float64(20480),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_disk_resize", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceDiskGetPath+"/resize", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "resize surfaces the size change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "10240 MB", "side effect names the current size")
		assert.Contains(t, effect, "20480 MB", "side effect names the target size")
		assert.NotEmpty(t, body["warnings"], "resize warns about power-off")
	})

	t.Run("still validates size", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskResizeTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(789),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "size is required")
	})
}

func TestLinodeInstanceDiskPasswordResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskPasswordResetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads disk metadata not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceDiskGetPath, linode.InstanceDisk{ID: 789})
		_, _, handler := tools.NewLinodeInstanceDiskPasswordResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(789),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_disk_password_reset", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceDiskGetPath+"/password", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskPasswordResetTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDiskID: float64(789),
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

// TestLinodeInstanceDiskDeleteToolDryRunDependencies exercises the Phase 2
// Tier A walk: config profiles that reference the disk are surfaced as removed
// dependencies (their device slot is cleared when the disk is deleted).
func TestLinodeInstanceDiskDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	targetDiskID := 10
	otherDiskID := 99

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/linode/instances/888/disks/10": linode.InstanceDisk{ID: 10, Label: "boot"},
		"/linode/instances/888/configs": linode.PaginatedResponse[linode.InstanceConfig]{
			Data: []linode.InstanceConfig{
				{ID: 5, Label: "uses-disk", Devices: map[string]*linode.ConfigDevice{
					"sda": {DiskID: &targetDiskID},
				}},
				{ID: 6, Label: "other-disk", Devices: map[string]*linode.ConfigDevice{
					"sda": {DiskID: &otherDiskID},
				}},
			},
		},
	})

	_, _, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(888),
		keyDiskID:   float64(10),
		keyDryRun:   true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_instance_disk_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 1, "only the config referencing this disk is surfaced")

	dep, gotMap := deps[0].(map[string]any)
	require.True(t, gotMap)
	assert.Equal(t, "instance_config", dep["kind"])
	assert.InDelta(t, 5, dep["id"], 0)

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}
