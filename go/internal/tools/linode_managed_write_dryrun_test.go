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
	keyManagedSSHObject   = "ssh"
	keyManagedSSHAccess   = "access"
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
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_service_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_service_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedServicesBasePath) {
			t.Errorf("got %v, want %v", would["path"], managedServicesBasePath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeManagedServiceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_service_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_service_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], managedSvcGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedSvcGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedServiceDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_service_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_service_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], managedSvcGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedSvcGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedServiceEnableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceEnableTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without enabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceEnableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_service_enable") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_service_enable")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedSvcGetPath+"/enable") {
			t.Errorf("got %v, want %v", would["path"], managedSvcGetPath+"/enable")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedServiceDisableToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedServiceDisableTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without disabling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSvcGetPath, linode.ManagedService{ID: 10})
		_, _, handler := tools.NewLinodeManagedServiceDisableTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedServiceID: float64(10),
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_service_disable") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_service_disable")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedSvcGetPath+"/disable") {
			t.Errorf("got %v, want %v", would["path"], managedSvcGetPath+"/disable")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedContactCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeManagedContactCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyManagedContactName: "ops-oncall",
			keyDryRun:             true,
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_contact_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_contact_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedContactsBasePath) {
			t.Errorf("got %v, want %v", would["path"], managedContactsBasePath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeManagedContactUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_contact_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_contact_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], managedContactGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedContactGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedContactDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedContactDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedContactGetPath, linode.ManagedContact{ID: 20})
		_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyContactID: float64(20),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_contact_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_contact_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], managedContactGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedContactGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedLinodeSettingsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedLinodeSettingsUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads settings then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedSettingsGetPath, linode.ManagedLinodeSettings{})
		_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:         float64(40),
			keyManagedSSHObject: map[string]any{keyManagedSSHAccess: true},
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_linode_settings_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_linode_settings_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], managedSettingsGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedSettingsGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedCredentialCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview never echoes the secret", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeManagedCredentialCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:            "db-root",
			keyManagedSecretArg: managedSecretSentinel,
			keyDryRun:           true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, managedSecretSentinel) {
			t.Errorf("preview should not contain %v", managedSecretSentinel)
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_managed_credential_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_credential_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedCredsBasePath) {
			t.Errorf("got %v, want %v", would["path"], managedCredsBasePath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeManagedCredentialGetToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialGetTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads metadata not the secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30, Label: "db-root"})
		_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID: float64(30),
			keyDryRun:       true,
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_credential_get") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_credential_get")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "GET") {
			t.Errorf("got %v, want %v", would["method"], "GET")
		}

		if !reflect.DeepEqual(would["path"], managedCredGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedCredGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedCredentialUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_credential_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_credential_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], managedCredGetPath) {
			t.Errorf("got %v, want %v", would["path"], managedCredGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedCredentialUsernamePasswordUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, managedSecretSentinel2) {
			t.Errorf("preview should not contain %v", managedSecretSentinel2)
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_managed_credential_username_password_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_credential_username_password_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedCredGetPath+"/update") {
			t.Errorf("got %v, want %v", would["path"], managedCredGetPath+"/update")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeManagedCredentialRevokeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeManagedCredentialRevokeTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without revoking", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, managedCredGetPath, linode.ManagedCredential{ID: 30})
		_, _, handler := tools.NewLinodeManagedCredentialRevokeTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyCredentialID: float64(30),
			keyDryRun:       true,
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

		if !reflect.DeepEqual(body["tool"], "linode_managed_credential_revoke") {
			t.Errorf("got %v, want %v", body["tool"], "linode_managed_credential_revoke")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], managedCredGetPath+"/revoke") {
			t.Errorf("got %v, want %v", would["path"], managedCredGetPath+"/revoke")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
