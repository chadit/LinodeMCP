package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	lkeTierParam             = "tier"
	lkeTierSeparatorError    = "tier must not contain path separators"
	lkeVersionSeparatorError = "version must not contain path separators"
)

func TestLinodeLKETierVersionGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeLKETierVersionGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_lke_tier_version_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotNil(t, handler, "handler should not be nil")
	})

	for _, testCase := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing tier", args: map[string]any{databaseVersionParam: lkeVersion129}, want: "tier is required"},
		{name: "missing version", args: map[string]any{lkeTierParam: classStandard}, want: "version is required"},
		{name: "tier path separator", args: map[string]any{lkeTierParam: "standard/extra", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier query separator", args: map[string]any{lkeTierParam: "standard?debug=true", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier traversal", args: map[string]any{lkeTierParam: pathTraversalValue, databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier padded whitespace", args: map[string]any{lkeTierParam: " standard ", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "version path separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29/edge"}, want: lkeVersionSeparatorError},
		{name: "version query separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29?debug=true"}, want: lkeVersionSeparatorError},
		{name: "version traversal", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: pathTraversalValue}, want: lkeVersionSeparatorError},
		{name: "tier fragment separator", args: map[string]any{lkeTierParam: "standard#fragment", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "version fragment separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29#fragment"}, want: lkeVersionSeparatorError},
		{name: "version padded whitespace", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: " 1.29 "}, want: lkeVersionSeparatorError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, testCase.args)
			result, err := handler(t.Context(), req)
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, testCase.want)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tierVersion := linode.LKETierVersion{ID: lkeVersion129, Tier: classStandard}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/lke/tiers/standard/versions/1.29", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(tierVersion), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKETierVersionGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{lkeTierParam: classStandard, databaseVersionParam: lkeVersion129})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkFalseWithMode(t, false, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, lkeVersion129, "response should contain version")
		expectContainsWithMode(t, false, textContent.Text, classStandard, "response should contain tier")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeLKETierVersionGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{lkeTierParam: classStandard, databaseVersionParam: lkeVersion129})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve LKE tier version")
	})
}
