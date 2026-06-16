package tools_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeInstanceFirewallsApplyToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceFirewallsApplyTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_firewall_apply" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_firewall_apply")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}
}

func TestLinodeInstanceFirewallsApplyToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceFirewallsApplyTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1), keyConfirm: true}, wantContains: errLinodeIDMin},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
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

func TestLinodeInstanceFirewallsApplyToolSuccessfulApply(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/firewalls/apply" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/firewalls/apply")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err := w.Write([]byte(jsonObjectEmpty))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceFirewallsApplyTool(srvCfg)

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

	if !strings.Contains(textContent.Text, "Firewall apply initiated") {
		t.Errorf("textContent.Text does not contain %v", "Firewall apply initiated")
	}
}

func TestLinodeInstanceFirewallsApplyToolApiFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/firewalls/apply" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/firewalls/apply")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceFirewallsApplyTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to apply firewalls for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to apply firewalls for instance 123")
	}
}
