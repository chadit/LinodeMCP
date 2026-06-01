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

func TestLinodeRegionGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeRegionGetTool(cfg)

		assert.Equal(t, "linode_region_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "region get should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyRegionID, "schema should include region_id")
		assert.Contains(t, tool.InputSchema.Required, keyRegionID, "region_id should be required")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only route should not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/regions/us-east", r.URL.Path, "request path should match the documented route")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyID: regionUSEast, keyLabel: regionLabelNewark, "country": countryUS, keyStatus: statusOK}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should be successful")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast, "response should include region id")
		assert.Contains(t, textContent.Text, regionLabelNewark, "response should include region label")
	})

	t.Run("api failure", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))

		require.NoError(t, err, "handler should return API failure as a tool error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_region_get", "response should identify failed request")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid region id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeRegionGetTool(cfg)

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissingRegion, args: map[string]any{}, want: keyRegionID + " must be a non-empty string"},
			{name: caseEmpty, args: map[string]any{keyRegionID: ""}, want: keyRegionID + " must be a non-empty string"},
			{name: caseNumber, args: map[string]any{keyRegionID: 123}, want: keyRegionID + " must be a non-empty string"},
			{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue}, want: errRegionIDSlug},
			{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue}, want: errRegionIDSlug},
			{name: "path traversal", args: map[string]any{keyRegionID: pathTraversalValue}, want: errRegionIDSlug},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "validation failures should be tool errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "validation message should explain invalid region_id")
			})
		}
	})
}
