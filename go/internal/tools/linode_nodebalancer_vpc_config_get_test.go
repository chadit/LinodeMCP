package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeNodeBalancerVPCConfigGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

		assert.Equal(t, "linode_nodebalancer_vpc_config_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyNodeBalancerID, "schema should include nodebalancer_id property")
		assert.Contains(t, props, keyVPCConfigID, "schema should include vpc_config_id property")
	})

	t.Run("required arguments", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing nodebalancer id", args: map[string]any{keyVPCConfigID: 456}, want: "nodebalancer_id is required"},
			{name: "missing vpc config id", args: map[string]any{keyNodeBalancerID: 123}, want: "vpc_config_id is required"},
			{name: "bad vpc config id", args: map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 0}, want: "vpc_config_id must be an integer greater than or equal to 1"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID:                456,
				keyVPCID:                 789,
				keyNodeBalancerID:        123,
				"subnet_id":              321,
				"ipv4_range":             "10.100.5.100/30",
				"ipv4_range_auto_assign": false,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 456})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")
		require.NotEmpty(t, result.Content, "result should include content")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "expected TextContent result")
		assert.Contains(t, textContent.Text, "\"id\": 456")
		assert.Contains(t, textContent.Text, "\"vpc_id\": 789")
		assert.Contains(t, textContent.Text, "\"nodebalancer_id\": 123")
		assert.Contains(t, textContent.Text, "\"ipv4_range\": \"10.100.5.100/30\"")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/nodebalancers/123/vpcs/456", r.URL.Path, "request path should include both IDs")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123, keyVPCConfigID: 456})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve VPC configuration 456 for NodeBalancer 123")
	})

	t.Run("validation rejects before client call", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeNodeBalancerVPCConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: 123})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		assert.True(t, result.IsError, "result should be a tool error")
		assert.Equal(t, int32(0), calls.Load(), "missing required args should not call the API")
	})
}
