package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestLinodeFirewallRulesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeFirewallRulesGetTool(&config.Config{})

	if tool.Name != "linode_firewall_rules_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_rules_get")
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
	if !strings.Contains(rawSchema, keyFirewallID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyFirewallID)
	}
}

func TestLinodeFirewallRulesListToolSuccess(t *testing.T) {
	t.Parallel()

	rules := linode.FirewallRules{
		InboundPolicy:  policyDrop,
		OutboundPolicy: policyAccept,
		Inbound: []linode.FirewallRule{{
			Action:   policyAccept,
			Protocol: "TCP",
			Ports:    "443",
			Label:    "allow-https",
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Rules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Rules)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(rules); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeFirewallRulesGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))
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

	if !strings.Contains(textContent.Text, firewallRuleLabelAllowHTTPS) {
		t.Errorf("textContent.Text does not contain %v", firewallRuleLabelAllowHTTPS)
	}

	if !strings.Contains(textContent.Text, policyDrop) {
		t.Errorf("textContent.Text does not contain %v", policyDrop)
	}
}

func TestLinodeFirewallRulesListToolRejectsInvalidFirewallIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		caseMissingFirewallPathID:   nil,
		caseZeroFirewallPathID:      float64(0),
		caseFractionalLinodeID:      float64(123.5),
		caseSlashFirewallPathID:     paymentMethodIDSlash,
		caseQueryFirewallPathID:     databaseInvalidInstanceIDQuery,
		caseTraversalFirewallPathID: pathTraversalValue,
	}

	for name, rawID := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := gentools.NewLinodeFirewallRulesGetTool(cfg)

			args := map[string]any{}
			if rawID != nil {
				args[keyFirewallID] = rawID
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

			wantFirewallID := errFirewallIDPositive
			if name == caseMissingFirewallPathID {
				wantFirewallID = errFirewallIDRequired
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, wantFirewallID) {
				t.Errorf("error text %q does not contain %q", text.Text, wantFirewallID)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallRulesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Rules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Rules)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeFirewallRulesGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve rules for firewall") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve rules for firewall")
	}
}
