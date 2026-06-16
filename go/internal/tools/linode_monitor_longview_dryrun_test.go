package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	longviewClientsBasePath = "/longview/clients"
	longviewClientGetPath   = longviewClientsBasePath + "/123"
	longviewPlanBasePath    = "/longview/plan"
	monitorDBaaSAlertsPath  = "/monitor/services/dbaas/alert-definitions"
	monitorAlertGetPath     = monitorDBaaSAlertsPath + "/20000"
)

func TestLinodeLongviewClientCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLongviewClientCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLongviewClientCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  longviewClientLabelFixture,
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

		if !reflect.DeepEqual(body["tool"], "linode_longview_client_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_longview_client_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], longviewClientsBasePath) {
			t.Errorf("got %v, want %v", would["path"], longviewClientsBasePath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLongviewClientCreateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeLongviewPlanUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLongviewPlanUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads plan then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, longviewPlanBasePath, linode.LongviewSubscription{ID: "longview-3"})
		_, _, handler := tools.NewLinodeLongviewPlanUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLongviewSubscription: longviewSubscriptionID,
			keyDryRun:               true,
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

		if !reflect.DeepEqual(body["tool"], "linode_longview_plan_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_longview_plan_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], longviewPlanBasePath) {
			t.Errorf("got %v, want %v", would["path"], longviewPlanBasePath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeLongviewClientUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLongviewClientUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads client then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, longviewClientGetPath, linode.LongviewClient{ID: 123, Label: longviewClientLabelFixture})
		_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: float64(123),
			keyLabel:    "renamed-client",
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

		if !reflect.DeepEqual(body["tool"], "linode_longview_client_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_longview_client_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], longviewClientGetPath) {
			t.Errorf("got %v, want %v", would["path"], longviewClientGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeLongviewClientDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLongviewClientDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, longviewClientGetPath, linode.LongviewClient{ID: 123})
		_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClientID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], "linode_longview_client_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_longview_client_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], longviewClientGetPath) {
			t.Errorf("got %v, want %v", would["path"], longviewClientGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeMonitorServiceTokenCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeMonitorServiceTokenCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeMonitorServiceTokenCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			monitorServiceTypeParam: monitorServiceToolTypeDatabase,
			keyEntityIDs:            []any{1, 2},
			keyDryRun:               true,
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

		if !reflect.DeepEqual(body["tool"], "linode_monitor_service_token_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_monitor_service_token_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/monitor/services/dbaas/token") {
			t.Errorf("got %v, want %v", would["path"], "/monitor/services/dbaas/token")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeMonitorServiceAlertDefinitionCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(dryRunNoCallServer(t))

		args := monitorAlertDefinitionCreateArgs()
		delete(args, keyConfirm)
		args[keyDryRun] = true

		result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

		if !reflect.DeepEqual(body["tool"], "linode_monitor_service_alert_definition_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_monitor_service_alert_definition_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], monitorDBaaSAlertsPath) {
			t.Errorf("got %v, want %v", would["path"], monitorDBaaSAlertsPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionCreateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			monitorServiceTypeParam: monitorServiceToolTypeDatabase,
			keyDryRun:               true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeMonitorServiceAlertDefinitionUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads definition then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, monitorAlertGetPath, linode.AlertDefinition{ID: 20000, Label: monitorAlertDefinitionToolLabel})
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			monitorServiceTypeParam:          monitorServiceToolTypeDatabase,
			monitorAlertIDParam:              float64(20000),
			monitorAlertDefinitionLabelParam: "renamed-alert",
			keyDryRun:                        true,
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

		if !reflect.DeepEqual(body["tool"], "linode_monitor_service_alert_definition_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_monitor_service_alert_definition_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], monitorAlertGetPath) {
			t.Errorf("got %v, want %v", would["path"], monitorAlertGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeMonitorServiceAlertDefinitionDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, monitorAlertGetPath, linode.AlertDefinition{ID: 20000})
		_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			monitorServiceTypeParam: monitorServiceToolTypeDatabase,
			monitorAlertIDParam:     float64(20000),
			keyDryRun:               true,
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

		if !reflect.DeepEqual(body["tool"], "linode_monitor_service_alert_definition_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_monitor_service_alert_definition_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], monitorAlertGetPath) {
			t.Errorf("got %v, want %v", would["path"], monitorAlertGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
