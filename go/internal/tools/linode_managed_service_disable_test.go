package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	managedServiceDisableToolName = "linode_managed_service_disable"
	managedServiceDisableToolPath = "/managed/services/9944/disable"
)

func TestLinodeManagedServiceDisableToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedServiceDisableTool(cfg)

	if tool.Name != managedServiceDisableToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedServiceDisableToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyManagedServiceID, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeManagedServiceDisableToolConfirmRequiredBeforeClientCall(t *testing.T) {
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

			_, _, handler := tools.NewLinodeManagedServiceDisableTool(managedServiceConfig(srv.URL))

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

func TestLinodeManagedServiceDisableToolInvalidServiceIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingServiceID, args: map[string]any{keyConfirm: true}, wantMessage: errManagedServiceIDRequired},
		{name: caseZeroServiceID, args: map[string]any{keyManagedServiceID: 0, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseNegativeServiceID, args: map[string]any{keyManagedServiceID: -1, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseStringServiceID, args: map[string]any{keyManagedServiceID: "9944", keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseFractionalServiceID, args: map[string]any{keyManagedServiceID: 9944.5, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseOversizedServiceID, args: map[string]any{keyManagedServiceID: managedServiceOversizedID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseSlashServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceSlashID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseQueryServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceQueryID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		{name: caseTraversalServiceID, args: map[string]any{keyManagedServiceID: pathTraversalValue, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
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

			_, _, handler := tools.NewLinodeManagedServiceDisableTool(managedServiceConfig(srv.URL))

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

func TestLinodeManagedServiceDisableToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedServiceDisableToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceDisableToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceDisableTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true}))
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

	var got map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &got); err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}

	if got["message"] != "Managed service disabled successfully" {
		t.Errorf("got[message] = %v, want %v", got["message"], "Managed service disabled successfully")
	}

	if got["service_id"] != float64(managedServiceToolIDValue) {
		t.Errorf("got[service_id] = %v, want %v", got["service_id"], managedServiceToolIDValue)
	}
}

func TestLinodeManagedServiceDisableToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedServiceDisableToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceDisableToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceDisableTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to disable linode_managed_service_disable") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to disable linode_managed_service_disable")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
