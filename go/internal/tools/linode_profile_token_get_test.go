package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	profileTokenGetToolName = "linode_profile_token_get"
	keyTokenID              = "token_id"
)

func TestLinodeProfileTokenGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileTokenGetTool(cfg)

		checkEqual(t, profileTokenGetToolName, tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "profile token lookup should be CapRead")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, keyTokenID, "schema should include token_id")

		if contains(props, keyConfirm) {
			t.Errorf("expected %v not to contain %v%s", props, keyConfirm, expectationMessage([]string{"read-only get tool must not require confirm"}))
		}

		expectContainsWithMode(t, false, tool.InputSchema.Required, keyTokenID, "token_id must be marked required")
	})

	t.Run("invalid token id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: "missing token_id", args: map[string]any{}},
			{name: "zero token_id", args: map[string]any{keyTokenID: 0}},
			{name: "slash token_id", args: map[string]any{keyTokenID: "12/34"}},
			{name: "query token_id", args: map[string]any{keyTokenID: "12?34"}},
			{name: "traversal token_id", args: map[string]any{keyTokenID: pathTraversalValue}},
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
				_, _, handler := tools.NewLinodeProfileTokenGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				expectNoError(t, err, "handler should not return transport error")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, "token_id must be a positive integer")
				checkEqual(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: "api-token", profileTokenScopesParam: "*"}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileTokenGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyTokenID: 12345}))

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "api-token", "response should include token label")
		expectContainsWithMode(t, false, textContent.Text, `"id": 12345`, "response should include token ID")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileTokenGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyTokenID: 12345}))

		expectNoError(t, err, "handler should return API failures as tool errors")
		expectNotNil(t, result, "result should not be nil")
		checkTrueWithMode(t, false, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_profile_token_get")
		assertErrorContains(t, result, errForbidden)
	})
}
