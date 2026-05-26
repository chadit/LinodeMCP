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
	keyManagedLinodeSettingsLinodeID    = "linode_id"
	managedLinodeSettingsGetToolName    = "linode_managed_linode_settings_get"
	managedLinodeSettingsToolIDValue    = 234
	managedLinodeSettingsOversizedID    = 9007199254740992.0
	managedLinodeSettingsToolPathValue  = "/managed/linode-settings/234"
	managedLinodeSettingsToolLabelValue = "linode123"
)

func TestLinodeManagedLinodeSettingsGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedLinodeSettingsGetTool(cfg)

		assert.Equal(t, managedLinodeSettingsGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "managed Linode settings lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyManagedLinodeSettingsLinodeID, "schema should include linode_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyManagedLinodeSettingsLinodeID, "linode_id must be marked required")
	})

	t.Run("invalid linode id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingLinodeID, args: map[string]any{}},
			{name: "zero linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: 0}},
			{name: "negative linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: -1}},
			{name: "string linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: "234"}},
			{name: "fractional linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: 234.5}},
			{name: "oversized linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsOversizedID}},
			{name: caseSlashLinodeID, args: map[string]any{keyManagedLinodeSettingsLinodeID: "234/235"}},
			{name: "query linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: "234?x=1"}},
			{name: caseTraversalLinodeID, args: map[string]any{keyManagedLinodeSettingsLinodeID: pathTraversalValue}},
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

				cfg := managedLinodeSettingsConfig(srv.URL)
				_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, "linode_id")
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		sshUser := keyGrantLinode
		sshPort := 22

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedLinodeSettingsToolPathValue, r.URL.Path, "request path should include Linode ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedLinodeSettings{
				ID:    managedLinodeSettingsToolIDValue,
				Label: managedLinodeSettingsToolLabelValue,
				Group: managedLinodeSettingsGroup,
				SSH: linode.ManagedLinodeSettingsSSH{
					Access: true,
					IP:     "203.0.113.1",
					Port:   &sshPort,
					User:   &sshUser,
				},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(managedLinodeSettingsConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsToolIDValue}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedLinodeSettingsToolLabelValue, "response should include label")
		assert.Contains(t, textContent.Text, "203.0.113.1", "response should include ssh settings")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedLinodeSettingsToolPathValue, r.URL.Path, "request path should include Linode ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(managedLinodeSettingsConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsToolIDValue}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_managed_linode_settings_get")
		assertErrorContains(t, result, errForbidden)
	})
}

func managedLinodeSettingsConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
