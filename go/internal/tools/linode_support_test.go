package tools_test

import (
	"context"
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
	supportTicketIDKey     = "ticket_id"
	supportTicketSummary   = "Cannot reach managed instance"
	supportTicketOpenedBy  = "adevi"
	errSupportTicketID     = "ticket_id must be a positive integer"
	caseFractionalTicketID = "fractional ticket id"
)

// End-to-end verification of support ticket listing.
func TestLinodeSupportTicketsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeSupportTicketsTool(cfg)

		assert.Equal(t, "linode_support_tickets", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tickets := linode.PaginatedResponse[linode.SupportTicket]{
			Data:    []linode.SupportTicket{{ID: 11111, Summary: supportTicketSummary, Status: "ticket-open", OpenedBy: supportTicketOpenedBy}},
			Page:    2,
			Pages:   3,
			Results: 75,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(tickets))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeSupportTicketsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, supportTicketSummary, "response should contain ticket summary")
		assert.Contains(t, textContent.Text, supportTicketOpenedBy, "response should contain opener")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeSupportTicketsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_support_tickets", "response should identify failed tool")
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
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeSupportTicketsTool(cfg)
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

func TestLinodeSupportTicketGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeSupportTicketGetTool(cfg)

		assert.Equal(t, "linode_support_ticket_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, supportTicketIDKey, "schema should include ticket_id")
		assert.Contains(t, tool.InputSchema.Required, supportTicketIDKey, "ticket_id must be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ticket := linode.SupportTicket{ID: 11111, Summary: supportTicketSummary, Status: "ticket-open", OpenedBy: supportTicketOpenedBy}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/support/tickets/11111", r.URL.Path, "request path should include ticket ID")
			assert.Empty(t, r.URL.RawQuery, "get ticket should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ticket))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeSupportTicketGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, supportTicketSummary, "response should contain ticket summary")
		assert.Contains(t, textContent.Text, supportTicketOpenedBy, "response should contain opener")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/support/tickets/11111", r.URL.Path, "request path should include ticket ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeSupportTicketGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_support_ticket_get", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid ticket id rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissing, args: map[string]any{}, wantMessage: "ticket_id is required"},
			{name: caseZero, args: map[string]any{supportTicketIDKey: 0}, wantMessage: errSupportTicketID},
			{name: caseNegative, args: map[string]any{supportTicketIDKey: -1}, wantMessage: errSupportTicketID},
			{name: caseString, args: map[string]any{supportTicketIDKey: "11111"}, wantMessage: errSupportTicketID},
			{name: caseFractionalTicketID, args: map[string]any{supportTicketIDKey: 1.5}, wantMessage: errSupportTicketID},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeSupportTicketGetTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid ticket_id should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeSupportTicketCloseTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeSupportTicketCloseTool(&config.Config{})

		assert.Equal(t, "linode_support_ticket_close", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "closing support ticket mutates state")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, supportTicketIDKey, "schema should include ticket_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, supportTicketIDKey, "ticket_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("requires confirm before client", func(t *testing.T) {
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

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				handler, cleanup := newSupportTicketCloseHandler(t, &calls)
				defer cleanup()

				args := map[string]any{supportTicketIDKey: float64(11111)}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls.Load(), "confirm failure must happen before client call")
			})
		}
	})

	t.Run("rejects invalid ticket id", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: "missing ticket id", args: map[string]any{keyConfirm: true}},
			{name: "zero ticket id", args: map[string]any{supportTicketIDKey: float64(0), keyConfirm: true}},
			{name: "fractional ticket id", args: map[string]any{supportTicketIDKey: float64(1.5), keyConfirm: true}},
			{name: "string ticket id", args: map[string]any{supportTicketIDKey: "11111", keyConfirm: true}},
			{name: "slash ticket id", args: map[string]any{supportTicketIDKey: "11/111", keyConfirm: true}},
			{name: "query ticket id", args: map[string]any{supportTicketIDKey: "11111?x=1", keyConfirm: true}},
			{name: "traversal ticket id", args: map[string]any{supportTicketIDKey: pathTraversalValue, keyConfirm: true}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				handler, cleanup := newSupportTicketCloseHandler(t, &calls)
				defer cleanup()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid ticket_id should be an error result")
				assertErrorContains(t, result, supportTicketIDKey)
				assert.Equal(t, int32(0), calls.Load(), "validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/support/tickets/11111/close", r.URL.Path, "request path should close the support ticket")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		_, _, handler := tools.NewLinodeSupportTicketCloseTool(supportTicketCloseConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Support ticket closed successfully", "response should include success message")
		assert.Contains(t, textContent.Text, "11111", "response should include ticket ID")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/support/tickets/11111/close", r.URL.Path, "request path should close the support ticket")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		_, _, handler := tools.NewLinodeSupportTicketCloseTool(supportTicketCloseConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to close linode_support_ticket_close")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeSupportTicketCloseTool(dryRunNoCallServer(t))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyDryRun: true}))

		require.NoError(t, err, "dry run should not return a handler error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should succeed without confirm")
		body := decodeSupportToolJSON(t, result)
		assert.Equal(t, "linode_support_ticket_close", body["tool"])
		would, ok := body["would_execute"].(map[string]any)
		require.True(t, ok, "would_execute should be an object")
		assert.Equal(t, http.MethodPost, would["method"])
		assert.Equal(t, "/support/tickets/11111/close", would["path"])
	})
}

func supportTicketCloseConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func newSupportTicketCloseHandler(t *testing.T, calls *atomic.Int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeSupportTicketCloseTool(supportTicketCloseConfig(srv.URL))

	return handler, srv.Close
}

func decodeSupportToolJSON(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))

	return body
}
