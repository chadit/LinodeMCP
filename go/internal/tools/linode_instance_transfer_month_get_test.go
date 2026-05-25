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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	toolLinodeInstanceTransferMonthGet = "linode_instance_transfer_month_get"
	transferKeyYear                    = "year"
	transferKeyMonth                   = "month"
)

func TestLinodeInstanceTransferMonthGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceTransferMonthGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, toolLinodeInstanceTransferMonthGet, tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, transferKeyYear, "schema should include year")
		assert.Contains(t, tool.InputSchema.Properties, transferKeyMonth, "schema should include month")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDInteger},
		{name: "missing year", args: map[string]any{keyLinodeID: 123, transferKeyMonth: 1}, wantContains: "year is required"},
		{name: "traversal year", args: map[string]any{keyLinodeID: 123, transferKeyYear: pathTraversalValue, transferKeyMonth: 1}, wantContains: "year must be an integer"},
		{name: "query month", args: map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: "1?query"}, wantContains: "month must be an integer"},
		{name: "month too large", args: map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: 13}, wantContains: "month must be"},
	}

	for _, validationTest := range validationTests {
		t.Run(validationTest.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, validationTest.args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, validationTest.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.Transfer{In: 1.5, Out: 2.5, Total: 4}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/transfer/2024/1", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceTransferMonthGetTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: 1}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"total": 4`, "response should contain transfer total")
	})
}
