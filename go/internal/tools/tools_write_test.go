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

// validTestSSHKey is a fake but valid-looking SSH key for testing purposes.
// It has the correct prefix and length to pass validation.
const validTestSSHKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 user@example.com"

// End-to-end verification of the SSH key creation workflow.
func TestLinodeSSHKeyCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

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
		{name: "missing label", args: map[string]any{"ssh_key": validTestSSHKey}, wantContains: "label is required"},
		{name: "missing ssh key", args: map[string]any{"label": "my-key"}, wantContains: "ssh_key is required"},
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
			Label: "my-key",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys", r.URL.Path, "request path should match SSH key endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(createdKey), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeSSHKeyCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":   "my-key",
			"ssh_key": validTestSSHKey,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "my-key", "response should contain the key label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the SSH key deletion workflow.
func TestLinodeSSHKeyDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_sshkey_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "sshkey_id", "schema should include sshkey_id property")
	})

	t.Run("missing sshkey id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeSSHKeyDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{"sshkey_id": float64(123)})
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceBootTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{})
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceBootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceRebootTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{})
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceRebootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceShutdownTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{})
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceShutdownTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "shutdown initiated successfully", "response should confirm shutdown")
	})
}

// End-to-end verification of the instance creation workflow.
func TestLinodeInstanceCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceCreateTool(cfg)

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
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         "requires confirm",
			args:         map[string]any{"region": "us-east", "type": "g6-nanode-1"},
			wantContains: "confirm=true",
		},
		{
			name:         "missing region",
			args:         map[string]any{"type": "g6-nanode-1", "confirm": true},
			wantContains: "region is required",
		},
		{
			name:         "missing type",
			args:         map[string]any{"region": "us-east", "confirm": true},
			wantContains: "type is required",
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

		instance := linode.Instance{
			ID:     456,
			Label:  "web-server",
			Region: "us-east",
			Status: "provisioning",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances", r.URL.Path, "request path should match instance endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east",
			"type":    "g6-nanode-1",
			"label":   "web-server",
			"confirm": true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "web-server", "response should contain the instance label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the instance deletion workflow.
func TestLinodeInstanceDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceDeleteTool(cfg)

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
			name:         "requires confirm",
			args:         map[string]any{"instance_id": float64(123)},
			wantContains: "confirm=true",
		},
		{
			name:         "missing instance id",
			args:         map[string]any{"confirm": true},
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"instance_id": float64(123),
			"confirm":     true,
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

// End-to-end verification of the instance resize workflow.
func TestLinodeInstanceResizeTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeInstanceResizeTool(cfg)

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
			name:         "requires confirm",
			args:         map[string]any{"instance_id": float64(123), "type": "g6-standard-1"},
			wantContains: "confirm=true",
		},
		{
			name:         "missing instance id",
			args:         map[string]any{"type": "g6-standard-1", "confirm": true},
			wantContains: "instance_id is required",
		},
		{
			name:         "missing type",
			args:         map[string]any{"instance_id": float64(123), "confirm": true},
			wantContains: "type is required",
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeInstanceResizeTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"instance_id": float64(123),
			"type":        "g6-standard-1",
			"confirm":     true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "resize", "response should mention resize")
		assert.Contains(t, textContent.Text, "g6-standard-1", "response should contain the new plan type")
	})
}

// End-to-end verification of the firewall creation workflow.
func TestLinodeFirewallCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeFirewallCreateTool(cfg)

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

	t.Run("missing label", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "label is required")
	})

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		firewall := linode.Firewall{
			ID:     789,
			Label:  "web-firewall",
			Status: "enabled",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/firewalls", r.URL.Path, "request path should match firewall endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(firewall), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeFirewallCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":          "web-firewall",
			"inbound_policy": "DROP",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeFirewallUpdateTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{"label": "new-label"})
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
			Status: "enabled",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/firewalls/789", r.URL.Path, "request path should match firewall endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(firewall), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeFirewallUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"firewall_id": float64(789),
			"label":       "updated-firewall",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeFirewallDeleteTool(cfg)

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

	t.Run("requires confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"firewall_id": float64(789)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeFirewallDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"firewall_id": float64(789),
			"confirm":     true,
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

// End-to-end verification of the domain creation workflow.
func TestLinodeDomainCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainCreateTool(cfg)

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
		{name: "missing domain", args: map[string]any{"type": "master"}, wantContains: "domain is required"},
		{name: "missing type", args: map[string]any{"domain": "example.com"}, wantContains: "type is required"},
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
			Domain: "example.com",
			Type:   "master",
			Status: "active",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains", r.URL.Path, "request path should match domain endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain":    "example.com",
			"type":      "master",
			"soa_email": "admin@example.com",
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "example.com", "response should contain the domain name")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the domain update workflow.
func TestLinodeDomainUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainUpdateTool(cfg)

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

	t.Run("missing domain id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"soa_email": "new@example.com"})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "domain_id is required")
	})

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		domain := linode.Domain{
			ID:     111,
			Domain: "example.com",
			Type:   "master",
			Status: "active",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111", r.URL.Path, "request path should match domain endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(domain), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain_id": float64(111),
			"soa_email": "new@example.com",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainDeleteTool(cfg)

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

	t.Run("requires confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"domain_id": float64(111)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain_id": float64(111),
			"confirm":   true,
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

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
			name:         "missing domain id",
			args:         map[string]any{"type": "A", "target": "192.168.1.1"},
			wantContains: "domain_id is required",
		},
		{
			name:         "missing type",
			args:         map[string]any{"domain_id": float64(111), "target": "192.168.1.1"},
			wantContains: "type is required",
		},
		{
			name:         "missing target",
			args:         map[string]any{"domain_id": float64(111), "type": "A"},
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
			Name:   "www",
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainRecordCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain_id": float64(111),
			"type":      "A",
			"name":      "www",
			"target":    "203.0.113.50",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

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
			name:         "missing domain id",
			args:         map[string]any{"record_id": float64(222), "target": "192.168.1.2"},
			wantContains: "domain_id is required",
		},
		{
			name:         "missing record id",
			args:         map[string]any{"domain_id": float64(111), "target": "192.168.1.2"},
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
			Name:   "www",
			Target: "192.168.1.2",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/domains/111/records/222", r.URL.Path, "request path should match record endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(record), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainRecordUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain_id": float64(111),
			"record_id": float64(222),
			"target":    "192.168.1.2",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

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
			name:         "missing domain id",
			args:         map[string]any{"record_id": float64(222)},
			wantContains: "domain_id is required",
		},
		{
			name:         "missing record id",
			args:         map[string]any{"domain_id": float64(111)},
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeDomainRecordDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"domain_id": float64(111),
			"record_id": float64(222),
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeVolumeCreateTool(cfg)

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
			name:         "requires confirm",
			args:         map[string]any{"label": "data-vol", "region": "us-east"},
			wantContains: "confirm=true",
		},
		{
			name:         "missing label",
			args:         map[string]any{"region": "us-east", "confirm": true},
			wantContains: "label is required",
		},
		{
			name:         "requires region or linode id",
			args:         map[string]any{"label": "data-vol", "confirm": true},
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
			Label:  "data-vol",
			Region: "us-east",
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeVolumeCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":   "data-vol",
			"region":  "us-east",
			"size":    float64(50),
			"confirm": true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, "data-vol", "response should contain the volume label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of the volume attach workflow.
func TestLinodeVolumeAttachTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeVolumeAttachTool(cfg)

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
			args:         map[string]any{"linode_id": float64(123)},
			wantContains: "volume_id is required",
		},
		{
			name:         "missing linode id",
			args:         map[string]any{"volume_id": float64(333)},
			wantContains: "linode_id is required",
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
			Label:    "data-vol",
			Region:   "us-east",
			LinodeID: &linodeID,
			Status:   "active",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes/333/attach", r.URL.Path, "request path should match attach endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(volume), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeVolumeAttachTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"volume_id": float64(333),
			"linode_id": float64(123),
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeVolumeDetachTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{})
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeVolumeDetachTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{"volume_id": float64(333)})
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeVolumeResizeTool(cfg)

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
			name:         "requires confirm",
			args:         map[string]any{"volume_id": float64(333), "size": float64(100)},
			wantContains: "confirm=true",
		},
		{
			name:         "missing volume id",
			args:         map[string]any{"size": float64(100), "confirm": true},
			wantContains: "volume_id is required",
		},
		{
			name: "missing size",
			args: map[string]any{"volume_id": float64(333), "confirm": true},
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
			Label:  "data-vol",
			Region: "us-east",
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeVolumeResizeTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"volume_id": float64(333),
			"size":      float64(100),
			"confirm":   true,
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeVolumeDeleteTool(cfg)

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

	t.Run("requires confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"volume_id": float64(333)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeVolumeDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"volume_id": float64(333),
			"confirm":   true,
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

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
			name:         "requires confirm",
			args:         map[string]any{"region": "us-east"},
			wantContains: "confirm=true",
		},
		{
			name:         "missing region",
			args:         map[string]any{"confirm": true},
			wantContains: "region is required",
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
			Region: "us-east",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/nodebalancers", r.URL.Path, "request path should match NodeBalancer endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(nodeBalancer), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeNodeBalancerCreateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east",
			"label":   "web-lb",
			"confirm": true,
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

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
		req := createRequestWithArgs(t, map[string]any{"label": "new-label"})
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
			Region: "us-east",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/nodebalancers/444", r.URL.Path, "request path should match NodeBalancer endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(nodeBalancer), "encoding response should succeed")
		}))
		defer srv.Close()

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeNodeBalancerUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"nodebalancer_id": float64(444),
			"label":           "updated-lb",
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
		"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
	}}
	tool, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

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

	t.Run("requires confirm", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{"nodebalancer_id": float64(444)})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "confirm=true")
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
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
		}}
		_, successHandler := tools.NewLinodeNodeBalancerDeleteTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			"nodebalancer_id": float64(444),
			"confirm":         true,
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
