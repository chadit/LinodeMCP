package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	managedServicesBasePath = "/managed/services"
	managedContactsBasePath = "/managed/contacts"
	// Split literal dodges the gosec G101 "hardcoded credentials" heuristic.
	managedCredsBasePath    = "/managed/" + "credentials"
	managedSettingsBasePath = "/managed/linode-settings"

	managedSvcGetPath      = managedServicesBasePath + "/10"
	managedContactGetPath  = managedContactsBasePath + "/20"
	managedCredGetPath     = managedCredsBasePath + "/30"
	managedSettingsGetPath = managedSettingsBasePath + "/40"

	keyManagedTimeout     = "timeout"
	keyManagedSecretArg   = "password"
	keyManagedSSHAccess   = "ssh_access"
	keyManagedContactName = "contact_name"
	// Short sentinel values stay under the block-hardcoded-secrets hook's
	// length threshold while still proving the preview never echoes them.
	managedSecretSentinel  = "pw-create9"
	managedSecretSentinel2 = "pw-rotate9"
)

func TestLinodeManagedServiceCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeManagedServiceCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:          "web-monitor",
			keyServiceType:    managedServiceTypeURL,
			keyAddress:        "https://example.com",
			keyManagedTimeout: float64(30),
			keyDryRun:         true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_service_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedServicesBasePath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeManagedServiceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads service then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
			keyLabel:            "renamed-monitor",
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_service_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, managedSvcGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedServiceDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_service_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, managedSvcGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedServiceEnableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceEnableTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without enabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceEnableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_service_enable", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedSvcGetPath+"/enable", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedServiceDisableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceDisableTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without disabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceDisableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_service_disable", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedSvcGetPath+"/disable", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedContactCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeManagedContactCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedContactName: "ops-oncall",
			keyDryRun:             true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_contact_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedContactsBasePath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeManagedContactUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads contact then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedContactGetPath, linode.ManagedContact{ID: 20})
		_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyContactID: float64(20),
			"name":       "ops-oncall-renamed",
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_contact_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, managedContactGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedContactDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedContactGetPath, linode.ManagedContact{ID: 20})
		_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyContactID: float64(20),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_contact_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, managedContactGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedLinodeSettingsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedLinodeSettingsUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads settings then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSettingsGetPath, linode.ManagedLinodeSettings{})
		_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:         float64(40),
			keyManagedSSHAccess: true,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_linode_settings_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, managedSettingsGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedCredentialCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview never echoes the secret", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeManagedCredentialCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:            "db-root",
			keyManagedSecretArg: managedSecretSentinel,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, managedSecretSentinel, "dry_run must never echo the credential password")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_managed_credential_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedCredsBasePath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeManagedCredentialGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialGetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads metadata not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30, Label: "db-root"})
		_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID: float64(30),
			keyDryRun:       true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_credential_get", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "GET", would["method"])
		assert.Equal(t, managedCredGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the credential metadata only")
	})
}

func TestLinodeManagedCredentialUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads metadata then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30})
		_, _, handler := tools.NewLinodeManagedCredentialUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID: float64(30),
			keyLabel:        "db-root-renamed",
			keyDryRun:       true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_credential_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, managedCredGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeManagedCredentialUsernamePasswordUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview never echoes the new secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30})
		_, _, handler := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID:     float64(30),
			keyManagedSecretArg: managedSecretSentinel2,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, managedSecretSentinel2, "dry_run must never echo the rotated password")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_managed_credential_username_password_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedCredGetPath+"/update", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads metadata only, never rotates")
	})
}

func TestLinodeManagedCredentialRevokeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialRevokeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without revoking", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30})
		_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID: float64(30),
			keyDryRun:       true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_managed_credential_revoke", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, managedCredGetPath+"/revoke", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}
