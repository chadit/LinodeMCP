package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const instanceDiskGetPath = "/linode/instances/123/disks/789"

func TestLinodeInstanceBackupCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupCreateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without snapshotting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

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

		if !reflect.DeepEqual(body["tool"], "linode_instance_backup_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_backup_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/backups") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/backups")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBackupCreateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeInstanceBackupRestoreToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupRestoreTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/backups/456/restore") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/backups/456/restore")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates target_linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBackupRestoreTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyBackupID: float64(456),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "target_linode_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "target_linode_id is required")
		}
	})
}

func TestLinodeInstanceBackupsEnableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupsEnableTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without enabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

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

		if !reflect.DeepEqual(body["tool"], "linode_instance_backups_enable") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_backups_enable")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/backups/enable") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/backups/enable")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceFirewallsApplyToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceFirewallsApplyTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without applying", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceFirewallsApplyTool(cfg)

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

		if !reflect.DeepEqual(body["tool"], "linode_instance_firewall_apply") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_firewall_apply")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/firewalls/apply") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/firewalls/apply")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceDiskCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceDiskCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeInstanceDiskCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
	_, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyLabel:    "data-disk",
		keySize:     float64(10240),
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_disk_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], instanceGetPath+"/disks") {
		t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/disks")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "data-disk") {
		t.Errorf("effect does not contain %v", "data-disk")
	}

	if !strings.Contains(effect, "10240") {
		t.Errorf("effect does not contain %v", "10240")
	}
}

func TestLinodeInstanceDiskCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceDiskCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keySize:     float64(10240),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

func TestLinodeInstanceDiskUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_disk_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], instanceDiskGetPath) {
			t.Errorf("got %v, want %v", would["path"], instanceDiskGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates disk_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeInstanceDiskCloneToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskCloneTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_disk_clone") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_clone")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceDiskGetPath+"/clone") {
			t.Errorf("got %v, want %v", would["path"], instanceDiskGetPath+"/clone")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, "25600 MB") {
			t.Errorf("effect does not contain %v", "25600 MB")
		}
	})
}

func TestLinodeInstanceDiskResizeToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceDiskResizeTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeInstanceDiskResizeToolDryRunPreviewWithoutResizing(t *testing.T) {
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_disk_resize") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_resize")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], instanceDiskGetPath+"/resize") {
		t.Errorf("got %v, want %v", would["path"], instanceDiskGetPath+"/resize")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "10240 MB") {
		t.Errorf("effect does not contain %v", "10240 MB")
	}

	if !strings.Contains(effect, "20480 MB") {
		t.Errorf("effect does not contain %v", "20480 MB")
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}
}

func TestLinodeInstanceDiskResizeToolDryRunStillValidatesSize(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceDiskResizeTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyDiskID:   float64(789),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "size is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "size is required")
	}
}

func TestLinodeInstanceDiskPasswordResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskPasswordResetTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_disk_password_reset") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_password_reset")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceDiskGetPath+"/password") {
			t.Errorf("got %v, want %v", would["path"], instanceDiskGetPath+"/password")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskPasswordResetTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDiskID: float64(789),
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_disk_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 1)
	}

	dep, gotMap := deps[0].(map[string]any)
	if !gotMap {
		t.Fatal("gotMap = false, want true")
	}

	if !reflect.DeepEqual(dep[tcKind], "instance_config") {
		t.Errorf("got %v, want %v", dep[tcKind], "instance_config")
	}

	if dep[keySupportTicketID] != float64(5) {
		t.Errorf("value = %v, want %v", dep[keySupportTicketID], float64(5))
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
