package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeFirewallTemplatesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallTemplatesListTool(&config.Config{})

		expectEqual(t, "linode_firewall_templates_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectEqual(t, profiles.CapRead, capability, "tool should be read capability")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.InputSchema.Properties, keyPage, "schema should include page property")
		expectContains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size property")
	})

	t.Run("success", func(t *testing.T) {
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
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/firewalls/templates", r.URL.Path, "request path should match")
			checkEqual(t, "2", r.URL.Query().Get(keyPage), "page query should match")
			checkEqual(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(templates))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallTemplatesListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: float64(2), keyPageSize: float64(50)})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		expectFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, purposeVPC, "response should include template slug")
		expectContains(t, textContent.Text, "inbound_policy", "response should include template rules")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/firewalls/templates", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			checkNoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallTemplatesListTool(cfg)

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_templates_list")
	})
}

func TestLinodeFirewallTemplateGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallTemplateGetTool(&config.Config{})

		expectEqual(t, "linode_firewall_template_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectEqual(t, profiles.CapRead, capability, "tool should be read capability")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.InputSchema.Properties, keySlug, "schema should include slug property")
		expectContains(t, tool.InputSchema.Required, keySlug, "schema should require slug")
		expectContains(t, tool.InputSchema.Properties, keyPage, "schema should include page property")
		expectContains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size property")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		templates := linode.PaginatedResponse[linode.FirewallTemplate]{
			Data: []linode.FirewallTemplate{{
				Slug: purposePublic,
				Rules: linode.FirewallRules{
					InboundPolicy:  policyDrop,
					OutboundPolicy: policyAccept,
				},
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/firewalls/templates/public", r.URL.Path, "request path should match")
			checkEqual(t, "1", r.URL.Query().Get(keyPage), "page query should match")
			checkEqual(t, "25", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(templates))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallTemplateGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySlug: purposePublic, keyPage: float64(1), keyPageSize: float64(25)})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		expectFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, purposePublic, "response should include template slug")
		expectContains(t, textContent.Text, "inbound_policy", "response should include template rules")
	})

	t.Run("rejects invalid slug before client call", func(t *testing.T) {
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

				expectNoError(t, err, "handler should not return Go error")
				expectNotNil(t, result, "handler should return a result")
				expectTrue(t, result.IsError, "invalid slug should be rejected")
				assertErrorContains(t, result, "slug")
				expectFalse(t, called.Load(), "client should not be called for invalid slug")
			})
		}
	})
	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/firewalls/templates/vpc", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			checkNoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallTemplateGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySlug: purposeVPC}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_template_get")
	})
}
