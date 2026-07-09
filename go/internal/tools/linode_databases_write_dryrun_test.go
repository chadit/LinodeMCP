package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

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

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
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

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
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
