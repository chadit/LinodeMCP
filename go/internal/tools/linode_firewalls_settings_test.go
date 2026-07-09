package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeFirewallSettingsListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallSettingsListTool(&config.Config{})

	if tool.Name != "linode_firewall_settings_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_settings_get")
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

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallSettingsListToolSuccess(t *testing.T) {
	t.Parallel()

	settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
		Linode: 100, NodeBalancer: 101, PublicInterface: 200, VPCInterface: 201,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewallsSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewallsSettings)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallSettingsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"page": float64(2), "page_size": float64(50)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, keyDefaultFirewallIDs) {
		t.Errorf("textContent.Text does not contain %v", keyDefaultFirewallIDs)
	}

	if !strings.Contains(textContent.Text, fwDeviceTypeNodeBalancer) {
		t.Errorf("textContent.Text does not contain %v", fwDeviceTypeNodeBalancer)
	}
}

func TestLinodeFirewallSettingsListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewallsSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewallsSettings)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallSettingsListTool(cfg)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_settings_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_settings_get")
	}
}

func TestLinodeFirewallSettingsUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

	if tool.Name != "linode_firewall_settings_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_settings_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDefaultFirewallIDs, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallSettingsUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
		Linode: 100, NodeBalancer: 101, PublicInterface: 102, VPCInterface: 103,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNetworkingFirewallsSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewallsSettings)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyDefaultFirewallIDs], map[string]int{keyDefaultFirewallLinode: 100, fwDeviceTypeNodeBalancer: 101, tcPublicInterface: 102, tcVPCInterface: 103}) {
			t.Errorf("body[keyDefaultFirewallIDs] = %v, want %v", body[keyDefaultFirewallIDs], map[string]int{keyDefaultFirewallLinode: 100, fwDeviceTypeNodeBalancer: 101, tcPublicInterface: 102, tcVPCInterface: 103})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100), fwDeviceTypeNodeBalancer: float64(101), tcPublicInterface: float64(102), tcVPCInterface: float64(103)},
		keyConfirm:            true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Default firewall settings updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "Default firewall settings updated successfully")
	}

	if !strings.Contains(textContent.Text, keyDefaultFirewallIDs) {
		t.Errorf("textContent.Text does not contain %v", keyDefaultFirewallIDs)
	}
}

func TestLinodeFirewallSettingsUpdateToolConfirmRequired(t *testing.T) {
	t.Parallel()

	for _, confirm := range []any{nil, false, boolStringTrue, float64(1)} {
		t.Run("reject", func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

			args := map[string]any{keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100)}}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeFirewallSettingsUpdateToolInvalidDefaultFirewallIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ids  any
		want string
	}{
		{name: caseMissing, ids: nil, want: "default_firewall_ids is required"},
		{name: "empty", ids: map[string]any{}, want: "non-empty object"},
		{name: "unsupported key", ids: map[string]any{keyDefaultFirewallLinode: float64(100), "bad": float64(101)}, want: "unsupported key"},
		{name: caseZero, ids: map[string]any{keyDefaultFirewallLinode: float64(0)}, want: errPositiveInteger},
		{name: caseNegative, ids: map[string]any{keyDefaultFirewallLinode: float64(-1)}, want: errPositiveInteger},
		{name: "fractional", ids: map[string]any{keyDefaultFirewallLinode: float64(1.5)}, want: errPositiveInteger},
		{name: caseString, ids: map[string]any{keyDefaultFirewallLinode: "100"}, want: errPositiveInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

			args := map[string]any{keyConfirm: true}
			if testCase.ids != nil {
				args[keyDefaultFirewallIDs] = testCase.ids
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeFirewallSettingsUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNetworkingFirewallsSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewallsSettings)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(100)},
		keyConfirm:            true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_firewall_settings_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_firewall_settings_update")
	}
}
