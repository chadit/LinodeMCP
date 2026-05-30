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
	dbPGInstancesPath      = "/databases/postgresql/instances"
	dbMySQLInstanceGetPath = "/databases/mysql/instances/123"
	dbPGInstanceGetPath    = dbPGInstancesPath + "/123"
)

func TestLinodeDatabaseInstanceCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:            databaseInstanceLabel,
			keyType:             databaseInstanceType,
			databaseEngineParam: databaseEngineID,
			keyRegion:           regionUSEast,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_instance_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, databaseInstancesPath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyType:             databaseInstanceType,
			databaseEngineParam: databaseEngineID,
			keyRegion:           regionUSEast,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:            databaseInstanceLabel,
			keyType:             databaseInstanceType,
			databaseEngineParam: databaseEnginePostgreSQLID,
			keyRegion:           regionUSEast,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbPGInstancesPath, would["path"])
	})
}

func TestLinodeDatabaseInstanceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads instance then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123, Label: databaseInstanceLabel})
		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyLabel:      testRenamedLabel,
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_instance_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads instance then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyLabel:      testRenamedLabel,
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, dbPGInstanceGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabaseInstancePatchToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstancePatchTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without patching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_instance_patch", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath+"/patch", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabasePostgreSQLInstancePatchToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstancePatchTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without patching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstancePatchTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_patch", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbPGInstanceGetPath+"/patch", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabaseInstanceSuspendToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceSuspendTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without suspending", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_instance_suspend", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath+"/suspend", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without suspending", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_suspend", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbPGInstanceGetPath+"/suspend", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabaseInstanceResumeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceResumeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without resuming", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_instance_resume", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath+"/resume", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without resuming", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_resume", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbPGInstanceGetPath+"/resume", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeDatabaseInstanceCredentialsGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceCredentialsGetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123, Label: databaseInstanceLabel})
		_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, "password", "dry_run must not surface credential material")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_database_instance_credentials_get", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "GET", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath+"/credentials", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the parent instance, never the credentials endpoint")
	})
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_credentials_get", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "GET", would["method"])
		assert.Equal(t, dbPGInstanceGetPath+"/credentials", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the parent instance, never the credentials endpoint")
	})
}

func TestLinodeDatabaseInstanceCredentialsResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceCredentialsResetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, "password", "dry_run must not surface the rotated credential")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_database_instance_credentials_reset", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbMySQLInstanceGetPath+"/credentials/reset", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the parent instance, never resets")
	})
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_database_postgresql_instance_credentials_reset", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, dbPGInstanceGetPath+"/credentials/reset", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the parent instance, never resets")
	})
}
