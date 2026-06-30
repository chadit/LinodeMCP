package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	supportTicketIDKey         = "ticket_id"
	supportTicketSummary       = "Cannot reach managed instance"
	supportTicketOpenedBy      = "adevi"
	errSupportTicketID         = "ticket_id must be a positive integer"
	errSupportTicketIDRequired = "ticket_id is required"
	caseFractionalTicketID     = "fractional ticket id"
	caseMissingTicketID        = "missing ticket id"
	caseZeroTicketID           = "zero ticket id"
)

// End-to-end verification of support ticket listing.
func TestLinodeSupportTicketsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeSupportTicketsTool(cfg)

	if tool.Name != "linode_support_ticket_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_support_ticket_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeSupportTicketsToolSuccess(t *testing.T) {
	t.Parallel()

	tickets := linode.PaginatedResponse[linode.SupportTicket]{
		Data:    []linode.SupportTicket{{ID: 11111, Summary: supportTicketSummary, Status: "ticket-open", OpenedBy: supportTicketOpenedBy}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(tickets); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, supportTicketSummary) {
		t.Errorf("textContent.Text does not contain %v", supportTicketSummary)
	}

	if !strings.Contains(textContent.Text, supportTicketOpenedBy) {
		t.Errorf("textContent.Text does not contain %v", supportTicketOpenedBy)
	}
}

func TestLinodeSupportTicketsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeSupportTicketsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeSupportTicketRepliesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeSupportTicketRepliesTool(cfg)

	if tool.Name != "linode_support_ticket_reply_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_support_ticket_reply_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[supportTicketIDKey]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", supportTicketIDKey)
	}

	if !slices.Contains(tool.InputSchema.Required, supportTicketIDKey) {
		t.Errorf("tool.InputSchema.Required does not contain %v", supportTicketIDKey)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeSupportTicketRepliesToolSuccess(t *testing.T) {
	t.Parallel()

	replies := linode.PaginatedResponse[linode.SupportTicketReply]{
		Data:    []linode.SupportTicketReply{{ID: 22222, Description: "We are investigating this ticket.", CreatedBy: supportTicketOpenedBy}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/support/tickets/11111/replies" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111/replies")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(replies); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketRepliesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111, keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "We are investigating this ticket.") {
		t.Errorf("textContent.Text does not contain %v", "We are investigating this ticket.")
	}

	if !strings.Contains(textContent.Text, supportTicketOpenedBy) {
		t.Errorf("textContent.Text does not contain %v", supportTicketOpenedBy)
	}
}

func TestLinodeSupportTicketRepliesToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/support/tickets/11111/replies" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111/replies")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketRepliesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeSupportTicketRepliesToolInvalidArgumentsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{}, wantMessage: errSupportTicketIDRequired},
		{name: caseZero, args: map[string]any{supportTicketIDKey: 0}, wantMessage: errSupportTicketID},
		{name: caseNegative, args: map[string]any{supportTicketIDKey: -1}, wantMessage: errSupportTicketID},
		{name: caseString, args: map[string]any{supportTicketIDKey: "11111"}, wantMessage: errSupportTicketID},
		{name: "path separator ticket id", args: map[string]any{supportTicketIDKey: pathSeparatorValue}, wantMessage: errSupportTicketID},
		{name: "query separator ticket id", args: map[string]any{supportTicketIDKey: querySeparatorValue}, wantMessage: errSupportTicketID},
		{name: "traversal ticket id", args: map[string]any{supportTicketIDKey: pathTraversalValue}, wantMessage: errSupportTicketID},
		{name: caseFractionalTicketID, args: map[string]any{supportTicketIDKey: 1.5}, wantMessage: errSupportTicketID},
		{name: paginationCasePageZero, args: map[string]any{supportTicketIDKey: 11111, keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageSizeString, args: map[string]any{supportTicketIDKey: 11111, keyPageSize: "25"}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeSupportTicketRepliesTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeSupportTicketGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeSupportTicketGetTool(cfg)

	if tool.Name != "linode_support_ticket_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_support_ticket_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), supportTicketIDKey) {
		t.Errorf("tool.RawInputSchema missing key %v", supportTicketIDKey)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeSupportTicketGetToolSuccess(t *testing.T) {
	t.Parallel()

	ticket := linode.SupportTicket{ID: 11111, Summary: supportTicketSummary, Status: "ticket-open", OpenedBy: supportTicketOpenedBy}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/support/tickets/11111" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(ticket); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, supportTicketSummary) {
		t.Errorf("textContent.Text does not contain %v", supportTicketSummary)
	}

	if !strings.Contains(textContent.Text, supportTicketOpenedBy) {
		t.Errorf("textContent.Text does not contain %v", supportTicketOpenedBy)
	}
}

func TestLinodeSupportTicketGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/support/tickets/11111" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeSupportTicketGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{supportTicketIDKey: 11111})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_support_ticket_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_support_ticket_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeSupportTicketGetToolInvalidTicketIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{}, wantMessage: errSupportTicketIDRequired},
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeSupportTicketCloseToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeSupportTicketCloseTool(&config.Config{})

	if tool.Name != "linode_support_ticket_close" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_support_ticket_close")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[supportTicketIDKey]; !ok {
		t.Errorf("props missing key %v", supportTicketIDKey)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}

	for _, key := range []string{supportTicketIDKey, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeSupportTicketCloseToolRequiresConfirmBeforeClient(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeSupportTicketCloseToolRejectsInvalidTicketId(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingTicketID, args: map[string]any{keyConfirm: true}},
		{name: caseZeroTicketID, args: map[string]any{supportTicketIDKey: float64(0), keyConfirm: true}},
		{name: caseFractionalTicketID, args: map[string]any{supportTicketIDKey: float64(1.5), keyConfirm: true}},
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, supportTicketIDKey) {
				t.Errorf("error text %q does not contain %q", text.Text, supportTicketIDKey)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeSupportTicketCloseToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/11111/close" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111/close")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeSupportTicketCloseTool(supportTicketCloseConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Support ticket closed successfully") {
		t.Errorf("textContent.Text does not contain %v", "Support ticket closed successfully")
	}

	if !strings.Contains(textContent.Text, "11111") {
		t.Errorf("textContent.Text does not contain %v", "11111")
	}
}

func TestLinodeSupportTicketCloseToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/11111/close" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/11111/close")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeSupportTicketCloseTool(supportTicketCloseConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to close linode_support_ticket_close") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to close linode_support_ticket_close")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeSupportTicketCloseToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeSupportTicketCloseTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketIDKey: float64(11111), keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	body := decodeSupportToolJSON(t, result)
	if !reflect.DeepEqual(body["tool"], "linode_support_ticket_close") {
		t.Errorf("got %v, want %v", body["tool"], "linode_support_ticket_close")
	}

	would, ok := body["would_execute"].(map[string]any)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !reflect.DeepEqual(would["method"], http.MethodPost) {
		t.Errorf("got %v, want %v", would["method"], http.MethodPost)
	}

	if !reflect.DeepEqual(would["path"], "/support/tickets/11111/close") {
		t.Errorf("got %v, want %v", would["path"], "/support/tickets/11111/close")
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "11111") {
		t.Errorf("effect does not contain %v", "11111")
	}
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
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return body
}
