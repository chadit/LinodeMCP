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
	keyContactID              = "contact_id"
	managedContactGetToolName = "linode_managed_contact_get"
	managedContactIDValue     = 174
	managedContactOversizedID = 9007199254740992.0
	managedContactPathValue   = "/managed/contacts/174"
	managedContactEmailValue  = "john.doe@example.org"
)

func TestLinodeManagedContactGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedContactGetTool(cfg)

		assert.Equal(t, managedContactGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "managed contact lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyContactID, "schema should include contact_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyContactID, "contact_id must be marked required")
	})

	t.Run("invalid contact id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: "missing contact id", args: map[string]any{}},
			{name: caseZeroContactID, args: map[string]any{keyContactID: 0}},
			{name: "negative contact id", args: map[string]any{keyContactID: -1}},
			{name: "string contact id", args: map[string]any{keyContactID: "174"}},
			{name: "fractional contact id", args: map[string]any{keyContactID: 174.5}},
			{name: "oversized contact id", args: map[string]any{keyContactID: managedContactOversizedID}},
			{name: "slash contact id", args: map[string]any{keyContactID: "174/175"}},
			{name: "query contact id", args: map[string]any{keyContactID: "174?x=1"}},
			{name: "traversal contact id", args: map[string]any{keyContactID: pathTraversalValue}},
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

				cfg := managedContactConfig(srv.URL)
				_, _, handler := tools.NewLinodeManagedContactGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, "contact_id")
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		phone := "123-456-7890"

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedContactPathValue, r.URL.Path, "request path should include contact ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedContact{
				ID:    managedContactIDValue,
				Name:  "John Doe",
				Email: managedContactEmailValue,
				Phone: linode.ManagedContactPhone{Primary: &phone},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedContactGetTool(managedContactConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyContactID: managedContactIDValue}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedContactEmailValue, "response should include email")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedContactPathValue, r.URL.Path, "request path should include contact ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedContactGetTool(managedContactConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyContactID: managedContactIDValue}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_managed_contact_get")
		assertErrorContains(t, result, errForbidden)
	})
}

func managedContactConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
