package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	accountSettingsTestPath = "/account/settings"
	accountUsersTestPath    = "/account/users"
	accountUserGetTestPath  = accountUsersTestPath + "/account-login-user"
)

func TestLinodeAccountSettingsManagedEnableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeAccountSettingsManagedEnableTool(&config.Config{})

		rawSchema := string(tool.RawInputSchema)
		if !strings.Contains(rawSchema, keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without enabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountSettingsTestPath, linode.AccountSettings{})
		_, _, handler := gentools.NewLinodeAccountSettingsManagedEnableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_settings_managed_enable") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_settings_managed_enable")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountSettingsTestPath+"/managed-enable") {
			t.Errorf("got %v, want %v", would["path"], accountSettingsTestPath+"/managed-enable")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeAccountUserUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeAccountUserUpdateTool(&config.Config{})

		rawSchema := string(tool.RawInputSchema)
		if !strings.Contains(rawSchema, keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads user then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountUserGetTestPath, linode.AccountUser{Username: accountLoginUsername})
		_, _, handler := gentools.NewLinodeAccountUserUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyUsername: accountLoginUsername,
			keyEmail:    "renamed@example.com",
			keyDryRun:   true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_user_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_user_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], accountUserGetTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountUserGetTestPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeAccountUserGrantsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeAccountUserGrantsUpdateTool(&config.Config{})

		rawSchema := string(tool.RawInputSchema)
		if !strings.Contains(rawSchema, keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads grants then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountUserGetTestPath+"/grants", linode.Grants{})
		_, _, handler := gentools.NewLinodeAccountUserGrantsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyUsername:    accountLoginUsername,
			keyGrantLinode: []any{map[string]any{keyBetaID: float64(123), keyPermissions: grantPermissionReadWrite}},
			keyDryRun:      true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_user_grants_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_user_grants_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], accountUserGetTestPath+"/grants") {
			t.Errorf("got %v, want %v", would["path"], accountUserGetTestPath+"/grants")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
