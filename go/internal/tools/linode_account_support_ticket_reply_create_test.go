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
	supportTicketReplyCreateToolName = "linode_support_ticket_reply_create"
	supportTicketReplyDescription    = "Thanks, here is more detail."
	supportTicketReplyCreatedBy      = "adevi"
)

func TestLinodeAccountSupportTicketReplyCreateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(&config.Config{})

	if tool.Name != supportTicketReplyCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, supportTicketReplyCreateToolName)
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

	if _, ok := props[keyDescription]; !ok {
		t.Errorf("props missing key %v", keyDescription)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}

	for _, key := range []string{supportTicketAttachmentTicketID, keyDescription, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountSupportTicketReplyCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

			args := map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: supportTicketReplyDescription}
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

func TestLinodeAccountSupportTicketReplyCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/123/replies" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/123/replies")
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

		if !reflect.DeepEqual(got[keyDescription], supportTicketReplyDescription) {
			t.Errorf("got[keyDescription] = %v, want %v", got[keyDescription], supportTicketReplyDescription)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: 456, keyDescription: supportTicketReplyDescription, "created_by": supportTicketReplyCreatedBy}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		supportTicketAttachmentTicketID: float64(123),
		keyDescription:                  supportTicketReplyDescription,
		keyConfirm:                      true,
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

	if !strings.Contains(textContent.Text, supportTicketReplyDescription) {
		t.Errorf("textContent.Text does not contain %v", supportTicketReplyDescription)
	}

	if !strings.Contains(textContent.Text, supportTicketReplyCreatedBy) {
		t.Errorf("textContent.Text does not contain %v", supportTicketReplyCreatedBy)
	}
}

func TestLinodeAccountSupportTicketReplyCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/support/tickets/123/replies" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/support/tickets/123/replies")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: supportTicketReplyDescription, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create support ticket reply") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create support ticket reply")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountSupportTicketReplyCreateToolRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingTicketID, args: map[string]any{keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketIDRequired},
		{name: caseZeroTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(0), keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: caseFractionalTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(1.5), keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id separator", args: map[string]any{supportTicketAttachmentTicketID: "123/replies", keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id query", args: map[string]any{supportTicketAttachmentTicketID: databaseInvalidInstanceIDQuery, keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id traversal", args: map[string]any{supportTicketAttachmentTicketID: "..", keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "missing description", args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyConfirm: true}, wantMessage: "description is required"},
		{name: caseBlankDescription, args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: blankString, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
		{name: "numeric description", args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: float64(1), keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

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

func TestLinodeAccountSupportTicketReplyCreateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		supportTicketAttachmentTicketID: float64(123),
		keyDescription:                  supportTicketReplyDescription,
		keyDryRun:                       true,
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

	if !reflect.DeepEqual(body["tool"], supportTicketReplyCreateToolName) {
		t.Errorf("got %v, want %v", body["tool"], supportTicketReplyCreateToolName)
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/support/tickets/123/replies") {
		t.Errorf("got %v, want %v", would["path"], "/support/tickets/123/replies")
	}

	bodyPreview, _ := would["body"].(map[string]any)
	if !reflect.DeepEqual(bodyPreview[keyDescription], supportTicketReplyDescription) {
		t.Errorf("bodyPreview[keyDescription] = %v, want %v", bodyPreview[keyDescription], supportTicketReplyDescription)
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
