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

const (
	keyLongviewSubscriptionID = "longview_subscription_id"
	longviewSubscriptionID    = "longview-10"
)

func TestLinodeLongviewSubscriptionGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeLongviewSubscriptionGetTool(cfg)

		assert.Equal(t, "linode_longview_subscription_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLongviewSubscriptionID, "schema should include longview_subscription_id")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/subscriptions/"+longviewSubscriptionID, r.URL.Path, "request path should include subscription ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyClientsIncluded: 10,
				keyID:              longviewSubscriptionID,
				keyLabel:           "Longview Pro 10 pack",
				keyPrice:           map[string]float64{keyHourly: 0.06, keyMonthly: 40},
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewSubscriptionGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLongviewSubscriptionID: longviewSubscriptionID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, longviewSubscriptionID, "response should contain subscription ID")
		assert.Contains(t, textContent.Text, "Longview Pro 10 pack", "response should contain subscription label")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/subscriptions/"+longviewSubscriptionID, r.URL.Path, "request path should include subscription ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewSubscriptionGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLongviewSubscriptionID: longviewSubscriptionID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_longview_subscription_get")
		assertErrorContains(t, result, errTemporaryFailure)
	})

	t.Run("invalid subscription ID rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: "longview_subscription_id is required"},
			{name: caseEmpty, args: map[string]any{keyLongviewSubscriptionID: ""}, want: "longview_subscription_id must be a non-empty string"},
			{name: caseNumeric, args: map[string]any{keyLongviewSubscriptionID: 123}, want: "longview_subscription_id must be a non-empty string"},
			{name: caseSlash, args: map[string]any{keyLongviewSubscriptionID: "longview/10"}, want: errLongviewSubscriptionIDNoSeparators},
			{name: caseQuery, args: map[string]any{keyLongviewSubscriptionID: "longview-10?query"}, want: errLongviewSubscriptionIDNoSeparators},
			{name: "fragment separator", args: map[string]any{keyLongviewSubscriptionID: "longview-10#fragment"}, want: errLongviewSubscriptionIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyLongviewSubscriptionID: pathTraversalValue}, want: errLongviewSubscriptionIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewSubscriptionGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid longview_subscription_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}
