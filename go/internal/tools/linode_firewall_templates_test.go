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

func TestLinodeFirewallTemplatesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallTemplatesListTool(&config.Config{})

	if tool.Name != "linode_firewall_template_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_template_list")
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
	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallTemplatesListToolSuccess(t *testing.T) {
	t.Parallel()

	templates := linode.PaginatedResponse[linode.FirewallTemplate]{
		Data: []linode.FirewallTemplate{{
			Slug: purposeVPC,
			Rules: linode.FirewallRules{
				InboundPolicy:  policyDrop,
				OutboundPolicy: policyAccept,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 5,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/templates" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/templates")
		}

		if r.URL.Query().Get(keyPage) != "2" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "50" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(templates); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallTemplatesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: float64(2), keyPageSize: float64(50)})

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

	if !strings.Contains(textContent.Text, purposeVPC) {
		t.Errorf("textContent.Text does not contain %v", purposeVPC)
	}

	if !strings.Contains(textContent.Text, "inbound_policy") {
		t.Errorf("textContent.Text does not contain %v", "inbound_policy")
	}
}

func TestLinodeFirewallTemplatesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/templates" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/templates")
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
	_, _, handler := tools.NewLinodeFirewallTemplatesListTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}
}

func TestLinodeFirewallTemplateGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallTemplateGetTool(&config.Config{})

	if tool.Name != "linode_firewall_template_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_template_get")
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
	for _, key := range []string{keySlug, keyPage, keyPageSize} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallTemplateGetToolSuccess(t *testing.T) {
	t.Parallel()

	// The by-slug templates endpoint returns a single bare template object. The
	// handler decodes it into the FirewallTemplate proto element, the same element
	// the template LIST path emits.
	const templateBody = `{"slug":"public","rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/templates/public" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/templates/public")
		}

		if r.URL.Query().Get(keyPage) != "1" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "1")
		}

		if r.URL.Query().Get(keyPageSize) != "25" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "25")
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(templateBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallTemplateGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySlug: purposePublic, keyPage: float64(1), keyPageSize: float64(25)})

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

	if !strings.Contains(textContent.Text, purposePublic) {
		t.Errorf("textContent.Text does not contain %v", purposePublic)
	}

	if !strings.Contains(textContent.Text, "inbound_policy") {
		t.Errorf("textContent.Text does not contain %v", "inbound_policy")
	}
}

func TestLinodeFirewallTemplateGetToolRejectsInvalidSlugBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidSlugs := []string{"", "public/vpc", "public?x=1", pathTraversalValue, " public", "PUBLIC", "internal"}
	for _, slug := range invalidSlugs {
		t.Run(slug, func(t *testing.T) {
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
			_, _, handler := tools.NewLinodeFirewallTemplateGetTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySlug: slug}))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "slug") {
				t.Errorf("error text %q does not contain %q", text.Text, "slug")
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallTemplateGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/firewalls/templates/vpc" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/templates/vpc")
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
	_, _, handler := tools.NewLinodeFirewallTemplateGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySlug: purposeVPC}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_template_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_template_get")
	}
}
