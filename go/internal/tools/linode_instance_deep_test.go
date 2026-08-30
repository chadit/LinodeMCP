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

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// TestLinodeInstanceBackupsListTool verifies the instance backups list tool
// registers correctly, validates linode_id, and returns backup data.
func TestLinodeInstanceBackupsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceBackupListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backup_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backup_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceBackupsListToolCaseMissingLinodeID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceBackupListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLinodeIDPositive) {
		t.Errorf("error text %q does not contain %q", text.Text, errLinodeIDPositive)
	}
}

func TestLinodeInstanceBackupsListToolSuccess(t *testing.T) {
	t.Parallel()

	backupsResp := InstanceBackupsResponse{
		Automatic: []InstanceBackup{
			{ID: 100, Label: "auto-2024-01-01", Status: statusSuccessful, Type: "auto"},
		},
		Snapshot: InstanceBackupSnapshots{
			Current: &InstanceBackup{ID: 200, Label: "my-snapshot", Status: statusSuccessful, Type: wordSnapshot},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(backupsResp); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceBackupListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123"})

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

	if !strings.Contains(textContent.Text, "auto-2024-01-01") {
		t.Errorf("textContent.Text does not contain %v", "auto-2024-01-01")
	}

	if !strings.Contains(textContent.Text, "my-snapshot") {
		t.Errorf("textContent.Text does not contain %v", "my-snapshot")
	}
}

// TestLinodeInstanceConfigsListTool verifies the instance configuration profile list tool
// registers correctly, validates inputs, and returns configuration profile data.
func TestLinodeInstanceConfigsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceConfigListTool(cfg)

	t.Parallel()

	if tool.Name != toolLinodeInstanceConfigList {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeInstanceConfigList)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyLinodeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeID)
	}

	if !strings.Contains(rawSchema, "page") {
		t.Errorf("tool.RawInputSchema missing key %v", "page")
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("tool.RawInputSchema missing key %v", keyPageSize)
	}
}

func TestLinodeInstanceConfigsListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceConfigListTool(cfg)

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

func TestLinodeInstanceConfigsListToolSuccess(t *testing.T) {
	t.Parallel()

	configs := []InstanceConfig{
		{ID: 77, Label: "boot-config", Kernel: configKernelLatest, RootDevice: "/dev/sda"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "50" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: configs, keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})

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

	if !strings.Contains(textContent.Text, "boot-config") {
		t.Errorf("textContent.Text does not contain %v", "boot-config")
	}

	if !strings.Contains(textContent.Text, configKernelLatest) {
		t.Errorf("textContent.Text does not contain %v", configKernelLatest)
	}
}

func TestLinodeInstanceConfigsListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}
}

// TestLinodeInstanceConfigInterfacesListTool verifies the configuration profile
// interfaces list tool registers correctly, validates inputs, and returns interface data.
func TestLinodeInstanceConfigInterfacesListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceConfigInterfaceListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_list")
	}

	const wantDescription = "Lists interfaces assigned to a specific configuration profile on a Linode instance."
	if tool.Description != wantDescription {
		t.Errorf("tool.Description = %q, want %q", tool.Description, wantDescription)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	var schema map[string]any
	if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
		t.Fatalf("unmarshal tool.RawInputSchema: %v", err)
	}

	if schema["additionalProperties"] != false {
		t.Errorf("schema additionalProperties = %v, want false", schema["additionalProperties"])
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %T, want object", schema["properties"])
	}

	if len(properties) != 3 {
		t.Errorf("schema properties = %v, want only environment, linode_id, and config_id", properties)
	}

	for _, key := range []string{keyLinodeID, keyConfigID} {
		property, ok := properties[key].(map[string]any)
		if !ok {
			t.Errorf("schema properties missing object key %v", key)

			continue
		}

		if property["type"] != schemaTypeInteger {
			t.Errorf("schema.Properties[%q].type = %v, want integer", key, property["type"])
		}

		required, requiredOK := schema["required"].([]any)
		if !requiredOK || !slices.Contains(required, any(key)) {
			t.Errorf("schema.Required missing key %v", key)
		}
	}
}

func TestLinodeInstanceConfigInterfacesListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceConfigInterfaceListTool(cfg)

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
		{name: caseZeroLinodeID, args: map[string]any{keyLinodeID: float64(0), keyConfigID: float64(456)}, wantContains: errLinodeIDMin},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-123), keyConfigID: float64(456)}, wantContains: errLinodeIDMin},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue}, wantContains: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue}, wantContains: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue}, wantContains: errConfigIDInteger},
		{name: caseZeroConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0)}, wantContains: errConfigIDMin},
		{name: caseNegativeConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(-456)}, wantContains: errConfigIDMin},
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

func TestLinodeInstanceConfigInterfacesListToolSuccess(t *testing.T) {
	t.Parallel()

	interfaces := []ConfigInterfaceResponse{
		{ID: 202, Active: true, Purpose: purposeVPC},
		{ID: 101, Active: true, Purpose: keyPublic},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/configs/456/interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/456/interfaces")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(interfaces); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigInterfaceListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})

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

	var output struct {
		Interfaces []struct {
			Purpose string `json:"purpose"`
			ID      int    `json:"id"`
		} `json:"interfaces"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &output); err != nil {
		t.Fatalf("unmarshal tool output: %v", err)
	}

	if output.Count != 2 {
		t.Errorf("output.Count = %v, want %v", output.Count, 2)
	}

	if len(output.Interfaces) != 2 {
		t.Fatalf("len(output.Interfaces) = %v, want %v", len(output.Interfaces), 2)
	}

	if output.Interfaces[0].ID != 202 || output.Interfaces[1].ID != 101 {
		t.Errorf("output interface order = [%d, %d], want [202, 101]", output.Interfaces[0].ID, output.Interfaces[1].ID)
	}

	if output.Interfaces[0].Purpose != purposeVPC || output.Interfaces[1].Purpose != keyPublic {
		t.Errorf("output purposes = [%q, %q], want [%q, %q]", output.Interfaces[0].Purpose, output.Interfaces[1].Purpose, purposeVPC, keyPublic)
	}
}

func TestLinodeInstanceConfigInterfacesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigInterfaceListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}
}

// TestLinodeInstanceBackupGetTool verifies the instance backup get tool
// registers correctly, validates required fields, and retrieves backup details.
func TestLinodeInstanceBackupGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceBackupGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backup_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backup_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceBackupGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceBackupGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyBackupID: "100"}, wantContains: errLinodeIDPositive},
		{name: "missing backup id", args: map[string]any{keyLinodeID: "123"}, wantContains: errBackupIDPositive},
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

func TestLinodeInstanceBackupGetToolSuccess(t *testing.T) {
	t.Parallel()

	backup := InstanceBackup{ID: 100, Label: "my-backup", Status: statusSuccessful, Type: wordSnapshot}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups/100" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups/100")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(backup); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceBackupGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyBackupID: "100"})

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

	if !strings.Contains(textContent.Text, "my-backup") {
		t.Errorf("textContent.Text does not contain %v", "my-backup")
	}

	if !strings.Contains(textContent.Text, statusSuccessful) {
		t.Errorf("textContent.Text does not contain %v", statusSuccessful)
	}
}

// TestLinodeInstanceConfigGetTool verifies the instance config get tool
// registers correctly, validates required fields, and retrieves config details.
func TestLinodeInstanceConfigGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceConfigGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceConfigGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceConfigGetTool(cfg)

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

func TestLinodeInstanceConfigGetToolSuccess(t *testing.T) {
	t.Parallel()

	configProfile := map[string]any{
		keyBetaID:     float64(456),
		keyLabel:      "boot-config",
		keyKernel:     configKernelLatest,
		keyNotInProto: valNotInProto,
		"devices": map[string]any{
			configDeviceSlotSDA: map[string]any{"disk_id": float64(10)},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/456")
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(configProfile); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})

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

	if !strings.Contains(textContent.Text, "boot-config") {
		t.Errorf("textContent.Text does not contain %v", "boot-config")
	}

	if !strings.Contains(textContent.Text, configKernelLatest) {
		t.Errorf("textContent.Text does not contain %v", configKernelLatest)
	}

	if strings.Contains(textContent.Text, "not_in_proto") {
		t.Errorf("textContent.Text unexpectedly contains dropped unknown field: %v", textContent.Text)
	}
}

func TestLinodeInstanceConfigGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/configs/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/configs/456")
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(456)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve config 456 for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve config 456 for instance 123")
	}
}

// TestLinodeInstanceBackupsCancelTool verifies the instance backups cancel tool
// registers correctly, validates required fields, and cancels the backup service.
func TestLinodeInstanceBackupsCancelToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceBackupsCancelTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backups_cancel" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backups_cancel")
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
}

func TestLinodeInstanceBackupsCancelToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceBackupsCancelTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
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

func TestLinodeInstanceBackupsCancelToolSuccessfulCancel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups/cancel" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups/cancel")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceBackupsCancelTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "Backup service canceled") {
		t.Errorf("textContent.Text does not contain %v", "Backup service canceled")
	}
}

// Dry-run coverage for instance backups cancel (POST action, WithID).
// The cancel is a POST, so would_execute.method must be POST and the
// fetch hits the instance, never mutating.
func TestLinodeInstanceBackupsCancelToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := gentools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstanceBackupsCancelToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	instanceBody := `{"id":123,"label":"web-01","status":"running","backups":{"enabled":true}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != instanceGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceGetPath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if _, writeErr := w.Write([]byte(instanceBody)); writeErr != nil {
				t.Errorf("w.Write() error = %v", writeErr)
			}

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeInstanceBackupsCancelTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyDryRun:   true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_backups_cancel") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_backups_cancel")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances/123/backups/cancel") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances/123/backups/cancel")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeInstanceBackupsCancelToolDryRunStillValidatesLinodeId(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLinodeIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errLinodeIDRequired)
	}
}

// TestLinodeInstanceDisksListTool verifies the instance disks list tool
// registers correctly, validates linode_id, and returns disk data.
func TestLinodeInstanceDisksListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceDiskListTool(cfg)

	t.Parallel()

	if tool.Name != toolInstanceDiskList {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolInstanceDiskList)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceDisksListToolCaseMissingLinodeID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceDiskListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLinodeIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errLinodeIDRequired)
	}
}

func TestLinodeInstanceDisksListToolSuccess(t *testing.T) {
	t.Parallel()

	disks := []InstanceDisk{
		{ID: 10, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady},
		{ID: 11, Label: "512 MB Swap Image", Size: 512, Filesystem: "swap", Status: statusReady},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Disks {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Disks)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: disks, keyPage: 1, keyPages: 1, keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceDiskListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if !strings.Contains(textContent.Text, imageUbuntu2404) {
		t.Errorf("textContent.Text does not contain %v", imageUbuntu2404)
	}

	if !strings.Contains(textContent.Text, "512 MB Swap Image") {
		t.Errorf("textContent.Text does not contain %v", "512 MB Swap Image")
	}
}

// TestLinodeInstanceDiskGetTool verifies the instance disk get tool
// registers correctly, validates required fields, and retrieves disk details.
func TestLinodeInstanceDiskGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceDiskGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceDiskGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceDiskGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyDiskID: float64(10)}, wantContains: errLinodeIDPositive},
		{name: "missing disk id", args: map[string]any{keyLinodeID: float64(123)}, wantContains: errDiskIDPositive},
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

func TestLinodeInstanceDiskGetToolSuccess(t *testing.T) {
	t.Parallel()

	disk := InstanceDisk{ID: 10, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Disks10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Disks10)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(disk); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceDiskGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})

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

	if !strings.Contains(textContent.Text, imageUbuntu2404) {
		t.Errorf("textContent.Text does not contain %v", imageUbuntu2404)
	}

	if !strings.Contains(textContent.Text, filesystemExt4) {
		t.Errorf("textContent.Text does not contain %v", filesystemExt4)
	}
}

// TestLinodeInstanceIPsListTool verifies the instance IPs list tool
// registers correctly, validates linode_id, and returns IP address data.
func TestLinodeInstanceIPsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceIPListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_ip_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_ip_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceIPsListToolCaseMissingLinodeID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceIPListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLinodeIDPositive) {
		t.Errorf("error text %q does not contain %q", text.Text, errLinodeIDPositive)
	}
}

func TestLinodeInstanceIPsListToolSuccess(t *testing.T) {
	t.Parallel()

	ips := InstanceIPAddresses{
		IPv4: &InstanceIPv4{
			Public: []IPAddress{
				{Address: testNetIPv4AddressOne, Public: true, Type: keyIPv4, Region: regionUSEast},
			},
			Private: []IPAddress{
				{Address: privateIPv4AddressOne, Public: false, Type: keyIPv4, Region: regionUSEast},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(ips); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceIPListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if !strings.Contains(textContent.Text, testNetIPv4AddressOne) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressOne)
	}

	if !strings.Contains(textContent.Text, privateIPv4AddressOne) {
		t.Errorf("textContent.Text does not contain %v", privateIPv4AddressOne)
	}
}

// TestLinodeInstanceIPGetTool verifies the instance IP get tool
// registers correctly, validates required fields, and retrieves IP details.
func TestLinodeInstanceIPGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeInstanceIPGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_ip_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_ip_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceIPGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceIPGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyAddress: testNetIPv4AddressOne}, wantContains: errLinodeIDPositive},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errAddressRequired},
		{name: caseMalformedAddress, args: map[string]any{keyLinodeID: float64(123), keyAddress: testInvalidIPValue}, wantContains: errAddressValidIP},
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

func TestLinodeInstanceIPGetToolSuccess(t *testing.T) {
	t.Parallel()

	ipAddr := IPAddress{
		Address: testNetIPv4AddressOne, Gateway: "203.0.113.0", SubnetMask: subnetMaskFixture,
		Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips20301131 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips20301131)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(ipAddr); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceIPGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne})

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

	if !strings.Contains(textContent.Text, testNetIPv4AddressOne) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressOne)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeInstanceVolumesListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceVolumeListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_volume_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_volume_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyLinodeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeID)
	}

	if !strings.Contains(string(tool.RawInputSchema), "page") {
		t.Errorf("tool.RawInputSchema missing key %v", "page")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyPageSize) {
		t.Errorf("tool.RawInputSchema missing key %v", keyPageSize)
	}
}

func TestLinodeInstanceVolumesListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceVolumeListTool(cfg)

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
		{name: caseInvalidPageSizeLow, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(10)}, wantContains: errStandardPageSizeRange},
		{name: caseInvalidPageSizeHigh, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(501)}, wantContains: errStandardPageSizeRange},
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

func TestLinodeInstanceVolumesListToolSuccess(t *testing.T) {
	t.Parallel()

	volumes := []Volume{
		{ID: 321, Label: "data-volume", Status: statusActive, Size: 50, Region: regionUSEast},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/volumes" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/volumes")
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "50" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: volumes, keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceVolumeListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})

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

	if !strings.Contains(textContent.Text, "data-volume") {
		t.Errorf("textContent.Text does not contain %v", "data-volume")
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeInstanceVolumesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceVolumeListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// TestLinodeInstanceIPAllocateToolPublicPresence pins the two refusals the
// public switch owes: it has no default, so an omitted one is a refusal rather
// than a silent false, and a value of the wrong type says so.
