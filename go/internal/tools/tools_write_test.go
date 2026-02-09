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

// =============================================================================
// SSH Key Write Tool Tests
// =============================================================================

func TestNewLinodeSSHKeyCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	assert.Equal(t, "linode_sshkey_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	// Verify required parameters exist in schema.
	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "ssh_key")
	assert.Contains(t, props, "environment")
}

// validTestSSHKey is a fake but valid-looking SSH key for testing purposes.
// It has the correct prefix and length to pass validation.
const validTestSSHKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 user@example.com"

func TestLinodeSSHKeyCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"ssh_key": validTestSSHKey})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeSSHKeyCreateTool_MissingSSHKey(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "my-key"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "ssh_key is required")
}

func TestLinodeSSHKeyCreateTool_Success(t *testing.T) {
	t.Parallel()

	createdKey := linode.SSHKey{
		ID:    123,
		Label: "my-key",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile/sshkeys", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(createdKey))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-key",
		"ssh_key": validTestSSHKey,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-key")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeSSHKeyDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	assert.Equal(t, "linode_sshkey_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "sshkey_id")
}

func TestLinodeSSHKeyDeleteTool_MissingSSHKeyID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "sshkey_id is required")
}

func TestLinodeSSHKeyDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile/sshkeys/123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"sshkey_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// Instance Write Tool Tests
// =============================================================================

func TestNewLinodeInstanceBootTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceBootTool(cfg)

	assert.Equal(t, "linode_instance_boot", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "instance_id")
	assert.Contains(t, props, "config_id")
}

func TestLinodeInstanceBootTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceBootTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "instance_id is required")
}

func TestLinodeInstanceBootTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/boot", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceBootTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "boot initiated successfully")
}

func TestNewLinodeInstanceRebootTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceRebootTool(cfg)

	assert.Equal(t, "linode_instance_reboot", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "instance_id")
	assert.Contains(t, props, "config_id")
}

func TestLinodeInstanceRebootTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceRebootTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "instance_id is required")
}

func TestLinodeInstanceRebootTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/reboot", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceRebootTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "reboot initiated successfully")
}

func TestNewLinodeInstanceShutdownTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	assert.Equal(t, "linode_instance_shutdown", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "instance_id")
}

func TestLinodeInstanceShutdownTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "instance_id is required")
}

func TestLinodeInstanceShutdownTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/shutdown", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "shutdown initiated successfully")
}

func TestNewLinodeInstanceCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceCreateTool(cfg)

	assert.Equal(t, "linode_instance_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "type")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "image")
	assert.Contains(t, props, "confirm")
}

func TestLinodeInstanceCreateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east",
		"type":   "g6-nanode-1",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeInstanceCreateTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"type":    "g6-nanode-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeInstanceCreateTool_MissingType(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "type is required")
}

func TestLinodeInstanceCreateTool_Success(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{
		ID:     456,
		Label:  "web-server",
		Region: "us-east",
		Status: "provisioning",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(instance))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east",
		"type":    "g6-nanode-1",
		"label":   "web-server",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-server")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeInstanceDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	assert.Equal(t, "linode_instance_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "instance_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeInstanceDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeInstanceDeleteTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"confirm": true})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "instance_id is required")
}

func TestLinodeInstanceDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"instance_id": float64(123),
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

func TestNewLinodeInstanceResizeTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceResizeTool(cfg)

	assert.Equal(t, "linode_instance_resize", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "instance_id")
	assert.Contains(t, props, "type")
	assert.Contains(t, props, "confirm")
}

func TestLinodeInstanceResizeTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"instance_id": float64(123),
		"type":        "g6-standard-1",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeInstanceResizeTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"type":    "g6-standard-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "instance_id is required")
}

func TestLinodeInstanceResizeTool_MissingType(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"instance_id": float64(123),
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "type is required")
}

func TestLinodeInstanceResizeTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/resize", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"instance_id": float64(123),
		"type":        "g6-standard-1",
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "resize")
	assert.Contains(t, textContent.Text, "g6-standard-1")
}

// =============================================================================
// Firewall Write Tool Tests
// =============================================================================

func TestNewLinodeFirewallCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeFirewallCreateTool(cfg)

	assert.Equal(t, "linode_firewall_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "inbound_policy")
	assert.Contains(t, props, "outbound_policy")
}

func TestLinodeFirewallCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeFirewallCreateTool_Success(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     789,
		Label:  "web-firewall",
		Status: "enabled",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/networking/firewalls", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(firewall))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":          "web-firewall",
		"inbound_policy": "DROP",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-firewall")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeFirewallUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	assert.Equal(t, "linode_firewall_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "firewall_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "status")
}

func TestLinodeFirewallUpdateTool_MissingFirewallID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "new-label"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "firewall_id is required")
}

func TestLinodeFirewallUpdateTool_Success(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     789,
		Label:  "updated-firewall",
		Status: "enabled",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/networking/firewalls/789", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(firewall))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"firewall_id": float64(789),
		"label":       "updated-firewall",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

func TestNewLinodeFirewallDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	assert.Equal(t, "linode_firewall_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "firewall_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeFirewallDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"firewall_id": float64(789)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeFirewallDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/networking/firewalls/789", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"firewall_id": float64(789),
		"confirm":     true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// Domain Write Tool Tests
// =============================================================================

func TestNewLinodeDomainCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainCreateTool(cfg)

	assert.Equal(t, "linode_domain_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain")
	assert.Contains(t, props, "type")
	assert.Contains(t, props, "soa_email")
}

func TestLinodeDomainCreateTool_MissingDomain(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"type": "master"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "domain is required")
}

func TestLinodeDomainCreateTool_MissingType(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"domain": "example.com"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "type is required")
}

func TestLinodeDomainCreateTool_Success(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{
		ID:     111,
		Domain: "example.com",
		Type:   "master",
		Status: "active",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(domain))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain":    "example.com",
		"type":      "master",
		"soa_email": "admin@example.com",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "example.com")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeDomainUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainUpdateTool(cfg)

	assert.Equal(t, "linode_domain_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain_id")
	assert.Contains(t, props, "soa_email")
	assert.Contains(t, props, "status")
}

func TestLinodeDomainUpdateTool_MissingDomainID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"soa_email": "new@example.com"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "domain_id is required")
}

func TestLinodeDomainUpdateTool_Success(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{
		ID:     111,
		Domain: "example.com",
		Type:   "master",
		Status: "active",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/111", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(domain))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"soa_email": "new@example.com",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

func TestNewLinodeDomainDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainDeleteTool(cfg)

	assert.Equal(t, "linode_domain_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeDomainDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"domain_id": float64(111)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeDomainDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/111", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// Domain Record Write Tool Tests
// =============================================================================

func TestNewLinodeDomainRecordCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	assert.Equal(t, "linode_domain_record_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain_id")
	assert.Contains(t, props, "type")
	assert.Contains(t, props, "target")
	assert.Contains(t, props, "name")
}

func TestLinodeDomainRecordCreateTool_MissingDomainID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"type":   "A",
		"target": "192.168.1.1",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "domain_id is required")
}

func TestLinodeDomainRecordCreateTool_MissingType(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"target":    "192.168.1.1",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "type is required")
}

func TestLinodeDomainRecordCreateTool_MissingTarget(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"type":      "A",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "target is required")
}

func TestLinodeDomainRecordCreateTool_Success(t *testing.T) {
	t.Parallel()

	record := linode.DomainRecord{
		ID:     222,
		Type:   "A",
		Name:   "www",
		Target: "203.0.113.50",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/111/records", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(record))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"type":      "A",
		"name":      "www",
		"target":    "203.0.113.50",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeDomainRecordUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	assert.Equal(t, "linode_domain_record_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain_id")
	assert.Contains(t, props, "record_id")
	assert.Contains(t, props, "target")
}

func TestLinodeDomainRecordUpdateTool_MissingDomainID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"record_id": float64(222),
		"target":    "192.168.1.2",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "domain_id is required")
}

func TestLinodeDomainRecordUpdateTool_MissingRecordID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"target":    "192.168.1.2",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "record_id is required")
}

func TestLinodeDomainRecordUpdateTool_Success(t *testing.T) {
	t.Parallel()

	record := linode.DomainRecord{
		ID:     222,
		Type:   "A",
		Name:   "www",
		Target: "192.168.1.2",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/111/records/222", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(record))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"record_id": float64(222),
		"target":    "192.168.1.2",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

func TestNewLinodeDomainRecordDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	assert.Equal(t, "linode_domain_record_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "domain_id")
	assert.Contains(t, props, "record_id")
}

func TestLinodeDomainRecordDeleteTool_MissingDomainID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"record_id": float64(222)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "domain_id is required")
}

func TestLinodeDomainRecordDeleteTool_MissingRecordID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"domain_id": float64(111)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "record_id is required")
}

func TestLinodeDomainRecordDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/domains/111/records/222", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"domain_id": float64(111),
		"record_id": float64(222),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// Volume Write Tool Tests
// =============================================================================

func TestNewLinodeVolumeCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumeCreateTool(cfg)

	assert.Equal(t, "linode_volume_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "size")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVolumeCreateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":  "data-vol",
		"region": "us-east",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVolumeCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeVolumeCreateTool_RequiresRegionOrLinodeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "data-vol",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "either region or linode_id is required")
}

func TestLinodeVolumeCreateTool_Success(t *testing.T) {
	t.Parallel()

	volume := linode.Volume{
		ID:     333,
		Label:  "data-vol",
		Region: "us-east",
		Size:   50,
		Status: "creating",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(volume))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "data-vol",
		"region":  "us-east",
		"size":    float64(50),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "data-vol")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeVolumeAttachTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumeAttachTool(cfg)

	assert.Equal(t, "linode_volume_attach", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "volume_id")
	assert.Contains(t, props, "linode_id")
	assert.Contains(t, props, "config_id")
}

func TestLinodeVolumeAttachTool_MissingVolumeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeAttachTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"linode_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "volume_id is required")
}

func TestLinodeVolumeAttachTool_MissingLinodeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeAttachTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"volume_id": float64(333)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "linode_id is required")
}

func TestLinodeVolumeAttachTool_Success(t *testing.T) {
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
		assert.Equal(t, "/volumes/333/attach", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(volume))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeAttachTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"volume_id": float64(333),
		"linode_id": float64(123),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "attached")
}

func TestNewLinodeVolumeDetachTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumeDetachTool(cfg)

	assert.Equal(t, "linode_volume_detach", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "volume_id")
}

func TestLinodeVolumeDetachTool_MissingVolumeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeDetachTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "volume_id is required")
}

func TestLinodeVolumeDetachTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes/333/detach", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeDetachTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"volume_id": float64(333)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "detached successfully")
}

func TestNewLinodeVolumeResizeTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumeResizeTool(cfg)

	assert.Equal(t, "linode_volume_resize", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "volume_id")
	assert.Contains(t, props, "size")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVolumeResizeTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"volume_id": float64(333),
		"size":      float64(100),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVolumeResizeTool_MissingVolumeID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"size":    float64(100),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "volume_id is required")
}

func TestLinodeVolumeResizeTool_MissingSize(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"volume_id": float64(333),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	// When size is 0 or missing, validation returns "size is required" or min size error.
	assertErrorContains(t, result, "size")
}

func TestLinodeVolumeResizeTool_Success(t *testing.T) {
	t.Parallel()

	volume := linode.Volume{
		ID:     333,
		Label:  "data-vol",
		Region: "us-east",
		Size:   100,
		Status: "resizing",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes/333/resize", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(volume))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeResizeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"volume_id": float64(333),
		"size":      float64(100),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "resize")
}

func TestNewLinodeVolumeDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	assert.Equal(t, "linode_volume_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "volume_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVolumeDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"volume_id": float64(333)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVolumeDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes/333", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"volume_id": float64(333),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// NodeBalancer Write Tool Tests
// =============================================================================

func TestNewLinodeNodeBalancerCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	assert.Equal(t, "linode_nodebalancer_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeNodeBalancerCreateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeNodeBalancerCreateTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"confirm": true})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeNodeBalancerCreateTool_Success(t *testing.T) {
	t.Parallel()

	nodeBalancer := linode.NodeBalancer{
		ID:     444,
		Label:  "web-lb",
		Region: "us-east",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodebalancers", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(nodeBalancer))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east",
		"label":   "web-lb",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-lb")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeNodeBalancerUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	assert.Equal(t, "linode_nodebalancer_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "nodebalancer_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "client_conn_throttle")
}

func TestLinodeNodeBalancerUpdateTool_MissingNodeBalancerID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "new-label"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "nodebalancer_id is required")
}

func TestLinodeNodeBalancerUpdateTool_Success(t *testing.T) {
	t.Parallel()

	nodeBalancer := linode.NodeBalancer{
		ID:     444,
		Label:  "updated-lb",
		Region: "us-east",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodebalancers/444", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(nodeBalancer))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"nodebalancer_id": float64(444),
		"label":           "updated-lb",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

func TestNewLinodeNodeBalancerDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	assert.Equal(t, "linode_nodebalancer_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "nodebalancer_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeNodeBalancerDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"nodebalancer_id": float64(444)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeNodeBalancerDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodebalancers/444", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"nodebalancer_id": float64(444),
		"confirm":         true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// Test Helpers
// =============================================================================

// assertErrorContains checks that the error result contains the expected substring.
func assertErrorContains(t *testing.T, result *mcp.CallToolResult, expected string) {
	t.Helper()

	require.NotEmpty(t, result.Content, "expected content in error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent type")
	assert.Contains(t, textContent.Text, expected)
}

// =============================================================================
// DNS Record Target Validation Tests
// =============================================================================

func TestValidateDNSRecordTarget_ValidPublicIPv4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
	}{
		{name: "standard public IP", target: "8.8.8.8"},
		{name: "another public IP", target: "1.1.1.1"},
		{name: "linode IP", target: "139.162.130.5"},
		{name: "high octet public", target: "203.0.113.1"},
		{name: "172 outside private range", target: "172.15.0.1"},
		{name: "172 above private range", target: "172.32.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tools.ValidateDNSRecordTarget("A", tt.target)
			assert.NoError(t, err)
		})
	}
}

func TestValidateDNSRecordTarget_PrivateIPv4Rejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
	}{
		{name: "10.x range", target: "10.0.0.1"},
		{name: "10.x high", target: "10.255.255.255"},
		{name: "192.168.x range", target: "192.168.1.1"},
		{name: "192.168.0.x", target: "192.168.0.100"},
		{name: "127 loopback", target: "127.0.0.1"},
		{name: "172.16 start of range", target: "172.16.0.1"},
		{name: "172.31 end of range", target: "172.31.255.255"},
		{name: "172.20 middle of range", target: "172.20.10.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tools.ValidateDNSRecordTarget("A", tt.target)
			assert.ErrorIs(t, err, tools.ErrDNSTargetPrivateIP)
		})
	}
}

func TestValidateDNSRecordTarget_InvalidIPv4Rejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
	}{
		{name: "octets too large", target: "999.999.999.999"},
		{name: "single octet over 255", target: "256.1.1.1"},
		{name: "not an IP", target: "not-an-ip"},
		{name: "empty octets", target: "1.2..4"},
		{name: "too few octets", target: "1.2.3"},
		{name: "IPv6 as A record", target: "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tools.ValidateDNSRecordTarget("A", tt.target)
			assert.ErrorIs(t, err, tools.ErrDNSTargetInvalidA)
		})
	}
}

func TestValidateDNSRecordTarget_ValidIPv6(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
	}{
		{name: "full address", target: "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{name: "compressed", target: "2001:db8::1"},
		{name: "loopback", target: "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tools.ValidateDNSRecordTarget("AAAA", tt.target)
			assert.NoError(t, err)
		})
	}
}

func TestValidateDNSRecordTarget_InvalidIPv6Rejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
	}{
		{name: "IPv4 as AAAA", target: "8.8.8.8"},
		{name: "not an IP", target: "not-an-ip"},
		{name: "empty", target: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.target == "" {
				err := tools.ValidateDNSRecordTarget("AAAA", tt.target)
				assert.ErrorIs(t, err, tools.ErrDNSTargetRequired)

				return
			}

			err := tools.ValidateDNSRecordTarget("AAAA", tt.target)
			assert.ErrorIs(t, err, tools.ErrDNSTargetInvalidAAAA)
		})
	}
}

func TestValidateDNSRecordTarget_EmptyTarget(t *testing.T) {
	t.Parallel()

	err := tools.ValidateDNSRecordTarget("A", "")
	assert.ErrorIs(t, err, tools.ErrDNSTargetRequired)
}
