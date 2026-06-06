package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	monitorServiceAlertDefinitionDeletePath     = "/monitor/services/dbaas/alert-definitions/20000"
	monitorServiceAlertDefinitionDeleteToolName = "linode_monitor_service_alert_definition_delete"
)

func monitorAlertDefinitionDeleteArgs() map[string]any {
	return map[string]any{
		monitorServiceTypeParam: monitorServiceToolTypeDatabase,
		monitorAlertIDParam:     20000,
		keyConfirm:              true,
	}
}

func TestLinodeMonitorServiceAlertDefinitionDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)
		assertEqual(t, monitorServiceAlertDefinitionDeleteToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapDestroy, capability, "tool should be destructive")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Required, monitorServiceTypeParam, "service type should be required")
		assertContains(t, tool.InputSchema.Required, monitorAlertIDParam, "alert ID should be required")
		assertContains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assertEqual(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Deleted "+monitorServiceAlertDefinitionDeleteToolName, "response should confirm deletion")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assertEqual(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to delete "+monitorServiceAlertDefinitionDeleteToolName, "response should identify failed tool")
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

				args := monitorAlertDefinitionDeleteArgs()
				if !testCase.set {
					delete(args, keyConfirm)
				}

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

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

	t.Run("invalid arguments reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingServiceType, args: map[string]any{monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeRequiredError},
			{name: caseSeparatorServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeSlash, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
			{name: caseQueryServiceType, args: map[string]any{monitorServiceTypeParam: invalidServiceTypeQuery, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
			{name: caseTraversalServiceType, args: map[string]any{monitorServiceTypeParam: pathTraversalValue, monitorAlertIDParam: 20000, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorServiceTypeInvalidError},
			{name: caseMissingAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDRequiredError},
			{name: caseZeroAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: 0, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDPositiveError},
			{name: caseStringAlertID, args: map[string]any{monitorServiceTypeParam: monitorServiceToolTypeDatabase, monitorAlertIDParam: "20000", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: monitorAlertIDPositiveError},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "invalid argument should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})

	t.Run("transient error is not replayed", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assertEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assertEqual(t, monitorServiceAlertDefinitionDeletePath, r.URL.Path, "request path should match")
			http.Error(w, "temporary", http.StatusServiceUnavailable)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

		req := createRequestWithArgs(t, monitorAlertDefinitionDeleteArgs())
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should surface transient failure as a tool error")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "transient failure should be an error result")
		assertEqual(t, int32(1), calls.Load(), "destructive route must not retry after transient failure")
	})
}
