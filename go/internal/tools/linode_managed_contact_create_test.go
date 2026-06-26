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
	managedContactCreateToolName    = "linode_managed_contact_create"
	managedContactNameParam         = "name"
	managedContactEmailParam        = "email"
	managedContactPhoneParam        = "phone"
	managedContactPhonePrimaryKey   = "primary"
	managedContactPhoneSecondaryKey = "secondary"
	managedContactNameFixture       = "John Doe"
	managedContactEmailFixture      = "john.doe@example.org"
	managedContactGroupFixture      = "on-call"
	managedContactPhoneFixture      = "123-456-7890"
	managedContactPhone2Fixture     = "555-1212"
	managedContactCreateEndpoint    = "/managed/contacts"
	errManagedContactFieldRequired  = "at least one managed contact field is required"
	errManagedContactReadOnlyField  = "id and updated are read-only"
	errManagedContactNameNonEmpty   = "name must be a non-empty string"
	errManagedContactEmailNonEmpty  = "email must be a non-empty string"
	errManagedContactPhoneNonEmpty  = "phone.primary must be a non-empty string"
)

func TestLinodeManagedContactCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedContactCreateTool(cfg)

	if tool.Name != managedContactCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedContactCreateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[managedContactNameParam]; !ok {
		t.Errorf("props missing key %v", managedContactNameParam)
	}

	if _, ok := props[managedContactEmailParam]; !ok {
		t.Errorf("props missing key %v", managedContactEmailParam)
	}

	if _, ok := props[managedContactPhoneParam]; !ok {
		t.Errorf("props missing key %v", managedContactPhoneParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeManagedContactCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

			args := map[string]any{managedContactNameParam: managedContactNameFixture}
			if testCase.set {
				args[keyConfirm] = testCase.value
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedContactCreateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "empty request", args: map[string]any{keyConfirm: true}, wantMessage: errManagedContactFieldRequired},
		{name: "managed contact read-only id", args: map[string]any{keyBetaID: 123, managedContactNameParam: managedContactNameFixture, keyConfirm: true}, wantMessage: errManagedContactReadOnlyField},
		{name: "read only updated", args: map[string]any{keyUpdated: "2018-01-01T00:01:01", managedContactNameParam: managedContactNameFixture, keyConfirm: true}, wantMessage: errManagedContactReadOnlyField},
		{name: "managed contact empty name", args: map[string]any{managedContactNameParam: "", keyConfirm: true}, wantMessage: errManagedContactNameNonEmpty},
		{name: "managed contact numeric email", args: map[string]any{managedContactEmailParam: 123, keyConfirm: true}, wantMessage: errManagedContactEmailNonEmpty},
		{name: "blank phone", args: map[string]any{managedContactPhoneParam: map[string]any{managedContactPhonePrimaryKey: blankString}, keyConfirm: true}, wantMessage: errManagedContactPhoneNonEmpty},
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
			_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedContactCreateToolSuccess(t *testing.T) {
	t.Parallel()

	name := managedContactNameFixture
	email := managedContactEmailFixture
	group := managedContactGroupFixture
	primary := managedContactPhoneFixture
	want := linode.CreateManagedContactRequest{
		Name:  &name,
		Email: &email,
		Group: &group,
		Phone: &linode.CreateManagedContactPhoneRequest{Primary: &primary},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedContactCreateEndpoint {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactCreateEndpoint)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreateManagedContactRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %+v, want %+v", got, want)
		}

		if got.Phone == nil || got.Phone.Primary == nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedContact{ID: 567, Name: managedContactNameFixture, Email: managedContactEmailFixture, Group: got.Group, Phone: linode.ManagedContactPhone{Primary: got.Phone.Primary}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedContactNameParam: managedContactNameFixture, managedContactEmailParam: managedContactEmailFixture, "group": managedContactGroupFixture, managedContactPhoneParam: map[string]any{managedContactPhonePrimaryKey: managedContactPhoneFixture}, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, managedContactNameFixture) {
		t.Errorf("textContent.Text does not contain %v", managedContactNameFixture)
	}

	if !strings.Contains(textContent.Text, managedContactEmailFixture) {
		t.Errorf("textContent.Text does not contain %v", managedContactEmailFixture)
	}
}

func TestLinodeManagedContactCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedContactCreateEndpoint {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactCreateEndpoint)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedContactNameParam: managedContactNameFixture, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_managed_contact_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_managed_contact_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
