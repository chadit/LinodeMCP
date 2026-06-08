package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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

func TestLinodeInstanceConfigCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_create")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyDevices]; !ok {
		t.Errorf("props missing key %v", keyDevices)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigCreateTool(cfg)

	validationTests := instanceConfigCreateValidationCases()
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	diskID := 456
	created := linode.InstanceConfig{
		ID:    789,
		Label: labelBootConfig,
		Devices: map[string]*linode.ConfigDevice{
			configDeviceSlotSDA: {DiskID: &diskID},
		},
	}
	distro := true
	want := linode.CreateConfigRequest{
		Label:       labelBootConfig,
		Devices:     map[string]*linode.ConfigDevice{configDeviceSlotSDA: {DiskID: &diskID}},
		Kernel:      configKernelLatest,
		MemoryLimit: 512,
		RootDevice:  tcDevSda,
		RunLevel:    envKeyDefault,
		VirtMode:    "paravirt",
		Helpers:     &linode.ConfigHelpers{Distro: &distro},
		Interfaces:  []linode.ConfigInterface{{Purpose: "public"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var got linode.CreateConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %+v, want %+v", got, want)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelBootConfig) {
		t.Errorf("textContent.Text does not contain %v", labelBootConfig)
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeInstanceConfigCreateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create configuration profile") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create configuration profile")
	}
}

func TestLinodeInstanceConfigInterfaceAddToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceAddTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_add" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_add")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyInterface]; !ok {
		t.Errorf("props missing key %v", keyInterface)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfaceAddToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfaceAddTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfaceAddToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	primary := true
	created := linode.ConfigInterface{Purpose: "vpc", Primary: &primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var got linode.ConfigInterface
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Purpose != created.Purpose {
			t.Errorf("got.Purpose = %v, want %v", got.Purpose, created.Purpose)
		}

		if got.Primary == nil {
			t.Fatal("primary should be set")
		}

		if !(*got.Primary) {
			t.Error("*got.Primary = false, want true")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, purposeVPC) {
		t.Errorf("textContent.Text does not contain %v", purposeVPC)
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeInstanceConfigInterfaceAddToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces")
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to add configuration profile interface") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to add configuration profile interface")
	}
}

func TestLinodeInstanceConfigInterfaceGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyInterfaceID]; !ok {
		t.Errorf("props missing key %v", keyInterfaceID)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfaceGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfaceGetTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfaceGetToolSuccessfulGet(t *testing.T) {
	t.Parallel()

	primary := true
	gotInterface := linode.ConfigInterfaceResponse{ID: 456, Active: true, Purpose: purposeVPC, Primary: primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(gotInterface); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, purposeVPC) {
		t.Errorf("textContent.Text does not contain %v", purposeVPC)
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}

	if !strings.Contains(textContent.Text, boolStringTrue) {
		t.Errorf("textContent.Text does not contain %v", boolStringTrue)
	}
}

func TestLinodeInstanceConfigInterfaceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve configuration profile interface") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve configuration profile interface")
	}
}

func TestLinodeInstanceConfigInterfaceDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyInterfaceID]; !ok {
		t.Errorf("props missing key %v", keyInterfaceID)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfaceDeleteToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfaceDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfaceDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "removed") {
		t.Errorf("textContent.Text does not contain %v", "removed")
	}
}

func TestLinodeInstanceConfigInterfaceDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to remove configuration profile interface") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to remove configuration profile interface")
	}
}

func TestLinodeInstanceConfigUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigUpdateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigUpdateToolSuccessfulUpdate(t *testing.T) {
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
		if r.URL.Path != tcLinodeInstances123Configs789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var got linode.UpdateConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label == nil {
			t.Fatal("label should be set")
		}

		if *got.Label != labelBootConfig {
			t.Errorf("*got.Label = %v, want %v", *got.Label, labelBootConfig)
		}

		if got.Devices == nil {
			t.Fatal("devices should be set")
		}

		if _, ok := (*got.Devices)[configDeviceSlotSDA]; !ok {
			t.Errorf("*got.Devices missing key %v", configDeviceSlotSDA)
		}

		if got.Kernel == nil {
			t.Fatal("kernel should be set")
		}

		if *got.Kernel != configKernelLatest {
			t.Errorf("*got.Kernel = %v, want %v", *got.Kernel, configKernelLatest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelBootConfig) {
		t.Errorf("textContent.Text does not contain %v", labelBootConfig)
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeInstanceConfigUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update configuration profile") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update configuration profile")
	}
}

func TestLinodeInstanceConfigInterfacesReorderToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfacesReorderTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interfaces_reorder" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interfaces_reorder")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyIDs]; !ok {
		t.Errorf("props missing key %v", keyIDs)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfacesReorderToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfacesReorderTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfacesReorderToolSuccessfulReorder(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces/order" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces/order")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var got linode.ReorderConfigInterfacesRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got.IDs, []int{101, 102, 103}) {
			t.Errorf("got.IDs = %v, want %v", got.IDs, []int{101, 102, 103})
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}

	if !strings.Contains(textContent.Text, "101") {
		t.Errorf("textContent.Text does not contain %v", "101")
	}
}

func TestLinodeInstanceConfigInterfacesReorderToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces/order" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces/order")
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to reorder interfaces") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to reorder interfaces")
	}
}

func TestLinodeInstanceConfigInterfaceUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyConfigID]; !ok {
		t.Errorf("props missing key %v", keyConfigID)
	}

	if _, ok := props[keyInterfaceID]; !ok {
		t.Errorf("props missing key %v", keyInterfaceID)
	}

	if _, ok := props[keyInterface]; !ok {
		t.Errorf("props missing key %v", keyInterface)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfaceUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceConfigInterfaceUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	updated := linode.ConfigInterfaceResponse{ID: 101, Purpose: keyPublic, Primary: true, IPRanges: []string{cidrV4}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces/101" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces/101")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var got linode.UpdateConfigInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Primary == nil {
			t.Fatal("got.Primary is nil")
		}

		if got.Primary != nil {
			if !(*got.Primary) {
				t.Error("*got.Primary = false, want true")
			}
		}

		if !reflect.DeepEqual(got.IPRanges, []string{cidrV4}) {
			t.Errorf("got.IPRanges = %v, want %v", got.IPRanges, []string{cidrV4})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "101") {
		t.Errorf("textContent.Text does not contain %v", "101")
	}

	if !strings.Contains(textContent.Text, cidrV4) {
		t.Errorf("textContent.Text does not contain %v", cidrV4)
	}
}

func TestLinodeInstanceConfigInterfaceUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/789/interfaces/101" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/789/interfaces/101")
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update configuration profile interface") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update configuration profile interface")
	}
}
