package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	supportTicketIDKey         = "ticket_id"
	supportTicketSummary       = "Cannot reach managed instance"
	supportTicketOpenedBy      = "adevi"
	errSupportTicketID         = "ticket_id must be a positive integer"
	errSupportTicketIDRequired = "ticket_id is required"
	caseFractionalTicketID     = "fractional ticket id"
)

// End-to-end verification of support ticket listing.
func TestLinodeSupportTicketsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeSupportTicketListTool(cfg)

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
	_, _, handler := gentools.NewLinodeSupportTicketListTool(cfg)

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
	_, _, handler := gentools.NewLinodeSupportTicketListTool(cfg)

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
			_, _, handler := gentools.NewLinodeSupportTicketListTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeSupportTicketReplyListTool(cfg)

	if tool.Name != "linode_support_ticket_reply_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_support_ticket_reply_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, supportTicketIDKey) {
		t.Errorf("tool.RawInputSchema missing key %v", supportTicketIDKey)
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
	_, _, handler := gentools.NewLinodeSupportTicketReplyListTool(cfg)

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
	_, _, handler := gentools.NewLinodeSupportTicketReplyListTool(cfg)

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
			_, _, handler := gentools.NewLinodeSupportTicketReplyListTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeSupportTicketGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSupportTicketGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSupportTicketGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve support ticket") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve support ticket")
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
			_, _, handler := gentools.NewLinodeSupportTicketGetTool(cfg)
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
