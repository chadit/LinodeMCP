package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeImageShareGroupImageDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

	if tool.Name != "linode_image_sharegroup_image_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_image_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyShareGroupID, keyImageID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyShareGroupID, keyImageID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupImageDeleteToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingShareGroupID, args: map[string]any{keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseSlashShareGroupID, args: map[string]any{keyShareGroupID: pathSeparatorValue, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseQueryShareGroupID, args: map[string]any{keyShareGroupID: shareGroupIDQueryValue, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseMissingImageID, args: map[string]any{keyShareGroupID: 1234, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageIDPositive},
		{name: "zero image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: 0, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageIDPositive},
		{name: "slash image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageIDPositive},
		{name: caseQueryImageID, args: map[string]any{keyShareGroupID: 1234, keyImageID: "5?6", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageIDPositive},
		{name: caseTraversalImageID, args: map[string]any{keyShareGroupID: 1234, keyImageID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageIDPositive},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeImageShareGroupImageDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/images/sharegroups/1234/images/5678" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/1234/images/5678")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "removed") {
		t.Errorf("textContent.Text does not contain %v", "removed")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestLinodeImageShareGroupImageDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_image_sharegroup_image_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_image_sharegroup_image_delete failed")
	}
}

// Dry-run coverage for image share group image delete. Preview fetches
// the PARENT group; would_execute targets the child image path.
func TestLinodeImageShareGroupImageDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeImageShareGroupImageDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeImageShareGroupImageDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	var methodsSeenMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeenMu.Lock()

		methodsSeen = append(methodsSeen, r.Method)

		methodsSeenMu.Unlock()

		if r.URL.Path != tcImagesSharegroups1234 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups1234)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: 1234}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 1234,
		keyImageID:      5678,
		keyDryRun:       true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_image_sharegroup_image_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_image_sharegroup_image_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/images/sharegroups/1234/images/5678") {
		t.Errorf("got %v, want %v", would["path"], "/images/sharegroups/1234/images/5678")
	}

	methodsSeenMu.Lock()

	seenMethods := append([]string(nil), methodsSeen...)

	methodsSeenMu.Unlock()

	if !reflect.DeepEqual(seenMethods, []string{http.MethodGet}) {
		t.Errorf("seenMethods = %v, want %v", seenMethods, []string{http.MethodGet})
	}
}

func TestLinodeImageShareGroupImageDeleteToolDryRunStillValidatesImageId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 1234,
		keyDryRun:       true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "image_id must be a positive integer") {
		t.Errorf("error text %q does not contain %q", text.Text, "image_id must be a positive integer")
	}
}
