package tools_test

import (
	"context"
	"encoding/json"
	"io"
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

const accountUserDeleteToolName = "linode_account_user_delete"

func TestLinodeAccountUserDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserDeleteTool(cfg)

	assert.Equal(t, accountUserDeleteToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapDestroy, capability, "user deletion should be CapDestroy")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, keyUsername, "schema should include username")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
}

func TestLinodeAccountUserDeleteRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newAccountUserDeleteHandler(t, &calls)
			defer cleanup()

			args := map[string]any{keyUsername: accountUserUsername}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return transport error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
		})
	}
}

func TestLinodeAccountUserDeleteRejectsInvalidUsername(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyConfirm: true}, wantMessage: errUsernameRequired},
		{name: caseEmptyUsername, args: map[string]any{keyUsername: "", keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserDeleteHandler(t, &calls)
			defer cleanup()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			require.NoError(t, err, "handler should not return transport error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantMessage)
			assert.Equal(t, int32(0), calls, "request validation must fail before client call")
		})
	}
}

func TestLinodeAccountUserDeleteSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/users/"+accountUserUsername, r.URL.Path, "request path should include username")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Empty(t, body, "delete request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserDeleteTool(accountUserDeleteConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyConfirm: true}))

	require.NoError(t, err, "handler should not return an error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "should not be an error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, "Account user deleted successfully", "response should include success message")
	assert.Contains(t, textContent.Text, accountUserUsername, "response should include username")
}

func TestLinodeAccountUserDeleteAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/account/users/"+accountUserUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserDeleteTool(accountUserDeleteConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyConfirm: true}))

	require.NoError(t, err, "handler should return API failures as tool errors")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.IsError, "API failure should be an error result")
	assertErrorContains(t, result, "Failed to delete linode_account_user_delete")
	assertErrorContains(t, result, errForbidden)
}

func accountUserDeleteConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func newAccountUserDeleteHandler(t *testing.T, calls *int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(calls, 1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeAccountUserDeleteTool(accountUserDeleteConfig(srv.URL))

	return handler, srv.Close
}
