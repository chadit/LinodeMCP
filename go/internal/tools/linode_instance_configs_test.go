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

const (
	keyRunLevel = "run_level"
	keyVirtMode = "virt_mode"
)

type instanceConfigCreateValidationCase struct {
	name         string
	args         map[string]any
	wantContains string
}

func instanceConfigCreateValidationCases() []instanceConfigCreateValidationCase {
	return []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "negative linode id", args: map[string]any{keyLinodeID: float64(-1), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "fractional linode id", args: map[string]any{keyLinodeID: float64(1.5), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "slash linode id", args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "query linode id", args: map[string]any{keyLinodeID: "123?query", keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "traversal linode id", args: map[string]any{keyLinodeID: pathTraversalValue, keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingLabel, args: map[string]any{keyLinodeID: float64(123), keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "non-string label", args: map[string]any{keyLinodeID: float64(123), keyLabel: float64(99), keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: "label must be a string"},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLinodeID: float64(123), keyLabel: "  ", keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "missing devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: "devices is required"},
		{name: "non-string devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: map[string]any{"sda": map[string]any{"disk_id": 456}}, keyConfirm: true}, wantContains: "devices must be a string"},
		{name: "null devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: databaseJSONNull, keyConfirm: true}, wantContains: "devices must be a JSON object"},
		{name: "empty devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: jsonObjectEmpty, keyConfirm: true}, wantContains: "devices must include at least one device slot"},
		{name: "invalid devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `[`, keyConfirm: true}, wantContains: "invalid devices JSON"},
		{name: "invalid device slot slash", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sd/a":{"disk_id":456}}`, keyConfirm: true}, wantContains: "device slot sd/a must be one of sda through sdh"},
		{name: "invalid device slot traversal", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"..":{"disk_id":456}}`, keyConfirm: true}, wantContains: "device slot .. must be one of sda through sdh"},
		{name: "null device", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":null}`, keyConfirm: true}, wantContains: "device sda must be an object"},
		{name: "empty device", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{}}`, keyConfirm: true}, wantContains: "device sda requires disk_id or volume_id"},
		{name: "unknown device field", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk":456}}`, keyConfirm: true}, wantContains: "invalid devices JSON"},
		{name: "invalid device id", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk_id":0}}`, keyConfirm: true}, wantContains: "disk_id must be greater than 0"},
		{name: "invalid volume id", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"volume_id":0}}`, keyConfirm: true}, wantContains: "volume_id must be greater than 0"},
		{name: "both device ids", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk_id":456,"volume_id":789}}`, keyConfirm: true}, wantContains: "can use disk_id or volume_id, not both"},
		{name: "negative memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: float64(-1), keyConfirm: true}, wantContains: "memory_limit must be an integer greater than or equal to 1"},
		{name: "fractional memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: 1.5, keyConfirm: true}, wantContains: "memory_limit must be an integer"},
		{name: "string memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: "64", keyConfirm: true}, wantContains: "memory_limit must be an integer"},
		{name: "non-string kernel", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, "kernel": float64(1), keyConfirm: true}, wantContains: "kernel must be a string"},
		{name: "non-string comments", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, "comments": float64(1), keyConfirm: true}, wantContains: "comments must be a string"},
		{name: "non-string root device", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, "root_device": float64(1), keyConfirm: true}, wantContains: "root_device must be a string"},
		{name: "non-string run level", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyRunLevel: float64(1), keyConfirm: true}, wantContains: "run_level must be a string"},
		{name: "non-string virt mode", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyVirtMode: float64(1), keyConfirm: true}, wantContains: "virt_mode must be a string"},
		{name: "non-string helpers", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyHelpers: map[string]any{}, keyConfirm: true}, wantContains: "helpers must be a string"},
		{name: "invalid helpers", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyHelpers: `[`, keyConfirm: true}, wantContains: errInvalidHelpersJSON},
		{name: "null helpers", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyHelpers: databaseJSONNull, keyConfirm: true}, wantContains: "helpers must be a JSON object"},
		{name: "unknown helpers field", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyHelpers: `{"typo":true}`, keyConfirm: true}, wantContains: errInvalidHelpersJSON},
		{name: "trailing helpers JSON", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyHelpers: `{"distro":true} {}`, keyConfirm: true}, wantContains: errInvalidHelpersJSON},
		{name: "non-string interfaces", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: []any{}, keyConfirm: true}, wantContains: "interfaces must be a string"},
		{name: "invalid interfaces", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `{`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "null interfaces", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: databaseJSONNull, keyConfirm: true}, wantContains: "interfaces must be a JSON array"},
		{name: "unknown interface field", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"public","typo":true}]`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "trailing interfaces JSON", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"public"}] {}`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "invalid interface purpose", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"bad"}]`, keyConfirm: true}, wantContains: "purpose must be public, vlan, or vpc"},
		{name: "invalid run level", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyRunLevel: "bad", keyConfirm: true}, wantContains: "run_level must be"},
		{name: "invalid virt mode", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyVirtMode: "bad", keyConfirm: true}, wantContains: "virt_mode must be"},
	}
}

func TestLinodeInstanceConfigCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyDevices, "schema should include devices")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := instanceConfigCreateValidationCases()
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

		diskID := 456
		created := linode.InstanceConfig{
			ID:    789,
			Label: labelBootConfig,
			Devices: map[string]*linode.ConfigDevice{
				"sda": {DiskID: &diskID},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var got linode.CreateConfigRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Equal(t, labelBootConfig, got.Label, "label should match")
			assert.Contains(t, got.Devices, "sda", "devices should include sda")
			assert.NotNil(t, got.Devices["sda"].DiskID, "sda disk_id should be set")
			assert.Equal(t, diskID, *got.Devices["sda"].DiskID, "disk ID should match")
			assert.Equal(t, "linode/latest-64bit", got.Kernel, "kernel should match")
			assert.Equal(t, 512, got.MemoryLimit, "memory limit should match")
			assert.Equal(t, "/dev/sda", got.RootDevice, "root device should match")
			assert.Equal(t, envKeyDefault, got.RunLevel, "run level should match")
			assert.Equal(t, "paravirt", got.VirtMode, "virtualization mode should match")

			if assert.NotNil(t, got.Helpers, "helpers should be set") && assert.NotNil(t, got.Helpers.Distro, "distro helper should be set") {
				assert.True(t, *got.Helpers.Distro, "distro helper should match")
			}

			if assert.Len(t, got.Interfaces, 1, "interfaces should include one entry") {
				assert.Equal(t, "public", got.Interfaces[0].Purpose, "interface purpose should match")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(created), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyLabel:       labelBootConfig,
			keyDevices:     configDevicesSDAJSON,
			"kernel":       "linode/latest-64bit",
			keyMemoryLimit: float64(512),
			"root_device":  "/dev/sda",
			keyRunLevel:    envKeyDefault,
			keyVirtMode:    "paravirt",
			keyHelpers:     `{"distro":true}`,
			keyInterfaces:  `[{"purpose":"public"}]`,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelBootConfig, "response should contain config label")
		assert.Contains(t, textContent.Text, "789", "response should contain config ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyLabel:    labelBootConfig,
			keyDevices:  configDevicesSDAJSON,
			keyConfirm:  true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to create configuration profile")
	})
}
