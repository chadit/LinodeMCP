package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeFirewallRuleVersionsListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallRuleVersionsListTool(&config.Config{})

	if tool.Name != "linode_firewall_rule_version_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_rule_version_list")
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

	if _, ok := tool.InputSchema.Properties[keyFirewallID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyFirewallID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyFirewallID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyFirewallID)
	}
}

func TestLinodeFirewallRuleVersionsListToolSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     123,
		Label:  "web-firewall",
		Status: statusEnabled,
		Rules: linode.FirewallRules{
			Version:     2,
			Fingerprint: "997dd135",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/123/history" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/history")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallRuleVersionsListTool(cfg)

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

	if !strings.Contains(textContent.Text, "web-firewall") {
		t.Errorf("textContent.Text does not contain %v", "web-firewall")
	}

	if !strings.Contains(textContent.Text, "997dd135") {
		t.Errorf("textContent.Text does not contain %v", "997dd135")
	}
}

func TestLinodeFirewallRuleVersionsListToolRejectsInvalidFirewallIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		caseMissingFirewallPathID:   nil,
		caseZeroFirewallPathID:      float64(0),
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
			_, _, handler := tools.NewLinodeFirewallRuleVersionsListTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errFirewallIDPositive) {
				t.Errorf("error text %q does not contain %q", text.Text, errFirewallIDPositive)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallRuleVersionsListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/123/history" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/history")
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
	_, _, handler := tools.NewLinodeFirewallRuleVersionsListTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_rule_version_list") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_rule_version_list")
	}
}

func TestLinodeFirewallRuleVersionGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallRuleVersionGetTool(&config.Config{})

	if tool.Name != "linode_firewall_rule_version_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_rule_version_get")
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

	for _, key := range []string{keyFirewallID, keyVersion} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyFirewallID, keyVersion} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeFirewallRuleVersionGetToolSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{ID: 123, Label: "web-firewall", Rules: linode.FirewallRules{Version: 2, Fingerprint: "997dd135", Inbound: []linode.FirewallRule{{Action: policyAccept, Protocol: "TCP", Ports: "443", Label: "allow-https"}}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/123/history/rules/2" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/history/rules/2")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallRuleVersionGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyVersion: float64(2)}))
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

	if !strings.Contains(textContent.Text, "web-firewall") {
		t.Errorf("textContent.Text does not contain %v", "web-firewall")
	}

	if !strings.Contains(textContent.Text, "997dd135") {
		t.Errorf("textContent.Text does not contain %v", "997dd135")
	}

	if !strings.Contains(textContent.Text, "allow-https") {
		t.Errorf("textContent.Text does not contain %v", "allow-https")
	}
}

func TestLinodeFirewallRuleVersionGetToolRejectsInvalidPathParamsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		caseMissingFirewallPathID:   {keyVersion: float64(2)},
		caseZeroFirewallPathID:      {keyFirewallID: float64(0), keyVersion: float64(2)},
		caseFractionalLinodeID:      {keyFirewallID: float64(123.5), keyVersion: float64(2)},
		caseSlashFirewallPathID:     {keyFirewallID: paymentMethodIDSlash, keyVersion: float64(2)},
		caseQueryFirewallPathID:     {keyFirewallID: databaseInvalidInstanceIDQuery, keyVersion: float64(2)},
		caseTraversalFirewallPathID: {keyFirewallID: pathTraversalValue, keyVersion: float64(2)},
		"missing version":           {keyFirewallID: float64(123)},
		"zero version":              {keyFirewallID: float64(123), keyVersion: float64(0)},
		"negative version":          {keyFirewallID: float64(123), keyVersion: float64(-1)},
		"fractional version":        {keyFirewallID: float64(123), keyVersion: float64(2.5)},
		"slash version":             {keyFirewallID: float64(123), keyVersion: "2/3"},
		"query version":             {keyFirewallID: float64(123), keyVersion: "2?x=1"},
		"traversal version":         {keyFirewallID: float64(123), keyVersion: pathTraversalValue},
		"backslash version":         {keyFirewallID: float64(123), keyVersion: `2\3`},
	}

	for name, args := range cases {
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
			_, _, handler := tools.NewLinodeFirewallRuleVersionGetTool(cfg)

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

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallRuleVersionGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/123/history/rules/2" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/history/rules/2")
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
	_, _, handler := tools.NewLinodeFirewallRuleVersionGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyVersion: float64(2)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_rule_version_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_rule_version_get")
	}
}
