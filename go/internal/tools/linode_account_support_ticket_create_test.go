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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	supportTicketCreateToolName = "linode_account_support_ticket_create"
	supportTicketCreateSummary  = "Need help"
	supportTicketCreateBody     = "Instance is unreachable"
	errSummaryRequired          = "summary is required"
	errSummaryNonEmpty          = "summary must be a non-empty string"
	errDescriptionRequired      = "description is required"
	errDescriptionNonEmpty      = "description must be a non-empty string"
	errSupportTicketIDPositive  = "linode_id must be a positive integer"
	errSupportTicketRegion      = "region must be a non-empty string"
	keySupportTicketLinodeID    = "linode_id"
	keySupportTicketRegion      = "region"
	keySupportTicketID          = "id"
	keySupportTicketSummary     = "summary"
	caseMissingSummary          = "missing summary"
	caseEmptySummary            = "empty summary"
	caseBlankSummary            = "blank summary"
	caseBlankDescription        = "blank description"
	caseNumericSummary          = "numeric summary"
	supportTicketStatusOpen     = "open"
	supportTicketSeverity       = "major"
)

func TestLinodeAccountSupportTicketCreateRejectsInvalidOptionalFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		field       string
		value       any
		wantMessage string
	}{
		{name: "zero id", field: keySupportTicketLinodeID, value: float64(0), wantMessage: errSupportTicketIDPositive},
		{name: "negative id", field: keySupportTicketLinodeID, value: float64(-1), wantMessage: errSupportTicketIDPositive},
		{name: "fractional id", field: keySupportTicketLinodeID, value: float64(1.5), wantMessage: errSupportTicketIDPositive},
		{name: "oversized id", field: keySupportTicketLinodeID, value: float64(9007199254740992), wantMessage: errSupportTicketIDPositive},
		{name: "string id", field: keySupportTicketLinodeID, value: "12345", wantMessage: errSupportTicketIDPositive},
		{name: "object id", field: keySupportTicketLinodeID, value: map[string]any{keySupportTicketID: float64(12345)}, wantMessage: errSupportTicketIDPositive},
		{name: "blank region", field: keySupportTicketRegion, value: blankString, wantMessage: errSupportTicketRegion},
		{name: "numeric region", field: keySupportTicketRegion, value: float64(1), wantMessage: errSupportTicketRegion},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

			args := map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody, keyConfirm: true}
			args[testCase.field] = testCase.value

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSupportTicketCreateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeAccountSupportTicketCreateTool(&config.Config{})

	if tool.Name != supportTicketCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, supportTicketCreateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketSummary]; !ok {
		t.Errorf("props missing key %v", keySupportTicketSummary)
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

	if _, ok := props[keySupportTicketLinodeID]; !ok {
		t.Errorf("props missing key %v", keySupportTicketLinodeID)
	}

	for _, key := range []string{keySupportTicketSummary, keyDescription, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountSupportTicketCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

			args := map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody}
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

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSupportTicketCreateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingSummary, args: map[string]any{keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryRequired},
		{name: caseEmptySummary, args: map[string]any{keySupportTicketSummary: "", keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
		{name: caseBlankSummary, args: map[string]any{keySupportTicketSummary: blankString, keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
		{name: caseNumericSummary, args: map[string]any{keySupportTicketSummary: 123, keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
		{name: "missing description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyConfirm: true}, wantMessage: errDescriptionRequired},
		{name: "empty description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: "", keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
		{name: caseBlankDescription, args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: blankString, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
		{name: "numeric description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: 123, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

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

func TestLinodeAccountSupportTicketCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcSupportTickets {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets)
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

		for key, want := range map[string]any{
			"summary":                           supportTicketCreateSummary,
			keyDescription:                      supportTicketCreateBody,
			"bucket":                            tcBackups,
			"database_id":                       float64(23456),
			"domain_id":                         float64(34567),
			"firewall_id":                       float64(45678),
			keySupportTicketLinodeID:            float64(12345),
			"lkecluster_id":                     float64(56789),
			"longviewclient_id":                 float64(67890),
			"managed_issue":                     tcManaged,
			"nodebalancer_id":                   float64(78901),
			keySupportTicketRegion:              placementGroupCreateRegion,
			monitorAlertDefinitionSeverityParam: supportTicketSeverity,
			"vlan":                              "vlan-a",
			"volume_id":                         float64(89012),
			keyVPCID:                            float64(90123),
		} {
			if !reflect.DeepEqual(got[key], want) {
				t.Errorf("got[%v] = %v, want %v", key, got[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.SupportTicket{ID: 987, Summary: supportTicketCreateSummary, Description: supportTicketCreateBody, Status: supportTicketStatusOpen}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keySupportTicketSummary:             supportTicketCreateSummary,
		keyDescription:                      supportTicketCreateBody,
		"bucket":                            tcBackups,
		"database_id":                       float64(23456),
		"domain_id":                         float64(34567),
		"firewall_id":                       float64(45678),
		keySupportTicketLinodeID:            float64(12345),
		"lkecluster_id":                     float64(56789),
		"longviewclient_id":                 float64(67890),
		"managed_issue":                     tcManaged,
		"nodebalancer_id":                   float64(78901),
		keySupportTicketRegion:              placementGroupCreateRegion,
		monitorAlertDefinitionSeverityParam: supportTicketSeverity,
		"vlan":                              "vlan-a",
		"volume_id":                         float64(89012),
		keyVPCID:                            float64(90123),
		keyConfirm:                          true,
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

	if !strings.Contains(textContent.Text, supportTicketCreateSummary) {
		t.Errorf("textContent.Text does not contain %v", supportTicketCreateSummary)
	}
}

func TestLinodeAccountSupportTicketCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
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
	_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_account_support_ticket_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_account_support_ticket_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountSupportTicketCreateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keySupportTicketSummary:             supportTicketCreateSummary,
		keyDescription:                      supportTicketCreateBody,
		keySupportTicketLinodeID:            float64(12345),
		monitorAlertDefinitionSeverityParam: supportTicketSeverity,
		keyDryRun:                           true,
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

	if !reflect.DeepEqual(body["tool"], supportTicketCreateToolName) {
		t.Errorf("got %v, want %v", body["tool"], supportTicketCreateToolName)
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], tcSupportTickets) {
		t.Errorf("got %v, want %v", would["path"], tcSupportTickets)
	}

	bodyPreview, _ := would["body"].(map[string]any)
	for key, want := range map[string]any{
		keySupportTicketSummary:             supportTicketCreateSummary,
		keyDescription:                      supportTicketCreateBody,
		keySupportTicketLinodeID:            float64(12345),
		monitorAlertDefinitionSeverityParam: supportTicketSeverity,
	} {
		if !reflect.DeepEqual(bodyPreview[key], want) {
			t.Errorf("bodyPreview[%v] = %v, want %v", key, bodyPreview[key], want)
		}
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

	if !strings.Contains(effect, supportTicketCreateSummary) {
		t.Errorf("effect does not contain %v", supportTicketCreateSummary)
	}
}
