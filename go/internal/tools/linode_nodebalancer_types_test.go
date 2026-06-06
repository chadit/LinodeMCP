package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// expect* helpers are fatal package-local checks from linode_assertions_test.go; check* helpers are nonfatal.

func TestLinodeNodeBalancerTypesTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		checkEqual(t, "linode_nodebalancer_types", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEmpty(t, tool.InputSchema.Required, "type lookup should not require arguments")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/types", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{
					keyID:      "common",
					keyLabel:   "Common",
					keyPrice:   map[string]float64{keyHourly: 0.015, keyMonthly: 10},
					"transfer": 1000,
				}},
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "common", "response should contain type id")
		expectContainsWithMode(t, false, textContent.Text, "Common", "response should contain type label")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/types", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerTypesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should return API failures as tool errors")
		expectNotNil(t, result, "result should not be nil")
		checkTrueWithMode(t, false, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "Failed to retrieve linode_nodebalancer_types", "response should identify failed tool")
		expectContainsWithMode(t, false, textContent.Text, errForbidden, "response should include API error detail")
	})
}
