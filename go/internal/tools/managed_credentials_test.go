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
	managedCredentialsToolPath          = "/managed/credentials"
	managedCredentialsSSHKeyToolPath    = "/managed/credentials/sshkey"
	managedCredentialsToolLabel         = "prod-password-1"
	managedCredentialsToolLastDecrypted = "2018-01-01T00:01:01"
	managedSSHKeyToolValue              = "ssh-rsa managedservices-test-key"
	managedCredentialsToolPassword      = "stored-password-value"
	managedCredentialsToolUsername      = "johndoe"
	managedCredentialsToolPasswordReq   = "password is required"
)

func TestLinodeManagedCredentialsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedCredentialsTool(cfg)

	if tool.Name != "linode_managed_credential_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_credential_list")
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

func TestLinodeManagedCredentialsToolSuccess(t *testing.T) {
	t.Parallel()

	credentials := linode.PaginatedResponse[linode.ManagedCredential]{
		Data: []linode.ManagedCredential{{
			ID:            9991,
			Label:         managedCredentialsToolLabel,
			LastDecrypted: managedCredentialsToolLastDecrypted,
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(credentials); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)

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

	if !strings.Contains(textContent.Text, managedCredentialsToolLabel) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialsToolLabel)
	}

	if !strings.Contains(textContent.Text, managedCredentialsToolLastDecrypted) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialsToolLastDecrypted)
	}
}

func TestLinodeManagedCredentialsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
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
			_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)
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

func TestLinodeManagedCredentialsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialsTool(cfg)

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

func TestLinodeManagedSSHKeyToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

	if tool.Name != "linode_managed_sshkey_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_sshkey_get")
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

func TestLinodeManagedSSHKeyToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsSSHKeyToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsSSHKeyToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keySSHKey: managedSSHKeyToolValue, keyNotInProto: valNotInProto}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	if !strings.Contains(textContent.Text, managedSSHKeyToolValue) {
		t.Errorf("textContent.Text does not contain %v", managedSSHKeyToolValue)
	}

	if strings.Contains(textContent.Text, valNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeManagedSSHKeyToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialsSSHKeyToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsSSHKeyToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedSSHKeyTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_managed_sshkey_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_managed_sshkey_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeManagedCredentialCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

	if tool.Name != "linode_managed_credential_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_credential_create")
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
	for _, key := range []string{keyConfirm, keyLabel, keyDiskPassword} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeManagedCredentialCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

			args := map[string]any{keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword}
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedCredentialCreateToolRequiredArgumentsRejectBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingLabel, args: map[string]any{keyConfirm: true, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyConfirm: true, keyLabel: blankString, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: errLabelRequired},
		{name: "missing password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel}, wantMessage: managedCredentialsToolPasswordReq},
		{name: "blank password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: blankString}, wantMessage: managedCredentialsToolPasswordReq},
		{name: "numeric label", args: map[string]any{keyConfirm: true, keyLabel: 12, keyDiskPassword: managedCredentialsToolPassword}, wantMessage: "label must be a string"},
		{name: "numeric password", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: 12}, wantMessage: "password must be a string"},
		{name: "numeric username", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword, keyUsername: 12}, wantMessage: "username must be a string"},
		{name: "blank username", args: map[string]any{keyConfirm: true, keyLabel: managedCredentialsToolLabel, keyDiskPassword: managedCredentialsToolPassword, keyUsername: blankString}, wantMessage: "username must be a non-empty string"},
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
			_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedCredentialCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath)
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

		for key, want := range map[string]any{
			keyLabel:        managedCredentialsToolLabel,
			keyDiskPassword: managedCredentialsToolPassword,
			keyUsername:     managedCredentialsToolUsername,
		} {
			if !reflect.DeepEqual(got[key], want) {
				t.Errorf("got[%v] = %v, want %v", key, got[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialsToolLabel,
			LastDecrypted: managedCredentialsToolLastDecrypted,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyConfirm:      true,
		keyLabel:        managedCredentialsToolLabel,
		keyDiskPassword: managedCredentialsToolPassword,
		keyUsername:     managedCredentialsToolUsername,
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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, managedCredentialsToolLabel) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialsToolLabel)
	}

	if !strings.Contains(textContent.Text, managedCredentialsToolLastDecrypted) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialsToolLastDecrypted)
	}

	if strings.Contains(textContent.Text, managedCredentialsToolPassword) {
		t.Errorf("textContent.Text should not contain %v", managedCredentialsToolPassword)
	}

	if !strings.Contains(textContent.Text, "Managed credential 9991 created successfully") {
		t.Errorf("textContent.Text does not contain the create confirmation message: %v", textContent.Text)
	}
}

func TestLinodeManagedCredentialCreateToolSuccessWithoutUsernameOmitsUsername(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got[keyLabel], managedCredentialsToolLabel) {
			t.Errorf("got[keyLabel] = %v, want %v", got[keyLabel], managedCredentialsToolLabel)
		}

		if !reflect.DeepEqual(got[keyDiskPassword], managedCredentialsToolPassword) {
			t.Errorf("got[keyDiskPassword] = %v, want %v", got[keyDiskPassword], managedCredentialsToolPassword)
		}

		if _, ok := got[keyUsername]; ok {
			t.Errorf("got has unexpected key %v", keyUsername)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{
			ID:            9991,
			Label:         managedCredentialsToolLabel,
			LastDecrypted: managedCredentialsToolLastDecrypted,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyConfirm:      true,
		keyLabel:        managedCredentialsToolLabel,
		keyDiskPassword: managedCredentialsToolPassword,
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
}

func TestLinodeManagedCredentialCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedCredentialsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyConfirm:      true,
		keyLabel:        managedCredentialsToolLabel,
		keyDiskPassword: managedCredentialsToolPassword,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_managed_credential_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_managed_credential_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeManagedCredentialUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

	if tool.Name != "linode_managed_credential_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_credential_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if !strings.Contains(string(tool.RawInputSchema), keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeManagedCredentialUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	label := "prod-password-2"
	updated := linode.ManagedCredential{ID: 9991, Label: label, LastDecrypted: managedCredentialsToolLastDecrypted}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedCredentialsToolPath+"/9991" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath+"/9991")
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

		if !reflect.DeepEqual(got[keyLabel], label) {
			t.Errorf("got[keyLabel] = %v, want %v", got[keyLabel], label)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyCredentialID: 9991, keyLabel: label, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, label) {
		t.Errorf("textContent.Text does not contain %v", label)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeManagedCredentialUpdateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		caseMissingConfirm: nil,
		caseFalseConfirm:   false,
		caseStringConfirm:  boolStringTrue,
		caseNumericConfirm: float64(1),
	}

	for name, confirm := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

			args := map[string]any{keyCredentialID: 9991, keyLabel: managedCredentialsToolLabel}
			if confirm != nil {
				args[keyConfirm] = confirm
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedCredentialUpdateToolValidationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingCredentialID, args: map[string]any{keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: errCredentialIDRequired},
		{name: caseZeroCredentialID, args: map[string]any{keyCredentialID: 0, keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: errCredentialIDPositive},
		{name: "slash credential id", args: map[string]any{keyCredentialID: "9991/2", keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: errCredentialIDPositive},
		{name: "query credential id", args: map[string]any{keyCredentialID: "9991?x=1", keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: errCredentialIDPositive},
		{name: caseTraversalCredentialID, args: map[string]any{keyCredentialID: pathTraversalValue, keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: errCredentialIDPositive},
		{name: caseMissingLabel, args: map[string]any{keyCredentialID: 9991, keyConfirm: true}, wantMessage: errLabelRequired},
		{name: caseNonStringLabel, args: map[string]any{keyCredentialID: 9991, keyLabel: 42, keyConfirm: true}, wantMessage: errLabelString},
		{name: "read-only fields", args: map[string]any{keyCredentialID: 9991, keySupportTicketID: 9991, "last_decrypted": "2026-01-01T00:00:00", keyLabel: managedCredentialsToolLabel, keyConfirm: true}, wantMessage: "Read-only fields are not accepted: id, last_decrypted"},
		{name: caseEmptyLabel, args: map[string]any{keyCredentialID: 9991, keyLabel: "", keyConfirm: true}, wantMessage: databaseLabelRequiredMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
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

func TestLinodeManagedCredentialUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedCredentialsToolPath+"/9991" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialsToolPath+"/9991")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyCredentialID: 9991, keyLabel: managedCredentialsToolLabel, keyConfirm: true}))
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

	if !strings.Contains(textContent.Text, "Failed to update linode_managed_credential_update") {
		t.Errorf("textContent.Text does not contain %v", "Failed to update linode_managed_credential_update")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}
