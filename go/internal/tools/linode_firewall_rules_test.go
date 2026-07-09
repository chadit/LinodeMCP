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
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeFirewallRulesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallRulesListTool(&config.Config{})

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
	_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

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
			_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

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
	_, _, handler := tools.NewLinodeFirewallRulesListTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_rules_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_rules_get")
	}
}

func TestLinodeFirewallRulesUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})

	if tool.Name != "linode_firewall_rules_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_rules_update")
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
	for _, key := range []string{keyFirewallID, keyInbound, keyOutbound, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallRulesUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcNetworkingFirewalls123Rules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Rules)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var got linode.FirewallRules
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(got.Inbound) != 1 {
			t.Errorf("len(got.Inbound) = %d, want %d", len(got.Inbound), 1)
		}

		if len(got.Inbound) == 1 && got.Inbound[0].Label != firewallRuleLabelAllowHTTPS {
			t.Errorf("got.Inbound[0].Label = %v, want %v", got.Inbound[0].Label, firewallRuleLabelAllowHTTPS)
		}

		if len(got.Outbound) != 0 {
			t.Errorf("got.Outbound = %v, want empty", got.Outbound)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.FirewallRules{InboundPolicy: policyDrop, Inbound: got.Inbound}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(123),
		keyInbound:    `[{"action":"ACCEPT","protocol":"TCP","ports":"443","label":"allow-https"}]`,
		keyOutbound:   []any{},
		keyConfirm:    true,
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

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}

	if !strings.Contains(textContent.Text, firewallRuleLabelAllowHTTPS) {
		t.Errorf("textContent.Text does not contain %v", firewallRuleLabelAllowHTTPS)
	}
}

func TestLinodeFirewallRulesUpdateToolRequiresExplicitConfirmBeforeClientCall(t *testing.T) {
	t.Parallel()

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
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
			_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

			args := map[string]any{keyFirewallID: float64(123), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray}
			if tt.set {
				args[keyConfirm] = tt.value
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallRulesUpdateToolRejectsInvalidInputsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args map[string]any
		want string
	}{
		caseMissingFirewallPathID:   {args: map[string]any{keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDRequired},
		caseZeroFirewallPathID:      {args: map[string]any{keyFirewallID: float64(0), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
		caseFractionalLinodeID:      {args: map[string]any{keyFirewallID: float64(123.5), keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
		caseSlashFirewallPathID:     {args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
		caseQueryFirewallPathID:     {args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
		caseTraversalFirewallPathID: {args: map[string]any{keyFirewallID: pathTraversalValue, keyInbound: databaseJSONArray, keyOutbound: databaseJSONArray, keyConfirm: true}, want: errFirewallIDPositive},
		"missing inbound":           {args: map[string]any{keyFirewallID: float64(123), keyOutbound: databaseJSONArray, keyConfirm: true}, want: "inbound is required"},
		"invalid inbound":           {args: map[string]any{keyFirewallID: float64(123), keyInbound: jsonObjectEmpty, keyOutbound: databaseJSONArray, keyConfirm: true}, want: "inbound must be an array of objects"},
		"null outbound":             {args: map[string]any{keyFirewallID: float64(123), keyInbound: databaseJSONArray, keyOutbound: `null`, keyConfirm: true}, want: "outbound must be an array of objects"},
	}

	for name, tt := range cases {
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
			_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallRulesUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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
	_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(123),
		keyInbound:    databaseJSONArray,
		keyOutbound:   databaseJSONArray,
		keyConfirm:    true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_firewall_rules_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_firewall_rules_update")
	}
}
