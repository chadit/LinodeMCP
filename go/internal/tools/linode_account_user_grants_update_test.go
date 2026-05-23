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

const accountUserGrantsUpdateToolName = "linode_account_user_grants_update"

func TestLinodeAccountUserGrantsUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	assert.Equal(t, accountUserGrantsUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapAdmin, capability, "grants update should require admin capability")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, keyUsername, "schema should include username")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, props, keyGlobal, "schema should include global grants")
	assert.Contains(t, props, keyGrantLinode, "schema should include resource grants")
	assert.Contains(t, props, keyGrantLKECluster, "schema should include LKE cluster grants")
	assert.Contains(t, tool.InputSchema.Required, keyUsername, "username must be marked required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
}

func TestLinodeAccountUserGrantsUpdateRequiresConfirm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}}},
		{name: caseConfirmFalse, args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}, keyConfirm: false}},
		{name: "confirm string", args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{}, keyConfirm: boolStringTrue}},
		{name: "confirm numeric", args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{}, keyConfirm: 1}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserGrantsUpdateHandler(t, &calls)
			defer cleanup()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			require.NoError(t, err, "handler should not return transport error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "missing or invalid confirm should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Equal(t, int32(0), calls, "confirm validation must fail before client call")
		})
	}
}

func TestLinodeAccountUserGrantsUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernameRequired},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: "fragment username", args: map[string]any{keyUsername: valueFragmentUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: "missing grants", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true}, wantMessage: "at least one grant section is required"},
		{name: "invalid global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: grantPermissionReadOnly}, wantMessage: errGlobalObject},
		{name: "null global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: nil}, wantMessage: errGlobalObject},
		{name: "empty global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{}}, wantMessage: errGlobalObject},
		{name: "unknown global field", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{"typo": true}}, wantMessage: errGlobalObject},
		{name: "invalid global permission", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{keyAccountAccess: "admin"}}, wantMessage: errGlobalObject},
		{name: "invalid grant array", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: grantPermissionReadWrite}, wantMessage: errGrantSectionsArray},
		{name: "invalid grant array element", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{123}}, wantMessage: errGrantSectionsArray},
		{name: "null grant array", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: nil}, wantMessage: errGrantSectionsArray},
		{name: "unknown grant field", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite, "typo": true}}}, wantMessage: errGrantSectionsArray},
		{name: "missing grant id", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyPermissions: grantPermissionReadWrite}}}, wantMessage: errGrantSectionsArray},
		{name: "zero grant id", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(0), keyPermissions: grantPermissionReadWrite}}}, wantMessage: errGrantSectionsArray},
		{name: "null grant permissions", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: nil}}}, wantMessage: errGrantSectionsArray},
		{name: "invalid grant permissions", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: "admin"}}}, wantMessage: errGrantSectionsArray},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserGrantsUpdateHandler(t, &calls)
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

func TestLinodeAccountUserGrantsUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		global, globalOK := body[keyGlobal].(map[string]any)
		if assert.True(t, globalOK, "global grants should be an object") {
			assert.Equal(t, map[string]any{keyAccountAccess: grantPermissionReadOnly}, global)
		}

		assert.Equal(t, []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}, body[keyGrantLinode])
		assert.Equal(t, []any{map[string]any{keyBetaID: float64(456), keyPermissions: grantPermissionReadOnly}}, body[keyGrantLKECluster])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission(grantPermissionReadOnly)},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission(grantPermissionReadWrite)}},
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyUsername:        accountLoginUsername,
		keyConfirm:         true,
		keyGlobal:          map[string]any{keyAccountAccess: grantPermissionReadOnly},
		keyGrantLinode:     []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}},
		keyGrantLKECluster: []any{map[string]any{keyBetaID: float64(456), keyPermissions: grantPermissionReadOnly}},
	}))

	require.NoError(t, err, "handler should not return an error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "should not be an error result")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, keyAccountAccess, "response should include global grant")
	assert.Contains(t, textContent.Text, grantPermissionReadWrite, "response should include resource grant")
}

func TestLinodeAccountUserGrantsUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/account/users/"+accountLoginUsername+"/grants", r.URL.Path, "request path should include username grants")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{}}))

	require.NoError(t, err, "handler should return API failures as tool errors")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.IsError, "API failure should be an error result")
	assertErrorContains(t, result, "Failed to update linode_account_user_grants_update")
	assertErrorContains(t, result, errForbidden)
}

func newAccountUserGrantsUpdateHandler(t *testing.T, calls *int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(calls, 1)
		w.WriteHeader(http.StatusOK)
	}))

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	return handler, srv.Close
}
