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

const accountUserGrantsToolName = "linode_account_user_grants"

func TestLinodeAccountUserGrantsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

		assert.Equal(t, accountUserGrantsToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "user grants lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyUsername, "schema should include username")
		assert.NotContains(t, props, keyConfirm, "read-only grants tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
	})

	t.Run("invalid username rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingUsername, args: map[string]any{}, wantMessage: errUsernameRequired},
			{name: caseEmptyUsername, args: map[string]any{keyUsername: ""}, wantMessage: errUsernameNonEmpty},
			{name: caseBlankUsername, args: map[string]any{keyUsername: blankString}, wantMessage: errUsernameNonEmpty},
			{name: caseNumericUsername, args: map[string]any{keyUsername: 123}, wantMessage: errUsernameNonEmpty},
			{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername}, wantMessage: errUsernamePathParamInvalid},
			{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername}, wantMessage: errUsernamePathParamInvalid},
			{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername}, wantMessage: errUsernamePathParamInvalid},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{
				Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")},
				Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission("read_write")}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "account_access", "response should include global grant")
		assert.Contains(t, textContent.Text, "read_write", "response should include resource grant")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_user_grants")
		assertErrorContains(t, result, errForbidden)
	})
}
