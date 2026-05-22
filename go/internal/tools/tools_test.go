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
			{name: "empty euuid", args: map[string]any{keyEUUID: ""}, wantMessage: "euuid must be a non-empty string"},
			{name: "numeric euuid", args: map[string]any{keyEUUID: 123}, wantMessage: "euuid must be a non-empty string"},
			{name: "euuid with slash", args: map[string]any{keyEUUID: "child/account"}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with query separator", args: map[string]any{keyEUUID: "child?account"}, wantMessage: errEUUIDNoSeparators},
			{name: "euuid with traversal", args: map[string]any{keyEUUID: ".."}, wantMessage: errEUUIDNoSeparators},
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
			{name: "blank", args: map[string]any{keyBetaID: "   "}, wantMessage: errBetaIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyBetaID: 123}, wantMessage: errBetaIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyBetaID: "example/open"}, wantMessage: errBetaIDChars},
			{name: caseQuery, args: map[string]any{keyBetaID: "example?open=1"}, wantMessage: errBetaIDChars},
			{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue}, wantMessage: errBetaIDChars},
			{name: "whitespace padded", args: map[string]any{keyBetaID: " example_open "}, wantMessage: errBetaIDChars},
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
			{name: "blank", args: map[string]any{keyBetaID: "   ", keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
			{name: caseNumeric, args: map[string]any{keyBetaID: 123, keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyBetaID: "example/open", keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: caseQuery, args: map[string]any{keyBetaID: "example?open=1", keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue, keyConfirm: true}, wantMessage: errBetaIDChars},
			{name: "whitespace padded", args: map[string]any{keyBetaID: " example_open ", keyConfirm: true}, wantMessage: errBetaIDChars},
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
			{ID: typeG6Nanode1, Label: "Nanode 1GB", Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
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
			{ID: typeG6Nanode1, Label: "Nanode 1GB", Class: "nanode"},
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
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Type: "manual", IsPublic: true, Deprecated: false},
			{ID: "private/12345", Label: "Custom Image", Type: "manual", IsPublic: false, Deprecated: false},
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
			{name: "missing", set: false},
			{name: "confirm false", value: false, set: true},
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
