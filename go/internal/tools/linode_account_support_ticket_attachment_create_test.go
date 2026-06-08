package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	supportTicketAttachmentCreateToolName = "linode_account_support_ticket_attachment_create"
	supportTicketAttachmentTicketID       = "ticket_id"
	supportTicketAttachmentFileParam      = "file"
	supportTicketAttachmentFile           = "attachment-content"
	supportTicketAttachmentFilename       = "diagnostics.txt"
	errSupportTicketAttachmentIDPositive  = "ticket_id must be a positive integer"
)

func TestLinodeAccountSupportTicketAttachmentCreateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(&config.Config{})

	if tool.Name != supportTicketAttachmentCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, supportTicketAttachmentCreateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[supportTicketAttachmentTicketID]; !ok {
		t.Errorf("props missing key %v", supportTicketAttachmentTicketID)
	}

	if _, ok := props[supportTicketAttachmentFileParam]; !ok {
		t.Errorf("props missing key %v", supportTicketAttachmentFileParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}

	for _, key := range []string{supportTicketAttachmentTicketID, supportTicketAttachmentFileParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountSupportTicketAttachmentCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(cfg)

			args := map[string]any{supportTicketAttachmentTicketID: float64(123), supportTicketAttachmentFileParam: supportTicketAttachmentFile}
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSupportTicketAttachmentCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/123/attachments" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/123/attachments")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got[supportTicketAttachmentFileParam], supportTicketAttachmentFile) {
			t.Errorf("got[supportTicketAttachmentFileParam] = %v, want %v", got[supportTicketAttachmentFileParam], supportTicketAttachmentFile)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: 654, "filename": supportTicketAttachmentFilename, "size": 128}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		supportTicketAttachmentTicketID:  float64(123),
		supportTicketAttachmentFileParam: supportTicketAttachmentFile,
		keyConfirm:                       true,
	}))
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

	if !strings.Contains(textContent.Text, supportTicketAttachmentFilename) {
		t.Errorf("textContent.Text does not contain %v", supportTicketAttachmentFilename)
	}
}

func TestLinodeAccountSupportTicketAttachmentCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/123/attachments" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/123/attachments")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketAttachmentTicketID: float64(123), supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create support ticket attachment") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create support ticket attachment")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountSupportTicketAttachmentCreateToolRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingTicketID, args: map[string]any{supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketIDRequired},
		{name: caseZeroTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(0), supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: caseFractionalTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(1.5), supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id separator", args: map[string]any{supportTicketAttachmentTicketID: "123/attachments", supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id query", args: map[string]any{supportTicketAttachmentTicketID: databaseInvalidInstanceIDQuery, supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id traversal", args: map[string]any{supportTicketAttachmentTicketID: "..", supportTicketAttachmentFileParam: supportTicketAttachmentFile, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "missing file", args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyConfirm: true}, wantMessage: "file is required"},
		{name: "blank file", args: map[string]any{supportTicketAttachmentTicketID: float64(123), supportTicketAttachmentFileParam: blankString, keyConfirm: true}, wantMessage: "file must be a non-empty string"},
		{name: "numeric file", args: map[string]any{supportTicketAttachmentTicketID: float64(123), supportTicketAttachmentFileParam: float64(1), keyConfirm: true}, wantMessage: "file must be a non-empty string"},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSupportTicketAttachmentCreateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAccountSupportTicketAttachmentCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		supportTicketAttachmentTicketID:  float64(123),
		supportTicketAttachmentFileParam: supportTicketAttachmentFile,
		keyDryRun:                        true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], supportTicketAttachmentCreateToolName) {
		t.Errorf("got %v, want %v", body["tool"], supportTicketAttachmentCreateToolName)
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/support/tickets/123/attachments") {
		t.Errorf("got %v, want %v", would["path"], "/support/tickets/123/attachments")
	}

	bodyPreview, _ := would["body"].(map[string]any)
	if !reflect.DeepEqual(bodyPreview[supportTicketAttachmentFileParam], supportTicketAttachmentFile) {
		t.Errorf("bodyPreview[supportTicketAttachmentFileParam] = %v, want %v", bodyPreview[supportTicketAttachmentFileParam], supportTicketAttachmentFile)
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "ticket 123") {
		t.Errorf("effect does not contain %v", "ticket 123")
	}
}
