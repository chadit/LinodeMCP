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
	accountUserCreateToolName = "linode_account_user_create"
	accountUserUsername       = "new-user"
	accountUserEmail          = "new-user@example.com"
	errUsernameRequired       = "username is required"
	errUsernameNonEmpty       = "username must be a non-empty string"
	errEmailRequired          = "email is required"
	errEmailNonEmpty          = "email must be a non-empty string"
)

func TestLinodeAccountUserCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUserCreateTool(cfg)

		assert.Equal(t, accountUserCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "user creation should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyUsername, "schema should include username")
		assert.Contains(t, props, keyEmail, "schema should include email")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyEmail, "email must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissingConfirm, set: false},
			{name: caseRequiresConfirm, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
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
				_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

				args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
			})
		}
	})

	t.Run("invalid request rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing username", args: map[string]any{keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameRequired},
			{name: "empty username", args: map[string]any{keyUsername: "", keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
			{name: "blank username", args: map[string]any{keyUsername: blankString, keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
			{name: "numeric username", args: map[string]any{keyUsername: 123, keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
			{name: "missing email", args: map[string]any{keyUsername: accountUserUsername, keyConfirm: true}, wantMessage: errEmailRequired},
			{name: "empty email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: "", keyConfirm: true}, wantMessage: errEmailNonEmpty},
			{name: "blank email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: blankString, keyConfirm: true}, wantMessage: errEmailNonEmpty},
			{name: "numeric email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: 123, keyConfirm: true}, wantMessage: errEmailNonEmpty},
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
				_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

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
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateAccountUserRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, accountUserUsername, got.Username)
			assert.Equal(t, accountUserEmail, got.Email)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUsername, Email: accountUserEmail, UserType: "default"}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountUserUsername, "response should include username")
		assert.Contains(t, textContent.Text, accountUserEmail, "response should include email")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_account_user_create")
		assertErrorContains(t, result, errForbidden)
	})
}
