package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeInterfacesUpgradeToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInterfacesUpgradeTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_interface_upgrade" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_interface_upgrade")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyLinodeID, keyConfigID, keyDryRun, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeInterfacesUpgradeToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInterfacesUpgradeTool(cfg)

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

			args := map[string]any{keyLinodeID: float64(123), keyConfigID: float64(4567), keyDryRun: true}
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

func TestLinodeInterfacesUpgradeToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInterfacesUpgradeTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1), keyConfirm: true}, wantContains: errLinodeIDMin},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: "zero config id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0), keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseNegativeConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(-1), keyConfirm: true}, wantContains: errConfigIDMin},
		{name: "string dry_run", args: map[string]any{keyLinodeID: float64(123), keyDryRun: boolStringTrue, keyConfirm: true}, wantContains: "dry_run must be a boolean"},
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

func TestLinodeInterfacesUpgradeToolSuccess(t *testing.T) {
	t.Parallel()

	configID := 4567

	var dryRun bool

	response := linode.UpgradeLinodeInterfacesResponse{
		ConfigID: configID,
		DryRun:   dryRun,
		Interfaces: []linode.InstanceInterface{
			{ID: 0, MACAddress: macAddressFixture},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/linode/instances/123/upgrade-interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/upgrade-interfaces")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var got linode.UpgradeLinodeInterfacesRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.ConfigID == nil {
			t.Fatal("config_id should be sent")
		}

		if *got.ConfigID != configID {
			t.Errorf("*got.ConfigID = %v, want %v", *got.ConfigID, configID)
		}

		if got.DryRun == nil {
			t.Fatal("dry_run should be sent")
		}

		if *got.DryRun {
			t.Error("*got.DryRun = true, want false")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123),
		keyConfigID: float64(configID),
		keyDryRun:   dryRun,
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

	if !strings.Contains(textContent.Text, macAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", macAddressFixture)
	}

	if !strings.Contains(textContent.Text, "4567") {
		t.Errorf("textContent.Text does not contain %v", "4567")
	}
}

func TestLinodeInterfacesUpgradeToolOmittedDryRunDefaultsToPreview(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/linode/instances/123/upgrade-interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/upgrade-interfaces")
		}

		var got linode.UpgradeLinodeInterfacesRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.ConfigID != nil {
			t.Errorf("got.ConfigID = %v, want nil", got.ConfigID)
		}

		if got.DryRun == nil {
			t.Fatal("dry_run should default to true")
		}

		if !(*got.DryRun) {
			t.Error("*got.DryRun = false, want true")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.UpgradeLinodeInterfacesResponse{DryRun: true}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeInterfacesUpgradeToolClientError(t *testing.T) {
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
	_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to upgrade interfaces for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to upgrade interfaces for instance 123")
	}
}
