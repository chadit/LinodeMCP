package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestLinodeInstanceBackupsListTool verifies the instance backups list tool
// registers correctly, validates linode_id, and returns backup data.
func TestLinodeInstanceBackupsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backups_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing linode id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "linode_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		backupsResp := linode.InstanceBackupsResponse{
			Automatic: []linode.InstanceBackup{
				{ID: 100, Label: "auto-2024-01-01", Status: "successful", Type: "auto"},
			},
			Snapshot: linode.InstanceBackupSnapshots{
				Current: &linode.InstanceBackup{ID: 200, Label: "my-snapshot", Status: "successful", Type: "snapshot"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(backupsResp), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "auto-2024-01-01", "response should contain automatic backup label")
		assert.Contains(t, textContent.Text, "my-snapshot", "response should contain snapshot label")
	})
}

// TestLinodeInstanceBackupGetTool verifies the instance backup get tool
// registers correctly, validates required fields, and retrieves backup details.
func TestLinodeInstanceBackupGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backup_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing linode id", args: map[string]any{"backup_id": "100"}, wantContains: "linode_id is required"},
		{name: "missing backup id", args: map[string]any{"linode_id": "123"}, wantContains: "backup_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		backup := linode.InstanceBackup{ID: 100, Label: "my-backup", Status: "successful", Type: "snapshot"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups/100", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(backup), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": "123", "backup_id": "100"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-backup", "response should contain backup label")
		assert.Contains(t, textContent.Text, "successful", "response should contain backup status")
	})
}

// TestLinodeInstanceBackupCreateTool verifies the instance backup creation tool
// registers correctly, validates required fields, and creates snapshots.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm or linode_id returns descriptive error
//  3. Success: Create snapshot through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_instance_backup_create" with required params
//   - Missing required fields return descriptive errors
//   - Successful creation returns snapshot details from API
//
// Purpose: End-to-end verification of instance backup creation workflow.
func TestLinodeInstanceBackupCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backup_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": "123"}, wantContains: "confirm=true"},
		{name: "missing linode id", args: map[string]any{"confirm": true}, wantContains: "linode_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		backup := linode.InstanceBackup{ID: 300, Label: "snapshot-manual", Status: "pending", Type: "snapshot"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(backup), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": "123", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Snapshot created", "response should confirm snapshot creation")
		assert.Contains(t, textContent.Text, "300", "response should contain backup ID")
	})
}

// TestLinodeInstanceBackupRestoreTool verifies the instance backup restore tool
// registers correctly, validates required fields, and restores backups.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, linode_id, or backup_id returns descriptive error
//  3. Success: Restore backup through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_instance_backup_restore" with required params
//   - Missing required fields return descriptive errors
//   - Successful restore returns confirmation message
//
// Purpose: End-to-end verification of instance backup restore workflow.
func TestLinodeInstanceBackupRestoreTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backup_restore", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "backup_id", "schema should include backup_id")
		assert.Contains(t, props, "target_linode_id", "schema should include target_linode_id")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": "123", "backup_id": "100", "target_linode_id": float64(456)}, wantContains: "confirm=true"},
		{name: "missing linode id", args: map[string]any{"backup_id": "100", "target_linode_id": float64(456), "confirm": true}, wantContains: "linode_id is required"},
		{name: "missing backup id", args: map[string]any{"linode_id": "123", "target_linode_id": float64(456), "confirm": true}, wantContains: "backup_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful restore", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups/100/restore", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupRestoreTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": "123", "backup_id": "100", "target_linode_id": float64(456), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "restore initiated", "response should confirm restore")
	})
}

// TestLinodeInstanceBackupsEnableTool verifies the instance backups enable tool
// registers correctly, validates required fields, and enables the backup service.
func TestLinodeInstanceBackupsEnableTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backups_enable", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": "123"}, wantContains: "confirm=true"},
		{name: "missing linode id", args: map[string]any{"confirm": true}, wantContains: "linode_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful enable", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups/enable", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupsEnableTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": "123", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Backup service enabled", "response should confirm backup enable")
	})
}

// TestLinodeInstanceBackupsCancelTool verifies the instance backups cancel tool
// registers correctly, validates required fields, and cancels the backup service.
func TestLinodeInstanceBackupsCancelTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backups_cancel", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": "123"}, wantContains: "confirm=true"},
		{name: "missing linode id", args: map[string]any{"confirm": true}, wantContains: "linode_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful cancel", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups/cancel", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceBackupsCancelTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": "123", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Backup service canceled", "response should confirm backup cancel")
	})
}

// TestLinodeInstanceDisksListTool verifies the instance disks list tool
// registers correctly, validates linode_id, and returns disk data.
func TestLinodeInstanceDisksListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDisksListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disks_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing linode id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "linode_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		disks := []linode.InstanceDisk{
			{ID: 10, Label: "Ubuntu 24.04 Disk", Size: 51200, Filesystem: "ext4", Status: "ready"},
			{ID: 11, Label: "512 MB Swap Image", Size: 512, Filesystem: "swap", Status: "ready"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": disks, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDisksListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Ubuntu 24.04 Disk", "response should contain disk label")
		assert.Contains(t, textContent.Text, "512 MB Swap Image", "response should contain swap disk label")
	})
}

// TestLinodeInstanceDiskGetTool verifies the instance disk get tool
// registers correctly, validates required fields, and retrieves disk details.
func TestLinodeInstanceDiskGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing linode id", args: map[string]any{"disk_id": float64(10)}, wantContains: "linode_id is required"},
		{name: "missing disk id", args: map[string]any{"linode_id": float64(123)}, wantContains: "disk_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		disk := linode.InstanceDisk{ID: 10, Label: "Ubuntu 24.04 Disk", Size: 51200, Filesystem: "ext4", Status: "ready"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Ubuntu 24.04 Disk", "response should contain disk label")
		assert.Contains(t, textContent.Text, "ext4", "response should contain filesystem type")
	})
}

// TestLinodeInstanceDiskCreateTool verifies the instance disk creation tool
// registers correctly, validates required fields, and creates disks.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, linode_id, or label returns descriptive error
//  3. Success: Create disk through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_instance_disk_create" with required params
//   - Missing required fields return descriptive errors
//   - Successful creation returns disk details from API
//
// Purpose: End-to-end verification of instance disk creation workflow.
func TestLinodeInstanceDiskCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "size", "schema should include size")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": float64(123), "label": "my-disk", "size": float64(1024)}, wantContains: "confirm=true"},
		{name: "missing linode id", args: map[string]any{"label": "my-disk", "size": float64(1024), "confirm": true}, wantContains: "linode_id is required"},
		{name: "missing label", args: map[string]any{"linode_id": float64(123), "size": float64(1024), "confirm": true}, wantContains: "label is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		disk := linode.InstanceDisk{ID: 50, Label: "my-disk", Size: 1024, Filesystem: "ext4", Status: "ready"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "label": "my-disk", "size": float64(1024), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-disk", "response should contain disk label")
		assert.Contains(t, textContent.Text, "50", "response should contain disk ID")
	})
}

// TestLinodeInstanceDiskUpdateTool verifies the instance disk update tool
// registers correctly, validates confirm, and updates disk labels.
func TestLinodeInstanceDiskUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "disk_id", "schema should include disk_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10), "label": "new-label"})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		disk := linode.InstanceDisk{ID: 10, Label: "renamed-disk", Size: 51200, Filesystem: "ext4", Status: "ready"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "disk_id": float64(10), "label": "renamed-disk", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeInstanceDiskDeleteTool verifies the instance disk delete tool
// registers correctly, validates confirm, and deletes disks.
func TestLinodeInstanceDiskDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted", "response should confirm deletion")
	})
}

// TestLinodeInstanceDiskCloneTool verifies the instance disk clone tool
// registers correctly, validates confirm, and clones disks.
func TestLinodeInstanceDiskCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskCloneTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_clone", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "disk_id", "schema should include disk_id")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		clonedDisk := linode.InstanceDisk{ID: 99, Label: "Ubuntu 24.04 Disk", Size: 51200, Filesystem: "ext4", Status: "ready"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10/clone", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(clonedDisk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskCloneTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "disk_id": float64(10), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "cloned", "response should confirm clone")
		assert.Contains(t, textContent.Text, "99", "response should contain cloned disk ID")
	})
}

// TestLinodeInstanceDiskResizeTool verifies the instance disk resize tool
// registers correctly, validates required fields, and resizes disks.
func TestLinodeInstanceDiskResizeTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceDiskResizeTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_resize", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "disk_id", "schema should include disk_id")
		assert.Contains(t, props, "size", "schema should include size")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": float64(123), "disk_id": float64(10), "size": float64(65536)}, wantContains: "confirm=true"},
		{name: "missing size", args: map[string]any{"linode_id": float64(123), "disk_id": float64(10), "confirm": true}, wantContains: "size is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful resize", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10/resize", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceDiskResizeTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "disk_id": float64(10), "size": float64(65536), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "resize initiated", "response should confirm resize")
		assert.Contains(t, textContent.Text, "65536", "response should contain new size")
	})
}

// TestLinodeInstanceIPsListTool verifies the instance IPs list tool
// registers correctly, validates linode_id, and returns IP address data.
func TestLinodeInstanceIPsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceIPsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ips_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing linode id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "linode_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ips := linode.InstanceIPAddresses{
			IPv4: &linode.InstanceIPv4{
				Public: []linode.IPAddress{
					{Address: "203.0.113.1", Public: true, Type: "ipv4", Region: "us-east"},
				},
				Private: []linode.IPAddress{
					{Address: "192.168.1.1", Public: false, Type: "ipv4", Region: "us-east"},
				},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ips), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceIPsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "203.0.113.1", "response should contain public IP")
		assert.Contains(t, textContent.Text, "192.168.1.1", "response should contain private IP")
	})
}

// TestLinodeInstanceIPGetTool verifies the instance IP get tool
// registers correctly, validates required fields, and retrieves IP details.
func TestLinodeInstanceIPGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceIPGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ip_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing linode id", args: map[string]any{"address": "203.0.113.1"}, wantContains: "linode_id is required"},
		{name: "missing address", args: map[string]any{"linode_id": float64(123)}, wantContains: "address is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ipAddr := linode.IPAddress{
			Address: "203.0.113.1", Gateway: "203.0.113.0", SubnetMask: "255.255.255.0",
			Prefix: 24, Type: "ipv4", Public: true, Region: "us-east", LinodeID: 123,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ipAddr), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceIPGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "address": "203.0.113.1"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "203.0.113.1", "response should contain IP address")
		assert.Contains(t, textContent.Text, "us-east", "response should contain region")
	})
}

// TestLinodeInstanceIPAllocateTool verifies the instance IP allocate tool
// registers correctly, validates confirm, and allocates new IP addresses.
func TestLinodeInstanceIPAllocateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceIPAllocateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ip_allocate", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "type", "schema should include type")
		assert.Contains(t, props, "public", "schema should include public")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "type": "ipv4", "public": true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful allocation", func(t *testing.T) {
		t.Parallel()

		ipAddr := linode.IPAddress{
			Address: "198.51.100.5", Gateway: "198.51.100.0", SubnetMask: "255.255.255.0",
			Prefix: 24, Type: "ipv4", Public: true, Region: "us-east", LinodeID: 123,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ipAddr), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceIPAllocateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "type": "ipv4", "public": true, "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "198.51.100.5", "response should contain allocated IP")
		assert.Contains(t, textContent.Text, "allocated", "response should confirm allocation")
	})
}

// TestLinodeInstanceIPDeleteTool verifies the instance IP delete tool
// registers correctly, validates required fields, and removes IP addresses.
func TestLinodeInstanceIPDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ip_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": float64(123), "address": "203.0.113.1"}, wantContains: "confirm=true"},
		{name: "missing address", args: map[string]any{"linode_id": float64(123), "confirm": true}, wantContains: "address is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceIPDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "address": "203.0.113.1", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed", "response should confirm removal")
		assert.Contains(t, textContent.Text, "203.0.113.1", "response should contain removed IP")
	})
}

// TestLinodeInstanceCloneTool verifies the instance clone tool
// registers correctly, validates confirm, and clones instances.
func TestLinodeInstanceCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceCloneTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_clone", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		instance := linode.Instance{ID: 999, Label: "my-linode-clone", Region: "us-east", Status: "provisioning"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/clone", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceCloneTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "label": "my-linode-clone", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "cloned", "response should confirm clone")
		assert.Contains(t, textContent.Text, "999", "response should contain new instance ID")
	})
}

// TestLinodeInstanceMigrateTool verifies the instance migrate tool
// registers correctly, validates confirm, and initiates instance migration.
func TestLinodeInstanceMigrateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceMigrateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_migrate", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "region", "schema should include region")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "region": "eu-west"})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful migration", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/migrate", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceMigrateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "region": "eu-west", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Migration initiated", "response should confirm migration")
		assert.Contains(t, textContent.Text, "eu-west", "response should contain target region")
	})
}

// TestLinodeInstanceRebuildTool verifies the instance rebuild tool
// registers correctly, validates required fields, and rebuilds instances.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, image, or root_pass returns descriptive error
//  3. Success: Rebuild instance through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_instance_rebuild" with required params
//   - Missing required fields return descriptive errors
//   - Successful rebuild returns instance details from API
//
// Purpose: End-to-end verification of instance rebuild workflow.
func TestLinodeInstanceRebuildTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceRebuildTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_rebuild", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "image", "schema should include image")
		assert.Contains(t, props, "root_pass", "schema should include root_pass")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": float64(123), "image": "linode/ubuntu24.04", "root_pass": "Str0ngP@ssw0rd!"}, wantContains: "confirm=true"},
		{name: "missing image", args: map[string]any{"linode_id": float64(123), "root_pass": "Str0ngP@ssw0rd!", "confirm": true}, wantContains: "image is required"},
		{name: "missing root pass", args: map[string]any{"linode_id": float64(123), "image": "linode/ubuntu24.04", "confirm": true}, wantContains: "root_pass is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful rebuild", func(t *testing.T) {
		t.Parallel()

		instance := linode.Instance{
			ID: 123, Label: "my-linode", Region: "us-east", Image: "linode/ubuntu24.04", Status: "rebuilding",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/rebuild", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceRebuildTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "image": "linode/ubuntu24.04", "root_pass": "Str0ngP@ssw0rd!", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "rebuilt", "response should confirm rebuild")
		assert.Contains(t, textContent.Text, "linode/ubuntu24.04", "response should contain image name")
	})
}

// TestLinodeInstanceRescueTool verifies the instance rescue tool
// registers correctly, validates confirm, and boots instances into rescue mode.
func TestLinodeInstanceRescueTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstanceRescueTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_rescue", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "devices", "schema should include devices")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	t.Run("missing confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
	})

	t.Run("successful rescue", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/rescue", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstanceRescueTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "rescue mode", "response should confirm rescue mode")
	})
}

// TestLinodeInstancePasswordResetTool verifies the instance password reset tool
// registers correctly, validates required fields, and resets root passwords.
func TestLinodeInstancePasswordResetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_password_reset", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "root_pass", "schema should include root_pass")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"linode_id": float64(123), "root_pass": "NewStr0ngP@ss!"}, wantContains: "confirm=true"},
		{name: "missing root pass", args: map[string]any{"linode_id": float64(123), "confirm": true}, wantContains: "root_pass is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful password reset", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/password", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeInstancePasswordResetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"linode_id": float64(123), "root_pass": "NewStr0ngP@ss!", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "password reset", "response should confirm password reset")
	})
}
