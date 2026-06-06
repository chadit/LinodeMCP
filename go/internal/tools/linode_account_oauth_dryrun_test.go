package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	accountOAuthClientsTestPath   = "/account/oauth-clients"
	oauthClientTestID             = "abc123"
	accountOAuthClientGetTestPath = accountOAuthClientsTestPath + "/" + oauthClientTestID
	keyOAuthThumbnail             = "thumbnail_png_base64"
	oauthThumbnailBase64          = "aGVsbG8="
)

func TestLinodeAccountOAuthClientCreateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountOAuthClientCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:       "my-app",
			"redirect_uri": "https://example.com/callback",
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_oauth_client_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountOAuthClientsTestPath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeAccountOAuthClientUpdateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountOAuthClientUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads client then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountOAuthClientGetTestPath, linode.OAuthClient{ID: oauthClientTestID})
		_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: oauthClientTestID,
			keyLabel:    "renamed-app",
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_oauth_client_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, accountOAuthClientGetTestPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads client then would PUT thumbnail", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountOAuthClientGetTestPath, linode.OAuthClient{ID: oauthClientTestID})
		_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID:       oauthClientTestID,
			keyOAuthThumbnail: oauthThumbnailBase64,
			keyDryRun:         true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_oauth_client_thumbnail_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, accountOAuthClientGetTestPath+"/thumbnail", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeAccountOAuthClientDeleteToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountOAuthClientDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountOAuthClientGetTestPath, linode.OAuthClient{ID: oauthClientTestID})
		_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: oauthClientTestID,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_oauth_client_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, accountOAuthClientGetTestPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeAccountOAuthClientResetSecretToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountOAuthClientResetSecretTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads client metadata not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountOAuthClientGetTestPath, linode.OAuthClient{ID: oauthClientTestID, Label: "my-app"})
		_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: oauthClientTestID,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_oauth_client_reset_secret", body["tool"])

		state, _ := body["current_state"].(map[string]any)
		assert.NotContains(t, state, "secret", "dry_run current_state must not surface the rotated client secret")

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountOAuthClientGetTestPath+"/reset-secret", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the client metadata only")
	})
}
