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
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const managedCredentialRevokeToolPath = "/managed/credentials/9991/revoke"

func TestLinodeManagedCredentialRevokeTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

		assert.Equal(t, "linode_managed_credential_revoke", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "managed credential revoke should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, managedCredentialIDParam, "schema should include credential_id")
		assert.Contains(t, props, keyConfirm, "mutating credential revoke tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, managedCredentialIDParam, "credential_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialRevokeToolPath, r.URL.Path, "request path should revoke credential")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "revoked")
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
				_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

				args := map[string]any{managedCredentialIDParam: managedCredentialID}
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

	t.Run("invalid credential id rejects before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingCredentialID, args: map[string]any{keyConfirm: true}},
			{name: caseZeroCredentialID, args: map[string]any{managedCredentialIDParam: 0, keyConfirm: true}},
			{name: "fractional credential id", args: map[string]any{managedCredentialIDParam: 1.5, keyConfirm: true}},
			{name: "string separator credential id", args: map[string]any{managedCredentialIDParam: pathSeparatorValue, keyConfirm: true}},
			{name: "query separator credential id", args: map[string]any{managedCredentialIDParam: querySeparatorValue, keyConfirm: true}},
			{name: caseTraversalCredentialID, args: map[string]any{managedCredentialIDParam: pathTraversalValue, keyConfirm: true}},
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
				_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				assertErrorContains(t, result, errManagedCredentialIDPositive)
				assert.Equal(t, int32(0), calls.Load(), "credential ID validation failure must happen before client call")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialRevokeToolPath, r.URL.Path, "request path should revoke credential")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: managedCredentialTemporaryError}},
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID, keyConfirm: true}))

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")
		assertErrorContains(t, result, "Failed to revoke linode_managed_credential_revoke")
	})
}
