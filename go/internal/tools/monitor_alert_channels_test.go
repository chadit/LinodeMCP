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
	monitorAlertChannelsToolPath     = "/monitor/alert-channels"
	monitorAlertChannelsToolName     = "linode_monitor_alert_channels"
	monitorAlertChannelsToolQuery    = "page=2&page_size=25"
	monitorAlertChannelToolID        = 10000
	monitorAlertChannelToolLabel     = "Read-Write Channel"
	monitorAlertChannelToolEmail     = "Users-with-read-write-access-to-resources"
	monitorAlertChannelToolSystem    = "system"
	monitorAlertChannelToolEmailType = "email"
)

func TestLinodeMonitorAlertChannelsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeMonitorAlertChannelsTool(cfg)
		assertEqual(t, monitorAlertChannelsToolName, tool.Name, "tool name should match")
		assertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, monitorAlertChannelsToolPath, r.URL.Path, "request path should match")
			assertEqual(t, monitorAlertChannelsToolQuery, r.URL.RawQuery, "request query should include pagination")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{
					keyID:          monitorAlertChannelToolID,
					keyLabel:       monitorAlertChannelToolLabel,
					keyType:        monitorAlertChannelToolSystem,
					"channel_type": monitorAlertChannelToolEmailType,
					"content": map[string]any{
						monitorAlertChannelToolEmailType: map[string]any{
							"email_addresses": []string{monitorAlertChannelToolEmail},
						},
					},
				}},
				keyPage:    2,
				keyPages:   3,
				keyResults: 75,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorAlertChannelsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, monitorAlertChannelToolLabel, "response should contain channel label")
		assertContains(t, textContent.Text, monitorAlertChannelToolEmail, "response should contain email recipient")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, monitorAlertChannelsToolPath, r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeMonitorAlertChannelsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		requireNoError(t, err, "handler should return API failures as tool errors")
		requireNotNil(t, result, "result should not be nil")
		assertTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to retrieve "+monitorAlertChannelsToolName, "response should identify failed tool")
		assertContains(t, textContent.Text, errForbidden, "response should include API error detail")
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
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorAlertChannelsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				requireNoError(t, err, "handler should return validation as a tool error")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}
