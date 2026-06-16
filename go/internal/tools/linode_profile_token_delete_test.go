package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyProfileTokenID             = "token_id"
	profileTokenDeleteConfirmText = "confirm=true"
	profileTokenIDIntegerError    = "token_id must be an integer"
)

func TestLinodeProfileTokenDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

	if tool.Name != "linode_profile_token_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_token_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyProfileTokenID]; !ok {
		t.Errorf("props missing key %v", keyProfileTokenID)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyDryRun]; !ok {
		t.Errorf("props missing key %v", keyDryRun)
	}

	for _, key := range []string{keyProfileTokenID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeProfileTokenDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Profile token revoked successfully") {
		t.Errorf("textContent.Text does not contain %v", "Profile token revoked successfully")
	}
}

func TestLinodeProfileTokenDeleteToolDryRunPreviewsDeleteWithoutClientCall(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	would, ok := body["would_execute"].(map[string]any)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcProfileTokens12345) {
		t.Errorf("got %v, want %v", would["path"], tcProfileTokens12345)
	}

	if requestCount.Load() != int32(0) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
	}
}

func TestLinodeProfileTokenDeleteToolConfirmGuardRejectsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		set     bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, set: true},
		{name: caseString, confirm: boolStringTrue, set: true},
		{name: caseNumber, confirm: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			cfg := profileTokenTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

			args := map[string]any{keyProfileTokenID: 12345.0}
			if testCase.set {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, profileTokenDeleteConfirmText) {
				t.Errorf("error text %q does not contain %q", text.Text, profileTokenDeleteConfirmText)
			}
		})
	}
}

func TestLinodeProfileTokenDeleteToolInvalidTokenIdRejectsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: "token_id is required"},
		{name: caseZero, args: map[string]any{keyProfileTokenID: 0, keyConfirm: true, keyConfirmedDryRun: true}, want: "token_id must be an integer greater than or equal to 1"},
		{name: caseString, args: map[string]any{keyProfileTokenID: "12345", keyConfirm: true, keyConfirmedDryRun: true}, want: profileTokenIDIntegerError},
		{name: caseFractionalServiceID, args: map[string]any{keyProfileTokenID: 12345.5, keyConfirm: true, keyConfirmedDryRun: true}, want: profileTokenIDIntegerError},
		{name: caseSlashServiceID, args: map[string]any{keyProfileTokenID: placementGroupSlashID, keyConfirm: true, keyConfirmedDryRun: true}, want: profileTokenIDIntegerError},
		{name: caseQueryServiceID, args: map[string]any{keyProfileTokenID: "12?token=1", keyConfirm: true, keyConfirmedDryRun: true}, want: profileTokenIDIntegerError},
		{name: caseTraversalServiceID, args: map[string]any{keyProfileTokenID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: profileTokenIDIntegerError},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			cfg := profileTokenTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeProfileTokenDeleteToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_profile_token_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_profile_token_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
