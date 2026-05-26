package tools_test

import (
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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	managedCredentialUsernamePasswordToolPath     = "/managed/credentials/9991/update"
	managedCredentialUsernamePasswordToolLabel    = "prod-password-1"
	managedCredentialUsernamePasswordToolTime     = "2018-01-01T00:01:01"
	managedCredentialUsernamePasswordToolPassword = "stored-password-value"
	managedCredentialUsernamePasswordToolUsername = "johndoe"
)

func TestLinodeManagedCredentialUsernamePasswordUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

		assert.Equal(t, "linode_managed_credential_username_password_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "credential username/password update should be admin capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Properties, managedCredentialIDParam, "schema should include credential_id")
		assert.Contains(t, tool.InputSchema.Properties, keyDiskPassword, "schema should include password")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseConfirmFalse, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

				args := map[string]any{managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls.Load(), "confirm failure must happen before client call")
			})
		}
	})

	t.Run("required arguments reject before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingCredentialID, args: map[string]any{keyConfirm: true, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
			{name: caseZeroCredentialID, args: map[string]any{keyConfirm: true, managedCredentialIDParam: 0, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
			{name: "slash credential id", args: map[string]any{keyConfirm: true, managedCredentialIDParam: "9991/2", keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
			{name: "query credential id", args: map[string]any{keyConfirm: true, managedCredentialIDParam: "9991?x=1", keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
			{name: caseTraversalCredentialID, args: map[string]any{keyConfirm: true, managedCredentialIDParam: "..", keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
			{name: "missing username password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991}, wantMessage: managedCredentialsToolPasswordReq},
			{name: "blank username password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: blankString}, wantMessage: managedCredentialsToolPasswordReq},
			{name: "numeric password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: 12}, wantMessage: "password must be a string"},
			{name: "numeric username", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword, keyUsername: 12}, wantMessage: "username must be a string"},
			{name: "blank username", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword, keyUsername: blankString}, wantMessage: "username must be a non-empty string"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)
				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls.Load(), "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialUsernamePasswordToolPath, r.URL.Path, "request path should update managed credential username and password")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var got map[string]any
			assert.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, managedCredentialUsernamePasswordToolPassword, got[keyDiskPassword])
			assert.Equal(t, managedCredentialUsernamePasswordToolUsername, got[keyUsername])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{ID: 9991, Label: managedCredentialUsernamePasswordToolLabel, LastDecrypted: managedCredentialUsernamePasswordToolTime}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyConfirm:               true,
			managedCredentialIDParam: 9991,
			keyDiskPassword:          managedCredentialUsernamePasswordToolPassword,
			keyUsername:              managedCredentialUsernamePasswordToolUsername,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedCredentialUsernamePasswordToolLabel, "response should contain credential label")
		assert.Contains(t, textContent.Text, managedCredentialUsernamePasswordToolTime, "response should contain last decrypted timestamp")
		assert.NotContains(t, textContent.Text, managedCredentialUsernamePasswordToolPassword, "response should not echo submitted password")
	})

	t.Run("success without username omits username", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialUsernamePasswordToolPath, r.URL.Path, "request path should update managed credential username and password")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var got map[string]any
			assert.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, managedCredentialUsernamePasswordToolPassword, got[keyDiskPassword])
			assert.NotContains(t, got, keyUsername, "username should be omitted when not provided")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{ID: 9991, Label: managedCredentialUsernamePasswordToolLabel, LastDecrypted: managedCredentialUsernamePasswordToolTime}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialUsernamePasswordToolPath, r.URL.Path, "request path should update managed credential username and password")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_managed_credential_username_password_update")
		assertErrorContains(t, result, errForbidden)
	})
}
