package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const firewallGetFixtureLabel = "edge-firewall"

func TestLinodeFirewallGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeFirewallGetTool(cfg)

	if tool.Name != "linode_firewall_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyFirewallID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyFirewallID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeFirewallGetToolSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     55,
		Label:  firewallGetFixtureLabel,
		Status: statusEnabled,
		Rules: linode.FirewallRules{
			Inbound:  []linode.FirewallRule{{}, {}},
			Outbound: []linode.FirewallRule{{}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/55" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/55")
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeFirewallGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: 55}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body[keyLabel] != firewallGetFixtureLabel {
		t.Errorf("got %v, want %v", body[keyLabel], firewallGetFixtureLabel)
	}

	rules, isMap := body["rules"].(map[string]any)
	if !isMap {
		t.Fatal("response is missing the rules object")
	}

	inbound, isArray := rules["inbound"].([]any)
	if !isArray || len(inbound) != 2 {
		t.Errorf("rules.inbound = %v, want 2 elements", rules["inbound"])
	}

	outbound, isArray := rules["outbound"].([]any)
	if !isArray || len(outbound) != 1 {
		t.Errorf("rules.outbound = %v, want 1 element", rules["outbound"])
	}
}

func TestLinodeFirewallGetToolRejectsInvalidFirewallIDBeforeClientCall(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("validation failure must not issue any request; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeFirewallGetTool(cfg)

	for name, args := range map[string]map[string]any{
		"missing firewall_id":  {},
		"zero firewall_id":     {keyFirewallID: 0},
		"negative firewall_id": {keyFirewallID: -3},
	} {
		result, err := handler(t.Context(), createRequestWithArgs(t, args))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}

		if !result.IsError {
			t.Errorf("%s: result.IsError = false, want true", name)
		}

		textContent, isText := result.Content[0].(mcp.TextContent)
		if !isText {
			t.Fatalf("%s: isText = false, want true", name)
		}

		if !strings.Contains(textContent.Text, errPositiveInteger) {
			t.Errorf("%s: textContent.Text does not contain %v", name, errPositiveInteger)
		}
	}
}

func TestLinodeFirewallGetToolClientFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeFirewallGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: 55}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve firewall") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve firewall")
	}
}
