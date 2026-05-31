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
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestLinodeInstanceBackupsListTool verifies the instance backups list tool
// registers correctly, validates linode_id, and returns backup data.
func TestLinodeInstanceBackupsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_backup_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingLinodeID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errLinodeIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		backupsResp := linode.InstanceBackupsResponse{
			Automatic: []linode.InstanceBackup{
				{ID: 100, Label: "auto-2024-01-01", Status: statusSuccessful, Type: "auto"},
			},
			Snapshot: linode.InstanceBackupSnapshots{
				Current: &linode.InstanceBackup{ID: 200, Label: "my-snapshot", Status: statusSuccessful, Type: wordSnapshot},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123"})
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

// TestLinodeInstanceConfigsListTool verifies the instance configuration profile list tool
// registers correctly, validates inputs, and returns configuration profile data.
func TestLinodeInstanceConfigsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, toolLinodeInstanceConfigList, tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, "page", "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},

		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
		{name: "invalid page", args: map[string]any{keyLinodeID: float64(123), keyPage: float64(0)}, wantContains: errInstanceFirewallsPageMin},
		{name: caseInvalidPageSizeLow, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(10)}, wantContains: errPageSizeRange},
		{name: caseInvalidPageSizeHigh, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(501)}, wantContains: errPageSizeRange},
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

		configs := []linode.InstanceConfig{
			{ID: 77, Label: "boot-config", Kernel: configKernelLatest, RootDevice: "/dev/sda"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: configs, keyPage: 2, keyPages: 3, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "boot-config", "response should contain config label")
		assert.Contains(t, textContent.Text, configKernelLatest, "response should contain config kernel")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list configuration profiles for instance 123")
	})
}

// TestLinodeInstanceConfigDeleteTool verifies the instance configuration profile delete tool
// registers correctly, validates confirm, and deletes configuration profiles.
func TestLinodeInstanceConfigDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
		{name: caseNumericConfirmRejected, value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789)}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
		})
	}

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyConfirm: true}, want: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyConfigID: float64(789), keyConfirm: true}, want: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyConfirm: true}, want: errLinodeIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, want: tools.ErrConfigIDRequired.Error()},
		{name: caseSeparatorConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: "789/..", keyConfirm: true}, want: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true}, want: errConfigIDInteger},
		{name: "zero config id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0), keyConfirm: true}, want: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.want)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Empty(t, r.URL.RawQuery, "request should not include query params")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
		assertErrorContains(t, result, "deleted")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(`{"errors":[{"reason":"locked"}]}`))
			assert.NoError(t, err, "error response should write")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to remove configuration profile 789 from instance 123")
	})
}

// TestLinodeInstanceConfigInterfacesListTool verifies the configuration profile
// interfaces list tool registers correctly, validates inputs, and returns interface data.
func TestLinodeInstanceConfigInterfacesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfacesListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interfaces_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: "456"}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errConfigIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: "456"}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: "456"}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: "456"}, wantContains: errLinodeIDInteger},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue}, wantContains: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(-456)}, wantContains: errConfigIDMin},
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

		interfaces := []linode.ConfigInterfaceResponse{{ID: 101, Active: true, Purpose: keyPublic}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/configs/456/interfaces", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(interfaces), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, keyPublic, "response should contain interface purpose")
		assert.Contains(t, textContent.Text, "101", "response should contain interface ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list interfaces for config 456 on instance 123")
	})
}

// TestLinodeInstanceBackupGetTool verifies the instance backup get tool
// registers correctly, validates required fields, and retrieves backup details.
func TestLinodeInstanceBackupGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupGetTool(cfg)

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
		{name: caseMissingLinodeID, args: map[string]any{keyBackupID: "100"}, wantContains: errLinodeIDRequired},
		{name: "missing backup id", args: map[string]any{keyLinodeID: "123"}, wantContains: "backup_id is required"},
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

		backup := linode.InstanceBackup{ID: 100, Label: "my-backup", Status: statusSuccessful, Type: wordSnapshot}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups/100", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(backup), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyBackupID: "100"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-backup", "response should contain backup label")
		assert.Contains(t, textContent.Text, statusSuccessful, "response should contain backup status")
	})
}

// TestLinodeInstanceConfigGetTool verifies the instance config get tool
// registers correctly, validates required fields, and retrieves config details.
func TestLinodeInstanceConfigGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceConfigGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: "456"}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errConfigIDRequired},
		{name: "malformed linode id", args: map[string]any{keyLinodeID: "123/../?bad=1", keyConfigID: "456"}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-123), keyConfigID: "456"}, wantContains: errLinodeIDInteger},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: "456/789"}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue}, wantContains: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(-456)}, wantContains: errConfigIDMin},
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

		configProfile := map[string]any{
			keyBetaID: float64(456),
			keyLabel:  "boot-config",
			keyKernel: configKernelLatest,
			"devices": map[string]any{
				configDeviceSlotSDA: map[string]any{"disk_id": float64(10)},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(configProfile), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "boot-config", "response should contain config label")
		assert.Contains(t, textContent.Text, configKernelLatest, "response should contain config kernel")
	})
	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/456", r.URL.Path, "request path should match")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve config 456 for instance 123")
	})
}

// End-to-end verification of instance backup creation workflow.
func TestLinodeInstanceBackupCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: "123"}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
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

		backup := linode.InstanceBackup{ID: 300, Label: "snapshot-manual", Status: "pending", Type: wordSnapshot}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/backups", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(backup), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyConfirm: true})
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

// End-to-end verification of instance backup restore workflow.
func TestLinodeInstanceBackupRestoreTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: "123", keyBackupID: "100", keyTargetLinodeID: float64(456)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyBackupID: "100", keyTargetLinodeID: float64(456), keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "missing backup id", args: map[string]any{keyLinodeID: "123", keyTargetLinodeID: float64(456), keyConfirm: true}, wantContains: "backup_id is required"},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupRestoreTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: "123", keyBackupID: "100", keyTargetLinodeID: float64(456), keyConfirm: true,
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: "123"}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupsEnableTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyConfirm: true})
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: "123"}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceBackupsCancelTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Backup service canceled", "response should confirm backup cancel")
	})
}

// Dry-run coverage for instance backups cancel (POST action, WithID).
// The cancel is a POST, so would_execute.method must be POST and the
// fetch hits the instance, never mutating.
func TestLinodeInstanceBackupsCancelToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		instanceBody := `{"id":123,"label":"web-01","status":"running","backups":{"enabled":true}}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, "/linode/instances/123", r.URL.Path,
				"dry_run must GET the instance, not the cancel endpoint")

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(instanceBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, "linode_instance_backups_cancel", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"], "cancel is a POST action")
		assert.Equal(t, "/linode/instances/123/backups/cancel", would["path"])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never POST")
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errLinodeIDRequired)
	})
}

// TestLinodeInstanceDisksListTool verifies the instance disks list tool
// registers correctly, validates linode_id, and returns disk data.
func TestLinodeInstanceDisksListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingLinodeID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errLinodeIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		disks := []linode.InstanceDisk{
			{ID: 10, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady},
			{ID: 11, Label: "512 MB Swap Image", Size: 512, Filesystem: "swap", Status: statusReady},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: disks, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, imageUbuntu2404, "response should contain disk label")
		assert.Contains(t, textContent.Text, "512 MB Swap Image", "response should contain swap disk label")
	})
}

// TestLinodeInstanceDiskGetTool verifies the instance disk get tool
// registers correctly, validates required fields, and retrieves disk details.
func TestLinodeInstanceDiskGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskGetTool(cfg)

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
		{name: caseMissingLinodeID, args: map[string]any{keyDiskID: float64(10)}, wantContains: errLinodeIDRequired},
		{name: "missing disk id", args: map[string]any{keyLinodeID: float64(123)}, wantContains: "disk_id is required"},
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

		disk := linode.InstanceDisk{ID: 10, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, imageUbuntu2404, "response should contain disk label")
		assert.Contains(t, textContent.Text, filesystemExt4, "response should contain filesystem type")
	})
}

// End-to-end verification of instance disk creation workflow.
func TestLinodeInstanceDiskCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelMyDisk, keySize: float64(1024)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyLabel: labelMyDisk, keySize: float64(1024), keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingLabel, args: map[string]any{keyLinodeID: float64(123), keySize: float64(1024), keyConfirm: true}, wantContains: errLabelRequired},
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

		disk := linode.InstanceDisk{ID: 50, Label: labelMyDisk, Size: 1024, Filesystem: filesystemExt4, Status: statusReady}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyLabel: labelMyDisk, keySize: float64(1024), keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelMyDisk, "response should contain disk label")
		assert.Contains(t, textContent.Text, "50", "response should contain disk ID")
	})
}

// TestLinodeInstanceDiskUpdateTool verifies the instance disk update tool
// registers correctly, validates confirm, and updates disk labels.
func TestLinodeInstanceDiskUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskUpdateTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyLabel: labelNew})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		disk := linode.InstanceDisk{ID: 10, Label: "renamed-disk", Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(disk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyDiskID: float64(10), keyLabel: "renamed-disk", keyConfirm: true,
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted", "response should confirm deletion")
	})
}

// Dry-run coverage for instance disk delete (ByTwoIDs helper).
func TestLinodeInstanceDiskDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceDiskDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		diskBody := `{"id":10,"label":"boot","size":25600,"filesystem":"ext4","status":"ready"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)

			if r.Method != http.MethodGet {
				t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/linode/instances/123/disks/10" {
				_, _ = w.Write([]byte(diskBody))

				return
			}

			// The Tier A walk also lists config profiles; an empty page keeps
			// this subtest on the no-mutation and preview-shape contract.
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDiskID:   float64(10),
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_instance_disk_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/linode/instances/123/disks/10", would["path"])

		require.NotEmpty(t, methodsSeen, "dry_run must read state")
		assert.NotContains(t, methodsSeen, http.MethodDelete,
			"dry_run must never issue a DELETE")
	})

	t.Run("still validates disk_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceDiskDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "disk_id is required")
	})
}

// TestLinodeInstanceDiskCloneTool verifies the instance disk clone tool
// registers correctly, validates confirm, and clones disks.
func TestLinodeInstanceDiskCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskCloneTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		clonedDisk := linode.InstanceDisk{ID: 99, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10/clone", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(clonedDisk), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskCloneTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true})
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskResizeTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keySize: float64(65536)}, wantContains: errConfirmEqualsTrue},
		{name: "missing size", args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true}, wantContains: "size is required"},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskResizeTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyDiskID: float64(10), keySize: float64(65536), keyConfirm: true,
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ip_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingLinodeID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errLinodeIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ips := linode.InstanceIPAddresses{
			IPv4: &linode.InstanceIPv4{
				Public: []linode.IPAddress{
					{Address: ip203_0_113_1, Public: true, Type: keyIPv4, Region: regionUSEast},
				},
				Private: []linode.IPAddress{
					{Address: ip192168_1_1, Public: false, Type: keyIPv4, Region: regionUSEast},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, ip203_0_113_1, "response should contain public IP")
		assert.Contains(t, textContent.Text, ip192168_1_1, "response should contain private IP")
	})
}

// TestLinodeInstanceIPGetTool verifies the instance IP get tool
// registers correctly, validates required fields, and retrieves IP details.
func TestLinodeInstanceIPGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPGetTool(cfg)

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
		{name: caseMissingLinodeID, args: map[string]any{keyAddress: ip203_0_113_1}, wantContains: errLinodeIDRequired},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errAddressRequired},
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
			Address: ip203_0_113_1, Gateway: "203.0.113.0", SubnetMask: subnetMaskFixture,
			Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ipAddr), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, ip203_0_113_1, "response should contain IP address")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain region")
	})
}

// TestLinodeInstanceIPAllocateTool verifies the instance IP allocate tool
// registers correctly, validates confirm, and allocates new IP addresses.
func TestLinodeInstanceIPAllocateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPAllocateTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyType: keyIPv4, purposePublic: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful allocation", func(t *testing.T) {
		t.Parallel()

		ipAddr := linode.IPAddress{
			Address: "198.51.100.5", Gateway: "198.51.100.0", SubnetMask: subnetMaskFixture,
			Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPAllocateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyType: keyIPv4, purposePublic: true, keyConfirm: true,
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

// TestLinodeInstanceIPUpdateRDNSTool verifies the instance IP RDNS update tool
// registers correctly, validates confirm and required fields, and updates RDNS.
func TestLinodeInstanceIPUpdateRDNSTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceIPUpdateRDNSTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_ip_update_rdns", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyAddress, "schema should include address")
		assert.Contains(t, props, keyRDNS, "schema should include rdns")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123), keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressRequired},
		{name: "address with slash", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1/24", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "address with query separator", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1?bad=1", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "address with dot traversal", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1..", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "missing rdns", args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyConfirm: true}, wantContains: "rdns is required"},
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

	t.Run("client error maps to tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid rdns"}]}`))
			assert.NoError(t, writeErr, "writing error response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPUpdateRDNSTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to assign RDNS")
		assertErrorContains(t, result, "invalid rdns")
	})

	t.Run("successful rdns update", func(t *testing.T) {
		t.Parallel()

		ipAddr := linode.IPAddress{
			Address: ip203_0_113_1, Gateway: "203.0.113.0", SubnetMask: subnetMaskFixture,
			Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123, RDNS: rdnsTestExampleOrg,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/ips/203.0.113.1", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

			var body map[string]*string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")

			rdns := body[keyRDNS]
			if assert.NotNil(t, rdns, "rdns should be present") {
				assert.Equal(t, rdnsTestExampleOrg, *rdns, "rdns should match request")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ipAddr), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPUpdateRDNSTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyRDNS: rdnsTestExampleOrg, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, rdnsTestExampleOrg, "response should contain updated RDNS")
		assert.Contains(t, textContent.Text, "updated", "response should confirm update")
	})
}

// TestLinodeInstanceIPDeleteTool verifies the instance IP delete tool
// registers correctly, validates required fields, and removes IP addresses.
func TestLinodeInstanceIPDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: ip203_0_113_1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, wantContains: errAddressRequired},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceIPDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyAddress: ip203_0_113_1, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed", "response should confirm removal")
		assert.Contains(t, textContent.Text, ip203_0_113_1, "response should contain removed IP")
	})
}

// Dry-run coverage for instance IP delete (mixed int+string IDs).
func TestLinodeInstanceIPDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceIPDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		ipBody := `{"address":"203.0.113.1","type":"ipv4","public":true,"linode_id":123}`
		expectedPath := "/linode/instances/123/ips/203.0.113.1"

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, expectedPath, r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(ipBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyAddress:  ip203_0_113_1,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, "linode_instance_ip_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, expectedPath, would["path"])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("still validates address", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceIPDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "address is required")
	})
}

// TestLinodeInstanceCloneTool verifies the instance clone tool
// registers correctly, validates confirm, and clones instances.
func TestLinodeInstanceCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceCloneTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		instance := linode.Instance{ID: 999, Label: "my-linode-clone", Region: regionUSEast, Status: "provisioning"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/clone", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceCloneTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyLabel: "my-linode-clone", keyConfirm: true,
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyRegion: regionEUWest})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceMigrateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyRegion: regionEUWest, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Migration initiated", "response should confirm migration")
		assert.Contains(t, textContent.Text, regionEUWest, "response should contain target region")
	})
}

// End-to-end verification of instance rebuild workflow.
func TestLinodeInstanceRebuildTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyRootPass: rootPassStrong}, wantContains: errConfirmEqualsTrue},
		{name: "missing image", args: map[string]any{keyLinodeID: float64(123), keyRootPass: rootPassStrong, keyConfirm: true}, wantContains: "image is required"},
		{name: "missing root pass", args: map[string]any{keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyConfirm: true}, wantContains: "root_pass is required"},
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
			ID: 123, Label: "my-linode", Region: regionUSEast, Image: imageIDUbuntu2404, Status: "rebuilding",
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceRebuildTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyRootPass: rootPassStrong, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "rebuilt", "response should confirm rebuild")
		assert.Contains(t, textContent.Text, imageIDUbuntu2404, "response should contain image name")
	})
}

// Dry-run coverage for instance rebuild (POST action, lower-level helper
// with captured-var Success). Verifies the preview fetches the instance,
// emits POST + rebuild path, and never mutates.
func TestLinodeInstanceRebuildToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceRebuildTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		instanceBody := `{"id":123,"label":"web-01","image":"linode/debian12","status":"running"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)

			if r.Method != http.MethodGet {
				t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "application/json")

			// The Phase 2 side-effects walk also lists the instance disks.
			if r.URL.Path == "/linode/instances/123/disks" {
				_, _ = w.Write([]byte(`{"data":[]}`))

				return
			}

			assert.Equal(t, "/linode/instances/123", r.URL.Path,
				"dry_run must GET the instance, not the rebuild endpoint")

			_, _ = w.Write([]byte(instanceBody))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyImage:    imageIDUbuntu2404,
			keyRootPass: rootPassStrong,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, "linode_instance_rebuild", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/linode/instances/123/rebuild", would["path"])

		assert.NotContains(t, methodsSeen, http.MethodPost,
			"dry_run must never issue a POST")
	})

	t.Run("still validates root_pass", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceRebuildTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyImage:    imageIDUbuntu2404,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		assert.True(t, result.IsError,
			"dry_run must validate required body args the same way the real call would")
		assertErrorContains(t, result, "root_pass is required")
	})
}

// TestLinodeInstanceRescueTool verifies the instance rescue tool
// registers correctly, validates confirm, and boots instances into rescue mode.
func TestLinodeInstanceRescueTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

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

	t.Run(caseMissingConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceRescueTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true})
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

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
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyRootPass: "NewStr0ngP@ss!"}, wantContains: errConfirmEqualsTrue},
		{name: "missing root pass", args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, wantContains: "root_pass is required"},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstancePasswordResetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyRootPass: "NewStr0ngP@ss!", keyConfirm: true,
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

// Dry-run coverage for instance password reset (POST action, WithID).
func TestLinodeInstancePasswordResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstancePasswordResetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		instanceBody := `{"id":123,"label":"web-01","status":"offline"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, "/linode/instances/123", r.URL.Path,
				"dry_run must GET the instance, not the password endpoint")

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(instanceBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyRootPass: rootPassStrong,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, "linode_instance_password_reset", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/linode/instances/123/password", would["path"])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never POST")
	})

	t.Run("still validates root_pass", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstancePasswordResetTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "root_pass is required")
	})
}

func TestLinodeInstanceVolumesListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceVolumeListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_volume_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, "page", "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: "separator linode id", args: map[string]any{keyLinodeID: "123/.."}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
		{name: "fractional linode id", args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
		{name: caseInvalidInstanceFirewallsPage, args: map[string]any{keyLinodeID: float64(123), keyPage: float64(0)}, wantContains: errInstanceFirewallsPageMin},
		{name: caseInvalidPageSizeLow, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(10)}, wantContains: "page_size must be an integer from 25 through 500"},
		{name: caseInvalidPageSizeHigh, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(501)}, wantContains: "page_size must be an integer from 25 through 500"},
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

		volumes := []linode.Volume{
			{ID: 321, Label: "data-volume", Status: statusActive, Size: 50, Region: regionUSEast},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/volumes", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: volumes, keyPage: 2, keyPages: 3, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceVolumeListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "data-volume", "response should contain volume label")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain volume region")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceVolumeListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list volumes for instance 123")
	})
}
