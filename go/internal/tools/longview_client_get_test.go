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

func TestLinodeLongviewClientGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeLongviewClientGetTool(cfg)

		assert.Equal(t, "linode_longview_client_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLongviewClientID, "schema should include longview_client_id")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success redacts secret fields", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID:              789,
				keyLabel:               longviewClientLabelFixture,
				keyLongviewAPIKey:      "secret-api-key",
				keyLongviewInstallCode: "secret-install-code",
				keyLongviewApps: map[string]bool{
					keyLongviewAppApache: true,
					databaseEngineName:   true,
					keyLongviewAppNginx:  false,
				},
				keyCreated: longviewClientCreatedFixture,
				keyUpdated: longviewClientUpdatedFixture,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLongviewClientID: "789"})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, longviewClientLabelFixture, "response should contain Longview client label")
		assert.Contains(t, textContent.Text, keyLongviewAppApache, "response should contain app data")
		assert.NotContains(t, textContent.Text, "secret-api-key", "response should omit API key")
		assert.NotContains(t, textContent.Text, "secret-install-code", "response should omit install code")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should include Longview client ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLongviewClientID: "789"})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_longview_client_get")
		assertErrorContains(t, result, errTemporaryFailure)
	})

	t.Run("invalid longview_client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: "longview_client_id is required"},
			{name: caseEmpty, args: map[string]any{keyLongviewClientID: ""}, want: "longview_client_id must be a non-empty string"},
			{name: caseNumeric, args: map[string]any{keyLongviewClientID: 789}, want: "longview_client_id must be a non-empty string"},
			{name: caseSlash, args: map[string]any{keyLongviewClientID: longviewClientSlashID}, want: errLongviewClientIDNoSeparators},
			{name: caseQuery, args: map[string]any{keyLongviewClientID: "789?query"}, want: errLongviewClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyLongviewClientID: pathTraversalValue}, want: errLongviewClientIDNoSeparators},
			{name: stageAlpha, args: map[string]any{keyLongviewClientID: idAbc123}, want: "longview_client_id must be a positive integer"},
			{name: caseZero, args: map[string]any{keyLongviewClientID: "0"}, want: "longview_client_id must be a positive integer"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid longview_client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLongviewClientGetOmitsSecretFields(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(linode.LongviewClient{ID: 789, Label: longviewClientLabelFixture})

	require.NoError(t, err, "Longview client response should marshal")
	assert.NotContains(t, string(payload), keyLongviewAPIKey, "Longview API key should not be part of the output type")
	assert.NotContains(t, string(payload), keyLongviewInstallCode, "Longview install code should not be part of the output type")
}
