package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func writeToolInstanceStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write([]byte(`{
		"title":"linode.com - my-linode (linode123456) - day (5 min avg)",
		"cpu":[[1521483600000,0.42]],
		"io":{"io":[[1521484800000,0.19]],"swap":[[1521484800000,0]]},
		"netv4":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]],"private_in":[[1521484800000,0]],"private_out":[[1521484800000,5.6]]},
		"netv6":{"in":[[1521484800000,0]],"out":[[1521484800000,0]],"private_in":[[1521484800000,195.18]],"private_out":[[1521484800000,5.6]]}
	}`))
	assert.NoError(t, err, "writing stats fixture should not fail")
}

func TestLinodeInstanceStatsGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceStatsGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_stats_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Required, keyLinodeID, "linode_id must be marked required")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/stats", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			writeToolInstanceStatsFixture(t, w)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceStatsGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		require.NotEmpty(t, result.Content, "result should include content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode.com - my-linode", "response should contain stats title")
		assert.Contains(t, textContent.Text, "0.42", "response should contain CPU data")
	})
	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/stats", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceStatsGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to get stats for instance 123")
		assertErrorContains(t, result, errForbidden)
	})
}

const (
	toolLinodeInstanceStatsMonthGet = "linode_instance_stats_month_get"
	keyStatsYear                    = "year"
	keyStatsMonth                   = "month"
	errStatsYearRange               = "year must be an integer between 2000 and 2037"
	errStatsMonthRange              = "month must be an integer between 1 and 12"
)

func TestLinodeInstanceStatsByYearMonthTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, toolLinodeInstanceStatsMonthGet, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "capability should be read")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyStatsYear, "schema should include year")
		assert.Contains(t, tool.InputSchema.Properties, keyStatsMonth, "schema should include month")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyStatsYear: float64(2024), keyStatsMonth: float64(8)}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyStatsYear: float64(2024), keyStatsMonth: float64(8)}, wantContains: "linode_id must be a positive integer"},
		{name: "missing year", args: map[string]any{keyLinodeID: float64(123), keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "year too low", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(1999), keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "year separator", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: "2024/..", keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "missing month", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024)}, wantContains: errStatsMonthRange},
		{name: "month too high", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(13)}, wantContains: errStatsMonthRange},
		{name: "month query", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: "8?query"}, wantContains: errStatsMonthRange},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return result")
			assert.True(t, result.IsError, "validation failure should return tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/stats/2024/8", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"cpu":   [][]float64{{1521483600000, 0.42}},
				"io":    map[string]any{"io": [][]float64{{1521484800000, 0.19}}, "swap": [][]float64{{1521484800000, 0}}},
				"netv4": map[string]any{"in": [][]float64{{1521484800000, 2004.36}}, "out": [][]float64{{1521484800000, 3928.91}}, "private_in": [][]float64{{1521484800000, 0}}, "private_out": [][]float64{{1521484800000, 5.6}}},
				"netv6": map[string]any{"in": [][]float64{{1521484800000, 10}}, "out": [][]float64{{1521484800000, 20}}, "private_in": [][]float64{{1521484800000, 0}}, "private_out": [][]float64{{1521484800000, 0}}},
				"title": "linode123 stats",
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(8)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return result")
		assert.False(t, result.IsError, "success should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode123 stats")
		assert.Contains(t, textContent.Text, "2004.36")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/stats/2024/8", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(8)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return result")
		assert.True(t, result.IsError, "API failure should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve statistics for instance 123 in 2024-08")
		assertErrorContains(t, result, errForbidden)
	})
}
