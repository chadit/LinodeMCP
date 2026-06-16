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

const accountUserGrantsUpdateToolName = "linode_account_user_grants_update"

func TestLinodeAccountUserGrantsUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	if tool.Name != accountUserGrantsUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, accountUserGrantsUpdateToolName)
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

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyGlobal]; !ok {
		t.Errorf("props missing key %v", keyGlobal)
	}

	if _, ok := props[keyGrantLinode]; !ok {
		t.Errorf("props missing key %v", keyGrantLinode)
	}

	if _, ok := props[keyGrantLKECluster]; !ok {
		t.Errorf("props missing key %v", keyGrantLKECluster)
	}

	for _, key := range []string{keyUsername, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeAccountUserGrantsUpdateRequiresConfirm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}}},
		{name: caseConfirmFalse, args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}, keyConfirm: false}},
		{name: "confirm string", args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{}, keyConfirm: boolStringTrue}},
		{name: "confirm numeric", args: map[string]any{keyUsername: accountLoginUsername, keyGrantLinode: []any{}, keyConfirm: 1}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserGrantsUpdateHandler(t, &calls)
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountUserGrantsUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingUsername, args: map[string]any{keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernameRequired},
		{name: caseSlashUsername, args: map[string]any{keyUsername: valueSlashUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: caseQueryUsername, args: map[string]any{keyUsername: valueQueryUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: "fragment username", args: map[string]any{keyUsername: valueFragmentUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: caseDotdotUsername, args: map[string]any{keyUsername: valueDotdotUsername, keyConfirm: true, keyGrantLinode: []any{}}, wantMessage: errUsernamePathParamInvalid},
		{name: "missing grants", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true}, wantMessage: "at least one grant section is required"},
		{name: "invalid global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: grantPermissionReadOnly}, wantMessage: errGlobalObject},
		{name: "null global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: nil}, wantMessage: errGlobalObject},
		{name: "empty global", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{}}, wantMessage: errGlobalObject},
		{name: "unknown global field", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{"typo": true}}, wantMessage: errGlobalObject},
		{name: "invalid global permission", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGlobal: map[string]any{keyAccountAccess: "admin"}}, wantMessage: errGlobalObject},
		{name: "invalid grant array", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: grantPermissionReadWrite}, wantMessage: errGrantSectionsArray},
		{name: "invalid grant array element", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{123}}, wantMessage: errGrantSectionsArray},
		{name: "null grant array", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: nil}, wantMessage: errGrantSectionsArray},
		{name: "unknown grant field", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite, "typo": true}}}, wantMessage: errGrantSectionsArray},
		{name: "missing grant id", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyPermissions: grantPermissionReadWrite}}}, wantMessage: errGrantSectionsArray},
		{name: "zero grant id", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(0), keyPermissions: grantPermissionReadWrite}}}, wantMessage: errGrantSectionsArray},
		{name: "null grant permissions", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: nil}}}, wantMessage: errGrantSectionsArray},
		{name: "invalid grant permissions", args: map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: "admin"}}}, wantMessage: errGrantSectionsArray},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			handler, cleanup := newAccountUserGrantsUpdateHandler(t, &calls)
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

func TestLinodeAccountUserGrantsUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyGlobal], map[string]any{keyAccountAccess: grantPermissionReadOnly}) {
			t.Errorf("body[keyGlobal] = %v, want %v", body[keyGlobal], map[string]any{keyAccountAccess: grantPermissionReadOnly})
		}

		if !reflect.DeepEqual(body[keyGrantLinode], []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}}) {
			t.Errorf("body[keyGrantLinode] = %v, want %v", body[keyGrantLinode], []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}})
		}

		if !reflect.DeepEqual(body[keyGrantLKECluster], []any{map[string]any{keyBetaID: float64(456), keyPermissions: grantPermissionReadOnly}}) {
			t.Errorf("body[keyGrantLKECluster] = %v, want %v", body[keyGrantLKECluster], []any{map[string]any{keyBetaID: float64(456), keyPermissions: grantPermissionReadOnly}})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Grants{
			Global: linode.GlobalGrants{AccountAccess: linode.GrantPermission(grantPermissionReadOnly)},
			Linode: []linode.Grant{{ID: 123, Permissions: linode.GrantPermission(grantPermissionReadWrite)}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyUsername:        accountLoginUsername,
		keyConfirm:         true,
		keyGlobal:          map[string]any{keyAccountAccess: grantPermissionReadOnly},
		keyGrantLinode:     []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}},
		keyGrantLKECluster: []any{map[string]any{keyBetaID: float64(456), keyPermissions: grantPermissionReadOnly}},
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

	if !strings.Contains(textContent.Text, keyAccountAccess) {
		t.Errorf("textContent.Text does not contain %v", keyAccountAccess)
	}

	if !strings.Contains(textContent.Text, grantPermissionReadWrite) {
		t.Errorf("textContent.Text does not contain %v", grantPermissionReadWrite)
	}
}

func TestLinodeAccountUserGrantsUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
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
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyUsername: accountLoginUsername, keyConfirm: true, keyGrantLinode: []any{}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_account_user_grants_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_account_user_grants_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func newAccountUserGrantsUpdateHandler(t *testing.T, calls *int32) (func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(calls, 1)
		w.WriteHeader(http.StatusOK)
	}))

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUserGrantsUpdateTool(cfg)

	return handler, srv.Close
}
