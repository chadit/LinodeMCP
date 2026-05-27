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

func TestLinodeIPv6RangesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeIPv6RangesListTool(&config.Config{})

		assert.Equal(t, "linode_ipv6_ranges_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success with pagination", func(t *testing.T) {
		t.Parallel()

		ranges := linode.PaginatedResponse[linode.IPv6Range]{
			Data: []linode.IPv6Range{{
				Range:  ipv6RangeFixture,
				Region: regionUSEast,
				Prefix: 124,
			}},
			Page:    2,
			Pages:   3,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/ipv6/ranges", r.URL.Path, "request path should match")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ranges))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, ipv6RangeFixture, "response should include range value")
		assert.Contains(t, textContent.Text, regionUSEast, "response should include region")
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/ipv6/ranges", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))

		require.NoError(t, err, "handler should return tool errors without Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_ipv6_ranges_list", "error should name the failing tool")
	})

	t.Run("invalid pagination rejects before client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return tool errors without Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "error should explain invalid pagination")
			})
		}
	})
}

func TestLinodeIPv6RangeCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("create definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})

		assert.Equal(t, "linode_ipv6_range_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyPrefixLength, "schema should include prefix_length")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyRouteTarget, "schema should include route_target")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("create success", func(t *testing.T) {
		t.Parallel()

		createdRange := linode.IPv6Range{Range: ipv6RangeFixture, Region: regionUSEast, Prefix: 124, RouteTarget: ipv6RouteTarget}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/networking/ipv6/ranges", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body linode.CreateIPv6RangeRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			if assert.NotNil(t, body.LinodeID, "linode_id should be sent") {
				assert.Equal(t, 12345, *body.LinodeID, "linode_id should match")
			}

			assert.Equal(t, 124, body.PrefixLength, "prefix_length should match")
			assert.Equal(t, ipv6RouteTarget, body.RouteTarget, "route_target should match")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(createdRange))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPrefixLength: 124,
			keyLinodeID:     12345,
			keyRouteTarget:  ipv6RouteTarget,
			keyConfirm:      true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "IPv6 range created", "response should include success message")
		assert.Contains(t, textContent.Text, ipv6RangeFixture, "response should include range value")
	})

	t.Run("create confirm required", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})
		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingConfirm, args: map[string]any{keyPrefixLength: 124}},
			{name: caseConfirmFalse, args: map[string]any{keyPrefixLength: 124, keyConfirm: false}},
			{name: caseString, args: map[string]any{keyPrefixLength: 124, keyConfirm: boolStringTrue}},
			{name: caseNumeric, args: map[string]any{keyPrefixLength: 124, keyConfirm: 1}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return tool errors without Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm=true", "error should require confirmation")
			})
		}
	})

	t.Run("create validation rejects before client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(cfg)
		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing prefix length", args: map[string]any{keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
			{name: "string prefix length", args: map[string]any{keyPrefixLength: "124", keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
			{name: "large prefix length", args: map[string]any{keyPrefixLength: 129, keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
			{name: caseZeroLinodeID, args: map[string]any{keyPrefixLength: 124, keyLinodeID: 0, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: "string route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: 123, keyConfirm: true}, wantMessage: "route_target must be a non-empty string"},
			{name: "malformed route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "not-an-ip", keyConfirm: true}, wantMessage: "route_target must be a valid IPv6 address"},
			{name: "ipv4 route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "192.0.2.1", keyConfirm: true}, wantMessage: "route_target must be a valid IPv6 address"},
			{name: "empty route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "", keyConfirm: true}, wantMessage: "route_target must be a non-empty string"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return tool errors without Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid args should be an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "error should explain invalid input")
			})
		}
	})
}
