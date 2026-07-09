package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeImageShareGroupTokenDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

	if tool.Name != "linode_image_sharegroup_token_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_token_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyTokenUUID, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupTokenDeleteToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "slash token rejected", args: map[string]any{keyTokenUUID: "token/uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "query token rejected", args: map[string]any{keyTokenUUID: "token?uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "fragment token rejected", args: map[string]any{keyTokenUUID: "token#uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "empty token rejected", args: map[string]any{keyTokenUUID: blankString, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "traversal token rejected", args: map[string]any{keyTokenUUID: "token..uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "numeric token rejected", args: map[string]any{keyTokenUUID: 123, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errTokenUUIDNonEmpty},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {
						Label:  envLabelDefault,
						Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
					},
				},
			}
			_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

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

func TestLinodeImageShareGroupTokenDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/images/sharegroups/tokens/"+shareGroupTokenGetUUID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID)
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

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestLinodeImageShareGroupTokenDeleteToolClientError(t *testing.T) {
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

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_image_sharegroup_token_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_image_sharegroup_token_delete failed")
	}
}

// Dry-run coverage for image share group token delete. CREDENTIAL SAFETY:
// the preview resolves the token to its PARENT share group (GET
// .../tokens/{uuid}/sharegroup), never fetching the token entity itself,
// so the token secret is not surfaced to the model.
func TestLinodeImageShareGroupTokenDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeImageShareGroupTokenDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeImageShareGroupTokenDeleteToolDryRunPreviewResolvesParentGroupNeverTheToken(t *testing.T) {
	t.Parallel()

	var pathsSeen []string

	var pathsSeenMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathsSeenMu.Lock()

		pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

		pathsSeenMu.Unlock()

		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sharegroup") {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: 1234}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		t.Errorf("dry_run must only resolve the parent group; got %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupTokenGetUUID,
		keyDryRun:    true,
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

	if !reflect.DeepEqual(body["tool"], "linode_image_sharegroup_token_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_image_sharegroup_token_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/images/sharegroups/tokens/"+shareGroupTokenGetUUID) {
		t.Errorf("got %v, want %v", would["path"], "/images/sharegroups/tokens/"+shareGroupTokenGetUUID)
	}

	pathsSeenMu.Lock()

	seenPaths := append([]string(nil), pathsSeen...)

	pathsSeenMu.Unlock()

	if len(seenPaths) != 1 {
		t.Fatalf("len(seenPaths) = %d, want %d", len(seenPaths), 1)
	}

	if !strings.Contains(seenPaths[0], "/sharegroup") {
		t.Errorf("seenPaths[0] does not contain %v", "/sharegroup")
	}
}

func TestLinodeImageShareGroupTokenDeleteToolDryRunStillValidatesTokenUuid(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}
