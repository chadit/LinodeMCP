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
	accountUserUpdateToolName            = "linode_account_user_update"
	accountUserUpdateNewUsername         = "renamed-user"
	accountUserUpdateEmail               = "updated-user@example.com"
	accountUserUpdateSSHKey              = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest"
	caseMissingUsername                  = "missing username"
	caseEmptyUsername                    = "empty username"
	caseBlankUsername                    = "blank username"
	caseNumericUsername                  = "numeric username"
	keyAccountUserRestricted             = "restricted"
	keyAccountUserSSHKeys                = "ssh_keys"
	keyAccountUserNewUsername            = "new_username"
	errAccountUserUpdateFieldRequired    = "at least one account user field is required"
	errAccountUserUpdateSSHKeys          = "ssh_keys must be an array of non-empty strings"
	errAccountUserUpdateRestrictedBool   = "restricted must be a boolean"
	errAccountUserUpdateNewUsernameBlank = "new_username must be a non-empty string"
)

func TestLinodeAccountUserUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserUpdateTool(cfg)

	if tool.Name != accountUserUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, accountUserUpdateToolName)
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

	if _, ok := props[keyAccountUserRestricted]; !ok {
		t.Errorf("props missing key %v", keyAccountUserRestricted)
	}

	if _, ok := props[keyAccountUserSSHKeys]; !ok {
		t.Errorf("props missing key %v", keyAccountUserSSHKeys)
	}

	if _, ok := props[keyAccountUserNewUsername]; !ok {
		t.Errorf("props missing key %v", keyAccountUserNewUsername)
	}

	for _, key := range []string{keyUsername, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountUserUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newAccountUserUpdateHandler(t, &calls)
			defer cleanup()

			args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserUpdateEmail}
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

func TestLinodeAccountUserUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := accountUserUpdateInvalidCases()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserUpdateHandler(t, &calls)
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountUserUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountUserUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountUserUsername)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyEmail:                 accountUserUpdateEmail,
			keyAccountUserRestricted: true,
			keyUsername:              accountUserUpdateNewUsername,
			keyAccountUserSSHKeys:    []any{accountUserUpdateSSHKey},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountUser{
			Username:   accountUserUpdateNewUsername,
			Email:      accountUserUpdateEmail,
			Restricted: true,
			SSHKeys:    []string{accountUserUpdateSSHKey},
			UserType:   envKeyDefault,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, accountUserUpdateSuccessArgs()))
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

	if !strings.Contains(textContent.Text, accountUserUpdateNewUsername) {
		t.Errorf("textContent.Text does not contain %v", accountUserUpdateNewUsername)
	}

	if !strings.Contains(textContent.Text, accountUserUpdateEmail) {
		t.Errorf("textContent.Text does not contain %v", accountUserUpdateEmail)
	}
}

func TestLinodeAccountUserUpdateAllowsEmptySSHKeys(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if _, ok := body[keyAccountUserSSHKeys]; !ok {
			t.Errorf("body missing key %v", keyAccountUserSSHKeys)
		}

		if v, ok := body[keyAccountUserSSHKeys].([]any); ok && len(v) != 0 {
			t.Errorf("body[keyAccountUserSSHKeys] = %v, want empty", body[keyAccountUserSSHKeys])
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountUser{Username: accountUserUsername, Email: accountUserUpdateEmail}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))
	args := map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: []any{}, keyConfirm: true}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeAccountUserUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/account/users/"+accountUserUsername {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountUserUsername)
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

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))
	args := map[string]any{keyUsername: accountUserUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_account_user_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_account_user_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func accountUserUpdateInvalidCases() []struct {
	name        string
	args        map[string]any
	wantMessage string
} {
	return []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernameRequired},
		{name: caseEmptyUsername, args: map[string]any{keyUsername: "", keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernameNonEmpty},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: "fragment username", args: map[string]any{keyUsername: "user#name", keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername, keyEmail: accountUserUpdateEmail, keyConfirm: true}, wantMessage: errUsernamePathParamInvalid},
		{name: "no update fields", args: map[string]any{keyUsername: accountUserUsername, keyConfirm: true}, wantMessage: errAccountUserUpdateFieldRequired},
		{name: "empty email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: "", keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "numeric email", args: map[string]any{keyUsername: accountUserUsername, keyEmail: 123, keyConfirm: true}, wantMessage: errEmailNonEmpty},
		{name: "restricted string", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserRestricted: boolStringTrue, keyConfirm: true}, wantMessage: errAccountUserUpdateRestrictedBool},
		{name: "empty new_username", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserNewUsername: "", keyConfirm: true}, wantMessage: errAccountUserUpdateNewUsernameBlank},
		{name: "bad ssh_keys type", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: "ssh-ed25519", keyConfirm: true}, wantMessage: errAccountUserUpdateSSHKeys},
		{name: "bad ssh_keys item", args: map[string]any{keyUsername: accountUserUsername, keyAccountUserSSHKeys: []any{123}, keyConfirm: true}, wantMessage: errAccountUserUpdateSSHKeys},
	}
}

func accountUserUpdateSuccessArgs() map[string]any {
	return map[string]any{
		keyUsername:               accountUserUsername,
		keyEmail:                  accountUserUpdateEmail,
		keyAccountUserRestricted:  true,
		keyAccountUserSSHKeys:     []any{accountUserUpdateSSHKey},
		keyAccountUserNewUsername: accountUserUpdateNewUsername,
		keyConfirm:                true,
	}
}

func accountUserUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func newAccountUserUpdateHandler(t *testing.T, calls *int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(calls, 1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeAccountUserUpdateTool(accountUserUpdateConfig(srv.URL))

	return handler, srv.Close
}
