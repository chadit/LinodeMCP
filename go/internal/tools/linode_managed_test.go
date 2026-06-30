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

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	managedContactsToolPath              = "/managed/contacts"
	managedContactsToolName              = "John Doe"
	managedContactsToolEmail             = "john.doe@example.org"
	managedContactDeleteIDKey            = "contact_id"
	managedContactDeleteID               = float64(567)
	managedContactUpdateIDKey            = "contact_id"
	managedContactUpdateIDMessage        = "contact_id must be a positive integer"
	managedContactUpdateEmptyCase        = "empty update"
	managedContactUpdateMutableRequired  = "at least one mutable contact field is required"
	managedContactUpdateGroupKey         = "group"
	managedContactUpdateCaseNumericEmail = "numeric email"
)

func TestLinodeManagedContactDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

	if tool.Name != "linode_managed_contact_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_contact_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeManagedContactDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedContactsToolPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath+"/567")
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
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedContactDeleteIDKey: managedContactDeleteID, keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "deleted successfully")
	}
}

func TestLinodeManagedContactDeleteToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	cases := map[string]any{
		caseMissingConfirm:         nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: float64(1),
	}

	t.Cleanup(func() {
		if calls.Load() != int32(0) {
			t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
		}
	})

	for name, confirm := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

			args := map[string]any{managedContactDeleteIDKey: managedContactDeleteID}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeManagedContactDeleteToolInvalidContactIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	cases := map[string]any{
		caseMissing:             nil,
		caseSlash:               "56/7",
		caseQuery:               "567?x=1",
		caseDotTraversal:        pathTraversalValue,
		"oversized contact id":  float64(9007199254740992),
		caseZeroContactID:       float64(0),
		"negative contact id":   float64(-1),
		"fractional contact id": float64(567.5),
	}

	t.Cleanup(func() {
		if calls.Load() != int32(0) {
			t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
		}
	})

	for name, contactID := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

			args := map[string]any{keyConfirm: true, keyConfirmedDryRun: true}
			if contactID != nil {
				args[managedContactDeleteIDKey] = contactID
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "contact_id must be a positive integer") {
				t.Errorf("error text %q does not contain %q", text.Text, "contact_id must be a positive integer")
			}
		})
	}
}

func TestLinodeManagedContactDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedContactsToolPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath+"/567")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedContactDeleteIDKey: managedContactDeleteID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_managed_contact_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_managed_contact_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeManagedContactsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedContactsTool(cfg)

	if tool.Name != "linode_managed_contact_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_contact_list")
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

func TestLinodeManagedContactsToolSuccess(t *testing.T) {
	t.Parallel()

	contacts := linode.PaginatedResponse[linode.ManagedContact]{
		Data: []linode.ManagedContact{{
			ID:    567,
			Name:  managedContactsToolName,
			Email: managedContactsToolEmail,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(contacts); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactsTool(cfg)

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

	if !strings.Contains(textContent.Text, managedContactsToolName) {
		t.Errorf("textContent.Text does not contain %v", managedContactsToolName)
	}

	if !strings.Contains(textContent.Text, managedContactsToolEmail) {
		t.Errorf("textContent.Text does not contain %v", managedContactsToolEmail)
	}
}

func TestLinodeManagedContactsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
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
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeManagedContactsTool(cfg)
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

func TestLinodeManagedContactsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactsTool(cfg)

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

func TestLinodeManagedContactUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

	if tool.Name != "linode_managed_contact_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_contact_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyConfirm)
	}

	for _, key := range []string{keyConfirm, managedContactUpdateIDKey} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeManagedContactUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	contact := linode.ManagedContact{ID: 567, Name: managedContactsToolName, Email: managedContactsToolEmail}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedContactsToolPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath+"/567")
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyName:                      managedContactsToolName,
			keyEmail:                     managedContactsToolEmail,
			managedContactUpdateGroupKey: "on-call",
		} {
			if !reflect.DeepEqual(got[key], want) {
				t.Errorf("got[%v] = %v, want %v", key, got[key], want)
			}
		}

		wantPhone := map[string]any{managedContactPhonePrimaryKey: managedContactPhoneFixture, managedContactPhoneSecondaryKey: managedContactPhone2Fixture}

		phone, ok := got[managedContactPhoneParam].(map[string]any)
		if !ok {
			t.Error("phone should be object")
		}

		if !reflect.DeepEqual(phone, wantPhone) {
			t.Errorf("phone = %v, want %v", phone, wantPhone)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(contact); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		managedContactUpdateIDKey:    567,
		keyName:                      managedContactsToolName,
		keyEmail:                     managedContactsToolEmail,
		managedContactUpdateGroupKey: "on-call",
		managedContactPhoneParam: map[string]any{
			managedContactPhonePrimaryKey:   managedContactPhoneFixture,
			managedContactPhoneSecondaryKey: managedContactPhone2Fixture,
		},
		keyConfirm: true,
	})

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

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, managedContactsToolName) {
		t.Errorf("textContent.Text does not contain %v", managedContactsToolName)
	}
}

func TestLinodeManagedContactUpdateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		confirm    any
		setConfirm bool
	}{
		{name: "missing confirm"},
		{name: caseFalseConfirm, confirm: false, setConfirm: true},
		{name: caseStringConfirm, confirm: boolStringTrue, setConfirm: true},
		{name: caseNumericConfirm, confirm: 1, setConfirm: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

			args := map[string]any{managedContactUpdateIDKey: 567, keyName: managedContactsToolName}
			if testCase.setConfirm {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

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

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeManagedContactUpdateToolInvalidInputRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing contact id", args: map[string]any{keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
		{name: "zero contact id", args: map[string]any{managedContactUpdateIDKey: 0, keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
		{name: "slash contact id", args: map[string]any{managedContactUpdateIDKey: "5/67", keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
		{name: "query contact id", args: map[string]any{managedContactUpdateIDKey: "567?x=1", keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
		{name: "traversal contact id", args: map[string]any{managedContactUpdateIDKey: pathTraversalValue, keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
		{name: managedContactUpdateEmptyCase, args: map[string]any{managedContactUpdateIDKey: 567, keyConfirm: true}, wantMessage: managedContactUpdateMutableRequired},
		{name: "numeric name", args: map[string]any{managedContactUpdateIDKey: 567, keyName: 123, keyConfirm: true}, wantMessage: "name must be a string"},
		{name: managedContactUpdateCaseNumericEmail, args: map[string]any{managedContactUpdateIDKey: 567, keyEmail: 123, keyConfirm: true}, wantMessage: "email must be a string"},
		{name: "numeric group", args: map[string]any{managedContactUpdateIDKey: 567, managedContactUpdateGroupKey: 123, keyConfirm: true}, wantMessage: "group must be a string"},
		{name: "numeric primary phone", args: map[string]any{managedContactUpdateIDKey: 567, managedContactPhoneParam: map[string]any{managedContactPhonePrimaryKey: 123}, keyConfirm: true}, wantMessage: "phone.primary must be a non-empty string"},
		{name: "numeric secondary phone", args: map[string]any{managedContactUpdateIDKey: 567, managedContactPhoneParam: map[string]any{managedContactPhoneSecondaryKey: 123}, keyConfirm: true}, wantMessage: "phone.secondary must be a non-empty string"},
		{name: "phone not object", args: map[string]any{managedContactUpdateIDKey: 567, managedContactPhoneParam: managedContactPhone2Fixture, keyConfirm: true}, wantMessage: "phone must be an object"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)
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

func TestLinodeManagedContactUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedContactsToolPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath+"/567")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedContactUpdateIDKey: 567, keyName: managedContactsToolName, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Failed to update linode_managed_contact") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update linode_managed_contact")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}
