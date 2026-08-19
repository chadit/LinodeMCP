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
	longviewClientsBasePath = "/longview/clients"
	longviewClientGetPath   = longviewClientsBasePath + "/123"
	longviewPlanBasePath    = "/longview/plan"
)

func TestLinodeLongviewClientCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeLongviewClientCreateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := gentools.NewLinodeLongviewClientCreateTool(dryRunNoCallServer(t))

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

		_, _, handler := gentools.NewLinodeLongviewClientCreateTool(&config.Config{})

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

		tool, _, _ := gentools.NewLinodeLongviewPlanUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads plan then would PUT", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, longviewPlanBasePath, linode.LongviewSubscription{ID: "longview-3"})
		_, _, handler := gentools.NewLinodeLongviewPlanUpdateTool(cfg)

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

func TestLinodeLongviewClientDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeLongviewClientDeleteTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, longviewClientGetPath, linode.LongviewClient{ID: 123})
		_, _, handler := gentools.NewLinodeLongviewClientDeleteTool(cfg)

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
