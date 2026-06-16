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

const accountUserGrantsToolName = "linode_account_user_grants_get"

func TestLinodeAccountUserGrantsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

	if tool.Name != accountUserGrantsToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, accountUserGrantsToolName)
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

	props := tool.InputSchema.Properties
	if _, ok := props[keyUsername]; !ok {
		t.Errorf("props missing key %v", keyUsername)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyUsername) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyUsername)
	}
}

func TestLinodeAccountUserGrantsToolInvalidUsernameRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{}, wantMessage: errUsernameRequired},
		{name: caseEmptyUsername, args: map[string]any{keyUsername: ""}, wantMessage: errUsernameNonEmpty},
		{name: caseBlankUsername, args: map[string]any{keyUsername: blankString}, wantMessage: errUsernameNonEmpty},
		{name: caseNumericUsername, args: map[string]any{keyUsername: 123}, wantMessage: errUsernameNonEmpty},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername}, wantMessage: errUsernamePathParamInvalid},
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
			_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

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

func TestLinodeAccountUserGrantsToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission("read_only")},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission("read_write")}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})

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

	if !strings.Contains(textContent.Text, "account_access") {
		t.Errorf("textContent.Text does not contain %v", "account_access")
	}

	if !strings.Contains(textContent.Text, "read_write") {
		t.Errorf("textContent.Text does not contain %v", "read_write")
	}
}

func TestLinodeAccountUserGrantsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/users/"+accountLoginUsername+"/grants" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/users/"+accountLoginUsername+"/grants")
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
	_, _, handler := tools.NewLinodeAccountUserGrantsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_account_user_grants_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_account_user_grants_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
