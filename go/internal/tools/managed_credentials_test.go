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
	managedCredentialsToolPath          = "/managed/credentials"
	managedCredentialsSSHKeyToolPath    = "/managed/credentials/sshkey"
	managedCredentialsToolLabel         = "prod-password-1"
	managedCredentialsToolLastDecrypted = "2018-01-01T00:01:01"
	managedSSHKeyToolValue              = "ssh-rsa managedservices-test-key"
	managedCredentialsToolPassword      = "stored-password-value"
	managedCredentialsToolUsername      = "johndoe"
	managedCredentialsToolPasswordReq   = "password is required"
)

func TestLinodeManagedCredentialsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedCredentialsTool(cfg)

		assert.Equal(t, "linode_managed_credentials", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		credentials := linode.PaginatedResponse[linode.ManagedCredential]{
			Data: []linode.ManagedCredential{{
				ID:            9991,
				Label:         managedCredentialsToolLabel,
				LastDecrypted: managedCredentialsToolLastDecrypted,
			}},
			Page:    2,
			Pages:   3,
			Results: 7,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedCredentialsToolPath, r.URL.Path, "request path should list managed credentials")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(credentials))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedCredentialsToolLabel, "response should contain credential label")
		assert.Contains(t, textContent.Text, managedCredentialsToolLastDecrypted, "response should contain last decrypted timestamp")
	})

	t.Run("invalid pagination rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "validation message should explain the bad argument")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedCredentialsToolPath, r.URL.Path, "request path should list managed credentials")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_managed_credentials", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeManagedSSHKeyTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

		assert.Equal(t, "linode_managed_ssh_key", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedCredentialsSSHKeyToolPath, r.URL.Path, "request path should retrieve Managed SSH key")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedSSHKey{SSHKey: managedSSHKeyToolValue}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedSSHKeyToolValue, "response should contain Managed SSH key")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedCredentialsSSHKeyToolPath, r.URL.Path, "request path should retrieve Managed SSH key")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_managed_ssh_key", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeManagedCredentialCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

		assert.Equal(t, "linode_managed_credential_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "credential creation should be admin capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
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
				_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

				args := map[string]any{keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

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
			{name: caseMissingLabel, args: map[string]any{keyConfirm: true, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: errLabelRequired},
			{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyConfirm: true, keyLabel: blankString, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: errLabelRequired},
			{name: "missing password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel}, wantMessage: managedCredentialsToolPasswordReq},
			{name: "blank password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: blankString}, wantMessage: managedCredentialsToolPasswordReq},
			{name: "numeric label", args: map[string]any{keyConfirm: true, keyLabel: 12, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: "label must be a string"},
			{name: "numeric password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: 12}, wantMessage: "password must be a string"},
			{name: "numeric username", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword, keyUsername: 12}, wantMessage: "username must be a string"},
			{name: "blank username", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword, keyUsername: blankString}, wantMessage: "username must be a non-empty string"},
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
				_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

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
			assert.Equal(t, managedCredentialsToolPath, r.URL.Path, "request path should create managed credentials")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var got map[string]any
			assert.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, managedCredentialsToolLabel, got[keyLabel])
			assert.Equal(t, managedCredentialsToolPassword, got[keyDiskPassword])
			assert.Equal(t, managedCredentialsToolUsername, got[keyUsername])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{
				ID:            9991,
				Label:         managedCredentialsToolLabel,
				LastDecrypted: managedCredentialsToolLastDecrypted,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyConfirm:      true,
			keyLabel:        managedCredentialsToolLabel,
			keyDiskPassword: managedCredentialsToolPassword,
			keyUsername:     managedCredentialsToolUsername,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedCredentialsToolLabel, "response should contain credential label")
		assert.Contains(t, textContent.Text, managedCredentialsToolLastDecrypted, "response should contain last decrypted timestamp")
		assert.NotContains(t, textContent.Text, managedCredentialsToolPassword, "response should not echo submitted password")
	})

	t.Run("success without username omits username", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialsToolPath, r.URL.Path, "request path should create managed credentials")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			var got map[string]any
			assert.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, managedCredentialsToolLabel, got[keyLabel])
			assert.Equal(t, managedCredentialsToolPassword, got[keyDiskPassword])
			assert.NotContains(t, got, keyUsername, "username should be omitted when not provided")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{
				ID:            9991,
				Label:         managedCredentialsToolLabel,
				LastDecrypted: managedCredentialsToolLastDecrypted,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyConfirm:      true,
			keyLabel:        managedCredentialsToolLabel,
			keyDiskPassword: managedCredentialsToolPassword,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedCredentialsToolPath, r.URL.Path, "request path should create managed credentials")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyConfirm:      true,
			keyLabel:        managedCredentialsToolLabel,
			keyDiskPassword: managedCredentialsToolPassword,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_managed_credential_create")
		assertErrorContains(t, result, errForbidden)
	})
}
