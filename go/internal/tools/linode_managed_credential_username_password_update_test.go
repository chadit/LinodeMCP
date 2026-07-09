package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	managedCredentialUsernamePasswordToolPath     = "/managed/credentials/9991/update"
	managedCredentialUsernamePasswordToolLabel    = "prod-password-1"
	managedCredentialUsernamePasswordToolTime     = "2018-01-01T00:01:01"
	managedCredentialUsernamePasswordToolPassword = "stored-password-value"
	managedCredentialUsernamePasswordToolUsername = "johndoe"
)

func TestLinodeManagedCredentialUsernamePasswordUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

	if tool.Name != "linode_managed_credential_username_password_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_credential_username_password_update")
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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyConfirm, managedCredentialIDParam, keyDiskPassword} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeManagedCredentialUsernamePasswordUpdateToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

			args := map[string]any{managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}
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

func TestLinodeManagedCredentialUsernamePasswordUpdateToolRequiredArgumentsRejectBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingCredentialID, args: map[string]any{keyConfirm: true, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errCredentialIDRequired},
		{name: caseZeroCredentialID, args: map[string]any{keyConfirm: true, managedCredentialIDParam: 0, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
		{name: "slash credential id", args: map[string]any{keyConfirm: true, managedCredentialIDParam: "9991/2", keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
		{name: "query credential id", args: map[string]any{keyConfirm: true, managedCredentialIDParam: "9991?x=1", keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
		{name: caseTraversalCredentialID, args: map[string]any{keyConfirm: true, managedCredentialIDParam: pathTraversalValue, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}, wantMessage: errManagedCredentialIDPositive},
		{name: "missing username password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991}, wantMessage: managedCredentialsToolPasswordReq},
		{name: "blank username password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: blankString}, wantMessage: managedCredentialsToolPasswordReq},
		{name: "numeric password", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: 12}, wantMessage: "password must be a string"},
		{name: "numeric username", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword, keyUsername: 12}, wantMessage: "username must be a string"},
		{name: "blank username", args: map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword, keyUsername: blankString}, wantMessage: "username must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedCredentialUsernamePasswordUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got[keyDiskPassword], managedCredentialUsernamePasswordToolPassword) {
			t.Errorf("got[keyDiskPassword] = %v, want %v", got[keyDiskPassword], managedCredentialUsernamePasswordToolPassword)
		}

		if !reflect.DeepEqual(got[keyUsername], managedCredentialUsernamePasswordToolUsername) {
			t.Errorf("got[keyUsername] = %v, want %v", got[keyUsername], managedCredentialUsernamePasswordToolUsername)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{ID: 9991, Label: managedCredentialUsernamePasswordToolLabel, LastDecrypted: managedCredentialUsernamePasswordToolTime}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyConfirm:               true,
		managedCredentialIDParam: 9991,
		keyDiskPassword:          managedCredentialUsernamePasswordToolPassword,
		keyUsername:              managedCredentialUsernamePasswordToolUsername,
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

	var got map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}

	wantMessage := "Managed credential 9991 updated successfully"
	if got["message"] != wantMessage {
		t.Errorf("got[message] = %v, want %v", got["message"], wantMessage)
	}

	// The id-echo carries the credential id; the credential metadata (label,
	// last_decrypted) and the secret are intentionally not echoed.
	if got["credential_id"] != float64(9991) {
		t.Errorf("got[credential_id] = %v, want %v", got["credential_id"], 9991)
	}

	if strings.Contains(textContent.Text, managedCredentialUsernamePasswordToolPassword) {
		t.Errorf("textContent.Text should not contain %v", managedCredentialUsernamePasswordToolPassword)
	}
}

func TestLinodeManagedCredentialUsernamePasswordUpdateToolSuccessWithoutUsernameOmitsUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordToolPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got[keyDiskPassword], managedCredentialUsernamePasswordToolPassword) {
			t.Errorf("got[keyDiskPassword] = %v, want %v", got[keyDiskPassword], managedCredentialUsernamePasswordToolPassword)
		}

		if _, ok := got[keyUsername]; ok {
			t.Errorf("got has unexpected key %v", keyUsername)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{ID: 9991, Label: managedCredentialUsernamePasswordToolLabel, LastDecrypted: managedCredentialUsernamePasswordToolTime}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}))
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

func TestLinodeManagedCredentialUsernamePasswordUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialUsernamePasswordToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialUsernamePasswordToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyConfirm: true, managedCredentialIDParam: 9991, keyDiskPassword: managedCredentialUsernamePasswordToolPassword}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_managed_credential_username_password_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_managed_credential_username_password_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
