package tools_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	paginationCasePageZero           = "page zero"
	paginationCasePageString         = "page string"
	paginationCasePageFractional     = "page fractional"
	paginationCasePageSizeTooSmall   = "page_size too small"
	paginationCasePageSizeTooLarge   = "page_size too large"
	paginationCasePageSizeString     = "page_size string"
	paginationCasePageSizeFractional = "page_size fractional"
	paginationMessagePageMustBe      = "page must be"
	invoiceItemLabel                 = "Nanode 1GB"
	messageInvoiceIDPositive         = "invoice_id must be a positive integer"
	keyLoginID                       = "login_id"
	accountLoginUsername             = "account-login-user"
	loginIDCaseZero                  = "login_id zero"
	loginIDCaseNegative              = "login_id negative"
	loginIDCaseFractional            = "login_id fractional"
	keyRedirectURI                   = "redirect_uri"
	keyID                            = "id"
	keyStatus                        = "status"
	keyClientID                      = "client_id"
	oauthClientID                    = "client-123"
	invalidClientIDSlash             = "client/123"
	invalidClientIDQuery             = "client?123"
	caseClientIDSlash                = "client_id with slash"
	caseClientIDQuerySeparator       = "client_id with query separator"
	caseClientIDEmpty                = "empty client_id"
	caseClientIDNumeric              = "numeric client_id"
	errClientIDRequired              = "client_id is required"
	errClientIDNonEmpty              = "client_id must be a non-empty string"
	errClientIDNoSeparators          = "client_id must not contain path separators"
	oauthClientLabel                 = "my app"
	oauthClientRedirectURI           = "https://example.com/callback"
	caseFalse                        = "false"
	caseBlank                        = "blank"
	caseWhitespacePadded             = "whitespace padded"
	errRedirectURIRequired           = "redirect_uri is required"
	blankString                      = "   "
	invalidBetaIDSlash               = "example/open"
	invalidBetaIDQuery               = "example?open=1"
	invalidBetaIDPadded              = " example_open "
	keyPublic                        = "public"
	keyThumbnailPNGBase64            = "thumbnail_png_base64"
	oauthClientThumbnailPNG          = "png-bytes"
)

// End-to-end verification of the hello tool.
func TestHelloTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := tools.NewHelloTool(nil)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "hello", tool.Name, "tool name should be hello")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("default name", func(t *testing.T) {
		t.Parallel()

		req := mcp.CallToolRequest{}
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "World", "default greeting should contain World")
		assert.Contains(t, textContent.Text, "LinodeMCP", "greeting should mention LinodeMCP")
	})

	t.Run("custom name", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{keyName: "Alice"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Alice", "greeting should contain the provided name")
	})
}

// End-to-end verification of the version tool.
func TestVersionTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := tools.NewVersionTool(nil)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "version", tool.Name, "tool name should be version")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")

		var info appinfo.Info

		err = json.Unmarshal([]byte(textContent.Text), &info)
		require.NoError(t, err, "version response should be valid JSON")
		assert.Equal(t, appinfo.Version, info.Version, "version should match appinfo.Version")
	})
}

// End-to-end verification of the instance listing workflow.
func TestLinodeInstancesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeInstanceListTool(cfg)

		assert.Equal(t, "linode_instance_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for missing environment")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: "", Token: ""},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for incomplete config")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instances := []linode.Instance{
			{ID: 1, Label: "web-1", Status: statusRunning},
			{ID: 2, Label: "db-1", Status: "stopped"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    instances,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "web-1", "response should contain first instance label")
		assert.Contains(t, textContent.Text, "db-1", "response should contain second instance label")
	})
}

// End-to-end verification of the profile tool.
func TestLinodeProfileTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeProfileTool(cfg)

		assert.Equal(t, "linode_profile", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for incomplete config")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		profile := linode.Profile{
			Username: "testuser",
			Email:    "test@example.com",
			UID:      42,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(profile))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "testuser", "response should contain the username")
	})
}

// End-to-end verification of the instance get workflow.
func TestLinodeInstanceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		assert.Equal(t, "linode_instance_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing instance ID", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for missing instance ID")
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: notANumber})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for invalid instance ID")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instance := linode.Instance{
			ID:     123,
			Label:  "test-instance",
			Status: statusRunning,
			Region: regionUSEast,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123", r.URL.Path, "request path should include instance ID")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: "123"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "test-instance", "response should contain instance label")
		assert.Contains(t, textContent.Text, statusRunning, "response should contain instance status")
	})
}

// End-to-end verification of account info retrieval.
func TestLinodeAccountTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeAccountTool(cfg)

		assert.Equal(t, "linode_account", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		account := linode.Account{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Company:   "TestCo",
			Balance:   100.50,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(account))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Test", "response should contain first name")
		assert.Contains(t, textContent.Text, "test@example.com", "response should contain email")
	})
}

func TestLinodeAccountTransferTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountTransferTool(cfg)

		assert.Equal(t, "linode_account_transfer", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, "environment", "schema should include optional environment")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only transfer tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.AccountTransfer{
			Billable: 10,
			Quota:    4000,
			Used:     123,
			RegionTransfers: []linode.AccountRegionTransfer{{
				ID:       "us-east",
				Billable: 2,
				Quota:    1000,
				Used:     50,
			}},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountTransferTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "4000", "response should contain transfer quota")
		assert.Contains(t, textContent.Text, "123", "response should contain used transfer")
		assert.Contains(t, textContent.Text, "us-east", "response should contain region id")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/transfer", r.URL.Path, "request path should be /account/transfer")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountTransferTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_transfer", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

// End-to-end verification of account settings retrieval.
func TestLinodeAccountSettingsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountSettingsTool(cfg)

		assert.Equal(t, "linode_account_settings", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		longviewSubscription := "longview-3"
		objectStorage := statusActive
		settings := linode.AccountSettings{
			BackupsEnabled:          true,
			Managed:                 false,
			NetworkHelper:           true,
			LongviewSubscription:    &longviewSubscription,
			ObjectStorage:           &objectStorage,
			InterfacesForNewLinodes: "linode_default_but_legacy_config_allowed",
			MaintenancePolicy:       maintenancePolicyMigrate,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(settings))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountSettingsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "backups_enabled", "response should contain backups setting")
		assert.Contains(t, textContent.Text, "network_helper", "response should contain network helper setting")
		assert.Contains(t, textContent.Text, "longview-3", "response should contain Longview subscription")
		assert.Contains(t, textContent.Text, maintenancePolicyMigrate, "response should contain maintenance policy")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountSettingsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for API 403")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed", "response should describe the API failure")
		assert.Contains(t, textContent.Text, errForbidden, "response should include the API reason")
	})
}

// End-to-end verification of account agreement retrieval.
func TestLinodeAccountAgreementsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountAgreementsTool(cfg)

		assert.Equal(t, "linode_account_agreements", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		agreements := linode.AccountAgreements{
			BillingAgreement:       true,
			EUModel:                true,
			MasterServiceAgreement: true,
			PrivacyPolicy:          false,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(agreements))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAgreementsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "billing_agreement", "response should contain billing agreement")
		assert.Contains(t, textContent.Text, keyPrivacyPolicy, "response should contain privacy policy")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAgreementsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for API 403")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed", "response should describe the API failure")
		assert.Contains(t, textContent.Text, errForbidden, "response should include the API reason")
	})
}

// End-to-end verification of account maintenance listing.
func TestLinodeAccountMaintenanceTool(t *testing.T) {
	t.Parallel()

	const (
		accountMaintenancePath       = "/account/maintenance"
		accountMaintenanceLabel      = "web-1"
		accountMaintenanceEntityType = "linode"
		accountMaintenanceURL        = "/v4/linode/instances/123"
	)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

		assert.Equal(t, "linode_account_maintenance", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		maintenance := linode.PaginatedResponse[linode.AccountMaintenance]{
			Data: []linode.AccountMaintenance{{
				Entity: linode.AccountMaintenanceEntity{ID: 123, Label: accountMaintenanceLabel, Type: accountMaintenanceEntityType, URL: accountMaintenanceURL},
				Reason: "Scheduled migration",
				Status: statusPending,
				Type:   "reboot",
				When:   "2026-06-01T00:00:00",
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(maintenance))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountMaintenanceLabel, "response should contain entity label")
		assert.Contains(t, textContent.Text, "Scheduled migration", "response should contain maintenance reason")
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
				_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)
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
			assert.Equal(t, accountMaintenancePath, r.URL.Path, "request path should be /account/maintenance")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_maintenance", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

// End-to-end verification of regional account availability retrieval.
func TestLinodeAccountAvailabilityGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

		assert.Equal(t, "linode_account_availability_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		availability := linode.AccountAvailability{
			Available:   []string{serviceLinodes, serviceNodeBalancers},
			Region:      regionUSEast,
			Unavailable: []string{"Kubernetes", serviceBlockStorage},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(availability))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain region")
		assert.Contains(t, textContent.Text, serviceLinodes, "response should contain available service")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/availability/"+regionUSEast, r.URL.Path, "request path should include region")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_availability_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid region rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingRegion, args: map[string]any{}, wantMessage: "region_id is required"},
			{name: caseEmpty, args: map[string]any{keyRegionID: ""}, wantMessage: "region_id must be a non-empty string"},
			{name: "number", args: map[string]any{keyRegionID: 123}, wantMessage: "region_id must be a non-empty string"},
			{name: caseSlash, args: map[string]any{keyRegionID: "us/east"}, wantMessage: errRegionIDSlug},
			{name: caseQuery, args: map[string]any{keyRegionID: "us-east?x=1"}, wantMessage: errRegionIDSlug},
			{name: caseDotTraversal, args: map[string]any{keyRegionID: pathTraversalValue}, wantMessage: errRegionIDSlug},
			{name: "whitespace", args: map[string]any{keyRegionID: "us east"}, wantMessage: errRegionIDSlug},
			{name: "fragment", args: map[string]any{keyRegionID: "us-east#frag"}, wantMessage: errRegionIDSlug},
			{name: "ampersand", args: map[string]any{keyRegionID: "us-east&x"}, wantMessage: errRegionIDSlug},
			{name: "equals", args: map[string]any{keyRegionID: "us-east=1"}, wantMessage: errRegionIDSlug},
			{name: "backslash", args: map[string]any{keyRegionID: `us\east`}, wantMessage: errRegionIDSlug},
			{name: "uppercase", args: map[string]any{keyRegionID: "US-east"}, wantMessage: errRegionIDSlug},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "validation failure should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})
}

// End-to-end verification of account notifications retrieval.
func TestLinodeAccountNotificationsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountNotificationsTool(cfg)

		assert.Equal(t, "linode_account_notifications", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		notifications := linode.PaginatedResponse[linode.AccountNotification]{
			Data: []linode.AccountNotification{{
				Label:    "Scheduled maintenance",
				Message:  "Maintenance is scheduled for a Linode.",
				Severity: "major",
				Type:     "maintenance",
			}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(notifications))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Scheduled maintenance", "response should contain notification label")
		assert.Contains(t, textContent.Text, "major", "response should contain notification severity")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/notifications", r.URL.Path, "request path should be /account/notifications")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_notifications", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: "page_size must be"},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: "page_size must be"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "validation failure should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})
}

// End-to-end verification of available beta programs retrieval.
func TestLinodeBetasTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeBetasTool(cfg)

		assert.Equal(t, "linode_betas", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := "This beta lets users try an example feature."
		betas := linode.PaginatedResponse[linode.BetaProgram]{
			Data: []linode.BetaProgram{{
				BetaClass:      "open",
				Description:    &description,
				Ended:          nil,
				GreenlightOnly: false,
				ID:             betaExampleOpen,
				Label:          labelExampleOpenBeta,
				MoreInfo:       "https://example.com/beta",
				Started:        "2024-01-02T03:04:05",
			}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/betas", r.URL.Path, "request path should be /betas")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(betas))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeBetasTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, betaExampleOpen, "response should contain beta id")
		assert.Contains(t, textContent.Text, labelExampleOpenBeta, "response should contain beta label")
		assert.Contains(t, textContent.Text, "greenlight_only", "response should contain beta availability flag")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/betas", r.URL.Path, "request path should be /betas")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeBetasTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_betas", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeBetasTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of available beta program retrieval.
func TestLinodeBetaGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeBetaGetTool(cfg)

		assert.Equal(t, "linode_beta_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyBetaID, "schema should include beta id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := "This beta lets users try an example feature."
		beta := linode.BetaProgram{
			BetaClass:      "open",
			Description:    &description,
			Ended:          nil,
			GreenlightOnly: false,
			ID:             betaExampleOpen,
			Label:          labelExampleOpenBeta,
			MoreInfo:       "https://example.com/beta",
			Started:        "2024-01-02T03:04:05",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(beta))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeBetaGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, betaExampleOpen, "response should contain beta id")
		assert.Contains(t, textContent.Text, labelExampleOpenBeta, "response should contain beta label")
		assert.Contains(t, textContent.Text, "greenlight_only", "response should contain beta availability flag")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeBetaGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_beta_get")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("invalid id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{}, wantMessage: errBetaIDRequired},
			{name: caseEmpty, args: map[string]any{keyBetaID: ""}, wantMessage: errBetaIDNonEmpty},
			{name: caseBlank, args: map[string]any{keyBetaID: blankString}, wantMessage: errBetaIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyBetaID: 123}, wantMessage: errBetaIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyBetaID: invalidBetaIDSlash}, wantMessage: errBetaIDChars},
			{name: caseQuery, args: map[string]any{keyBetaID: invalidBetaIDQuery}, wantMessage: errBetaIDChars},
			{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue}, wantMessage: errBetaIDChars},
			{name: caseWhitespacePadded, args: map[string]any{keyBetaID: invalidBetaIDPadded}, wantMessage: errBetaIDChars},
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
				_, _, handler := tools.NewLinodeBetaGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "validation failure should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})
}

// End-to-end verification of enrolled account beta programs retrieval.
func TestLinodeAccountBetasTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountBetasTool(cfg)

		assert.Equal(t, "linode_account_betas", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := "This is an open public beta for an example feature."
		betas := linode.PaginatedResponse[linode.AccountBetaProgram]{
			Data: []linode.AccountBetaProgram{{
				Description: &description,
				Ended:       nil,
				Enrolled:    "2023-09-11T00:00:00",
				ID:          betaExampleOpen,
				Label:       labelExampleOpenBeta,
				Started:     "2023-07-11T00:00:00",
			}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(betas))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, betaExampleOpen, "response should contain beta id")
		assert.Contains(t, textContent.Text, labelExampleOpenBeta, "response should contain beta label")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_betas", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account invoice lookup.
func TestLinodeAccountInvoiceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

		assert.Equal(t, "linode_account_invoice_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		invoice := linode.AccountInvoice{ID: accountInvoiceID, Date: "2024-01-31T00:00:00", Label: "Invoice #12345", Total: 11.00}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(invoice))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "12345", "response should contain invoice id")
		assert.Contains(t, textContent.Text, "Invoice #12345", "response should contain invoice label")
		assert.Contains(t, textContent.Text, "11", "response should contain invoice total")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices/12345", r.URL.Path, "request path should include invoice id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_invoice_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid invoice id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: "missing invoice id", args: map[string]any{}},
			{name: "string invoice id", args: map[string]any{keyInvoiceID: "12345"}},
			{name: "zero invoice id", args: map[string]any{keyInvoiceID: 0}},
			{name: "negative invoice id", args: map[string]any{keyInvoiceID: -1}},
			{name: "fractional invoice id", args: map[string]any{keyInvoiceID: 1.5}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid invoice id should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "invoice_id", "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account OAuth clients retrieval.
func TestLinodeAccountOAuthClientsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)
		assert.Equal(t, "linode_account_oauth_clients", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		clients := map[string]any{
			keyData: []map[string]any{{
				keyID:          "2737bf16b39ab5d7b4a1",
				keyLabel:       "example-client",
				keyRedirectURI: "https://example.com/oauth/callback",
				"secret":       "super-secret-client-secret",
				keyStatus:      statusActive,
			}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(clients))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "2737bf16b39ab5d7b4a1", "response should contain client id")
		assert.Contains(t, textContent.Text, "example-client", "response should contain client label")
		assert.NotContains(t, textContent.Text, "super-secret-client-secret", "response should not expose OAuth client secrets")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should be /account/oauth-clients")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_oauth_clients", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of child account lookup.
func TestLinodeAccountChildAccountGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

		assert.Equal(t, "linode_account_child_account_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		childAccount := linode.ChildAccount{
			EUUID:         childAccountEUUID,
			Company:       companyAcme,
			Email:         "jkowalski@example.com",
			FirstName:     "John",
			LastName:      "Smith",
			BillingSource: "external",
			CreditCard: linode.ChildAccountCreditCard{
				Expiry:   "11/2024",
				LastFour: "0111",
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(childAccount))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, childAccountEUUID, "response should contain child account euuid")
		assert.Contains(t, textContent.Text, companyAcme, "response should contain child account company")
		assert.Contains(t, textContent.Text, "0111", "response should contain child account credit card last four")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56", r.URL.Path, "request path should include child account euuid")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_child_account_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid euuid rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing euuid", args: map[string]any{}, wantMessage: "euuid is required"},
			{name: "empty euuid", args: map[string]any{keyEUUID: ""}, wantMessage: errEUUIDNonEmpty},
			{name: "numeric euuid", args: map[string]any{keyEUUID: 123}, wantMessage: errEUUIDNonEmpty},
			{name: "euuid with slash", args: map[string]any{keyEUUID: "child/account"}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with query separator", args: map[string]any{keyEUUID: "child?account"}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with traversal", args: map[string]any{keyEUUID: pathTraversalValue}, wantMessage: errEUUIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid euuid should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account events retrieval.
func TestLinodeAccountEventsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEventsTool(cfg)

		assert.Equal(t, "linode_account_events", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		events := linode.PaginatedResponse[linode.AccountEvent]{
			Data:    []linode.AccountEvent{{ID: 123, Action: "ticket_create", Status: "failed", Username: "adevi"}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/events", r.URL.Path, "request path should be /account/events")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(events))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEventsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "ticket_create", "response should contain event action")
		assert.Contains(t, textContent.Text, "adevi", "response should contain event username")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/events", r.URL.Path, "request path should be /account/events")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEventsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_events", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountEventsTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account user listing.
func TestLinodeAccountUsersTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUsersTool(cfg)

		assert.Equal(t, "linode_account_users", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		users := linode.PaginatedResponse[linode.AccountUser]{
			Data:    []linode.AccountUser{{Username: accountLoginUsername, Email: "user@example.com", Restricted: true, TFAEnabled: true}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(users))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUsersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountLoginUsername, "response should contain username")
		assert.Contains(t, textContent.Text, "user@example.com", "response should contain email")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/users", r.URL.Path, "request path should be /account/users")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountUsersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_users", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountUsersTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account login listing.
func TestLinodeAccountLoginsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountLoginsTool(cfg)

		assert.Equal(t, "linode_account_logins", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		logins := linode.PaginatedResponse[linode.AccountLogin]{
			Data:    []linode.AccountLogin{{ID: 123, Username: accountLoginUsername, IP: "203.0.113.10", Status: "successful"}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(logins))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountLoginUsername, "response should contain username")
		assert.Contains(t, textContent.Text, "203.0.113.10", "response should contain login IP")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/logins", r.URL.Path, "request path should be /account/logins")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_logins", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account login retrieval.
func TestLinodeAccountLoginGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountLoginGetTool(cfg)

		assert.Equal(t, "linode_account_login_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLoginID, "schema should include login_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		login := linode.AccountLogin{ID: 123, Username: accountLoginUsername, IP: "203.0.113.10", Status: "successful"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(login))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountLoginUsername, "response should contain username")
		assert.Contains(t, textContent.Text, "203.0.113.10", "response should contain login IP")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/logins/123", r.URL.Path, "request path should include account login ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_login_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid login_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissing, args: map[string]any{}},
			{name: loginIDCaseZero, args: map[string]any{keyLoginID: 0}},
			{name: loginIDCaseNegative, args: map[string]any{keyLoginID: -1}},
			{name: loginIDCaseFractional, args: map[string]any{keyLoginID: 12.5}},
			{name: "huge numeric", args: map[string]any{keyLoginID: 1e100}},
			{name: "above safe integer", args: map[string]any{keyLoginID: 9007199254740992.0}},
			{name: "path separator string", args: map[string]any{keyLoginID: "12/34"}},
			{name: "query separator string", args: map[string]any{keyLoginID: "12?debug=true"}},
			{name: "traversal string", args: map[string]any{keyLoginID: ".."}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid login_id should be an error result")
				assert.False(t, called.Load(), "invalid login_id should be rejected before client call")
			})
		}
	})
}

// End-to-end verification of child account listing.
func TestLinodeAccountChildAccountsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

		assert.Equal(t, "linode_account_child_accounts", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		childAccounts := linode.PaginatedResponse[linode.ChildAccount]{
			Data: []linode.ChildAccount{{
				EUUID:         childAccountEUUID,
				Company:       companyAcme,
				Email:         "jkowalski@example.com",
				FirstName:     "John",
				LastName:      "Smith",
				BillingSource: "external",
				CreditCard: linode.ChildAccountCreditCard{
					Expiry:   "11/2024",
					LastFour: "0111",
				},
			}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(childAccounts))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, childAccountEUUID, "response should contain child account euuid")
		assert.Contains(t, textContent.Text, companyAcme, "response should contain child account company")
		assert.Contains(t, textContent.Text, "0111", "response should contain child account credit card last four")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/child-accounts", r.URL.Path, "request path should be /account/child-accounts")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_child_accounts", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account OAuth client lookup.
func TestLinodeAccountOAuthClientGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyClientID, "schema should include client_id")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"id": oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, "status": statusActive, "secret": "server-secret",
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, oauthClientID, "response should contain client id")
		assert.Contains(t, textContent.Text, oauthClientLabel, "response should contain client label")
		assert.NotContains(t, textContent.Text, "server-secret", "response should not expose OAuth client secrets")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_oauth_client_get")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("invalid client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: errClientIDRequired},
			{name: caseClientIDEmpty, args: map[string]any{keyClientID: ""}, want: errClientIDNonEmpty},
			{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123}, want: errClientIDNonEmpty},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue}, want: errClientIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

// End-to-end verification of account OAuth client update.
func TestLinodeAccountOAuthClientUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "OAuth client update should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyClientID, "schema should include client_id")
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyRedirectURI, "schema should include redirect_uri")
		assert.Contains(t, props, keyPublic, "schema should include public")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: caseString, confirm: boolStringTrue, include: true},
			{name: caseNumeric, confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)

				args := map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid args reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true, keyLabel: oauthClientLabel}, want: errClientIDRequired},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: "no update fields", args: map[string]any{keyClientID: oauthClientID, keyConfirm: true}, want: "at least one of label, redirect_uri, or public is required"},
			{name: "blank label", args: map[string]any{keyClientID: oauthClientID, keyLabel: blankString, keyConfirm: true}, want: errLabelRequired},
			{name: "blank redirect_uri", args: map[string]any{keyClientID: oauthClientID, keyRedirectURI: blankString, keyConfirm: true}, want: errRedirectURIRequired},
			{name: "string public", args: map[string]any{keyClientID: oauthClientID, keyPublic: boolStringTrue, keyConfirm: true}, want: "public must be a boolean"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid args should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var got linode.UpdateOAuthClientRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))

			if assert.NotNil(t, got.Label) {
				assert.Equal(t, oauthClientLabel, *got.Label)
			}

			if assert.NotNil(t, got.RedirectURI) {
				assert.Equal(t, oauthClientRedirectURI, *got.RedirectURI)
			}

			if assert.NotNil(t, got.Public) {
				assert.True(t, *got.Public)
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel, Public: true, RedirectURI: oauthClientRedirectURI, Status: statusActive}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyPublic: true, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "OAuth client updated successfully", "response should include success message")
		assert.Contains(t, textContent.Text, oauthClientID, "response should include client id")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_account_oauth_client_update")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account OAuth client thumbnail update.
func TestLinodeAccountOAuthClientThumbnailUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_thumbnail_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "OAuth client thumbnail update should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyClientID, "schema should include client_id")
		assert.Contains(t, props, keyThumbnailPNGBase64, "schema should include thumbnail_png_base64")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: caseString, confirm: boolStringTrue, include: true},
			{name: caseNumeric, confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)

				args := map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG))}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDRequired},
			{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("invalid thumbnail rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			want  string
		}{
			{name: caseMissing, want: "thumbnail_png_base64 is required"},
			{name: caseString, value: blankString, want: "thumbnail_png_base64 must be a non-empty string"},
			{name: caseNumeric, value: 123, want: "thumbnail_png_base64 must be a non-empty string"},
			{name: "malformed base64", value: "not base64", want: "thumbnail_png_base64 must be valid standard base64"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
				args := map[string]any{keyClientID: oauthClientID, keyConfirm: true}

				if testCase.value != nil {
					args[keyThumbnailPNGBase64] = testCase.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid thumbnail should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/oauth-clients/client-123/thumbnail", r.URL.Path, "request path should update client thumbnail")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, "image/png", r.Header.Get("Content-Type"))

			got, err := io.ReadAll(r.Body)
			assert.NoError(t, err, "reading thumbnail body should not fail")
			assert.Equal(t, []byte(oauthClientThumbnailPNG), got, "thumbnail update should send decoded PNG bytes")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "OAuth client thumbnail updated successfully", "response should include success message")
		assert.Contains(t, textContent.Text, oauthClientID, "response should include client id")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/oauth-clients/client-123/thumbnail", r.URL.Path, "request path should update client thumbnail")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_account_oauth_client_thumbnail_update")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account OAuth client thumbnail retrieval.
func TestLinodeAccountOAuthClientThumbnailGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_thumbnail_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "OAuth client thumbnail get should be CapRead")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyClientID, "schema should include client_id")
		assert.NotContains(t, props, keyConfirm, "read-only tool should not require confirm")
	})

	t.Run("invalid client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: errClientIDRequired},
			{name: caseClientIDEmpty, args: map[string]any{keyClientID: ""}, want: errClientIDNonEmpty},
			{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123}, want: errClientIDNonEmpty},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue}, want: errClientIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		thumbnailPNG := []byte("png-bytes")

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients/client-123/thumbnail", r.URL.Path, "request path should get client thumbnail")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "image/png")
			_, writeErr := w.Write(thumbnailPNG)
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, oauthClientID, "response should include client id")
		assert.Contains(t, textContent.Text, base64.StdEncoding.EncodeToString(thumbnailPNG), "response should include base64-encoded thumbnail")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/oauth-clients/client-123/thumbnail", r.URL.Path, "request path should get client thumbnail")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Not Found"}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to get OAuth client thumbnail")
	})
}

// End-to-end verification of account OAuth client deletion.
func TestLinodeAccountOAuthClientDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "OAuth client deletion should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyClientID, "schema should include client_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: caseString, confirm: boolStringTrue, include: true},
			{name: caseNumeric, confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)

				args := map[string]any{keyClientID: oauthClientID}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errClientIDRequired},
			{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true}, want: errClientIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "DELETE request should not send a body")
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "OAuth client deleted successfully", "response should include success message")
		assert.Contains(t, textContent.Text, oauthClientID, "response should include client id")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/oauth-clients/client-123", r.URL.Path, "request path should include client id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to delete linode_account_oauth_client_delete")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account OAuth client secret reset.
func TestLinodeAccountOAuthClientResetSecretTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_reset_secret", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "OAuth client secret reset should be CapAdmin")
		assert.Contains(t, tool.Description, "WARNING", "tool should warn about the one-time secret")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyClientID, "schema should include client_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: caseString, confirm: boolStringTrue, include: true},
			{name: caseNumeric, confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)

				args := map[string]any{keyClientID: oauthClientID}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid client_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errClientIDRequired},
			{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyConfirm: true}, want: errClientIDNonEmpty},
			{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyConfirm: true}, want: errClientIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true}, want: errClientIDNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid client_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/oauth-clients/client-123/reset-secret", r.URL.Path, "request path should reset client secret")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "reset request should not send a body")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.OAuthClientSecret{Secret: "new-secret-once"}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "OAuth client secret reset successfully", "response should include success message")
		assert.Contains(t, textContent.Text, "IMPORTANT", "response should warn about one-time secret")
		assert.Contains(t, textContent.Text, "new-secret-once", "response should contain the one-time secret")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/oauth-clients/client-123/reset-secret", r.URL.Path, "request path should reset client secret")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to reset linode_account_oauth_client_reset_secret")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account OAuth client creation.
func TestLinodeAccountOAuthClientCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)

		assert.Equal(t, "linode_account_oauth_client_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "OAuth client creation should be CapAdmin")
		assert.Contains(t, tool.Description, "WARNING", "tool should warn about the one-time secret")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "redirect_uri", "schema should include redirect_uri")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: "string", confirm: boolStringTrue, include: true},
			{name: "number", confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)

				args := map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("missing required args reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing label", args: map[string]any{keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
			{name: "non-string label", args: map[string]any{keyLabel: 123, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
			{name: "blank label", args: map[string]any{keyLabel: blankString, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
			{name: "missing redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyConfirm: true}, want: errRedirectURIRequired},
			{name: "non-string redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: 123, keyConfirm: true}, want: errRedirectURIRequired},
			{name: "blank redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: blankString, keyConfirm: true}, want: errRedirectURIRequired},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing arg should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var got linode.CreateOAuthClientRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, oauthClientLabel, got.Label)
			assert.Equal(t, oauthClientRedirectURI, got.RedirectURI)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.CreatedOAuthClient{
				ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once",
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "OAuth client created successfully", "response should include success message")
		assert.Contains(t, textContent.Text, "IMPORTANT", "response should warn about one-time secret")
		assert.Contains(t, textContent.Text, "secret-once", "response should contain the one-time secret")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/oauth-clients", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_account_oauth_client_create")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account invoice listing.
func TestLinodeAccountInvoiceItemsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

		assert.Equal(t, "linode_account_invoice_items", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "invoice_id", "schema should include invoice_id")
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		items := linode.PaginatedResponse[linode.AccountInvoiceItem]{
			Data:    []linode.AccountInvoiceItem{{Label: invoiceItemLabel, Quantity: 1, Total: 5.00, Type: "linode", UnitPrice: 5.00}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(items))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID, keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, invoiceItemLabel, "response should contain item label")
		assert.Contains(t, textContent.Text, "5", "response should contain item total")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices/12345/items", r.URL.Path, "request path should include invoice id and items")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_invoice_items", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid inputs reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing invoice id", args: map[string]any{}, wantMessage: "invoice_id is required"},
			{name: "invoice id string", args: map[string]any{keyInvoiceID: "12345"}, wantMessage: messageInvoiceIDPositive},
			{name: "invoice id fraction", args: map[string]any{keyInvoiceID: 12345.5}, wantMessage: messageInvoiceIDPositive},
			{name: "invoice id separator", args: map[string]any{keyInvoiceID: "12345/items"}, wantMessage: messageInvoiceIDPositive},
			{name: "invoice id query delimiter", args: map[string]any{keyInvoiceID: "12345?items"}, wantMessage: messageInvoiceIDPositive},
			{name: "invoice id traversal", args: map[string]any{keyInvoiceID: ".."}, wantMessage: messageInvoiceIDPositive},
			{name: paginationCasePageZero, args: map[string]any{keyInvoiceID: accountInvoiceID, keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyInvoiceID: accountInvoiceID, keyPageSize: 501}, wantMessage: errPageSizeRange},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountPaymentMethodsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

		assert.Equal(t, "linode_account_payment_methods", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		methods := linode.PaginatedResponse[linode.AccountPaymentMethod]{
			Data:    []linode.AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(methods))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, paymentMethodCreditCard, "response should contain payment method type")
		assert.Contains(t, textContent.Text, "1111", "response should contain payment method details")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_payment_methods", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountPaymentMethodGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)

		assert.Equal(t, "linode_account_payment_method_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyPaymentMethodID, "schema should include payment_method_id")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, paymentMethodCreditCard, "response should contain payment method type")
		assert.Contains(t, textContent.Text, paymentMethodLastFour, "response should contain payment method details")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_payment_method_get")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("invalid payment_method_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: errPaymentMethodIDRequired},
			{name: caseEmpty, args: map[string]any{keyPaymentMethodID: ""}, want: errPaymentMethodIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyPaymentMethodID: 123}, want: errPaymentMethodIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyPaymentMethodID: paymentMethodIDSlash}, want: errPaymentMethodIDNoSeparators},
			{name: caseQuery, args: map[string]any{keyPaymentMethodID: paymentMethodIDQuery}, want: errPaymentMethodIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyPaymentMethodID: pathTraversalValue}, want: errPaymentMethodIDNoSeparators},
			{name: stageAlpha, args: map[string]any{keyPaymentMethodID: idAbc123}, want: errPaymentMethodIDPositive},
			{name: caseZero, args: map[string]any{keyPaymentMethodID: "0"}, want: errPaymentMethodIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid payment_method_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLinodeAccountPaymentMethodCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

		assert.Equal(t, "linode_account_payment_method_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "type", "schema should include type")
		assert.Contains(t, props, keyData, "schema should include data")
		assert.Contains(t, props, keyIsDefault, "schema should include is_default")
		assert.Contains(t, props, keyConfirm, "mutating create tool must require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr)

			if decodeErr != nil {
				return
			}

			assert.Equal(t, paymentMethodCreditCard, body[keyType])
			assert.Equal(t, true, body[keyIsDefault])
			assert.Equal(t, map[string]any{keyToken: paymentMethodToken}, body[keyData])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 321, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, paymentMethodCreatedMessage)
		assert.Contains(t, textContent.Text, paymentMethodLastFour)
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payment-methods", r.URL.Path, "request path should be /account/payment-methods")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create linode_account_payment_method_create", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: casePaymentMethodConfirmFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumeric, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

				args := map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true}
				if testCase.name != "missing" {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "response should require confirmation")
			})
		}
	})

	t.Run("required argument validation rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing type", args: map[string]any{keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true}, want: "type is required"},
			{name: "missing data", args: map[string]any{keyType: paymentMethodCreditCard, keyIsDefault: true, keyConfirm: true}, want: "data is required"},
			{name: "missing is_default", args: map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyConfirm: true}, want: "is_default must be a boolean"},
			{name: "string is_default", args: map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: boolStringTrue, keyConfirm: true}, want: "is_default must be a boolean"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountPaymentMethodDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

		assert.Equal(t, "linode_account_payment_method_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPaymentMethodID, "schema should include payment_method_id")
		assert.Contains(t, props, keyConfirm, "mutating delete tool must require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, paymentMethodDeletedMessage)
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/payment-methods/123", r.URL.Path, "request path should include payment method id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to delete linode_account_payment_method_delete")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: casePaymentMethodConfirmFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumeric, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

				args := map[string]any{keyPaymentMethodID: paymentMethodID}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("invalid payment_method_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errPaymentMethodIDRequired},
			{name: caseEmpty, args: map[string]any{keyPaymentMethodID: "", keyConfirm: true}, want: errPaymentMethodIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyPaymentMethodID: 123, keyConfirm: true}, want: errPaymentMethodIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyPaymentMethodID: paymentMethodIDSlash, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: caseQuery, args: map[string]any{keyPaymentMethodID: paymentMethodIDQuery, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyPaymentMethodID: pathTraversalValue, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: "alpha", args: map[string]any{keyPaymentMethodID: idAbc123, keyConfirm: true}, want: errPaymentMethodIDPositive},
			{name: "zero", args: map[string]any{keyPaymentMethodID: "0", keyConfirm: true}, want: errPaymentMethodIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid payment_method_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLinodeAccountPaymentMethodMakeDefaultTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

		assert.Equal(t, "linode_account_payment_method_make_default", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPaymentMethodID, "schema should include payment_method_id")
		assert.Contains(t, props, keyConfirm, "mutating make-default tool must require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "make-default request should not send a body")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Payment method set as default successfully")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payment-methods/123/make-default", r.URL.Path, "request path should include payment method id and make-default action")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to set linode_account_payment_method_make_default")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: casePaymentMethodConfirmFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumeric, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

				args := map[string]any{keyPaymentMethodID: paymentMethodID}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("invalid payment_method_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errPaymentMethodIDRequired},
			{name: caseEmpty, args: map[string]any{keyPaymentMethodID: "", keyConfirm: true}, want: errPaymentMethodIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyPaymentMethodID: 123, keyConfirm: true}, want: errPaymentMethodIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyPaymentMethodID: paymentMethodIDSlash, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: caseQuery, args: map[string]any{keyPaymentMethodID: paymentMethodIDQuery, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyPaymentMethodID: pathTraversalValue, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: "alpha", args: map[string]any{keyPaymentMethodID: idAbc123, keyConfirm: true}, want: errPaymentMethodIDPositive},
			{name: "zero", args: map[string]any{keyPaymentMethodID: "0", keyConfirm: true}, want: errPaymentMethodIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid payment_method_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLinodeAccountPaymentsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentsTool(cfg)

		assert.Equal(t, "linode_account_payments", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		payments := linode.PaginatedResponse[linode.AccountPayment]{
			Data:    []linode.AccountPayment{{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(payments))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "654", "response should contain payment ID")
		assert.Contains(t, textContent.Text, "20.25", "response should contain payment amount")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_payments", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountPaymentGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

		assert.Equal(t, "linode_account_payment_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPaymentID, "schema should include payment_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		payment := linode.AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(payment))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPaymentID: 654})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "654", "response should contain payment ID")
		assert.Contains(t, textContent.Text, "20.25", "response should contain payment amount")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/payments/654", r.URL.Path, "request path should include payment ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPaymentID: 654})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_payment_get")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("invalid payment_id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing payment_id", args: map[string]any{}, want: "payment_id is required"},
			{name: "payment_id zero", args: map[string]any{keyPaymentID: 0}, want: errPaymentIDPositive},
			{name: "payment_id fractional", args: map[string]any{keyPaymentID: 1.5}, want: errPaymentIDPositive},
			{name: "payment_id oversized", args: map[string]any{keyPaymentID: 9007199254740992.0}, want: errPaymentIDPositive},
			{name: "payment_id string slash", args: map[string]any{keyPaymentID: "1/2"}, want: errPaymentIDPositive},
			{name: "payment_id string query", args: map[string]any{keyPaymentID: "1?x=2"}, want: errPaymentIDPositive},
			{name: caseDotTraversal, args: map[string]any{keyPaymentID: pathTraversalValue}, want: errPaymentIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid payment_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLinodeAccountInvoicesTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountInvoicesTool(cfg)

		assert.Equal(t, "linode_account_invoices", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		invoices := linode.PaginatedResponse[linode.AccountInvoice]{
			Data:    []linode.AccountInvoice{{ID: 987, Date: "2024-01-31T00:00:00", Label: "Invoice 987", Total: 42.50}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(invoices))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Invoice 987", "response should contain invoice label")
		assert.Contains(t, textContent.Text, "42.5", "response should contain invoice total")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/invoices", r.URL.Path, "request path should be /account/invoices")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_invoices", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account entity transfer listing.
func TestLinodeAccountEntityTransfersTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEntityTransfersTool(cfg)

		assert.Equal(t, "linode_account_entity_transfers", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfers := linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{
				Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
				IsSender: true,
				Status:   statusPending,
				Token:    accountEntityTransferToken,
			}},
			Page:    2,
			Pages:   4,
			Results: 80,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfers))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEntityTransfersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountEntityTransferToken, "response should contain transfer token")
		assert.Contains(t, textContent.Text, "pending", "response should contain transfer status")
		assert.Contains(t, textContent.Text, "111", "response should contain entity ids")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEntityTransfersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_entity_transfers", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountEntityTransfersTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountServiceTransfersTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

		assert.Equal(t, "linode_account_service_transfers", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfers := linode.PaginatedResponse[linode.AccountEntityTransfer]{
			Data: []linode.AccountEntityTransfer{{
				Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
				IsSender: true,
				Status:   statusPending,
				Token:    accountEntityTransferToken,
			}},
			Page:    2,
			Pages:   4,
			Results: 80,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfers))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountEntityTransferToken, "response should contain transfer token")
		assert.Contains(t, textContent.Text, "pending", "response should contain transfer status")
		assert.Contains(t, textContent.Text, "111", "response should contain entity ids")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_service_transfers", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeAccountEventGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEventGetTool(cfg)

		assert.Equal(t, "linode_account_event_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyEventID, "schema should include event_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/account/events/123", r.URL.Path, "request path should include event ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEvent{
				ID:     accountEventID,
				Action: accountEventAction,
				Status: statusSuccessful,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountEventAction, "response should include event action")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_event_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: errEventIDRequired},
			{name: caseString, args: map[string]any{keyEventID: "123"}, want: errEventIDPositive},
			{name: loginIDCaseZero, args: map[string]any{keyEventID: float64(0)}, want: errEventIDPositive},
			{name: loginIDCaseNegative, args: map[string]any{keyEventID: float64(-1)}, want: errEventIDPositive},
			{name: loginIDCaseFractional, args: map[string]any{keyEventID: 123.5}, want: errEventIDPositive},
			{name: "overflow", args: map[string]any{keyEventID: 1e100}, want: errEventIDPositive},
			{name: caseSlash, args: map[string]any{keyEventID: "12/3"}, want: errEventIDPositive},
			{name: caseQuery, args: map[string]any{keyEventID: "12?3"}, want: errEventIDPositive},
			{name: caseDotTraversal, args: map[string]any{keyEventID: pathTraversalValue}, want: errEventIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool validation errors are returned as error results")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "validation should return error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "validation message should explain the error")
			})
		}
	})
}

// End-to-end verification of marking an account event as seen.
func TestLinodeAccountEventSeenTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEventSeenTool(cfg)

		assert.Equal(t, "linode_account_event_seen", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "event seen marking should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyEventID, "schema should include event_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

				args := map[string]any{keyEventID: float64(accountEventID)}
				if tt.set {
					args[keyConfirm] = tt.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "confirm validation should return a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing/false/non-boolean confirm should be an error")
				assert.Equal(t, int32(0), calls.Load(), "client should not be called before confirm=true")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/events/123/seen", r.URL.Path, "request path should mark event seen")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "request should not include a body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID), keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "marked as seen", "response should confirm the state change")
		assert.Contains(t, textContent.Text, "123", "response should include event ID")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID), keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")
		assertErrorContains(t, result, "Failed to mark linode_account_event_seen")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errEventIDRequired},
			{name: caseString, args: map[string]any{keyEventID: "123", keyConfirm: true}, want: errEventIDPositive},
			{name: loginIDCaseZero, args: map[string]any{keyEventID: float64(0), keyConfirm: true}, want: errEventIDPositive},
			{name: loginIDCaseNegative, args: map[string]any{keyEventID: float64(-1), keyConfirm: true}, want: errEventIDPositive},
			{name: loginIDCaseFractional, args: map[string]any{keyEventID: 123.5, keyConfirm: true}, want: errEventIDPositive},
			{name: "overflow", args: map[string]any{keyEventID: 1e100, keyConfirm: true}, want: errEventIDPositive},
			{name: caseSlash, args: map[string]any{keyEventID: "12/3", keyConfirm: true}, want: errEventIDPositive},
			{name: caseQuery, args: map[string]any{keyEventID: "12?3", keyConfirm: true}, want: errEventIDPositive},
			{name: caseDotTraversal, args: map[string]any{keyEventID: pathTraversalValue, keyConfirm: true}, want: errEventIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool validation errors are returned as error results")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "validation should return error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})
}

func TestLinodeAccountServiceTransferGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

		assert.Equal(t, "linode_account_service_transfer_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.AccountEntityTransfer{
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
			IsSender: true,
			Status:   statusPending,
			Token:    accountServiceTransferToken,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountServiceTransferToken, "response should contain transfer token")
		assert.Contains(t, textContent.Text, "pending", "response should contain transfer status")
		assert.Contains(t, textContent.Text, "111", "response should contain entity ids")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_service_transfer_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: ""}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue}, wantMessage: errTokenNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account service transfer creation.
func TestLinodeAccountServiceTransferCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

		assert.Equal(t, "linode_account_service_transfer_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "service transfer creation should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeIDs, "schema should include linode_ids")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

				args := map[string]any{keyLinodeIDs: []any{float64(123)}}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid linode ids rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing linode_ids", args: map[string]any{keyConfirm: true}, wantMessage: "linode_ids is required"},
			{name: "empty linode_ids", args: map[string]any{keyLinodeIDs: []any{}, keyConfirm: true}, wantMessage: "linode_ids must include at least one ID"},
			{name: "string linode_ids", args: map[string]any{keyLinodeIDs: "123", keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "string element", args: map[string]any{keyLinodeIDs: []any{"123"}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "zero element", args: map[string]any{keyLinodeIDs: []any{float64(0)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "negative element", args: map[string]any{keyLinodeIDs: []any{float64(-1)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "fractional element", args: map[string]any{keyLinodeIDs: []any{1.5}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "overflow element", args: map[string]any{keyLinodeIDs: []any{float64(1 << 63)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

				req := createRequestWithArgs(t, tt.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, tt.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateAccountServiceTransferRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, []int{123, 456}, got.Entities.Linodes)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{
				Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}},
				Status:   statusPending,
				Token:    "service-transfer-token",
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123), float64(456)}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		require.NotEmpty(t, result.Content, "response should include content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "service-transfer-token", "response should include transfer token")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/service-transfers", r.URL.Path, "request path should be /account/service-transfers")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123)}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create linode_account_service_transfer_create", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeAccountServiceTransferAcceptTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

		assert.Equal(t, "linode_account_service_transfer_accept", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "service transfer acceptance should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissingConfirm},
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
				_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

				args := map[string]any{keyToken: accountServiceTransferToken}
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
	})

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true}, wantMessage: errTokenNoSeparators},
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
				_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true}))

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		assertErrorContains(t, result, accountServiceTransferToken)
		assertErrorContains(t, result, "accepted successfully")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/service-transfers/service-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true}))

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to accept linode_account_service_transfer_accept")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account service transfer cancellation.
func TestLinodeAccountServiceTransferDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

		assert.Equal(t, "linode_account_service_transfer_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "service transfer cancellation should be CapDestroy")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

				args := map[string]any{keyToken: accountServiceTransferToken}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true}, wantMessage: errTokenNoSeparators},
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
				_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		assertErrorContains(t, result, accountServiceTransferToken)
		assertErrorContains(t, result, "canceled successfully")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/service-transfers/service-token-example", r.URL.Path, "request path should include transfer token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to delete linode_account_service_transfer_delete")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account entity transfer retrieval.
func TestLinodeAccountEntityTransferGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEntityTransferGetTool(cfg)

		assert.Equal(t, "linode_account_entity_transfer_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.AccountEntityTransfer{
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
			IsSender: true,
			Status:   statusPending,
			Token:    accountEntityTransferToken,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEntityTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountEntityTransferToken, "response should contain transfer token")
		assert.Contains(t, textContent.Text, "pending", "response should contain transfer status")
		assert.Contains(t, textContent.Text, "111", "response should contain entity ids")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountEntityTransferGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_entity_transfer_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: ""}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue}, wantMessage: errTokenNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountEntityTransferGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account entity transfer cancellation.
func TestLinodeAccountEntityTransferAcceptTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEntityTransferAcceptTool(cfg)

		assert.Equal(t, "linode_account_entity_transfer_accept", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "transfer acceptance should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountEntityTransferAcceptTool(cfg)

				args := map[string]any{keyToken: accountEntityTransferToken}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true}, wantMessage: errTokenNoSeparators},
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
				_, _, handler := tools.NewLinodeAccountEntityTransferAcceptTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferAcceptTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		assertErrorContains(t, result, accountEntityTransferToken)
		assertErrorContains(t, result, "accepted successfully")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example/accept", r.URL.Path, "request path should include transfer token accept action")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferAcceptTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to accept linode_account_entity_transfer_accept")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeAccountEntityTransferDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEntityTransferDeleteTool(cfg)

		assert.Equal(t, "linode_account_entity_transfer_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "transfer cancellation should be CapDestroy")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyToken, "schema should include token")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountEntityTransferDeleteTool(cfg)

				args := map[string]any{keyToken: accountEntityTransferToken}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid token rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, wantMessage: errTokenRequired},
			{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true}, wantMessage: errTokenNonEmpty},
			{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true}, wantMessage: errTokenNoSeparators},
			{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true}, wantMessage: errTokenNoSeparators},
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
				_, _, handler := tools.NewLinodeAccountEntityTransferDeleteTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		assertErrorContains(t, result, accountEntityTransferToken)
		assertErrorContains(t, result, "canceled successfully")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/account/entity-transfers/transfer-token-example", r.URL.Path, "request path should include transfer token")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyToken: accountEntityTransferToken, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to delete linode_account_entity_transfer_delete")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of enrolled account beta program retrieval.
func TestLinodeAccountBetaGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountBetaGetTool(cfg)

		assert.Equal(t, "linode_account_beta_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyBetaID, "schema should include beta id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := "This is an open public beta for an example feature."
		beta := linode.AccountBetaProgram{
			Description: &description,
			Ended:       nil,
			Enrolled:    "2023-09-11T00:00:00",
			ID:          betaExampleOpen,
			Label:       labelExampleOpenBeta,
			Started:     "2023-07-11T00:00:00",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(beta))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, betaExampleOpen, "response should contain beta id")
		assert.Contains(t, textContent.Text, labelExampleOpenBeta, "response should contain beta label")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/betas/"+betaExampleOpen, r.URL.Path, "request path should include beta id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_account_beta_get")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("invalid id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingConfirm, args: map[string]any{}, wantMessage: errBetaIDRequired},
			{name: caseEmpty, args: map[string]any{keyBetaID: ""}, wantMessage: errBetaIDNonEmpty},
			{name: caseBlank, args: map[string]any{keyBetaID: blankString}, wantMessage: errBetaIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyBetaID: 123}, wantMessage: errBetaIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyBetaID: invalidBetaIDSlash}, wantMessage: errBetaIDChars},
			{name: caseQuery, args: map[string]any{keyBetaID: invalidBetaIDQuery}, wantMessage: errBetaIDChars},
			{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue}, wantMessage: errBetaIDChars},
			{name: caseWhitespacePadded, args: map[string]any{keyBetaID: invalidBetaIDPadded}, wantMessage: errBetaIDChars},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "validation failure should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
			})
		}
	})
}

// End-to-end verification of account entity transfer creation.
func TestLinodeAccountEntityTransferCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountEntityTransferCreateTool(cfg)

		assert.Equal(t, "linode_account_entity_transfer_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "transfer creation should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeIDs, "schema should include linode_ids")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountEntityTransferCreateTool(cfg)

				args := map[string]any{keyLinodeIDs: []any{float64(123)}}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid linode ids rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing linode_ids", args: map[string]any{keyConfirm: true}, wantMessage: "linode_ids is required"},
			{name: "empty linode_ids", args: map[string]any{keyLinodeIDs: []any{}, keyConfirm: true}, wantMessage: "linode_ids must include at least one ID"},
			{name: "string linode_ids", args: map[string]any{keyLinodeIDs: "123", keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "string element", args: map[string]any{keyLinodeIDs: []any{"123"}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "zero element", args: map[string]any{keyLinodeIDs: []any{float64(0)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
			{name: "fractional element", args: map[string]any{keyLinodeIDs: []any{1.5}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountEntityTransferCreateTool(cfg)

				req := createRequestWithArgs(t, tt.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, tt.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateAccountEntityTransferRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, []int{123, 456}, got.Entities.Linodes)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountEntityTransfer{
				Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}},
				Status:   statusPending,
				Token:    "transfer-token",
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123), float64(456)}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")
		require.NotEmpty(t, result.Content, "response should include content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "transfer-token", "response should include transfer token")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/entity-transfers", r.URL.Path, "request path should be /account/entity-transfers")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountEntityTransferCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123)}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create linode_account_entity_transfer_create", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

// End-to-end verification of child account proxy user token creation.
func TestLinodeAccountChildAccountTokenTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

		assert.Equal(t, "linode_account_child_account_token", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "token creation should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyEUUID, "schema should include euuid")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

				args := map[string]any{keyEUUID: childAccountEUUID}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid euuid rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing euuid", args: map[string]any{keyConfirm: true}, wantMessage: "euuid is required"},
			{name: "empty euuid", args: map[string]any{keyEUUID: "", keyConfirm: true}, wantMessage: errEUUIDNonEmpty},
			{name: "numeric euuid", args: map[string]any{keyEUUID: 123, keyConfirm: true}, wantMessage: errEUUIDNonEmpty},
			{name: "euuid with slash", args: map[string]any{keyEUUID: "child/account", keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with query separator", args: map[string]any{keyEUUID: "child?account", keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with traversal", args: map[string]any{keyEUUID: pathTraversalValue, keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
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
				_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid euuid should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "euuid validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "token creation should not send a request body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ProxyUserToken{ID: 918, Label: "proxy-token", Scopes: "*", Token: "abcdefghijklmnop", Expiry: "2024-05-01T00:16:01"}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		assertErrorContains(t, result, "abcdefghijklmnop")
		assertErrorContains(t, result, "proxy-token")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token", r.URL.Path, "request path should include child account euuid and token suffix")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_account_child_account_token")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account beta enrollment.
func TestLinodeAccountBetaEnrollTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

		assert.Equal(t, "linode_account_beta_enroll", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "beta enrollment should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyBetaID, "schema should include beta id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)

					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

				args := map[string]any{keyBetaID: betaExampleOpen}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("invalid id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingConfirm, args: map[string]any{keyConfirm: true}, wantMessage: errBetaIDRequired},
			{name: caseEmpty, args: map[string]any{keyBetaID: "", keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
			{name: caseBlank, args: map[string]any{keyBetaID: blankString, keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyBetaID: 123, keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyBetaID: invalidBetaIDSlash, keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: caseQuery, args: map[string]any{keyBetaID: invalidBetaIDQuery, keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue, keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: caseWhitespacePadded, args: map[string]any{keyBetaID: invalidBetaIDPadded, keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: "control", args: map[string]any{keyBetaID: "example\nopen", keyConfirm: true}, wantMessage: errBetaIDChars},
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
				_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid id should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "id validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, betaExampleOpen, body[keyBetaID])

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account beta enrollment requested successfully", "response should contain success message")
		assert.Contains(t, textContent.Text, betaExampleOpen, "response should contain beta id")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/betas", r.URL.Path, "request path should be /account/betas")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to enroll linode_account_beta_enroll")
		assertErrorContains(t, result, errForbidden)
	})
}

// End-to-end verification of account availability retrieval.
func TestLinodeAccountAvailabilityTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

		assert.Equal(t, "linode_account_availability", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		availability := linode.PaginatedResponse[linode.AccountAvailability]{
			Data: []linode.AccountAvailability{{
				Available:   []string{serviceLinodes, serviceNodeBalancers},
				Region:      regionUSEast,
				Unavailable: []string{"Kubernetes", serviceBlockStorage},
			}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(availability))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain region")
		assert.Contains(t, textContent.Text, serviceLinodes, "response should contain available service")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/account/availability", r.URL.Path, "request path should be /account/availability")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_account_availability", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

// End-to-end verification of account agreement acknowledgement.
func TestLinodeAccountAgreementsAcknowledgeTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

		assert.Equal(t, "linode_account_agreements_acknowledge", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "agreement acknowledgement should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, "billing_agreement", "schema should include agreement fields")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)

					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

				args := map[string]any{keyPrivacyPolicy: true}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("empty acknowledgement rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "at least one account agreement field is required")
		assert.Equal(t, int32(0), calls, "empty acknowledgement must fail before client call")
	})

	t.Run("false agreement rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPrivacyPolicy: false, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "privacy_policy must be true when provided")
		assert.Equal(t, int32(0), calls, "false agreement must fail before client call")
	})

	t.Run("malformed field rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPrivacyPolicy: boolStringTrue, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "privacy_policy must be a boolean")
		assert.Equal(t, int32(0), calls, "malformed field must fail before client call")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/agreements", r.URL.Path, "request path should be /account/agreements")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, true, body["billing_agreement"])
			assert.Equal(t, true, body[keyPrivacyPolicy])
			assert.NotContains(t, body, "eu_model", "omitted fields should not be sent")

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"billing_agreement": true, keyPrivacyPolicy: true, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account agreements acknowledged successfully", "response should contain success message")
	})
}

// End-to-end verification of region listing and filtering.
func TestLinodeRegionsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeRegionListTool(cfg)

		assert.Equal(t, "linode_region_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: regionUSEast, Label: "Newark, NJ", Country: countryUS, Capabilities: []string{"Linodes", serviceBlockStorage}, Status: statusOK},
			{ID: regionEUWest, Label: "London, UK", Country: "uk", Capabilities: []string{"Linodes"}, Status: statusOK},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/regions", r.URL.Path, "request path should be /regions")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    regions,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeRegionListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain us-east region")
		assert.Contains(t, textContent.Text, regionEUWest, "response should contain eu-west region")
	})

	t.Run("filter by country", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: regionUSEast, Label: "Newark, NJ", Country: countryUS, Status: statusOK},
			{ID: regionUSWest, Label: "Fremont, CA", Country: countryUS, Status: statusOK},
			{ID: regionEUWest, Label: "London, UK", Country: "uk", Status: statusOK},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    regions,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeRegionListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"country": countryUS})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain us-east")
		assert.Contains(t, textContent.Text, regionUSWest, "response should contain us-west")
		assert.NotContains(t, textContent.Text, regionEUWest, "response should not contain eu-west")
	})
}

// End-to-end verification of type listing and filtering.
func TestLinodeTypesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeTypeListTool(cfg)

		assert.Equal(t, "linode_type_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: typeG6Nanode1, Label: invoiceItemLabel, Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
			{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard, Disk: 81920, Memory: 4096, VCPUs: 2},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/types", r.URL.Path, "request path should be /linode/types")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeTypeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, typeG6Nanode1, "response should contain nanode type")
		assert.Contains(t, textContent.Text, typeG6Standard2, "response should contain standard type")
	})

	t.Run("filter by class", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: typeG6Nanode1, Label: invoiceItemLabel, Class: "nanode"},
			{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard},
			{ID: "g6-standard-4", Label: "Linode 8GB", Class: classStandard},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeTypeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"class": classStandard})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.NotContains(t, textContent.Text, typeG6Nanode1, "response should not contain nanode type")
	})
}

// End-to-end verification of volume listing and filtering.
func TestLinodeVolumesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeVolumeListTool(cfg)

		assert.Equal(t, "linode_volume_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Status: statusActive, Size: 100, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Status: statusActive, Size: 50, Region: regionEUWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes", r.URL.Path, "request path should be /volumes")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelDataVol, "response should contain first volume label")
		assert.Contains(t, textContent.Text, labelBackupVol, "response should contain second volume label")
	})

	t.Run("filter by region", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Region: regionEUWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, labelDataVol, "response should contain matching volume")
		assert.NotContains(t, textContent.Text, labelBackupVol, "response should not contain non-matching volume")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Region: regionEUWest},
			{ID: 3, Label: "data-backup", Region: regionUSWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"label_contains": "backup"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, labelBackupVol, "response should contain backup-vol")
		assert.Contains(t, textContent.Text, "data-backup", "response should contain data-backup")
	})
}

// End-to-end verification of image listing and filtering.
func TestLinodeImagesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeImageListTool(cfg)

		assert.Equal(t, "linode_image_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Type: typeManualImage, IsPublic: true, Deprecated: false},
			{ID: "private/12345", Label: "Custom Image", Type: typeManualImage, IsPublic: false, Deprecated: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/images", r.URL.Path, "request path should be /images")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, imageIDUbuntu2204, "response should contain public image")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
	})

	t.Run("filter by public", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, IsPublic: true},
			{ID: "private/12345", Label: "Custom Image", IsPublic: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"is_public": "false"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
		assert.NotContains(t, textContent.Text, imageIDUbuntu2204, "response should not contain public image")
	})

	t.Run("filter by deprecated", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Deprecated: false},
			{ID: "linode/ubuntu18.04", Label: "Ubuntu 18.04", Deprecated: true},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"deprecated": "true"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "linode/ubuntu18.04", "response should contain deprecated image")
		assert.NotContains(t, textContent.Text, imageIDUbuntu2204, "response should not contain non-deprecated image")
	})
}

func TestLinodeImageShareGroupTokensListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

		assert.Equal(t, "linode_image_sharegroup_tokens_list", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		updated := "2025-08-04T11:09:09"
		expiry := "2025-09-04T10:09:09"
		tokens := []linode.ImageShareGroupToken{
			{
				TokenUUID:              "13428362-5458-4dad-b14b-8d0d4d648f8c",
				Status:                 statusActive,
				Label:                  "Backend Services - Engineering",
				Created:                imageShareGroupTokenCreated,
				Updated:                &updated,
				Expiry:                 &expiry,
				ValidForShareGroupUUID: "e1d0e58b-f89f-4237-84ab-b82077342359",
				ShareGroupUUID:         "e1d0e58b-f89f-4237-84ab-b82077342359",
				ShareGroupLabel:        shareGroupLabelFixture,
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/images/sharegroups/tokens", r.URL.Path, "request path should be /images/sharegroups/tokens")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    tokens,
				keyPage:    2,
				keyPages:   3,
				keyResults: 7,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "response should include count")
		assert.Contains(t, textContent.Text, "Backend Services - Engineering", "response should contain token label")
		assert.Contains(t, textContent.Text, "13428362-5458-4dad-b14b-8d0d4d648f8c", "response should contain token UUID")
	})

	t.Run("invalid pagination", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "invalid page_size should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, errPageSizeRange)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve image share group tokens")
	})
}

func TestLinodeImageShareGroupsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

		assert.Equal(t, "linode_image_sharegroups_list", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := shareGroupDescription
		updated := shareGroupUpdated
		shareGroups := []linode.ImageShareGroup{
			{
				ID:           1,
				UUID:         "1533863e-16a4-47b5-b829-ac0f35c13278",
				Label:        shareGroupLabelFixture,
				Description:  &description,
				IsSuspended:  false,
				Created:      shareGroupCreated,
				Updated:      &updated,
				ImagesCount:  2,
				MembersCount: 3,
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    shareGroups,
				keyPage:    2,
				keyPages:   3,
				keyResults: 7,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "response should include count")
		assert.Contains(t, textContent.Text, shareGroupLabelFixture, "response should contain share group label")
		assert.Contains(t, textContent.Text, "1533863e-16a4-47b5-b829-ac0f35c13278", "response should contain share group UUID")
	})

	t.Run("invalid pagination", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "invalid page_size should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, errPageSizeRange)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve image share groups")
	})
}

// createRequestWithArgs builds a CallToolRequest with the given arguments.
func createRequestWithArgs(t *testing.T, args map[string]any) mcp.CallToolRequest {
	t.Helper()

	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// End-to-end verification of account cancellation.
func TestLinodeAccountCancelTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountCancelTool(cfg)

		assert.Equal(t, "linode_account_cancel", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "account cancellation should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyComments, "schema should include comments")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

				args := map[string]any{keyComments: "leaving"}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("malformed comments rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyComments: 123, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "comments must be a string")
		assert.Equal(t, int32(0), calls, "malformed comments must fail before client call")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "leaving", body[keyComments])

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyComments: "leaving", keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account canceled successfully", "response should contain success message")
		assert.Contains(t, textContent.Text, "https://example.test/survey", "response should contain survey link")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/cancel", r.URL.Path, "request path should be /account/cancel")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"could not charge card"}]}`))
			assert.NoError(t, writeErr)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to cancel account")
		assertErrorContains(t, result, "could not charge card")
	})
}

// End-to-end verification of account update.
func TestLinodeAccountUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUpdateTool(cfg)

		assert.Equal(t, "linode_account_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "account updates should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, "email", "schema should include editable account fields")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)

					w.WriteHeader(http.StatusNoContent)
				}))
				defer srv.Close()

				cfg := &config.Config{
					Environments: map[string]config.EnvironmentConfig{
						envKeyDefault: {
							Label:  envLabelDefault,
							Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
						},
					},
				}
				_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

				args := map[string]any{keyEmail: emailUpdatedExample}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("empty update rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "at least one account field is required")
		assert.Equal(t, int32(0), calls, "empty update must fail before client call")
	})

	t.Run("malformed field rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: 123, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "email must be a string")
		assert.Equal(t, int32(0), calls, "malformed field must fail before client call")
	})
	t.Run("api error produces tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{"field": "email", keyReason: "invalid email format"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update account")
		assertErrorContains(t, result, "invalid email format")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		account := linode.Account{FirstName: nameUpdatedTest, LastName: "User", Email: emailUpdatedExample}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, emailUpdatedExample, body["email"])
			assert.Equal(t, nameUpdatedTest, body["first_name"])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(account))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, "first_name": nameUpdatedTest, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account updated successfully", "response should contain success message")
		assert.Contains(t, textContent.Text, emailUpdatedExample, "response should contain updated email")
	})
}

// End-to-end verification of the SSH key get workflow.
func TestLinodeSSHKeyGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

		assert.Equal(t, "linode_sshkey_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing sshkey id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for missing sshkey_id")
	})

	t.Run("zero sshkey id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(0)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for zero sshkey_id")
	})

	t.Run("negative sshkey id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(-1)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "should reject negative sshkey_id")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		sshKey := linode.SSHKey{
			ID:      42,
			Label:   testKeyLabel,
			SSHKey:  "ssh-rsa AAAA test@example.com",
			Created: "2024-01-01T00:00:00Z",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/profile/sshkeys/42", r.URL.Path, "request path should include SSH key ID")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(sshKey))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(42)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
	})
}

func TestLinodeSSHKeyGetToolAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "Not found"}},
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(999)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should return an error result for API 404")
}

func TestLinodeDomainZoneFileGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

		assert.Equal(t, "linode_domain_zone_file_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "domain zone file get should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyDomainID, "schema should include domain_id")
	})

	t.Run("invalid domain id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseZero, value: 0, set: true},
			{name: "negative", value: -1, set: true},
			{name: "slash", value: "1/2", set: true},
			{name: "query", value: "1?x=2", set: true},
			{name: "path traversal", value: pathTraversalValue, set: true},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

				args := map[string]any{}
				if tt.set {
					args[keyDomainID] = tt.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return a transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "should return an error result")
				assertErrorContains(t, result, "domain_id must be a positive integer")
				assert.Equal(t, int32(0), calls.Load(), "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/domains/123/zone-file", r.URL.Path, "request path should include domain ID and zone-file suffix")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err, "request body should read")
			assert.Empty(t, body, "GET request should not include a body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"zone_file": []string{"; example.com [123]", domainZoneTTL},
			}), "encoding domain zone file response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"zone_file": [`, "response should include zone_file")
		assert.Contains(t, textContent.Text, "; example.com [123]", "response should include zone file line")
		assert.Contains(t, textContent.Text, domainZoneTTL, "response should include TTL line")
	})

	t.Run("api error maps to tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "domain not found"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: 999})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "should return an error result for API 404")
		assertErrorContains(t, result, "Failed to retrieve zone file for domain 999")
	})
}

func TestLinodeDomainRecordGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		assert.Equal(t, "linode_domain_record_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "domain record get should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing domain id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyRecordID: 456})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "domain_id must be a positive integer", "should explain missing domain_id")
	})

	t.Run("missing record id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: 123})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "record_id must be a positive integer", "should explain missing record_id")
	})

	t.Run("negative domain id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: -1, keyRecordID: 456})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "domain_id must be a positive integer", "should explain invalid domain_id")
	})

	t.Run("negative record id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: 123, keyRecordID: -1})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "record_id must be a positive integer", "should explain invalid record_id")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/domains/123/records/456", r.URL.Path, "request path should include domain and record IDs")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"id":      456,
				keyType:   "A",
				keyName:   hostWWW,
				keyTarget: ip203_0_113_1,
			}), "encoding domain record response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyDomainID: 123, keyRecordID: 456})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"id": 456`, "response should contain record ID")
		assert.Contains(t, textContent.Text, hostWWW, "response should contain record name")
		assert.Contains(t, textContent.Text, ip203_0_113_1, "response should contain target")
	})
}

// TestLinodeAccountSettingsManagedEnableTool provides end-to-end verification of enabling Linode Managed.
func TestLinodeAccountSettingsManagedEnableTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

		assert.Equal(t, "linode_account_settings_managed_enable", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "managed enable should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

				args := map[string]any{}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("api error produces tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "managed could not be enabled"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to enable Linode Managed")
		assertErrorContains(t, result, "managed could not be enabled")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/settings/managed-enable", r.URL.Path, "request path should be /account/settings/managed-enable")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Empty(t, body, "request should not include a body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Linode Managed enabled successfully", "response should contain success message")
	})
}

// End-to-end verification of account settings update.
func TestLinodeAccountSettingsUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		assert.Equal(t, "linode_account_settings_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "account settings updates should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, "backups_enabled", "schema should include editable settings fields")
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

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)

					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

				args := map[string]any{"network_helper": false}
				if tt.set {
					args[keyConfirm] = tt.value
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

	t.Run("empty update rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "at least one account settings field is required")
		assert.Equal(t, int32(0), calls, "empty update must fail before client call")
	})

	t.Run("malformed field rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"backups_enabled": "true", keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "backups_enabled must be a boolean")
		assert.Equal(t, int32(0), calls, "malformed field must fail before client call")
	})

	t.Run("malformed string field rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyMaintenancePolicy: 123, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "maintenance_policy must be a string")
		assert.Equal(t, int32(0), calls, "malformed field must fail before client call")
	})

	t.Run("api error produces tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{"field": keyMaintenancePolicy, keyReason: "invalid maintenance policy"}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyMaintenancePolicy: "invalid", keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update account settings")
		assertErrorContains(t, result, "invalid maintenance policy")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		settings := linode.AccountSettings{BackupsEnabled: true, NetworkHelper: false, MaintenancePolicy: maintenancePolicyMigrate}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account/settings", r.URL.Path, "request path should be /account/settings")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, true, body["backups_enabled"])
			assert.Equal(t, false, body["network_helper"])
			assert.Equal(t, maintenancePolicyMigrate, body[keyMaintenancePolicy])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(settings))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"backups_enabled": true, "network_helper": false, keyMaintenancePolicy: maintenancePolicyMigrate, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account settings updated successfully", "response should contain success message")
		assert.Contains(t, textContent.Text, maintenancePolicyMigrate, "response should contain updated settings")
	})
}
