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
		{name: caseMissingLabel, args: map[string]any{"ssh_key": validTestSSHKey}, wantContains: errLabelRequired},
		{name: "missing ssh key", args: map[string]any{keyLabel: keyNameTest}, wantContains: "ssh_key is required"},
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
			keyLabel:  keyNameTest,
			"ssh_key": validTestSSHKey,
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeSSHKeyDeleteTool(successCfg)

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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceBootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123)})
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceRebootTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123)})
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeInstanceShutdownTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123)})
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
			name:         "missing instance id",
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
		req := createRequestWithArgs(t, map[string]any{})
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
		req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew})
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
		{name: "missing domain", args: map[string]any{keyType: keyMaster}, wantContains: "domain is required"},
		{name: caseMissingType, args: map[string]any{"domain": domainExample}, wantContains: errTypeRequired},
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
			"domain":    domainExample,
			keyType:     keyMaster,
			keySoaEmail: "admin@example.com",
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
		req := createRequestWithArgs(t, map[string]any{keySoaEmail: "new@example.com"})
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
			args:         map[string]any{keyType: "A", keyTarget: ip192168_1_1},
			wantContains: errDomainIDRequired,
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyDomainID: float64(111), keyTarget: ip192168_1_1},
			wantContains: errTypeRequired,
		},
		{
			name:         "missing target",
			args:         map[string]any{keyDomainID: float64(111), keyType: "A"},
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
			args:         map[string]any{keyRecordID: float64(222), keyTarget: ip192168_1_2},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111), keyTarget: ip192168_1_2},
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
			args:         map[string]any{keyRecordID: float64(222)},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111)},
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
			args:         map[string]any{keyLinodeID: float64(123)},
			wantContains: "volume_id is required",
		},
		{
			name:         caseMissingLinodeID,
			args:         map[string]any{keyVolumeID: float64(333)},
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
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeVolumeDetachTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333)})
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
		req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew})
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
