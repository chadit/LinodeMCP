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
	accountUserGetToolName      = "linode_account_user_get"
	errUsernamePathParamInvalid = "username must not contain '/', '?', or '..'"
)

func TestLinodeAccountUserGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUserGetTool(cfg)

		assert.Equal(t, accountUserGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "user lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyUsername, "schema should include username")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
	})

	t.Run("invalid username rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing username", args: map[string]any{}, wantMessage: errUsernameRequired},
			{name: "empty username", args: map[string]any{keyUsername: ""}, wantMessage: errUsernameNonEmpty},
			{name: "blank username", args: map[string]any{keyUsername: blankString}, wantMessage: errUsernameNonEmpty},
			{name: "numeric username", args: map[string]any{keyUsername: 123}, wantMessage: errUsernameNonEmpty},
			{name: "slash username", args: map[string]any{keyUsername: "user/name"}, wantMessage: errUsernamePathParamInvalid},
			{name: "query username", args: map[string]any{keyUsername: "user?name"}, wantMessage: errUsernamePathParamInvalid},
			{name: "dotdot username", args: map[string]any{keyUsername: "user..name"}, wantMessage: errUsernamePathParamInvalid},
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
				_, _, handler := tools.NewLinodeAccountUserGetTool(cfg)

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
			assert.Equal(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountLoginUsername, Email: "user@example.com", UserType: "default"}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountLoginUsername, "response should include username")
		assert.Contains(t, textContent.Text, "user@example.com", "response should include email")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/users/"+accountLoginUsername, r.URL.Path, "request path should include username")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_user_get")
		assertErrorContains(t, result, errForbidden)
	})
}
