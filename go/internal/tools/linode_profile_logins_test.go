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

func TestLinodeProfileLoginsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileLoginsTool(cfg)

		assert.Equal(t, "linode_profile_logins", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "profile logins should be read-only")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "page", "schema should expose optional page")
		assert.Contains(t, props, "page_size", "schema should expose optional page_size")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/profile/logins", r.URL.Path, "request path should be /profile/logins")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
				Data: []linode.AccountLogin{{ID: 321, Username: accountLoginUsername, IP: "203.0.113.20"}},
				Page: 2,
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
		_, _, handler := tools.NewLinodeProfileLoginsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2.0, keyPageSize: 25.0})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return a Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "success should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountLoginUsername, "response should include login username")
		assert.Contains(t, textContent.Text, "203.0.113.20", "response should include login IP")
	})

	t.Run("invalid pagination", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeProfileLoginsTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation failures are returned as error results")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be rejected before client call")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe pagination validation")
			})
		}
	})

	t.Run("upstream error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "forbidden"}}}))
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
		_, _, handler := tools.NewLinodeProfileLoginsTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))

		require.NoError(t, err, "upstream failures are returned as error results")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_profile_logins")
	})
}
