package tools_test

import (
	"context"
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
	accountUserUpdateToolName            = "linode_account_user_update"
	accountUserUpdateNewUsername         = "renamed-user"
	accountUserUpdateEmail               = "updated-user@example.com"
	accountUserUpdateSSHKey              = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest"
	caseMissingUsername                  = "missing username"
	caseEmptyUsername                    = "empty username"
	keyAccountUserRestricted             = "restricted"
	keyAccountUserSSHKeys                = "ssh_keys"
	keyAccountUserNewUsername            = "new_username"
	errAccountUserUpdateFieldRequired    = "at least one account user field is required"
	errAccountUserUpdateSSHKeys          = "ssh_keys must be an array of non-empty strings"
	errAccountUserUpdateRestrictedBool   = "restricted must be a boolean"
	errAccountUserUpdateNewUsernameBlank = "new_username must be a non-empty string"
)

func TestLinodeAccountUserUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserUpdateTool(cfg)

	assert.Equal(t, accountUserUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapAdmin, capability, "user update should be CapAdmin")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, keyUsername, "schema should include username")
	assert.Contains(t, props, keyEmail, "schema should include email")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, props, keyAccountUserRestricted, "schema should include restricted")
	assert.Contains(t, props, keyAccountUserSSHKeys, "schema should include ssh_keys")
	assert.Contains(t, props, keyAccountUserNewUsername, "schema should include new_username")
	assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
}

func TestLinodeAccountUserUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newAccountUserUpdateHandler(t, &calls)
			defer cleanup()

			args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserUpdateEmail}
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

func TestLinodeAccountUserUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := accountUserUpdateInvalidCases()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserUpdateHandler(t, &calls)
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

func TestLinodeAccountUserUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountUserUsername, r.URL.Path, "request path should include username")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, accountUserUpdateEmail, body[keyEmail])
		assert.Equal(t, true, body[keyAccountUserRestricted])
		assert.Equal(t, accountUserUpdateNewUsername, body[keyUsername])
		assert.Equal(t, []any{accountUserUpdateSSHKey}, body[keyAccountUserSSHKeys])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{
			Username:   accountUserUpdateNewUsername,
			Email:      accountUserUpdateEmail,
			Restricted: true,
			SSHKeys:    []string{accountUserUpdateSSHKey},
			UserType:   envKeyDefault,
		}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, accountUserUpdateSuccessArgs()))

	require.NoError(t, err, "handler should not return an error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "should not be an error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, accountUserUpdateNewUsername, "response should include username")
	assert.Contains(t, textContent.Text, accountUserUpdateEmail, "response should include email")
}

func TestLinodeAccountUserUpdateAllowsEmptySSHKeys(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Contains(t, body, keyAccountUserSSHKeys)
		assert.Empty(t, body[keyAccountUserSSHKeys])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUsername, Email: accountUserUpdateEmail}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))
	args := map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: []any{}, keyConfirm: true}
	result, err := handler(t.Context(), createRequestWithArgs(t, args))

	require.NoError(t, err, "handler should not return an error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "empty ssh_keys should be sent as an explicit clear")
}

func TestLinodeAccountUserUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountUserUsername, r.URL.Path, "request path should include username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))
	args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}
	result, err := handler(t.Context(), createRequestWithArgs(t, args))

	require.NoError(t, err, "handler should return API failures as tool errors")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.IsError, "API failure should be an error result")
	assertErrorContains(t, result, "Failed to update linode_account_user_update")
	assertErrorContains(t, result, errForbidden)
}

func accountUserUpdateInvalidCases() []struct {
	name        string
	args        map[string]any
	wantMessage string
} {
	return []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernameRequired},
		{name: caseEmptyUsername, args: map[string]any{keyUsername: "", keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: "fragment username", args: map[string]any{keyUsername: "user#name", keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: "no update fields", args: map[string]any{keyUsername: accountUserUsername, keyConfirm: true}, wantMessage: errAccountUserUpdateFieldRequired},
		{name: "empty email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: "", keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "numeric email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: 123, keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "restricted string", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserRestricted: boolStringTrue, keyConfirm: true}, wantMessage: errAccountUserUpdateRestrictedBool},
		{name: "empty new_username", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserNewUsername: "", keyConfirm: true}, wantMessage: errAccountUserUpdateNewUsernameBlank},
		{name: "bad ssh_keys type", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: "ssh-ed25519", keyConfirm: true}, wantMessage: errAccountUserUpdateSSHKeys},
		{name: "bad ssh_keys item", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: []any{123}, keyConfirm: true}, wantMessage: errAccountUserUpdateSSHKeys},
	}
}

func accountUserUpdateSuccessArgs() map[string]any {
	return map[string]any{
		keyUsername:               accountUserUsername,
		keyEmail:                  accountUserUpdateEmail,
		keyAccountUserRestricted:  true,
		keyAccountUserSSHKeys:     []any{accountUserUpdateSSHKey},
		keyAccountUserNewUsername: accountUserUpdateNewUsername,
		keyConfirm:                true,
	}
}

func accountUserUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func newAccountUserUpdateHandler(t *testing.T, calls *int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(calls, 1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))

	return handler, srv.Close
}
