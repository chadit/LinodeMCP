package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	monitorServiceTokenCreateToolName = "linode_monitor_service_token_create"
	monitorServiceTokenToolPath       = "/monitor/services/dbaas/token"
	errMonitorServiceTokenEntityIDs   = "entity_ids must be a non-empty array of positive integers"
)

func monitorServiceTokenCreateArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam: monitorServiceToolTypeDatabase,
		keyEntityIDs:            []any{10, 20},
		keyConfirm:              true,
	}
}

// The package-local assert* helpers report failures and continue; require*
// helpers stop the current test. assertNoError reports and returns false so
// HTTP handler checks can return without calling FailNow from another goroutine.

func TestLinodeMonitorServiceTokenCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)
		assertEqual(t, monitorServiceTokenCreateToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapWrite, capability, "tool should be write-capable")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assertContains(t, tool.InputSchema.Required, keyEntityIDs, "entity IDs should be required")
		assertContains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			assertEqual(t, monitorServiceTokenToolPath, r.URL.Path, "request path should match")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assertNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assertEqual(t, []any{float64(10), float64(20)}, body[keyEntityIDs])

			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyToken: "monitor-token"}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

		req := createRequestWithArgs(t, monitorServiceTokenCreateArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "monitor-token", "response should contain token")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			assertEqual(t, monitorServiceTokenToolPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

		req := createRequestWithArgs(t, monitorServiceTokenCreateArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to create "+monitorServiceTokenCreateToolName, "response should identify failed tool")
		assertContains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseFalseConfirmRejected, value: false, set: true},
			{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
			{name: caseNumericConfirmRejected, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := monitorServiceTokenCreateArgs()
				if !testCase.set {
					delete(args, keyConfirm)
				}

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)
				requireNoError(t, err, "handler should return confirmation failures as tool errors")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, "confirm=true", "response should require confirm=true")
			})
		}
	})

	t.Run("invalid input rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			mutate      func(map[string]any)
			wantMessage string
		}{
			{name: caseSeparatorServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeSlash }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseQueryServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = invalidServiceTypeQuery }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseTraversalServiceType, mutate: func(args map[string]any) { args[monitorServiceTypeParam] = pathTraversalValue }, wantMessage: monitorServiceTypeInvalidError},
			{name: caseMissingEntityIDs, mutate: func(args map[string]any) { delete(args, keyEntityIDs) }, wantMessage: errMonitorServiceTokenEntityIDs},
			{name: "empty entity ids", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{} }, wantMessage: errMonitorServiceTokenEntityIDs},
			{name: "zero entity id", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{0} }, wantMessage: errMonitorServiceTokenEntityIDs},
			{name: caseStringEntityID, mutate: func(args map[string]any) { args[keyEntityIDs] = []any{"10"} }, wantMessage: errMonitorServiceTokenEntityIDs},
			{name: "fractional entity id", mutate: func(args map[string]any) { args[keyEntityIDs] = []any{10.5} }, wantMessage: errMonitorServiceTokenEntityIDs},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := monitorServiceTokenCreateArgs()
				testCase.mutate(args)

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)
				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}
