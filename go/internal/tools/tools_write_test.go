package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// validTestSSHKey is a fake but valid-looking SSH key for testing purposes.
// It has the correct prefix and length to pass validation.
const validTestSSHKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 user@example.com"

// End-to-end verification of the SSH key creation workflow.
func TestLinodeSSHKeyCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_sshkey_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "ssh_key", "schema should include ssh_key property")
		assert.Contains(t, props, "environment", "schema should include environment property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLabel, args: map[string]any{"ssh_key": validTestSSHKey, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "missing ssh key", args: map[string]any{keyLabel: keyNameTest, keyConfirm: true}, wantContains: "ssh_key is required"},
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

		createdKey := linode.SSHKey{
			ID:    123,
			Label: keyNameTest,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys", r.URL.Path, "request path should match SSH key endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(createdKey), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeSSHKeyCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   keyNameTest,
			"ssh_key":  validTestSSHKey,
			keyConfirm: true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, keyNameTest, "response should contain the key label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the SSH key update workflow.
func TestLinodeSSHKeyUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_sshkey_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keySSHKeyID, "schema should include sshkey_id property")
		assert.Contains(t, props, keyLabel, "schema should include label property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
	})

	t.Run("confirm must be literal true before client call", func(t *testing.T) {
		t.Parallel()

		var callCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount.Add(1)

			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeSSHKeyUpdateTool(successCfg)

		tests := []struct {
			name    string
			confirm any
			set     bool
		}{
			{name: "missing"},
			{name: "false", confirm: false, set: true},
			{name: "string true", confirm: boolStringTrue, set: true},
			{name: "numeric one", confirm: 1, set: true},
		}

		for _, tt := range tests {
			args := map[string]any{keySSHKeyID: float64(123), keyLabel: keyNameTest}
			if tt.set {
				args[keyConfirm] = tt.confirm
			}

			req := createRequestWithArgs(t, args)
			result, err := successHandler(t.Context(), req)

			require.NoError(t, err, "handler should not return Go error for %s", tt.name)
			require.NotNil(t, result, "handler should return a result for %s", tt.name)
			assert.True(t, result.IsError, "result should be a tool error for %s", tt.name)
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Zero(t, callCount.Load(), "confirm failures should happen before the client call for %s", tt.name)
		}
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing sshkey id", args: map[string]any{keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "zero sshkey id", args: map[string]any{keySSHKeyID: float64(0), keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "negative sshkey id", args: map[string]any{keySSHKeyID: float64(-1), keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id with slash", args: map[string]any{keySSHKeyID: "123/45", keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id with query", args: map[string]any{keySSHKeyID: "123?x=1", keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id traversal", args: map[string]any{keySSHKeyID: pathTraversalValue, keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: caseMissingLabel, args: map[string]any{keySSHKeyID: float64(123), keyConfirm: true}, wantContains: errLabelRequired},
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

	t.Run("api failure returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys/123", r.URL.Path, "request path should match SSH key endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			http.Error(w, "bad request", http.StatusBadRequest)
		}))
		defer srv.Close()

		failureCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, failureHandler := tools.NewLinodeSSHKeyUpdateTool(failureCfg)

		req := createRequestWithArgs(t, map[string]any{
			keySSHKeyID: float64(123),
			keyLabel:    keyNameTest,
			keyConfirm:  true,
		})
		result, err := failureHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "failed to change label")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		updatedKey := linode.SSHKey{
			ID:    123,
			Label: keyNameTest,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys/123", r.URL.Path, "request path should match SSH key endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var req linode.UpdateSSHKeyRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&req), "request body should decode")
			assert.Equal(t, keyNameTest, req.Label, "request body should include the new label")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(updatedKey), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeSSHKeyUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keySSHKeyID: float64(123),
			keyLabel:    keyNameTest,
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
		assert.Contains(t, textContent.Text, keyNameTest, "response should contain the key label")
	})
}

// End-to-end verification of the SSH key deletion workflow.
func TestLinodeSSHKeyDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_sshkey_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keySSHKeyID, "schema should include sshkey_id property")
	})

	t.Run("missing sshkey id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "sshkey_id is required")
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys/123", r.URL.Path, "request path should match SSH key endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeSSHKeyDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(123), keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// End-to-end verification of the instance boot workflow.
func TestLinodeInstanceBootTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceBootTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_boot", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "instance_id", "schema should include instance_id property")
		assert.Contains(t, props, "config_id", "schema should include config_id property")
	})

	t.Run("missing instance id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "instance_id is required")
	})

	t.Run("successful boot", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/boot", r.URL.Path, "request path should match boot endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceBootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "boot initiated successfully", "response should confirm boot")
	})
}

// End-to-end verification of the instance reboot workflow.
func TestLinodeInstanceRebootTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceRebootTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_reboot", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "instance_id", "schema should include instance_id property")
		assert.Contains(t, props, "config_id", "schema should include config_id property")
	})

	t.Run("missing instance id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "instance_id is required")
	})

	t.Run("successful reboot", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/reboot", r.URL.Path, "request path should match reboot endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceRebootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "reboot initiated successfully", "response should confirm reboot")
	})
}

// End-to-end verification of the instance shutdown workflow.
func TestLinodeInstanceShutdownTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_shutdown", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "instance_id", "schema should include instance_id property")
	})

	t.Run("missing instance id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "instance_id is required")
	})

	t.Run("successful shutdown", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/shutdown", r.URL.Path, "request path should match shutdown endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceShutdownTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "shutdown initiated successfully", "response should confirm shutdown")
	})
}

// End-to-end verification of the instance creation workflow under the current
// Linode Interfaces generation. The wire shape matches BIMHelperScripts
// linode_add_network at api-common.sh:378 exactly.
func TestLinodeInstanceCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "type", "schema should include type property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "image", "schema should include image property")
		assert.Contains(t, props, keyFirewallID, "schema should include firewall_id property under current Interfaces generation")
		assert.Contains(t, props, "route_ipv4", "schema should include route_ipv4 property")
		assert.Contains(t, props, "route_ipv6", "schema should include route_ipv6 property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")

		// private_ip is replaced by interface-level VPC routing in the current
		// API and must not be a tool parameter.
		assert.NotContains(t, props, "private_ip", "schema must not include legacy private_ip property")

		// firewall_id is a hard requirement of the current API.
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "firewall_id must be marked required")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyRegion: regionUSEast, keyType: typeG6Nanode1, keyFirewallID: 12345},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyType: typeG6Nanode1, keyFirewallID: 12345, keyConfirm: true},
			wantContains: errRegionRequired,
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyRegion: regionUSEast, keyFirewallID: 12345, keyConfirm: true},
			wantContains: errTypeRequired,
		},
		{
			name:         caseMissingFirewallID,
			args:         map[string]any{keyRegion: regionUSEast, keyType: typeG6Nanode1, keyConfirm: true},
			wantContains: errFirewallIDRequired,
		},
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

	t.Run("body shape matches BIMHelperScripts reference", func(t *testing.T) {
		t.Parallel()

		var capturedBody map[string]any

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances", r.URL.Path, "request path should match instance endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody), "request body should be valid JSON")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Instance{ID: 456, Label: "web-server", Region: regionUSEast, Status: "provisioning"}))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:     regionUSEast,
			keyType:       typeG6Nanode1,
			keyLabel:      "web-server",
			keyFirewallID: 12345,
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		// Top-level wire fields per linode_add_network at api-common.sh:378.
		assert.Equal(t, "linode", capturedBody["interface_generation"], "interface_generation must be 'linode'")

		interfaces, interfacesOK := capturedBody["interfaces"].([]any)
		require.True(t, interfacesOK, "interfaces must be present as an array")
		require.Len(t, interfaces, 1, "exactly one interface must be sent")

		iface, ifaceOK := interfaces[0].(map[string]any)
		require.True(t, ifaceOK, "interface element must be an object")

		// public: {} is sent so the API uses defaults; no nested fields under it.
		pub, pubOK := iface["public"].(map[string]any)
		require.True(t, pubOK, "public must be an object")
		assert.Empty(t, pub, "public must be an empty object so the API assigns defaults")

		// default_route: both families default to true.
		route, routeOK := iface["default_route"].(map[string]any)
		require.True(t, routeOK, "default_route must be an object")
		assert.Equal(t, true, route["ipv4"], "default_route.ipv4 should be true by default")
		assert.Equal(t, true, route["ipv6"], "default_route.ipv6 should be true by default")

		// firewall_id at interface level (not top-level).
		assert.InEpsilon(t, float64(12345), iface["firewall_id"], 0.0001, "firewall_id must be at the interface level")
		assert.NotContains(t, capturedBody, "firewall_id", "firewall_id must not appear at top level")

		textContent, textOK := result.Content[0].(mcp.TextContent)
		require.True(t, textOK, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "web-server", "response should contain the instance label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})

	t.Run("route flags omit ipv4 key when false", func(t *testing.T) {
		t.Parallel()

		var capturedBody map[string]any

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Instance{ID: 789, Label: "v6-only", Region: regionUSEast}))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:     regionUSEast,
			keyType:       typeG6Nanode1,
			keyFirewallID: 12345,
			"route_ipv4":  false,
			"route_ipv6":  true,
			keyConfirm:    true,
		})
		_, err := successHandler(t.Context(), req)
		require.NoError(t, err)

		interfaces, interfacesOK := capturedBody["interfaces"].([]any)
		require.True(t, interfacesOK)

		iface, ifaceOK := interfaces[0].(map[string]any)
		require.True(t, ifaceOK, "interface element must be an object")

		route, routeOK := iface["default_route"].(map[string]any)
		require.True(t, routeOK, "default_route must be present")

		// The wire shape must omit the ipv4 key entirely when false, not send
		// "ipv4": false. The API treats absence as "not the default route" for
		// that family.
		_, hasIPv4 := route["ipv4"]
		assert.False(t, hasIPv4, "default_route.ipv4 key must be omitted when route_ipv4 is false")
		assert.Equal(t, true, route["ipv6"], "default_route.ipv6 must be sent as true")
	})
}

// Instance GET response parsing under the current Interfaces generation must
// surface interface_generation and interfaces[] on the returned struct.
func TestLinodeInstanceGetParsesInterfaces(t *testing.T) {
	t.Parallel()

	firewallID := 12345
	respBody := linode.Instance{
		ID:                  321,
		Label:               "web-01",
		Status:              statusRunning,
		Region:              regionUSEast,
		InterfaceGeneration: "linode",
		Interfaces: []linode.InstanceInterface{
			{
				ID:           1,
				Public:       &linode.InterfacePublicConfig{},
				DefaultRoute: &linode.InterfaceDefaultRoute{IPv4: true, IPv6: true},
				FirewallID:   &firewallID,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/321", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(respBody))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: "321"})
	result, err := handler(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, textOK := result.Content[0].(mcp.TextContent)
	require.True(t, textOK)

	// Parse the JSON response and assert structurally so the test does not
	// depend on the marshaler's whitespace choices. The GET handler returns
	// the Instance unwrapped at the top level.
	var parsed linode.Instance

	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &parsed), "tool result must be valid JSON")
	assert.Equal(t, "linode", parsed.InterfaceGeneration, "interface_generation must be surfaced")
	require.Len(t, parsed.Interfaces, 1, "interfaces array must be populated")
	assert.Equal(t, 1, parsed.Interfaces[0].ID, "interface ID must be parsed")
	require.NotNil(t, parsed.Interfaces[0].FirewallID, "firewall_id must be parsed")
	assert.Equal(t, 12345, *parsed.Interfaces[0].FirewallID, "firewall_id value must match")
}

// End-to-end verification of the instance deletion workflow.
func TestLinodeInstanceDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "instance_id", "schema should include instance_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyInstanceID: float64(123)},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingInstanceID,
			args:         map[string]any{keyConfirm: true},
			wantContains: "instance_id is required",
		},
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
			assert.Equal(t, "/linode/instances/123", r.URL.Path, "request path should match instance endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})

	t.Run("dry_run schema property", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, tool.InputSchema.Properties, "dry_run",
			"schema must advertise the dry_run boolean to the model")
	})

	t.Run("dry_run returns preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		instanceBody := `{"id":456,"label":"web-test","type":"g6-standard-1","region":"us-east","status":"running"}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, "/linode/instances/456", r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(instanceBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, dryRunHandler := tools.NewLinodeInstanceDeleteTool(dryRunCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(456),
			keyDryRun:     true,
		})
		result, err := dryRunHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result)
		require.False(t, result.IsError, "dry_run with valid args should not be a tool error")

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_instance_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		require.True(t, isWouldObject, "would_execute must be a JSON object")
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/linode/instances/456", would["path"])

		state, stateIsObject := body["current_state"].(map[string]any)
		require.True(t, stateIsObject)
		assert.InDelta(t, 456, state[keyBetaID], 0)
		assert.Equal(t, "web-test", state[keyLabel])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET request, never DELETE")
	})

	t.Run("dry_run does not require confirm", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "dry_run path must only issue GET")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":789,"label":"no-confirm","status":"running"}`))
		}))
		defer srv.Close()

		dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, dryRunHandler := tools.NewLinodeInstanceDeleteTool(dryRunCfg)

		// Intentionally omit confirm; the dry-run path must not gate on it.
		req := createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(789),
			keyDryRun:     true,
		})
		result, err := dryRunHandler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError,
			"dry_run without confirm must succeed; confirm only gates real execution")
	})

	t.Run("dry_run still validates instance_id", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError,
			"dry_run with missing instance_id must error out the same way the real call would")
		assertErrorContains(t, result, "instance_id is required")
	})
}

// End-to-end verification of the instance resize workflow.
func TestLinodeInstanceResizeTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_resize", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "instance_id", "schema should include instance_id property")
		assert.Contains(t, props, "type", "schema should include type property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyInstanceID: float64(123), keyType: typeG6Standard1},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         "missing instance id",
			args:         map[string]any{keyType: typeG6Standard1, keyConfirm: true},
			wantContains: "instance_id is required",
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyInstanceID: float64(123), keyConfirm: true},
			wantContains: errTypeRequired,
		},
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
			assert.Equal(t, "/linode/instances/123/resize", r.URL.Path, "request path should match resize endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceResizeTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyType:       typeG6Standard1,
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "resize", "response should mention resize")
		assert.Contains(t, textContent.Text, typeG6Standard1, "response should contain the new plan type")
	})
}

// End-to-end verification of the firewall creation workflow.
func TestLinodeFirewallCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_firewall_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "inbound_policy", "schema should include inbound_policy property")
		assert.Contains(t, props, "outbound_policy", "schema should include outbound_policy property")
	})

	t.Run(caseMissingLabel, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errLabelRequired)
	})

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		firewall := linode.Firewall{
			ID:     789,
			Label:  "web-firewall",
			Status: statusEnabled,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/firewalls", r.URL.Path, "request path should match firewall endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(firewall), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeFirewallCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:         "web-firewall",
			"inbound_policy": "DROP",
			keyConfirm:       true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "web-firewall", "response should contain the firewall label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the firewall update workflow.
func TestLinodeFirewallUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_firewall_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "firewall_id", "schema should include firewall_id property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "status", "schema should include status property")
	})

	t.Run("missing firewall id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "firewall_id is required")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		firewall := linode.Firewall{
			ID:     789,
			Label:  "updated-firewall",
			Status: statusEnabled,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/firewalls/789", r.URL.Path, "request path should match firewall endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(firewall), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeFirewallUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(789),
			keyLabel:      "updated-firewall",
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// End-to-end verification of the firewall deletion workflow.
func TestLinodeFirewallDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_firewall_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "firewall_id", "schema should include firewall_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run(caseRequiresConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(789)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/firewalls/789", r.URL.Path, "request path should match firewall endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeFirewallDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(789),
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// End-to-end verification of the domain import workflow.
func TestLinodeDomainImportTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainImportTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_import", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain", "schema should include domain property")
		assert.Contains(t, props, keyRemoteNameserver, "schema should include remote_nameserver property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: "numeric", value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run("confirm "+tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyDomain: domainExample, keyRemoteNameserver: remoteNameserverExample}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
		})
	}

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomain, args: map[string]any{keyRemoteNameserver: remoteNameserverExample, keyConfirm: true}, wantContains: errDomainRequired},
		{name: "missing remote nameserver", args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: "remote_nameserver is required"},
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

	t.Run("successful import", func(t *testing.T) {
		t.Parallel()

		domain := linode.Domain{
			ID:     111,
			Domain: domainExample,
			Type:   keyMaster,
			Status: statusActive,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/import", r.URL.Path, "request path should match domain import endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, domainExample, body["domain"], "domain should be sent")
			assert.Equal(t, remoteNameserverExample, body[keyRemoteNameserver], "remote_nameserver should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainImportTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomain:           domainExample,
			keyRemoteNameserver: remoteNameserverExample,
			keyConfirm:          true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, domainExample, "response should contain the domain name")
		assert.Contains(t, textContent.Text, "imported successfully", "response should confirm import")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"invalid domain"}]}`))
			assert.NoError(t, err, "writing API error should succeed")
		}))
		defer srv.Close()

		errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errorHandler := tools.NewLinodeDomainImportTool(errorCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomain:           domainExample,
			keyRemoteNameserver: remoteNameserverExample,
			keyConfirm:          true,
		})
		result, err := errorHandler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to import domain")
	})
}

// End-to-end verification of the domain clone workflow.
func TestLinodeDomainCloneTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainCloneTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_clone", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyDomainID, "schema should include domain_id property")
		assert.Contains(t, props, keyDomain, "schema should include domain property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
		assert.Contains(t, tool.InputSchema.Required, keyDomainID, "domain_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyDomain, "domain must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: "numeric", value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run("confirm "+tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyDomainID: float64(111), keyDomain: domainExample}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
		})
	}

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomainID, args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: "zero domain id", args: map[string]any{keyDomainID: 0, keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: "negative domain id", args: map[string]any{keyDomainID: -1, keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: caseMissingDomain, args: map[string]any{keyDomainID: float64(111), keyConfirm: true}, wantContains: errDomainRequired},
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

	t.Run("successful clone", func(t *testing.T) {
		t.Parallel()

		domain := linode.Domain{ID: 222, Domain: domainExample, Type: keyMaster, Status: statusActive}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111/clone", r.URL.Path, "request path should match domain clone endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, domainExample, body[keyDomain], "domain should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainCloneTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111), keyDomain: domainExample, keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, domainExample, "response should contain the domain name")
		assert.Contains(t, textContent.Text, "cloned", "response should confirm clone")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"invalid domain"}]}`))
			assert.NoError(t, err, "writing API error should succeed")
		}))
		defer srv.Close()

		errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errorHandler := tools.NewLinodeDomainCloneTool(errorCfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111), keyDomain: domainExample, keyConfirm: true})
		result, err := errorHandler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to clone domain")
	})
}

// End-to-end verification of the domain creation workflow.
func TestLinodeDomainCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain", "schema should include domain property")
		assert.Contains(t, props, "type", "schema should include type property")
		assert.Contains(t, props, "soa_email", "schema should include soa_email property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomain, args: map[string]any{keyType: keyMaster, keyConfirm: true}, wantContains: errDomainRequired},
		{name: caseMissingType, args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: errTypeRequired},
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

		domain := linode.Domain{
			ID:     111,
			Domain: domainExample,
			Type:   keyMaster,
			Status: statusActive,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains", r.URL.Path, "request path should match domain endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomain:   domainExample,
			keyType:     keyMaster,
			keySoaEmail: "admin@example.com",
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, domainExample, "response should contain the domain name")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the domain update workflow.
func TestLinodeDomainUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain_id", "schema should include domain_id property")
		assert.Contains(t, props, "soa_email", "schema should include soa_email property")
		assert.Contains(t, props, "status", "schema should include status property")
	})

	t.Run(caseMissingDomainID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keySoaEmail: "new@example.com", keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errDomainIDRequired)
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		domain := linode.Domain{
			ID:     111,
			Domain: domainExample,
			Type:   keyMaster,
			Status: statusActive,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111", r.URL.Path, "request path should match domain endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(111),
			keySoaEmail: "new@example.com",
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// End-to-end verification of the domain deletion workflow.
func TestLinodeDomainDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain_id", "schema should include domain_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run(caseRequiresConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111", r.URL.Path, "request path should match domain endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(111),
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// End-to-end verification of the domain record creation workflow.
func TestLinodeDomainRecordCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_record_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain_id", "schema should include domain_id property")
		assert.Contains(t, props, "type", "schema should include type property")
		assert.Contains(t, props, "target", "schema should include target property")
		assert.Contains(t, props, "name", "schema should include name property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyType: "A", keyTarget: ip192168_1_1, keyConfirm: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyDomainID: float64(111), keyTarget: ip192168_1_1, keyConfirm: true},
			wantContains: errTypeRequired,
		},
		{
			name:         "missing target",
			args:         map[string]any{keyDomainID: float64(111), keyType: "A", keyConfirm: true},
			wantContains: "target is required",
		},
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

		record := linode.DomainRecord{
			ID:     222,
			Type:   "A",
			Name:   hostWWW,
			Target: "203.0.113.50",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111/records", r.URL.Path, "request path should match record endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(record), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainRecordCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(111),
			keyType:     "A",
			keyName:     hostWWW,
			keyTarget:   "203.0.113.50",
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the domain record update workflow.
func TestLinodeDomainRecordUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_record_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain_id", "schema should include domain_id property")
		assert.Contains(t, props, "record_id", "schema should include record_id property")
		assert.Contains(t, props, "target", "schema should include target property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyRecordID: float64(222), keyTarget: ip192168_1_2, keyConfirm: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111), keyTarget: ip192168_1_2, keyConfirm: true},
			wantContains: "record_id is required",
		},
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

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		record := linode.DomainRecord{
			ID:     222,
			Type:   "A",
			Name:   hostWWW,
			Target: ip192168_1_2,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111/records/222", r.URL.Path, "request path should match record endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(record), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainRecordUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(111),
			keyRecordID: float64(222),
			keyTarget:   ip192168_1_2,
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// End-to-end verification of the domain record deletion workflow.
func TestLinodeDomainRecordDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_domain_record_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "domain_id", "schema should include domain_id property")
		assert.Contains(t, props, "record_id", "schema should include record_id property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyRecordID: float64(222), keyConfirm: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111), keyConfirm: true},
			wantContains: "record_id is required",
		},
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
			assert.Equal(t, "/domains/111/records/222", r.URL.Path, "request path should match record endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeDomainRecordDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(111),
			keyRecordID: float64(222),
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// End-to-end verification of the volume creation workflow.
func TestLinodeVolumeCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "size", "schema should include size property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyLabel: labelDataVol, keyRegion: regionUSEast},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingLabel,
			args:         map[string]any{keyRegion: regionUSEast, keyConfirm: true},
			wantContains: errLabelRequired,
		},
		{
			name:         "requires region or linode id",
			args:         map[string]any{keyLabel: labelDataVol, keyConfirm: true},
			wantContains: "either region or linode_id is required",
		},
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

		volume := linode.Volume{
			ID:     333,
			Label:  labelDataVol,
			Region: regionUSEast,
			Size:   50,
			Status: "creating",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes", r.URL.Path, "request path should match volume endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(volume), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   labelDataVol,
			keyRegion:  regionUSEast,
			keySize:    float64(50),
			keyConfirm: true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, labelDataVol, "response should contain the volume label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the volume attach workflow.
func TestLinodeVolumeAttachTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeAttachTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_attach", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "volume_id", "schema should include volume_id property")
		assert.Contains(t, props, "linode_id", "schema should include linode_id property")
		assert.Contains(t, props, "config_id", "schema should include config_id property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         "missing volume id",
			args:         map[string]any{keyLinodeID: float64(123), keyConfirm: true},
			wantContains: "volume_id is required",
		},
		{
			name:         caseMissingLinodeID,
			args:         map[string]any{keyVolumeID: float64(333), keyConfirm: true},
			wantContains: errLinodeIDRequired,
		},
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

	t.Run("successful attachment", func(t *testing.T) {
		t.Parallel()

		linodeID := 123
		volume := linode.Volume{
			ID:       333,
			Label:    labelDataVol,
			Region:   regionUSEast,
			LinodeID: &linodeID,
			Status:   statusActive,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333/attach", r.URL.Path, "request path should match attach endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(volume), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeAttachTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLinodeID: float64(123),
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "attached", "response should confirm attachment")
	})
}

// End-to-end verification of the volume detach workflow.
func TestLinodeVolumeDetachTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_detach", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "volume_id", "schema should include volume_id property")
	})

	t.Run("missing volume id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "volume_id is required")
	})

	t.Run("successful detachment", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333/detach", r.URL.Path, "request path should match detach endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeDetachTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333), keyConfirm: true})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "detached successfully", "response should confirm detachment")
	})
}

// End-to-end verification of the volume resize workflow.
func TestLinodeVolumeResizeTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeResizeTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_resize", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "volume_id", "schema should include volume_id property")
		assert.Contains(t, props, "size", "schema should include size property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyVolumeID: float64(333), keySize: float64(100)},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         "missing volume id",
			args:         map[string]any{keySize: float64(100), keyConfirm: true},
			wantContains: "volume_id is required",
		},
		{
			name: "missing size",
			args: map[string]any{keyVolumeID: float64(333), keyConfirm: true},
			// When size is 0 or missing, validation returns "size is required" or min size error.
			wantContains: "size",
		},
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

		volume := linode.Volume{
			ID:     333,
			Label:  labelDataVol,
			Region: regionUSEast,
			Size:   100,
			Status: "resizing",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333/resize", r.URL.Path, "request path should match resize endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(volume), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeResizeTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keySize:     float64(100),
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "resize", "response should mention resize")
	})
}

// End-to-end verification of the volume deletion workflow.
func TestLinodeVolumeDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "volume_id", "schema should include volume_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run(caseRequiresConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333", r.URL.Path, "request path should match volume endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// End-to-end verification of the NodeBalancer creation workflow.
func TestLinodeNodeBalancerCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyRegion: regionUSEast},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyConfirm: true},
			wantContains: errRegionRequired,
		},
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

		nodeBalancer := linode.NodeBalancer{
			ID:     444,
			Label:  "web-lb",
			Region: regionUSEast,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/nodebalancers", r.URL.Path, "request path should match NodeBalancer endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(nodeBalancer), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeNodeBalancerCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast,
			keyLabel:   "web-lb",
			keyConfirm: true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "web-lb", "response should contain the NodeBalancer label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the NodeBalancer update workflow.
func TestLinodeNodeBalancerUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "nodebalancer_id", "schema should include nodebalancer_id property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "client_conn_throttle", "schema should include client_conn_throttle property")
	})

	t.Run("missing nodebalancer id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "nodebalancer_id is required")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		nodeBalancer := linode.NodeBalancer{
			ID:     444,
			Label:  "updated-lb",
			Region: regionUSEast,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/nodebalancers/444", r.URL.Path, "request path should match NodeBalancer endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(nodeBalancer), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeNodeBalancerUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(444),
			keyLabel:          "updated-lb",
			keyConfirm:        true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// End-to-end verification of the NodeBalancer deletion workflow.
func TestLinodeNodeBalancerDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "nodebalancer_id", "schema should include nodebalancer_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run(caseRequiresConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/nodebalancers/444", r.URL.Path, "request path should match NodeBalancer endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeNodeBalancerDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(444),
			keyConfirm:        true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// assertErrorContains checks that the error result contains the expected substring.
func assertErrorContains(t *testing.T, result *mcp.CallToolResult, expected string) {
	t.Helper()

	require.NotEmpty(t, result.Content, "expected content in error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent type")
	assert.Contains(t, textContent.Text, expected, "error text should contain expected substring")
}

// End-to-end verification of the volume update workflow.
func TestLinodeVolumeUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_volume_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "volume_id", "schema should include volume_id property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "tags", "schema should include tags property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run(caseRequiresConfirm, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errConfirmEqualsTrue)
	})

	t.Run("missing volume_id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "volume_id is required")
	})

	t.Run("missing label and tags", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333), keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "at least one of label or tags is required")
	})

	t.Run("successful update with label", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333", r.URL.Path, "request path should match volume endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Volume{ID: 333, Label: "updated-volume", Size: 20, Region: "us-east", Status: "active"}))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLabel:    "updated-volume",
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
	})

	t.Run("successful update with tags", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/444", r.URL.Path, "request path should match volume endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Volume{ID: 444, Label: "tagged-volume", Size: 50, Region: "us-west", Status: "active", Tags: []string{"production", "db"}}))
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(444),
			"tags":      "production, db",
			keyConfirm:  true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
	})

	t.Run("updater error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errors": [{"reason": "internal server error"}]}`)) // errcheck: test mock; write failure is acceptable
		}))
		defer srv.Close()

		errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errorHandler := tools.NewLinodeVolumeUpdateTool(errorCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyVolumeID: float64(333),
			keyLabel:    "new-label",
			keyConfirm:  true,
		})
		result, err := errorHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		tc, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, tc.Text, "update failed", "response should mention failure")
	})
}

// End-to-end verification of the image creation workflow.
func TestLinodeImageCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeImageCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_image_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "create tool should be a write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "disk_id", "schema should include disk_id property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "description", "schema should include description property")
		assert.Contains(t, props, "cloud_init", "schema should include cloud_init property")
		assert.Contains(t, props, "tags", "schema should include tags property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyDiskID: 123}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing disk id", args: map[string]any{keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "zero disk id", args: map[string]any{keyDiskID: 0, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "negative disk id", args: map[string]any{keyDiskID: -1, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requestCount.Add(1)
			}))
			t.Cleanup(srv.Close)

			validationCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, validationHandler := tools.NewLinodeImageCreateTool(validationCfg)

			req := createRequestWithArgs(t, tt.args)
			result, err := validationHandler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
			assert.Equal(t, int32(0), requestCount.Load(), "validation should reject before client call")
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		created := linode.Image{ID: "private/15", Label: "custom-image", Status: "creating", CreatedBy: "tester"}

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, "/images", r.URL.Path, "request path should be /images")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.InEpsilon(t, 123, body[keyDiskID], 0, "disk_id should be sent")
			assert.Equal(t, "custom-image", body["label"], "label should be sent")
			assert.Equal(t, "test image", body["description"], "description should be sent")
			assert.Equal(t, true, body["cloud_init"], "cloud_init should be sent")
			assert.Equal(t, []any{"blue", "green"}, body["tags"], "tags should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(created), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeImageCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyDiskID:     123,
			keyLabel:      "custom-image",
			"description": "test image",
			"cloud_init":  true,
			"tags":        "blue, green",
			keyConfirm:    true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")
		assert.Equal(t, int32(1), requestCount.Load(), "handler should call the client once")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "private/15", "response should contain the image ID")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})

	t.Run("client error propagates", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"disk not found"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errHandler := tools.NewLinodeImageCreateTool(errCfg)

		req := createRequestWithArgs(t, map[string]any{keyDiskID: 123, keyConfirm: true})
		result, err := errHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create image")
	})
}

func TestLinodeImageShareGroupTokenCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeImageShareGroupTokenCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_image_sharegroup_token_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "token creation should be admin capability because it returns token material")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyValidForShareGroupUUID, "schema should include valid_for_sharegroup_uuid property")
		assert.Contains(t, props, keyLabel, "schema should include label property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing share group uuid", args: map[string]any{keyConfirm: true}, wantContains: errValidForShareGroupUUID},
		{name: "empty share group uuid", args: map[string]any{keyValidForShareGroupUUID: blankString, keyConfirm: true}, wantContains: errValidForShareGroupUUID},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requestCount.Add(1)
			}))
			t.Cleanup(srv.Close)

			validationCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, validationHandler := tools.NewLinodeImageShareGroupTokenCreateTool(validationCfg)

			req := createRequestWithArgs(t, tt.args)
			result, err := validationHandler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
			assert.Equal(t, int32(0), requestCount.Load(), "validation should reject before client call")
		})
	}

	t.Run("successful token creation", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, "/images/sharegroups/tokens", r.URL.Path, "request path should be /images/sharegroups/tokens")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, "release-token", body[keyLabel], "label should be sent")
			assert.Equal(t, shareGroupUUIDFixture, body[keyValidForShareGroupUUID], "share group UUID should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyToken:                  shareGroupTokenValueFixture,
				keyTokenUUID:              shareGroupTokenUUIDFixture,
				keyStatus:                 statusActive,
				keyLabel:                  "release-token",
				"created":                 imageShareGroupTokenCreated,
				"updated":                 nil,
				"expiry":                  nil,
				keyValidForShareGroupUUID: shareGroupUUIDFixture,
				"sharegroup_uuid":         shareGroupUUIDFixture,
				"sharegroup_label":        shareGroupLabelFixture,
			}), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeImageShareGroupTokenCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyValidForShareGroupUUID: shareGroupUUIDFixture,
			keyLabel:                  "release-token",
			keyConfirm:                true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")
		assert.Equal(t, int32(1), requestCount.Load(), "handler should call the client once")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, shareGroupTokenUUIDFixture, "response should contain the token UUID")
		assert.Contains(t, textContent.Text, shareGroupTokenValueFixture, "response should contain token material")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})

	t.Run("client error propagates", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"share group not found"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errHandler := tools.NewLinodeImageShareGroupTokenCreateTool(errCfg)

		req := createRequestWithArgs(t, map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: true})
		result, err := errHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create image share group token")
	})
}

// End-to-end verification of the StackScript creation workflow.
func TestLinodeStackScriptCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeStackScriptCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_stackscript_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "script", "schema should include script property")
		assert.Contains(t, props, "images", "schema should include images property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: testDebian12Image},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         "missing label",
			args:         map[string]any{keyScript: testStackScript, keyImages: testDebian12Image, keyConfirm: true},
			wantContains: "label is required",
		},
		{
			name:         "missing script",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyImages: testDebian12Image, keyConfirm: true},
			wantContains: "script is required",
		},
		{
			name:         "missing images",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyConfirm: true},
			wantContains: "images is required",
		},
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

		created := linode.StackScript{
			ID:       456,
			Label:    testStackScriptLabel,
			Script:   testStackScript + "\necho hello",
			Images:   []string{testDebian12Image},
			IsPublic: false,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/stackscripts", r.URL.Path, "request path should match stackscript endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(created), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeStackScriptCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   testStackScriptLabel,
			keyScript:  testStackScript + "\necho hello",
			keyImages:  testDebian12Image,
			keyConfirm: true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, testStackScriptLabel, "response should contain the script label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})

	t.Run("client error propagates", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"label is not unique"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errHandler := tools.NewLinodeStackScriptCreateTool(errCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   testStackScriptLabel,
			keyScript:  testStackScript,
			keyImages:  testDebian12Image,
			keyConfirm: true,
		})
		result, err := errHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
	})

	t.Run("empty images after trim rejected", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   testStackScriptLabel,
			keyScript:  testStackScript,
			keyImages:  " , ",
			keyConfirm: true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "images is required")
	})
}
