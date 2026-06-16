package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	accountServiceTransfersTestPath = "/account/service-transfers"
	transferTestToken               = "tok-abc"
	serviceTransferGetPath          = accountServiceTransfersTestPath + "/" + transferTestToken
)

func TestLinodeAccountServiceTransferCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeIDs: []any{float64(123)},
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

		if !reflect.DeepEqual(body["tool"], "linode_account_service_transfer_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_service_transfer_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountServiceTransfersTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountServiceTransfersTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeAccountServiceTransferAcceptToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferAcceptTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without accepting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, serviceTransferGetPath, map[string]any{keyToken: transferTestToken})
		_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyToken:  transferTestToken,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_service_transfer_accept") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_service_transfer_accept")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], serviceTransferGetPath+"/accept") {
			t.Errorf("got %v, want %v", would["path"], serviceTransferGetPath+"/accept")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeAccountServiceTransferDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, serviceTransferGetPath, map[string]any{keyToken: transferTestToken})
		_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyToken:  transferTestToken,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_service_transfer_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_service_transfer_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], serviceTransferGetPath) {
			t.Errorf("got %v, want %v", would["path"], serviceTransferGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
