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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyManagedServiceID          = "service_id"
	managedServiceGetToolName    = "linode_managed_service_get"
	managedServiceToolIDValue    = 9944
	managedServiceToolPathValue  = "/managed/services/9944"
	managedServiceToolLabelValue = "prod-1"
	managedServiceToolAddress    = "https://example.org"
	managedServiceOversizedID    = 9007199254740992.0
)

func TestLinodeManagedServiceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedServiceGetTool(cfg)

		assert.Equal(t, managedServiceGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "managed service lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyManagedServiceID, "schema should include service_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyManagedServiceID, "service_id must be marked required")
	})

	t.Run("invalid service id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingServiceID, args: map[string]any{}},
			{name: caseZeroServiceID, args: map[string]any{keyManagedServiceID: 0}},
			{name: caseNegativeServiceID, args: map[string]any{keyManagedServiceID: -1}},
			{name: caseStringServiceID, args: map[string]any{keyManagedServiceID: "9944"}},
			{name: caseFractionalServiceID, args: map[string]any{keyManagedServiceID: 9944.5}},
			{name: caseOversizedServiceID, args: map[string]any{keyManagedServiceID: managedServiceOversizedID}},
			{name: caseSlashServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceSlashID}},
			{name: caseQueryServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceQueryID}},
			{name: caseTraversalServiceID, args: map[string]any{keyManagedServiceID: pathTraversalValue}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, "service_id")
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedService{
				ID:          managedServiceToolIDValue,
				Label:       managedServiceToolLabelValue,
				ServiceType: managedServiceTypeURL,
				Status:      "ok",
				Address:     managedServiceToolAddress,
				Timeout:     30,
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedServiceToolLabelValue, "response should include label")
		assert.Contains(t, textContent.Text, managedServiceToolAddress, "response should include address")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_managed_service_get")
		assertErrorContains(t, result, errForbidden)
	})
}

func managedServiceConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
