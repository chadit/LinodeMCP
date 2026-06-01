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

func TestLinodeTagDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeTagDeleteTool(cfg)

		assert.Equal(t, "linode_tag_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destructive")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, tagLabelParamTest, "schema should include tag_label")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, tagLabelParamTest, "tag_label must be required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/tags/prod%2Fweb", r.URL.EscapedPath(), "request path should URL-encode tag label")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, envProd, "response should include deleted tag label")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissing, args: map[string]any{tagLabelParamTest: envProd}},
			{name: caseFalse, args: map[string]any{tagLabelParamTest: envProd, keyConfirm: false}},
			{name: caseString, args: map[string]any{tagLabelParamTest: envProd, keyConfirm: boolStringTrue}},
			{name: "number", args: map[string]any{tagLabelParamTest: envProd, keyConfirm: 1}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeTagDeleteTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm must be true", "response should describe confirm requirement")
			})
		}
	})

	t.Run("invalid tag label rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing tag", args: map[string]any{keyConfirm: true}, want: tagLabelRequiredMessage},
			{name: caseEmpty, args: map[string]any{tagLabelParamTest: "", keyConfirm: true}, want: tagLabelRequiredMessage},
			{name: "query", args: map[string]any{tagLabelParamTest: envProd + "?web", keyConfirm: true}, want: tagLabelPathErrorMessage},
			{name: caseFragment, args: map[string]any{tagLabelParamTest: envProd + "#web", keyConfirm: true}, want: tagLabelPathErrorMessage},
			{name: "tag label traversal", args: map[string]any{tagLabelParamTest: pathTraversalValue, keyConfirm: true}, want: tagLabelPathErrorMessage},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeTagDeleteTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid tag label should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})

	t.Run("dry run previews without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/tags/"+envProd+"/web", linode.PaginatedResponse[linode.TaggedObject]{
			Data: []linode.TaggedObject{{keyLabel: taggedObjectLabelFixture}},
		})
		_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should not be an error result")
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read current tag state")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "dry_run", "response should identify dry run")
		assert.Contains(t, textContent.Text, "DELETE", "response should describe planned method")
		assert.Contains(t, textContent.Text, "/tags/prod%2Fweb", "response should include encoded delete path")
		assert.Contains(t, textContent.Text, "dependencies", "delete should surface tagged objects as dependencies")
		assert.Contains(t, textContent.Text, "removed", "tagged objects are untagged, not deleted")
		assert.Contains(t, textContent.Text, "tagged object", "warning should state the tagged-object count")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/tags/"+envProd, r.URL.Path, "request path should be /tags/prod")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode_tag_delete failed", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeTagsTool(t *testing.T) {
	t.Parallel()

	const tagLabel = "production"

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeTagsTool(cfg)

		assert.Equal(t, "linode_tags", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tags := linode.PaginatedResponse[linode.Tag]{
			Data:    []linode.Tag{{Label: tagLabel}},
			Page:    2,
			Pages:   3,
			Results: 51,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(tags))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, tagLabel, "response should contain tag label")
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
				_, _, handler := tools.NewLinodeTagsTool(cfg)
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
			assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_tags", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

const (
	toolLinodeTagCreate         = "linode_tag_create"
	tagCreateLabelFixture       = "production"
	tagCreateSuccessMessage     = "Tag 'production' created successfully"
	tagCreateConfirmError       = "This creates a Linode tag. Set confirm=true to proceed."
	tagCreateDomainsParam       = "domains"
	tagCreateLinodesParam       = "linodes"
	tagCreateNodeBalancersParam = "nodebalancers"
	tagCreateVolumesParam       = "volumes"
)

func TestLinodeTagCreateToolLabelOnlySuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.Equal(t, map[string]any{keyLabel: tagCreateLabelFixture}, body, "label-only create should omit optional resource arrays")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Tag{Label: tagCreateLabelFixture}))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeTagCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: tagCreateLabelFixture, keyConfirm: true})
	result, err := handler(t.Context(), req)

	require.NoError(t, err, "handler should not return an error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "should not be an error result")
}

func TestLinodeTagCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeTagCreateTool(cfg)

		assert.Equal(t, toolLinodeTagCreate, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, tagCreateDomainsParam, "schema should include domains")
		assert.Contains(t, props, tagCreateLinodesParam, "schema should include linodes")
		assert.Contains(t, props, tagCreateNodeBalancersParam, "schema should include nodebalancers")
		assert.Contains(t, props, tagCreateVolumesParam, "schema should include volumes")
		assert.Contains(t, props, keyConfirm, "mutating tag tool must expose confirm")
		assert.Contains(t, props, "dry_run", "mutating tag tool must support dry_run")
		assert.Contains(t, tool.InputSchema.Required, keyLabel, "label must be marked required")
		assert.NotContains(t, tool.InputSchema.Required, keyConfirm, "confirm is enforced by handler so dry_run can omit it")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr)

			if decodeErr != nil {
				return
			}

			assert.Equal(t, tagCreateLabelFixture, body[keyLabel])
			assert.Equal(t, []any{float64(101), float64(102)}, body[tagCreateLinodesParam])
			assert.Equal(t, []any{float64(201)}, body[tagCreateDomainsParam])
			assert.Equal(t, []any{float64(301)}, body[tagCreateNodeBalancersParam])
			assert.Equal(t, []any{float64(401)}, body[tagCreateVolumesParam])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.Tag{Label: tagCreateLabelFixture}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:                    tagCreateLabelFixture,
			tagCreateLinodesParam:       []any{float64(101), float64(102)},
			tagCreateDomainsParam:       []any{float64(201)},
			tagCreateNodeBalancersParam: []any{float64(301)},
			tagCreateVolumesParam:       []any{float64(401)},
			keyConfirm:                  true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, tagCreateSuccessMessage)
	})

	t.Run("dry run previews request without confirm or client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeTagCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:              tagCreateLabelFixture,
			tagCreateLinodesParam: []any{float64(101)},
			"dry_run":             true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"method": "POST"`)
		assert.Contains(t, textContent.Text, `"path": "/tags"`)
		assert.Contains(t, textContent.Text, tagCreateLabelFixture)
		assert.Contains(t, textContent.Text, "side_effects")
		assert.Contains(t, textContent.Text, "new tag")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTagCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyLabel: tagCreateLabelFixture, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create tag", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumeric, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeTagCreateTool(cfg)

				args := map[string]any{keyLabel: tagCreateLabelFixture}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, tagCreateConfirmError, "response should require confirmation")
			})
		}
	})

	t.Run("invalid inputs reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissingLabel, args: map[string]any{keyConfirm: true}, want: errLabelRequired},
			{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLabel: blankString, keyConfirm: true}, want: errLabelRequired},
			{name: "string linode ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{"101"}, keyConfirm: true}, want: "linodes must be an array of positive integers"},
			{name: caseZeroLinodeID, args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateLinodesParam: []any{float64(0)}, keyConfirm: true}, want: "linodes must be an array of positive integers"},
			{name: "empty domain ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateDomainsParam: []any{}, keyConfirm: true}, want: "domains must include at least one ID"},
			{name: "non-array nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: "301", keyConfirm: true}, want: "nodebalancers must be an array of positive integers"},
			{name: "fractional nodebalancer ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateNodeBalancersParam: []any{float64(301.5)}, keyConfirm: true}, want: "nodebalancers must be an array of positive integers"},
			{name: "negative volume ids", args: map[string]any{keyLabel: tagCreateLabelFixture, tagCreateVolumesParam: []any{float64(-401)}, keyConfirm: true}, want: "volumes must be an array of positive integers"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeTagCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want)
			})
		}
	})
}
