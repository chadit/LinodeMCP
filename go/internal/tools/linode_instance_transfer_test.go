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

func TestLinodeInstanceTransferGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeInstanceTransferGetTool(cfg)

		assert.Equal(t, "linode_instance_transfer_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "transfer lookup should be read capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		require.NotNil(t, handler, "handler should not be nil")
	})

	invalidCases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
	}

	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {
						Label:  envLabelDefault,
						Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
					},
				},
			}
			_, _, handler := tools.NewLinodeInstanceTransferGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)
			result, err := handler(t.Context(), req)

			require.NoError(t, err, "tool errors are returned as error results, not Go errors")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid input should return an error result")
			textContent, ok := result.Content[0].(mcp.TextContent)
			require.True(t, ok, "content should be TextContent")
			assert.Contains(t, textContent.Text, testCase.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.InstanceTransfer{Billable: 0, Quota: 2000, Used: 22956600198}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/transfer", r.URL.Path, "request path should include Linode ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "22956600198", "response should contain transfer usage")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/transfer", r.URL.Path, "request path should include Linode ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve Linode instance transfer statistics")
	})
}
