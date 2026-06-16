package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const managedServiceDeleteToolName = "linode_managed_service_delete"

func TestLinodeManagedServiceDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedServiceDeleteTool(cfg)

	if tool.Name != managedServiceDeleteToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedServiceDeleteToolName)
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyManagedServiceID]; !ok {
		t.Errorf("props missing key %v", keyManagedServiceID)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyManagedServiceID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeManagedServiceDeleteToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

			args := map[string]any{keyManagedServiceID: managedServiceToolIDValue}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceDeleteToolInvalidServiceIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDRequired},
		{name: caseZeroServiceID, args: map[string]any{keyManagedServiceID: 0, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseNegativeServiceID, args: map[string]any{keyManagedServiceID: -1, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseStringServiceID, args: map[string]any{keyManagedServiceID: "9944", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseFractionalServiceID, args: map[string]any{keyManagedServiceID: 9944.5, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseOversizedServiceID, args: map[string]any{keyManagedServiceID: managedServiceOversizedID, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseSlashServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceSlashID, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseQueryServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceQueryID, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseTraversalServiceID, args: map[string]any{keyManagedServiceID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errManagedServiceIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "deleted successfully")
	}
}

func TestLinodeManagedServiceDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_managed_service_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_managed_service_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
