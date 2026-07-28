package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], databaseInstancesPath) {
			t.Errorf("got %v, want %v", would["path"], databaseInstancesPath)
		}

		wantBody := map[string]any{
			keyLabel:            databaseInstanceLabel,
			keyType:             databaseInstanceType,
			databaseEngineParam: databaseEngineID,
			keyRegion:           regionUSEast,
		}
		if !reflect.DeepEqual(would["body"], wantBody) {
			t.Errorf("got %v, want %v", would["body"], wantBody)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		wantEffects := []any{"A MySQL Managed Database 'primary-db' will be created or restored."}
		if !reflect.DeepEqual(body["side_effects"], wantEffects) {
			t.Errorf("got %v, want %v", body["side_effects"], wantEffects)
		}

		wantWarnings := []any{"Creating a Managed Database can incur billing."}
		if !reflect.DeepEqual(body["warnings"], wantWarnings) {
			t.Errorf("got %v, want %v", body["warnings"], wantWarnings)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCreateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstancesPath) {
			t.Errorf("got %v, want %v", would["path"], dbPGInstancesPath)
		}

		wantBody := map[string]any{
			keyLabel:            databaseInstanceLabel,
			keyType:             databaseInstanceType,
			databaseEngineParam: databaseEnginePostgreSQLID,
			keyRegion:           regionUSEast,
		}
		if !reflect.DeepEqual(would["body"], wantBody) {
			t.Errorf("got %v, want %v", would["body"], wantBody)
		}

		wantEffects := []any{"A PostgreSQL Managed Database 'primary-db' will be created or restored."}
		if !reflect.DeepEqual(body["side_effects"], wantEffects) {
			t.Errorf("got %v, want %v", body["side_effects"], wantEffects)
		}

		wantWarnings := []any{"Creating a Managed Database can incur billing."}
		if !reflect.DeepEqual(body["warnings"], wantWarnings) {
			t.Errorf("got %v, want %v", body["warnings"], wantWarnings)
		}
	})
}

func TestLinodeDatabaseInstanceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath) {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath)
		}

		wantBody := map[string]any{keyLabel: testRenamedLabel}
		if !reflect.DeepEqual(would["body"], wantBody) {
			t.Errorf("got %v, want %v", would["body"], wantBody)
		}

		state, _ := body["current_state"].(map[string]any)
		if !reflect.DeepEqual(state["label"], databaseInstanceLabel) {
			t.Errorf("got %v, want %v", state["label"], databaseInstanceLabel)
		}

		wantEffects := []any{"MySQL Managed Database 123 will be updated."}
		if !reflect.DeepEqual(body["side_effects"], wantEffects) {
			t.Errorf("got %v, want %v", body["side_effects"], wantEffects)
		}

		wantWarnings := []any{"Updating a Managed Database can change service behavior."}
		if !reflect.DeepEqual(body["warnings"], wantWarnings) {
			t.Errorf("got %v, want %v", body["warnings"], wantWarnings)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates the update payload", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath) {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath)
		}

		wantBody := map[string]any{keyLabel: testRenamedLabel}
		if !reflect.DeepEqual(would["body"], wantBody) {
			t.Errorf("got %v, want %v", would["body"], wantBody)
		}

		state, _ := body["current_state"].(map[string]any)
		if !reflect.DeepEqual(state["id"], float64(123)) {
			t.Errorf("got %v, want %v", state["id"], float64(123))
		}

		wantEffects := []any{"PostgreSQL Managed Database 123 will be updated."}
		if !reflect.DeepEqual(body["side_effects"], wantEffects) {
			t.Errorf("got %v, want %v", body["side_effects"], wantEffects)
		}

		wantWarnings := []any{"Updating a Managed Database can change service behavior."}
		if !reflect.DeepEqual(body["warnings"], wantWarnings) {
			t.Errorf("got %v, want %v", body["warnings"], wantWarnings)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabaseInstancePatchToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstancePatchTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without patching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstancePatchTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_patch") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_patch")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath+"/patch") {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath+"/patch")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabasePostgreSQLInstancePatchToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstancePatchTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without patching", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstancePatchTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_patch") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_patch")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath+"/patch") {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath+"/patch")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabaseInstanceSuspendToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceSuspendTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without suspending", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceSuspendTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_suspend") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_suspend")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath+"/suspend") {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath+"/suspend")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceSuspendToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without suspending", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_suspend") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_suspend")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath+"/suspend") {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath+"/suspend")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabaseInstanceResumeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceResumeTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without resuming", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceResumeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_resume") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_resume")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath+"/resume") {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath+"/resume")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceResumeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without resuming", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_resume") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_resume")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath+"/resume") {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath+"/resume")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabaseInstanceCredentialsGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceCredentialsGetTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123, Label: databaseInstanceLabel})
		_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, "password") {
			t.Errorf("preview should not contain %v", "password")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_credentials_get") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_credentials_get")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "GET") {
			t.Errorf("got %v, want %v", would["method"], "GET")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath+"/credentials") {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath+"/credentials")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_credentials_get") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_credentials_get")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "GET") {
			t.Errorf("got %v, want %v", would["method"], "GET")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath+"/credentials") {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath+"/credentials")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabaseInstanceCredentialsResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabaseInstanceCredentialsResetTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbMySQLInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabaseInstanceCredentialsResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, "password") {
			t.Errorf("preview should not contain %v", "password")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_database_mysql_instance_credentials_reset") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_mysql_instance_credentials_reset")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbMySQLInstanceGetPath+"/credentials/reset") {
			t.Errorf("got %v, want %v", would["path"], dbMySQLInstanceGetPath+"/credentials/reset")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeDatabasePostgreSQLInstanceCredentialsResetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads the instance not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, dbPGInstanceGetPath, linode.DatabaseInstance{ID: 123})
		_, _, handler := tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_database_postgresql_instance_credentials_reset") {
			t.Errorf("got %v, want %v", body["tool"], "linode_database_postgresql_instance_credentials_reset")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], dbPGInstanceGetPath+"/credentials/reset") {
			t.Errorf("got %v, want %v", would["path"], dbPGInstanceGetPath+"/credentials/reset")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

// TestDatabaseInstanceCreateDryRunHonorsCancellation proves the create preview
// stops when the caller goes away. The create walk builds its sentences from
// the request alone, so nothing else in that path would notice a canceled
// context and the preview would render as if the caller were still listening.
func TestDatabaseInstanceCreateDryRunHonorsCancellation(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(dryRunNoCallServer(t))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyLabel:            databaseInstanceLabel,
		keyType:             databaseInstanceType,
		databaseEngineParam: databaseEngineID,
		keyRegion:           regionUSEast,
		keyDryRun:           true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true; text: %s", dryRunResultText(t, result))
	}

	if !strings.Contains(dryRunResultText(t, result), "database-create side-effect walk canceled") {
		t.Errorf("text = %q, want the canceled create walk", dryRunResultText(t, result))
	}
}

// fetchCompletedContext reports cancellation from the moment the dry-run state
// fetch returns. The update preview hands the walk the same context it fetched
// with, so a context canceled up front never reaches the walk: the GET fails
// first and the handler stops at "Failed to fetch state". The window the walk's
// guard covers sits between those two steps, microseconds wide against a live
// API, so the test opens it on purpose. Done() stays nil, which leaves the GET
// itself to run normally.
type fetchCompletedContext struct {
	fetched *atomic.Bool
}

func (fetchCompletedContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (fetchCompletedContext) Done() <-chan struct{} { return nil }

func (fetchCompletedContext) Value(any) any { return nil }

func (c fetchCompletedContext) Err() error {
	if c.fetched.Load() {
		return context.Canceled
	}

	return nil
}

// dryRunFetchThenCancelServer serves state on GET and marks the fetch complete
// as it answers, so the context the handler carries reports cancellation from
// the moment the state comes back.
func dryRunFetchThenCancelServer(t *testing.T, wantGetPath string, state any, fetched *atomic.Bool) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		}

		if r.URL.Path != wantGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, wantGetPath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(state); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		fetched.Store(true)
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// TestDatabaseInstanceUpdateDryRunHonorsCancellation covers the update walk's
// half of the same contract: a caller that disappears after the state GET gets
// an error rather than a preview assembled for nobody.
func TestDatabaseInstanceUpdateDryRunHonorsCancellation(t *testing.T) {
	t.Parallel()

	fetched := &atomic.Bool{}
	cfg := dryRunFetchThenCancelServer(t, dbMySQLInstanceGetPath,
		linode.DatabaseInstance{ID: 123, Label: databaseInstanceLabel}, fetched)

	_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

	ctx := fetchCompletedContext{fetched: fetched}

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyLabel:      testRenamedLabel,
		keyDryRun:     true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fetched.Load() {
		t.Fatal("the preview never fetched state, so the walk was not the step that failed")
	}

	if !result.IsError {
		t.Fatalf("result.IsError = false, want true; text: %s", dryRunResultText(t, result))
	}

	if !strings.Contains(dryRunResultText(t, result), "database-update side-effect walk canceled") {
		t.Errorf("text = %q, want the canceled update walk", dryRunResultText(t, result))
	}
}
