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
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(1.5), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingLabel, args: map[string]any{keyLinodeID: float64(123), keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "non-string label", args: map[string]any{keyLinodeID: float64(123), keyLabel: float64(99), keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: "label must be a string"},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLinodeID: float64(123), keyLabel: "  ", keyDevices: configDevicesSDAJSON, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "missing devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: "devices is required"},
		{name: "non-string devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: map[string]any{configDeviceSlotSDA: map[string]any{keyDiskID: 456}}, keyConfirm: true}, wantContains: "devices must be a string"},
		{name: "null devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: databaseJSONNull, keyConfirm: true}, wantContains: "devices must be a JSON object"},
		{name: "empty devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: jsonObjectEmpty, keyConfirm: true}, wantContains: "devices must include at least one device slot"},
		{name: "invalid devices", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `[`, keyConfirm: true}, wantContains: errInvalidDevicesJSON},
		{name: "invalid device slot slash", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sd/a":{"disk_id":456}}`, keyConfirm: true}, wantContains: "device slot sd/a must be one of sda through sdh"},
		{name: "invalid device slot traversal", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"..":{"disk_id":456}}`, keyConfirm: true}, wantContains: "device slot .. must be one of sda through sdh"},
		{name: "null device", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":null}`, keyConfirm: true}, wantContains: "device sda must be an object"},
		{name: "empty device", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{}}`, keyConfirm: true}, wantContains: "device sda requires disk_id or volume_id"},
		{name: "unknown device field", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk":456}}`, keyConfirm: true}, wantContains: errInvalidDevicesJSON},
		{name: "invalid device id", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk_id":0}}`, keyConfirm: true}, wantContains: "disk_id must be greater than 0"},
		{name: "invalid volume id", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"volume_id":0}}`, keyConfirm: true}, wantContains: "volume_id must be greater than 0"},
		{name: "both device ids", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: `{"sda":{"disk_id":456,"volume_id":789}}`, keyConfirm: true}, wantContains: "can use disk_id or volume_id, not both"},
		{name: "negative memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: float64(-1), keyConfirm: true}, wantContains: "memory_limit must be an integer greater than or equal to 1"},
		{name: "fractional memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: 1.5, keyConfirm: true}, wantContains: "memory_limit must be an integer"},
		{name: "string memory limit", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyMemoryLimit: "64", keyConfirm: true}, wantContains: "memory_limit must be an integer"},
		{name: "non-string kernel", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyKernel: float64(1), keyConfirm: true}, wantContains: "kernel must be a string"},
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
		{name: caseUnknownInterfaceField, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"public","typo":true}]`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "read-only interface id", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"id":101,"purpose":"public"}]`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "trailing interfaces JSON", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"public"}] {}`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: caseInvalidInterfacePurpose, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyInterfaces: `[{"purpose":"bad"}]`, keyConfirm: true}, wantContains: "purpose must be public, vlan, or vpc"},
		{name: "invalid run level", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyRunLevel: stageBeta, keyConfirm: true}, wantContains: "run_level must be"},
		{name: "invalid virt mode", args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyDevices: configDevicesSDAJSON, keyVirtMode: stageBeta, keyConfirm: true}, wantContains: "virt_mode must be"},
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
				configDeviceSlotSDA: {DiskID: &diskID},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var got linode.CreateConfigRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Equal(t, labelBootConfig, got.Label, "label should match")
			assert.Contains(t, got.Devices, configDeviceSlotSDA, "devices should include sda")
			assert.NotNil(t, got.Devices[configDeviceSlotSDA].DiskID, "sda disk_id should be set")
			assert.Equal(t, diskID, *got.Devices[configDeviceSlotSDA].DiskID, "disk ID should match")
			assert.Equal(t, configKernelLatest, got.Kernel, "kernel should match")
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
			keyKernel:      configKernelLatest,
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

func TestLinodeInstanceConfigInterfaceAddTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceAddTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interface_add", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyInterface, "schema should include interface")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: configInterfacePublicJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1), keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: "invalid config id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0), keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: paymentMethodIDSlash, keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: shareGroupIDQueryValue, keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyInterface: configInterfacePublicJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: []any{}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: caseUnknownInterfaceField, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: `{"purpose":"public","typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: "trailing interface JSON", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: `{"purpose":"public"} {}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseInvalidInterfacePurpose, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: `{"purpose":"bad"}`, keyConfirm: true}, wantContains: "interface.purpose must be public, vlan, or vpc"},
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

		primary := true
		created := linode.ConfigInterface{Purpose: "vpc", Primary: &primary}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var got linode.ConfigInterface
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Equal(t, created.Purpose, got.Purpose, "purpose should match")

			if assert.NotNil(t, got.Primary, "primary should be set") {
				assert.True(t, *got.Primary, "primary should match")
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
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceAddTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:  float64(123),
			keyConfigID:  float64(789),
			keyInterface: `{"purpose":"vpc","primary":true}`,
			keyConfirm:   true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, purposeVPC, "response should contain interface purpose")
		assert.Contains(t, textContent.Text, "789", "response should contain config ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceAddTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:  float64(123),
			keyConfigID:  float64(789),
			keyInterface: configInterfacePublicJSON,
			keyConfirm:   true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to add configuration profile interface")
	})
}

func TestLinodeInstanceConfigInterfaceGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interface_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "capability should be read")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyInterfaceID, "schema should include interface_id")
		assert.NotContains(t, props, keyConfirm, "read-only tool should not require confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: shareGroupIDQueryValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789)}, wantContains: errInterfaceIDPositive},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(-1)}, wantContains: errInterfaceIDPositive},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathSeparatorValue}, wantContains: errInterfaceIDPositive},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: shareGroupIDQueryValue}, wantContains: errInterfaceIDPositive},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathTraversalValue}, wantContains: errInterfaceIDPositive},
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

	t.Run("successful get", func(t *testing.T) {
		t.Parallel()

		primary := true
		gotInterface := linode.ConfigInterfaceResponse{ID: 456, Active: true, Purpose: purposeVPC, Primary: primary}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Empty(t, r.URL.RawQuery, "request should not include query params")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(gotInterface), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(789),
			keyInterfaceID: float64(456),
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, purposeVPC, "response should contain interface purpose")
		assert.Contains(t, textContent.Text, "456", "response should contain interface ID")
		assert.Contains(t, textContent.Text, "true", "response should contain active flag")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(789),
			keyInterfaceID: float64(456),
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve configuration profile interface")
	})
}

func TestLinodeInstanceConfigInterfaceDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interface_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "capability should be destroy")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyInterfaceID, "schema should include interface_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

			args := map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(456)}
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

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDPositive},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: shareGroupIDQueryValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errConfigIDPositive},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errInterfaceIDPositive},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errInterfaceIDPositive},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: shareGroupIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errInterfaceIDPositive},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errInterfaceIDPositive},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Empty(t, r.URL.RawQuery, "request should not include query params")
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr, "writing empty response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed", "response should report interface removal")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/456", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to remove configuration profile interface")
	})
}

func TestLinodeInstanceConfigUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyLabel: labelBootConfig}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyLabel: labelBootConfig, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseNoUpdateFields, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true}, wantContains: "at least one configuration field must be provided"},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyLabel: "  ", keyConfirm: true}, wantContains: errLabelRequired},
		{name: "null devices", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: databaseJSONNull, keyConfirm: true}, wantContains: "devices must be a JSON object"},
		{name: "empty devices", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: jsonObjectEmpty, keyConfirm: true}, wantContains: "devices must include at least one device slot"},
		{name: "invalid devices", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `[`, keyConfirm: true}, wantContains: errInvalidDevicesJSON},
		{name: "invalid device slot traversal", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"..":{"disk_id":456}}`, keyConfirm: true}, wantContains: "device slot .. must be one of sda through sdh"},
		{name: "null device", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":null}`, keyConfirm: true}, wantContains: "device sda must be an object"},
		{name: "empty device", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":{}}`, keyConfirm: true}, wantContains: "device sda requires disk_id or volume_id"},
		{name: "unknown device field", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":{"disk":456}}`, keyConfirm: true}, wantContains: errInvalidDevicesJSON},
		{name: "invalid device id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":{"disk_id":0}}`, keyConfirm: true}, wantContains: "disk_id must be greater than 0"},
		{name: "invalid volume id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":{"volume_id":0}}`, keyConfirm: true}, wantContains: "volume_id must be greater than 0"},
		{name: "both device ids", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyDevices: `{"sda":{"disk_id":456,"volume_id":789}}`, keyConfirm: true}, wantContains: "can use disk_id or volume_id, not both"},
		{name: "null helpers", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyHelpers: databaseJSONNull, keyConfirm: true}, wantContains: "helpers must be a JSON object"},
		{name: "unknown helpers field", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyHelpers: `{"typo":true}`, keyConfirm: true}, wantContains: errInvalidHelpersJSON},
		{name: "trailing helpers JSON", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyHelpers: `{"distro":true} {}`, keyConfirm: true}, wantContains: errInvalidHelpersJSON},
		{name: "null interfaces", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaces: databaseJSONNull, keyConfirm: true}, wantContains: "interfaces must be a JSON array"},
		{name: caseUnknownInterfaceField, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaces: `[{"purpose":"public","typo":true}]`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "read-only interface id update", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaces: `[{"id":101,"purpose":"public"}]`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: "trailing interfaces JSON", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaces: `[{"purpose":"public"}] {}`, keyConfirm: true}, wantContains: errInvalidInterfacesJSON},
		{name: caseInvalidInterfacePurpose, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaces: `[{"purpose":"bad"}]`, keyConfirm: true}, wantContains: "purpose must be public, vlan, or vpc"},
		{name: "invalid run level", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyRunLevel: stageBeta, keyConfirm: true}, wantContains: "run_level must be"},
		{name: "invalid virt mode", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyVirtMode: stageBeta, keyConfirm: true}, wantContains: "virt_mode must be"},
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

		diskID := 456
		updated := linode.InstanceConfig{
			ID:    789,
			Label: labelBootConfig,
			Devices: map[string]*linode.ConfigDevice{
				configDeviceSlotSDA: {DiskID: &diskID},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

			var got linode.UpdateConfigRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

			if assert.NotNil(t, got.Label, "label should be set") {
				assert.Equal(t, labelBootConfig, *got.Label, "label should match")
			}

			if assert.NotNil(t, got.Devices, "devices should be set") {
				assert.Contains(t, *got.Devices, configDeviceSlotSDA, "devices should include sda")
			}

			if assert.NotNil(t, got.Kernel, "kernel should be set") {
				assert.Equal(t, configKernelLatest, *got.Kernel, "kernel should match")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(updated), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(789),
			keyLabel:    labelBootConfig,
			keyDevices:  configDevicesSDAJSON,
			keyKernel:   configKernelLatest,
			keyConfirm:  true,
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
			assert.Equal(t, "/linode/instances/123/configs/789", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(789),
			keyLabel:    labelBootConfig,
			keyConfirm:  true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update configuration profile")
	})
}

func TestLinodeInstanceConfigInterfacesReorderTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfacesReorderTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interfaces_reorder", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyIDs, "schema should include ids")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errConfigIDRequired},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyIDs: singleInterfaceIDsJSON, keyConfirm: true}, wantContains: errConfigIDInteger},
		{name: "missing ids", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true}, wantContains: "ids is required"},
		{name: "non-string ids", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: []any{float64(101)}, keyConfirm: true}, wantContains: "ids must be a string"},
		{name: "empty ids", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: databaseJSONArray, keyConfirm: true}, wantContains: "ids must include at least one interface ID"},
		{name: "invalid ids", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: "[", keyConfirm: true}, wantContains: "ids must be a JSON array"},
		{name: "zero id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: "[0]", keyConfirm: true}, wantContains: "ids must contain only positive integer"},
		{name: "duplicate id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: "[101,101]", keyConfirm: true}, wantContains: "ids must not contain duplicate interface IDs"},
		{name: "string id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyIDs: `["101"]`, keyConfirm: true}, wantContains: "ids must be a JSON array"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful reorder", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/order", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var got linode.ReorderConfigInterfacesRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Equal(t, []int{101, 102, 103}, got.IDs, "ids should match")

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr, "writing empty response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesReorderTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(789),
			keyIDs:      "[101,102,103]",
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "789", "response should contain config ID")
		assert.Contains(t, textContent.Text, "101", "response should contain interface ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/order", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesReorderTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(789),
			keyIDs:      singleInterfaceIDsJSON,
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to reorder interfaces")
	})
}

func TestLinodeInstanceConfigInterfaceUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_config_interface_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyConfigID, "schema should include config_id")
		assert.Contains(t, props, keyInterfaceID, "schema should include interface_id")
		assert.Contains(t, props, keyInterface, "schema should include interface")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: paymentMethodIDSlash, keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: shareGroupIDQueryValue, keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyInterfaceID: float64(101), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errConfigIDPositive},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: paymentMethodIDSlash, keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: shareGroupIDQueryValue, keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathTraversalValue, keyInterface: interfacePrimaryJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: []any{}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: caseNoUpdateFields, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: jsonObjectEmpty, keyConfirm: true}, wantContains: "at least one interface update field"},
		{name: caseUnknownInterfaceField, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(101), keyInterface: `{"primary":true,"typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
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

		updated := linode.ConfigInterfaceResponse{ID: 101, Purpose: keyPublic, Primary: true, IPRanges: []string{"10.0.0.0/24"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/101", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

			var got linode.UpdateConfigInterfaceRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.NotNil(t, got.Primary, "primary should be set")

			if got.Primary != nil {
				assert.True(t, *got.Primary, "primary should match")
			}

			assert.Equal(t, []string{"10.0.0.0/24"}, got.IPRanges, "IP ranges should match")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(updated), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(789),
			keyInterfaceID: float64(101),
			keyInterface:   `{"primary":true,"ip_ranges":["10.0.0.0/24"]}`,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "101", "response should contain interface ID")
		assert.Contains(t, textContent.Text, "10.0.0.0/24", "response should contain IP range")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/configs/789/interfaces/101", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(789),
			keyInterfaceID: float64(101),
			keyInterface:   interfacePrimaryJSON,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update configuration profile interface")
	})
}
