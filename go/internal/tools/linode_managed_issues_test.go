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
	managedIssueGetToolName    = "linode_managed_issue_get"
	managedIssueIDParam        = "issue_id"
	managedIssueIDValue        = 823
	managedIssueGetToolPath    = "/managed/issues/823"
	managedIssueOversizedID    = 9007199254740992.0
	managedIssueToolCreated    = "2018-01-01T00:01:01"
	managedIssuesToolPath      = "/managed/issues"
	managedIssuesToolLabel     = "Managed Issue opened!"
	managedIssuesToolEntityURL = "/support/tickets/98765"
)

func TestLinodeManagedIssueGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedIssueGetTool(cfg)

		assert.Equal(t, managedIssueGetToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "managed issue lookup should be CapRead")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, managedIssueIDParam, "schema should include issue_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
		assert.Contains(t, tool.InputSchema.Required, managedIssueIDParam, "issue_id must be marked required")
	})

	t.Run("invalid issue id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: "missing issue id", args: map[string]any{}},
			{name: "zero issue id", args: map[string]any{managedIssueIDParam: 0}},
			{name: "negative issue id", args: map[string]any{managedIssueIDParam: -1}},
			{name: "string issue id", args: map[string]any{managedIssueIDParam: "823"}},
			{name: "fractional issue id", args: map[string]any{managedIssueIDParam: 823.5}},
			{name: "oversized issue id", args: map[string]any{managedIssueIDParam: managedIssueOversizedID}},
			{name: "slash issue id", args: map[string]any{managedIssueIDParam: pathSeparatorValue}},
			{name: "query issue id", args: map[string]any{managedIssueIDParam: querySeparatorValue}},
			{name: "traversal issue id", args: map[string]any{managedIssueIDParam: pathTraversalValue}},
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

				cfg := managedIssueConfig(srv.URL)
				_, _, handler := tools.NewLinodeManagedIssueGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, "issue_id")
				assert.Equal(t, int32(0), calls.Load(), "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedIssueGetToolPath, r.URL.Path, "request path should include issue ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedIssue{
				ID:       managedIssueIDValue,
				Created:  managedIssueToolCreated,
				Services: []int{654},
				Entity: linode.ManagedIssueEntity{
					ID:    98765,
					Label: managedIssuesToolLabel,
					Type:  "ticket",
					URL:   managedIssuesToolEntityURL,
				},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedIssueGetTool(managedIssueConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedIssueIDParam: managedIssueIDValue}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedIssuesToolLabel, "response should contain issue ticket label")
		assert.Contains(t, textContent.Text, managedIssuesToolEntityURL, "response should contain issue ticket URL")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedIssueGetToolPath, r.URL.Path, "request path should include issue ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedIssueGetTool(managedIssueConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedIssueIDParam: managedIssueIDValue}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_managed_issue_get")
		assertErrorContains(t, result, errForbidden)
	})
}

func managedIssueConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func TestLinodeManagedIssuesTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedIssuesTool(cfg)

		assert.Equal(t, "linode_managed_issues", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		issues := linode.PaginatedResponse[linode.ManagedIssue]{
			Data: []linode.ManagedIssue{{
				ID:       823,
				Created:  "2018-01-01T00:01:01",
				Services: []int{654},
				Entity: linode.ManagedIssueEntity{
					ID:    98765,
					Label: managedIssuesToolLabel,
					Type:  "ticket",
					URL:   managedIssuesToolEntityURL,
				},
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedIssuesToolPath, r.URL.Path, "request path should be /managed/issues")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(issues))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedIssuesToolLabel, "response should contain issue ticket label")
		assert.Contains(t, textContent.Text, managedIssuesToolEntityURL, "response should contain issue ticket URL")
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
				_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)
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
			assert.Equal(t, managedIssuesToolPath, r.URL.Path, "request path should be /managed/issues")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_managed_issues", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}
