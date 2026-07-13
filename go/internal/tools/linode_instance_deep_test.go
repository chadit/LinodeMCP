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
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestLinodeInstanceBackupsListTool verifies the instance backups list tool
// registers correctly, validates linode_id, and returns backup data.
func TestLinodeInstanceBackupsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupListTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceBackupListTool(cfg)

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

func TestLinodeInstanceBackupsListToolSuccess(t *testing.T) {
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
	_, _, srvHandler := tools.NewLinodeInstanceBackupListTool(srvCfg)

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
	tool, capability, handler := tools.NewLinodeInstanceConfigListTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceConfigListTool(cfg)

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

	configs := []linode.InstanceConfig{
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
	_, _, srvHandler := tools.NewLinodeInstanceConfigListTool(srvCfg)

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
	_, _, srvHandler := tools.NewLinodeInstanceConfigListTool(srvCfg)

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

// TestLinodeInstanceConfigDeleteTool verifies the instance configuration profile delete tool
// registers correctly, validates confirm, and deletes configuration profiles.
func TestLinodeInstanceConfigDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceConfigDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyLinodeID, keyConfigID, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigDeleteToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigDeleteTool(cfg)

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

func TestLinodeInstanceConfigDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceConfigDeleteTool(cfg)

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, want: tools.ErrConfigIDRequired.Error()},
		{name: caseSeparatorConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: "789/..", keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: "zero config id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0), keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDMin},
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.want) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.want)
			}
		})
	}
}

func TestLinodeInstanceConfigDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceConfigDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "deleted") {
		t.Errorf("error text %q does not contain %q", text.Text, "deleted")
	}
}

func TestLinodeInstanceConfigDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		_, err := w.Write([]byte(`{"errors":[{"reason":"locked"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceConfigDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to remove configuration profile 789 from instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to remove configuration profile 789 from instance 123")
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
	tool, capability, handler := tools.NewLinodeInstanceConfigInterfacesListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_list")
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
	for _, key := range []string{keyLinodeID, keyConfigID} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
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
	_, _, handler := tools.NewLinodeInstanceConfigInterfacesListTool(cfg)

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

	interfaces := []linode.ConfigInterfaceResponse{{ID: 101, Active: true, Purpose: keyPublic}}

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
	_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesListTool(srvCfg)

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

	if !strings.Contains(textContent.Text, keyPublic) {
		t.Errorf("textContent.Text does not contain %v", keyPublic)
	}

	if !strings.Contains(textContent.Text, "101") {
		t.Errorf("textContent.Text does not contain %v", "101")
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
	_, _, srvHandler := tools.NewLinodeInstanceConfigInterfacesListTool(srvCfg)

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
	tool, _, handler := tools.NewLinodeInstanceBackupGetTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceBackupGetTool(cfg)

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

	backup := linode.InstanceBackup{ID: 100, Label: "my-backup", Status: statusSuccessful, Type: wordSnapshot}

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
	_, _, srvHandler := tools.NewLinodeInstanceBackupGetTool(srvCfg)

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
	tool, _, handler := tools.NewLinodeInstanceConfigGetTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceConfigGetTool(cfg)

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
	_, _, srvHandler := tools.NewLinodeInstanceConfigGetTool(srvCfg)

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
	_, _, srvHandler := tools.NewLinodeInstanceConfigGetTool(srvCfg)

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

// End-to-end verification of instance backup creation workflow.
func TestLinodeInstanceBackupCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backup_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backup_create")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceBackupCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceBackupCreateTool(cfg)

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

func TestLinodeInstanceBackupCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	backup := linode.InstanceBackup{ID: 300, Label: "snapshot-manual", Status: "pending", Type: wordSnapshot}

	const requestLabel = "nightly-snap"

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
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
	_, _, srvHandler := tools.NewLinodeInstanceBackupCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyLabel: requestLabel, keyConfirm: true})

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

	if capturedBody["label"] != requestLabel {
		t.Errorf("capturedBody[label] = %v, want %v", capturedBody["label"], requestLabel)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Snapshot created") {
		t.Errorf("textContent.Text does not contain %v", "Snapshot created")
	}

	if !strings.Contains(textContent.Text, "300") {
		t.Errorf("textContent.Text does not contain %v", "300")
	}
}

// End-to-end verification of instance backup restore workflow.
func TestLinodeInstanceBackupRestoreToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backup_restore" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backup_restore")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "backup_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "backup_id")
	}

	if !strings.Contains(rawSchema, "target_linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "target_linode_id")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceBackupRestoreToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceBackupRestoreTool(cfg)

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

func TestLinodeInstanceBackupRestoreToolSuccessfulRestore(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups/100/restore" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups/100/restore")
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
	_, _, srvHandler := tools.NewLinodeInstanceBackupRestoreTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: "123", keyBackupID: "100", keyTargetLinodeID: float64(456), keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "restore initiated") {
		t.Errorf("textContent.Text does not contain %v", "restore initiated")
	}
}

// TestLinodeInstanceBackupsEnableTool verifies the instance backups enable tool
// registers correctly, validates required fields, and enables the backup service.
func TestLinodeInstanceBackupsEnableToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_backups_enable" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_backups_enable")
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

func TestLinodeInstanceBackupsEnableToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceBackupsEnableTool(cfg)

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

func TestLinodeInstanceBackupsEnableToolSuccessfulEnable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/backups/enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/backups/enable")
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
	_, _, srvHandler := tools.NewLinodeInstanceBackupsEnableTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Backup service enabled") {
		t.Errorf("textContent.Text does not contain %v", "Backup service enabled")
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
	tool, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: "123"}, wantContains: errConfirmEqualsTrue},
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
	_, _, srvHandler := tools.NewLinodeInstanceBackupsCancelTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: "123", keyConfirm: true, keyConfirmedDryRun: true})

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

	tool, _, _ := tools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
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
	_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(cfg)

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

	_, _, handler := tools.NewLinodeInstanceBackupsCancelTool(&config.Config{})
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
	tool, _, handler := tools.NewLinodeInstanceDiskListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_list")
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
	_, _, handler := tools.NewLinodeInstanceDiskListTool(cfg)

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

	disks := []linode.InstanceDisk{
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
	_, _, srvHandler := tools.NewLinodeInstanceDiskListTool(srvCfg)

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
	tool, _, handler := tools.NewLinodeInstanceDiskGetTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceDiskGetTool(cfg)

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

	disk := linode.InstanceDisk{ID: 10, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

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
	_, _, srvHandler := tools.NewLinodeInstanceDiskGetTool(srvCfg)

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

// End-to-end verification of instance disk creation workflow.
func TestLinodeInstanceDiskCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_create")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, monitorAlertDefinitionLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", managedServiceLabelParam)
	}

	if !strings.Contains(rawSchema, "size") {
		t.Errorf("tool.RawInputSchema missing key %v", "size")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceDiskCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

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

func TestLinodeInstanceDiskCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	disk := linode.InstanceDisk{ID: 50, Label: labelMyDisk, Size: 1024, Filesystem: filesystemExt4, Status: statusReady}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Disks {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Disks)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
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
	_, _, srvHandler := tools.NewLinodeInstanceDiskCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyLabel: labelMyDisk, keySize: float64(1024), keyRootPass: rootPassStrong, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, labelMyDisk) {
		t.Errorf("textContent.Text does not contain %v", labelMyDisk)
	}

	if !strings.Contains(textContent.Text, "50") {
		t.Errorf("textContent.Text does not contain %v", "50")
	}
}

// TestLinodeInstanceDiskUpdateTool verifies the instance disk update tool
// registers correctly, validates confirm, and updates disk labels.
func TestLinodeInstanceDiskUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "disk_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "disk_id")
	}

	if !strings.Contains(rawSchema, monitorAlertDefinitionLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", managedServiceLabelParam)
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceDiskUpdateToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyLabel: labelNew})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceDiskUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	disk := linode.InstanceDisk{ID: 10, Label: "renamed-disk", Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Disks10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Disks10)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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
	_, _, srvHandler := tools.NewLinodeInstanceDiskUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyDiskID: float64(10), keyLabel: "renamed-disk", keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// TestLinodeInstanceDiskDeleteTool verifies the instance disk delete tool
// registers correctly, validates confirm, and deletes disks.
func TestLinodeInstanceDiskDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_delete")
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

func TestLinodeInstanceDiskDeleteToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceDiskDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Disks10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Disks10)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceDiskDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}
}

// Dry-run coverage for instance disk delete (ByTwoIDs helper).
func TestLinodeInstanceDiskDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceDiskDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstanceDiskDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
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

		if r.URL.Path == tcLinodeInstances123Disks10 {
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

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_instance_disk_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_disk_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcLinodeInstances123Disks10) {
		t.Errorf("got %v, want %v", would["path"], tcLinodeInstances123Disks10)
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeInstanceDiskDeleteToolDryRunStillValidatesDiskId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceDiskDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "disk_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "disk_id is required")
	}
}

// TestLinodeInstanceDiskCloneTool verifies the instance disk clone tool
// registers correctly, validates confirm, and clones disks.
func TestLinodeInstanceDiskCloneToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskCloneTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_clone" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_clone")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "disk_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "disk_id")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceDiskCloneToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskCloneTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceDiskCloneToolSuccessfulClone(t *testing.T) {
	t.Parallel()

	clonedDisk := linode.InstanceDisk{ID: 99, Label: imageUbuntu2404, Size: 51200, Filesystem: filesystemExt4, Status: statusReady}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/disks/10/clone" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/10/clone")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(clonedDisk); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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

	if !strings.Contains(textContent.Text, "cloned") {
		t.Errorf("textContent.Text does not contain %v", "cloned")
	}

	if !strings.Contains(textContent.Text, "99") {
		t.Errorf("textContent.Text does not contain %v", "99")
	}
}

// TestLinodeInstanceDiskResizeTool verifies the instance disk resize tool
// registers correctly, validates required fields, and resizes disks.
func TestLinodeInstanceDiskResizeToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceDiskResizeTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_resize" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_resize")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "disk_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "disk_id")
	}

	if !strings.Contains(rawSchema, "size") {
		t.Errorf("tool.RawInputSchema missing key %v", "size")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceDiskResizeToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskResizeTool(cfg)

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

func TestLinodeInstanceDiskResizeToolSuccessfulResize(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/disks/10/resize" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/10/resize")
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
	_, _, srvHandler := tools.NewLinodeInstanceDiskResizeTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyDiskID: float64(10), keySize: float64(65536), keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "resize initiated") {
		t.Errorf("textContent.Text does not contain %v", "resize initiated")
	}

	if !strings.Contains(textContent.Text, "65536") {
		t.Errorf("textContent.Text does not contain %v", "65536")
	}

	// The id-echo proto carries new_size_mb (the canonical field name),
	// linode_id, and disk_id; it must not leak the legacy "size" key.
	var resize struct {
		LinodeID  int `json:"linode_id"`
		DiskID    int `json:"disk_id"`
		NewSizeMB int `json:"new_size_mb"`
	}

	if err := json.Unmarshal([]byte(textContent.Text), &resize); err != nil {
		t.Fatalf("unmarshal resize body: %v", err)
	}

	if resize.LinodeID != 123 || resize.DiskID != 10 || resize.NewSizeMB != 65536 {
		t.Errorf("resize id-echo = %+v, want {123 10 65536}", resize)
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
	tool, _, handler := tools.NewLinodeInstanceIPListTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceIPListTool(cfg)

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

func TestLinodeInstanceIPsListToolSuccess(t *testing.T) {
	t.Parallel()

	ips := linode.InstanceIPAddresses{
		IPv4: &linode.InstanceIPv4{
			Public: []linode.IPAddress{
				{Address: testNetIPv4AddressOne, Public: true, Type: keyIPv4, Region: regionUSEast},
			},
			Private: []linode.IPAddress{
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
	_, _, srvHandler := tools.NewLinodeInstanceIPListTool(srvCfg)

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
	tool, _, handler := tools.NewLinodeInstanceIPGetTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceIPGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyAddress: testNetIPv4AddressOne}, wantContains: errLinodeIDRequired},
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

	ipAddr := linode.IPAddress{
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
	_, _, srvHandler := tools.NewLinodeInstanceIPGetTool(srvCfg)

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

// TestLinodeInstanceIPAllocateTool verifies the instance IP allocate tool
// registers correctly, validates confirm, and allocates new IP addresses.
func TestLinodeInstanceIPAllocateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPAllocateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_ip_allocate" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_ip_allocate")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "type") {
		t.Errorf("tool.RawInputSchema missing key %v", "type")
	}

	if !strings.Contains(rawSchema, "public") {
		t.Errorf("tool.RawInputSchema missing key %v", "public")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceIPAllocateToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceIPAllocateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyType: keyIPv4, purposePublic: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceIPAllocateToolSuccessfulAllocation(t *testing.T) {
	t.Parallel()

	ipAddr := linode.IPAddress{
		Address: "198.51.100.5", Gateway: "198.51.100.0", SubnetMask: subnetMaskFixture,
		Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
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
	_, _, srvHandler := tools.NewLinodeInstanceIPAllocateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyType: keyIPv4, purposePublic: true, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "198.51.100.5") {
		t.Errorf("textContent.Text does not contain %v", "198.51.100.5")
	}

	if !strings.Contains(textContent.Text, "allocated") {
		t.Errorf("textContent.Text does not contain %v", "allocated")
	}
}

// TestLinodeInstanceIPUpdateRDNSTool verifies the instance IP RDNS update tool
// registers correctly, validates confirm and required fields, and updates RDNS.
func TestLinodeInstanceIPUpdateRDNSToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceIPUpdateRDNSTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_ip_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_ip_update")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyLinodeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeID)
	}

	if !strings.Contains(rawSchema, keyAddress) {
		t.Errorf("tool.RawInputSchema missing key %v", keyAddress)
	}

	if !strings.Contains(rawSchema, keyRDNS) {
		t.Errorf("tool.RawInputSchema missing key %v", keyRDNS)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceIPUpdateRDNSToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceIPUpdateRDNSTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123), keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressRequired},
		{name: "address with slash", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1/24", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "address with query separator", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1?bad=1", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "address with dot traversal", args: map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.1..", keyRDNS: rdnsTestExampleOrg, keyConfirm: true}, wantContains: errAddressValidIP},
		{name: "missing rdns", args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyConfirm: true}, wantContains: "rdns is required"},
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

func TestLinodeInstanceIPUpdateRDNSToolClientErrorMapsToToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips20301131 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips20301131)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid rdns"}]}`))
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
	_, _, srvHandler := tools.NewLinodeInstanceIPUpdateRDNSTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to assign RDNS") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to assign RDNS")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "invalid rdns") {
		t.Errorf("error text %q does not contain %q", text.Text, "invalid rdns")
	}
}

func TestLinodeInstanceIPUpdateRDNSToolSuccessfulRdnsUpdate(t *testing.T) {
	t.Parallel()

	ipAddr := linode.IPAddress{
		Address: testNetIPv4AddressOne, Gateway: "203.0.113.0", SubnetMask: subnetMaskFixture,
		Prefix: 24, Type: keyIPv4, Public: true, Region: regionUSEast, LinodeID: 123, RDNS: rdnsTestExampleOrg,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips20301131 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips20301131)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var body map[string]*string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		rdns := body[keyRDNS]
		if rdns == nil {
			t.Fatal("rdns should be present")
		}

		if *rdns != rdnsTestExampleOrg {
			t.Errorf("*rdns = %v, want %v", *rdns, rdnsTestExampleOrg)
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
	_, _, srvHandler := tools.NewLinodeInstanceIPUpdateRDNSTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyRDNS: rdnsTestExampleOrg, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, rdnsTestExampleOrg) {
		t.Errorf("textContent.Text does not contain %v", rdnsTestExampleOrg)
	}

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

// TestLinodeInstanceIPDeleteTool verifies the instance IP delete tool
// registers correctly, validates required fields, and removes IP addresses.
func TestLinodeInstanceIPDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_ip_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_ip_delete")
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

func TestLinodeInstanceIPDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingAddress, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errAddressRequired},
		{name: caseMalformedAddress, args: map[string]any{keyLinodeID: float64(123), keyAddress: testInvalidIPValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errAddressValidIP},
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

func TestLinodeInstanceIPDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Ips20301131 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Ips20301131)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

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
		keyLinodeID: float64(123), keyAddress: testNetIPv4AddressOne, keyConfirm: true, keyConfirmedDryRun: true,
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

	if !strings.Contains(textContent.Text, "removed") {
		t.Errorf("textContent.Text does not contain %v", "removed")
	}

	if !strings.Contains(textContent.Text, testNetIPv4AddressOne) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressOne)
	}
}

// Dry-run coverage for instance IP delete (mixed int+string IDs).
func TestLinodeInstanceIPDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceIPDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstanceIPDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	ipBody := `{"address":"203.0.113.1","type":"ipv4","public":true,"linode_id":123}`
	expectedPath := "/linode/instances/123/ips/203.0.113.1"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != expectedPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, expectedPath)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if _, writeErr := w.Write([]byte(ipBody)); writeErr != nil {
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
	_, _, handler := tools.NewLinodeInstanceIPDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyAddress:  testNetIPv4AddressOne,
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_ip_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_ip_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], expectedPath) {
		t.Errorf("got %v, want %v", would["path"], expectedPath)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeInstanceIPDeleteToolDryRunStillValidatesAddress(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceIPDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "address is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "address is required")
	}
}

// TestLinodeInstanceCloneTool verifies the instance clone tool
// registers correctly, validates confirm, and clones instances.
func TestLinodeInstanceCloneToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceCloneTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_clone" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_clone")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceCloneToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceCloneTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceCloneToolSuccessfulClone(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{ID: 999, Label: "my-linode-clone", Region: regionUSEast, Status: "provisioning"}

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/clone" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/clone")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(instance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
		"configs": []any{float64(11), float64(22)}, "disks": []any{float64(33)},
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

	if !reflect.DeepEqual(capturedBody["configs"], []any{float64(11), float64(22)}) {
		t.Errorf("capturedBody[configs] = %v, want %v", capturedBody["configs"], []any{float64(11), float64(22)})
	}

	if !reflect.DeepEqual(capturedBody["disks"], []any{float64(33)}) {
		t.Errorf("capturedBody[disks] = %v, want %v", capturedBody["disks"], []any{float64(33)})
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "cloned") {
		t.Errorf("textContent.Text does not contain %v", "cloned")
	}

	if !strings.Contains(textContent.Text, "999") {
		t.Errorf("textContent.Text does not contain %v", "999")
	}
}

// TestLinodeInstanceMigrateTool verifies the instance migrate tool
// registers correctly, validates confirm, and initiates instance migration.
func TestLinodeInstanceMigrateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_migrate" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_migrate")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, keySupportTicketRegion) {
		t.Errorf("tool.RawInputSchema missing key %v", keySupportTicketRegion)
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceMigrateToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyRegion: regionEUWest})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceMigrateToolSuccessfulMigration(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/migrate" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/migrate")
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
	_, _, srvHandler := tools.NewLinodeInstanceMigrateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyRegion: regionEUWest, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "Migration initiated") {
		t.Errorf("textContent.Text does not contain %v", "Migration initiated")
	}

	if !strings.Contains(textContent.Text, regionEUWest) {
		t.Errorf("textContent.Text does not contain %v", regionEUWest)
	}
}

// End-to-end verification of instance rebuild workflow.
func TestLinodeInstanceRebuildToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_rebuild" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_rebuild")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "image") {
		t.Errorf("tool.RawInputSchema missing key %v", "image")
	}

	if !strings.Contains(rawSchema, "root_pass") {
		t.Errorf("tool.RawInputSchema missing key %v", "root_pass")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceRebuildToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyRootPass: rootPassStrong}, wantContains: errConfirmEqualsTrue},
		{name: "missing image", args: map[string]any{keyLinodeID: float64(123), keyRootPass: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "image is required"},
		{name: "missing authentication", args: map[string]any{keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "at least one authentication method is required"},
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

func TestLinodeInstanceRebuildToolSuccessfulRebuild(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{
		ID: 123, Label: "my-linode", Region: regionUSEast, Image: imageIDUbuntu2404, Status: "rebuilding",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/rebuild" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/rebuild")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(instance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceRebuildTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyRootPass: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true,
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

	if !strings.Contains(textContent.Text, "rebuilt") {
		t.Errorf("textContent.Text does not contain %v", "rebuilt")
	}

	if !strings.Contains(textContent.Text, imageIDUbuntu2404) {
		t.Errorf("textContent.Text does not contain %v", imageIDUbuntu2404)
	}
}

// Dry-run coverage for instance rebuild (POST action, lower-level helper
// with captured-var Success). Verifies the preview fetches the instance,
// emits POST + rebuild path, and never mutates.
func TestLinodeInstanceRebuildToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceRebuildTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstanceRebuildToolDryRunPreviewWithoutMutating(t *testing.T) {
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
		if r.URL.Path == tcLinodeInstances123Disks {
			_, _ = w.Write([]byte(`{"data":[]}`))

			return
		}

		if r.URL.Path != instanceGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceGetPath)
		}

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

	if !reflect.DeepEqual(body["tool"], "linode_instance_rebuild") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_rebuild")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances/123/rebuild") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances/123/rebuild")
	}

	if slices.Contains(methodsSeen, http.MethodPost) {
		t.Errorf("methodsSeen should not contain %v", http.MethodPost)
	}
}

func TestLinodeInstanceRebuildToolDryRunStillValidatesRootPass(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceRebuildTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyImage:    imageIDUbuntu2404,
		keyDryRun:   true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one authentication method is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "at least one authentication method is required")
	}
}

// TestLinodeInstanceRescueTool verifies the instance rescue tool
// registers correctly, validates confirm, and boots instances into rescue mode.
func TestLinodeInstanceRescueToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_rescue" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_rescue")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "devices") {
		t.Errorf("tool.RawInputSchema missing key %v", "devices")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceRescueToolCaseMissingConfirm(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeInstanceRescueToolSuccessfulRescue(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/rescue" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/rescue")
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
	_, _, srvHandler := tools.NewLinodeInstanceRescueTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "rescue mode") {
		t.Errorf("textContent.Text does not contain %v", "rescue mode")
	}
}

// TestLinodeInstancePasswordResetTool verifies the instance password reset tool
// registers correctly, validates required fields, and resets root passwords.
func TestLinodeInstancePasswordResetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_password_reset" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_password_reset")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "linode_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "linode_id")
	}

	if !strings.Contains(rawSchema, "root_pass") {
		t.Errorf("tool.RawInputSchema missing key %v", "root_pass")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstancePasswordResetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyRootPass: "NewStr0ngP@ss!"}, wantContains: errConfirmEqualsTrue},
		{name: "missing root pass", args: map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "root_pass is required"},
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

func TestLinodeInstancePasswordResetToolSuccessfulPasswordReset(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/password" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/password")
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
	_, _, srvHandler := tools.NewLinodeInstancePasswordResetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyRootPass: "NewStr0ngP@ss!", keyConfirm: true, keyConfirmedDryRun: true,
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

	if !strings.Contains(textContent.Text, "password reset") {
		t.Errorf("textContent.Text does not contain %v", "password reset")
	}
}

// Dry-run coverage for instance password reset (POST action, WithID).
func TestLinodeInstancePasswordResetToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstancePasswordResetTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstancePasswordResetToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	instanceBody := `{"id":123,"label":"web-01","status":"offline"}`

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
	_, _, handler := tools.NewLinodeInstancePasswordResetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyRootPass: rootPassStrong,
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_password_reset") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_password_reset")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances/123/password") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances/123/password")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeInstancePasswordResetToolDryRunStillValidatesRootPass(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstancePasswordResetTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyDryRun:   true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "root_pass is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "root_pass is required")
	}
}

func TestLinodeInstanceVolumesListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceVolumeListTool(cfg)

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
	_, _, handler := tools.NewLinodeInstanceVolumeListTool(cfg)

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

	volumes := []linode.Volume{
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
	_, _, srvHandler := tools.NewLinodeInstanceVolumeListTool(srvCfg)

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
	_, _, srvHandler := tools.NewLinodeInstanceVolumeListTool(srvCfg)

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
