package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

const (
	accountOAuthClientsTestPath   = "/account/oauth-clients"
	oauthClientTestID             = "abc123"
	accountOAuthClientGetTestPath = accountOAuthClientsTestPath + "/" + oauthClientTestID
)

func TestLinodeAccountOAuthClientCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeAccountOauthClientCreateTool(&config.Config{})

		rawSchema := string(tool.RawInputSchema)
		if !strings.Contains(rawSchema, keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := gentools.NewLinodeAccountOauthClientCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:       "my-app",
			"redirect_uri": "https://example.com/callback",
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

		if !reflect.DeepEqual(body["tool"], "linode_account_oauth_client_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_oauth_client_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountOAuthClientsTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountOAuthClientsTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeAccountOAuthClientResetSecretToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeAccountOauthClientSecretResetTool(&config.Config{})

		rawSchema := string(tool.RawInputSchema)
		if !strings.Contains(rawSchema, keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads client metadata not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountOAuthClientGetTestPath, OAuthClient{ID: oauthClientTestID, Label: "my-app"})
		_, _, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: oauthClientTestID,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_oauth_client_secret_reset") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_oauth_client_secret_reset")
		}

		state, _ := body["current_state"].(map[string]any)
		if _, ok := state["secret"]; ok {
			t.Errorf("state has unexpected key %v", "secret")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountOAuthClientGetTestPath+"/reset-secret") {
			t.Errorf("got %v, want %v", would["path"], accountOAuthClientGetTestPath+"/reset-secret")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
