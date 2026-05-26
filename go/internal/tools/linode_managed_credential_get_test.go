package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	managedCredentialIDParam        = "credential_id"
	managedCredentialID             = 9991
	managedCredentialLabel          = "prod-password-1"
	managedCredentialLastDecrypted  = "2018-01-01T00:01:01"
	managedCredentialPath           = "/managed/credentials/9991"
	errManagedCredentialIDPositive  = "credential_id must be a positive integer"
	managedCredentialTemporaryError = "temporary failure"
)

func TestLinodeManagedCredentialGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		assert.Equal(t, "linode_managed_credential_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "managed credential get should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, managedCredentialIDParam, "schema should include credential_id")
		assert.NotContains(t, props, keyConfirm, "read-only credential get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedCredentialPath, r.URL.Path, "request path should include credential ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel, LastDecrypted: managedCredentialLastDecrypted}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedCredentialLabel)
		assert.Contains(t, textContent.Text, managedCredentialLastDecrypted)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, managedCredentialPath, r.URL.Path, "request path should include credential ID")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: managedCredentialTemporaryError}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_managed_credential_get")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "configuration failure should return an error result")
	})

	t.Run("invalid credential id", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingCredentialID, args: map[string]any{}},
			{name: caseZeroCredentialID, args: map[string]any{managedCredentialIDParam: 0}},
			{name: "fractional credential id", args: map[string]any{managedCredentialIDParam: 1.5}},
			{name: "string separator credential id", args: map[string]any{managedCredentialIDParam: pathSeparatorValue}},
			{name: "query separator credential id", args: map[string]any{managedCredentialIDParam: querySeparatorValue}},
			{name: caseTraversalCredentialID, args: map[string]any{managedCredentialIDParam: pathTraversalValue}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "https://example.invalid", Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				assertErrorContains(t, result, errManagedCredentialIDPositive)
			})
		}
	})
}
