package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	accountUserCreateToolName = "linode_account_user_create"
	accountUserUsername       = "new-user"
	accountUserEmail          = "new-user@example.com"
	errUsernameRequired       = "username is required"
	errUsernameNonEmpty       = "username must be a non-empty string"
	errEmailRequired          = "email is required"
	errEmailNonEmpty          = "email must be a non-empty string"
)

func TestLinodeAccountUserCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserCreateTool(cfg)

	if tool.Name != accountUserCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, accountUserCreateToolName)
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
	if _, ok := props[keyUsername]; !ok {
		t.Errorf("props missing key %v", keyUsername)
	}

	if _, ok := props[keyEmail]; !ok {
		t.Errorf("props missing key %v", keyEmail)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyUsername, keyEmail, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountUserCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

			args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail}
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

func TestLinodeAccountUserCreateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameRequired},
		{name: caseEmptyUsername, args: map[string]any{keyUsername: "", keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: caseBlankUsername, args: map[string]any{keyUsername: blankString, keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: caseNumericUsername, args: map[string]any{keyUsername: 123, keyEmail: accountUserEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: "missing email", args: map[string]any{keyUsername: accountUserUsername, keyConfirm: true}, wantMessage: errEmailRequired},
		{name: "empty email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: "", keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "blank email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: blankString, keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "numeric email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: 123, keyConfirm: true}, wantMessage: errEmailNonEmpty},
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
			_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

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

func TestLinodeAccountUserCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountUsersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUsersTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreateAccountUserRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Username != accountUserUsername {
			t.Errorf("got.Username = %v, want %v", got.Username, accountUserUsername)
		}

		if got.Email != accountUserEmail {
			t.Errorf("got.Email = %v, want %v", got.Email, accountUserEmail)
		}

		if got.Restricted == nil || !*got.Restricted {
			t.Errorf("got.Restricted = %v, want true", got.Restricted)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUsername, Email: accountUserEmail, UserType: "default"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail, keyRestricted: true, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, accountUserUsername) {
		t.Errorf("textContent.Text does not contain %v", accountUserUsername)
	}

	if !strings.Contains(textContent.Text, accountUserEmail) {
		t.Errorf("textContent.Text does not contain %v", accountUserEmail)
	}
}

func TestLinodeAccountUserCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountUsersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUsersTestPath)
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
	_, _, handler := tools.NewLinodeAccountUserCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserEmail, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_account_user_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_account_user_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
