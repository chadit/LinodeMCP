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

func writeToolNodeBalancerStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{
		"title":"nodebalancer.example.com (nodebalancer123) - day (5 min avg)",
		"connections":[[1521483600000,12.5]],
		"traffic":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]]}
	}`))
	checkNoError(t, err, "writing stats fixture should not fail")
}

func TestLinodeNodeBalancerStatsGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerStatsGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		checkEqual(t, "linode_nodebalancer_stats_get", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID, "schema should include nodebalancer_id")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID, "nodebalancer_id must be marked required")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1)}, wantContains: errNodeBalancerIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/444/stats", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			writeToolNodeBalancerStatsFixture(t, w)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerStatsGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "success should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "nodebalancer.example.com", "response should contain stats title")
		expectContainsWithMode(t, false, textContent.Text, "2004.36", "response should contain inbound traffic")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/nodebalancers/444/stats", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerStatsGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "API failure should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve NodeBalancer 444 stats")
		assertErrorContains(t, result, errForbidden)
	})
}
