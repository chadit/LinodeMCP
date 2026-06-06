package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	tagLabelParamTest        = "tag_label"
	tagLabelRequiredMessage  = "tag_label must be a non-empty string"
	tagLabelPathErrorMessage = "tag_label must not contain"
	taggedObjectLabelFixture = "tagged-web-1"
)

func TestLinodeTaggedObjectsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeTaggedObjectsTool(cfg)

		checkEqual(t, "linode_tagged_objects", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		checkNotEmpty(t, tool.Description, "tool should have a description")
		requireNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		checkContains(t, props, tagLabelParamTest, "schema should include tag_label")
		checkContains(t, props, keyPage, "schema should include page")
		checkContains(t, props, keyPageSize, "schema should include page_size")
		checkNoConfirm(t, props, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		objects := linode.PaginatedResponse[linode.TaggedObject]{
			Data:    []linode.TaggedObject{{keyBetaID: float64(123), keyLabel: taggedObjectLabelFixture, keyType: monitorAlertDefinitionToolServiceType}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/tags/prod%2Fweb", r.URL.EscapedPath(), "request path should URL-encode tag label")
			checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(objects))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd + "/web", keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		checkFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, taggedObjectLabelFixture, "response should contain tagged object label")
		checkContains(t, textContent.Text, monitorAlertDefinitionToolServiceType, "response should contain tagged object type")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/tags/"+envProd, r.URL.Path, "request path should be /tags/prod")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{tagLabelParamTest: envProd})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		checkTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, "Failed to retrieve linode_tagged_objects", "response should identify failed tool")
		checkContains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid tag label rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{}, want: tagLabelRequiredMessage},
			{name: caseEmpty, args: map[string]any{tagLabelParamTest: ""}, want: tagLabelRequiredMessage},
			{name: "query", args: map[string]any{tagLabelParamTest: envProd + "?web"}, want: tagLabelPathErrorMessage},
			{name: caseFragment, args: map[string]any{tagLabelParamTest: envProd + "#web"}, want: tagLabelPathErrorMessage},
			{name: "tag label traversal", args: map[string]any{tagLabelParamTest: pathTraversalValue}, want: tagLabelPathErrorMessage},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeTaggedObjectsTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				checkTrue(t, result.IsError, "invalid tag label should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				checkContains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})
}
