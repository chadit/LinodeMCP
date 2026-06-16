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

func TestLinodeImageShareGroupMemberTokenDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(cfg)

	if tool.Name != "linode_image_sharegroup_member_token_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_member_token_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyShareGroupID, keyTokenUUID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyShareGroupID, keyTokenUUID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupMemberTokenDeleteToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingShareGroupID, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseSlashShareGroupID, args: map[string]any{keyShareGroupID: pathSeparatorValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseQueryShareGroupID, args: map[string]any{keyShareGroupID: pathQueryValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: "slash token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token/uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "query token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token?uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "fragment token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token#uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "traversal token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token..uuid", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "empty token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: blankString, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "numeric token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: 123, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "invalid uuid rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: invalidTokenUUID, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "token_uuid must be a UUID"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

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

func TestLinodeImageShareGroupMemberTokenDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID)
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

	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "revoked") {
		t.Errorf("textContent.Text does not contain %v", "revoked")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestLinodeImageShareGroupMemberTokenDeleteToolClientError(t *testing.T) {
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

	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_image_sharegroup_member_token_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_image_sharegroup_member_token_delete failed")
	}
}

// Dry-run coverage for image share group member token delete. CREDENTIAL
// SAFETY: the preview fetches the PARENT share group, never the member
// token entity, so the token secret is not surfaced to the model.
func TestLinodeImageShareGroupMemberTokenDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeImageShareGroupMemberTokenDeleteToolDryRunPreviewFetchesParentGroupNeverTheToken(t *testing.T) {
	t.Parallel()

	var pathsSeen []string

	var pathsSeenMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathsSeenMu.Lock()

		pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

		pathsSeenMu.Unlock()

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

	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 1234,
		keyTokenUUID:    shareGroupTokenGetUUID,
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

	if !reflect.DeepEqual(body["tool"], "linode_image_sharegroup_member_token_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_image_sharegroup_member_token_delete")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID) {
		t.Errorf("got %v, want %v", would["path"], "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID)
	}

	pathsSeenMu.Lock()

	seenPaths := append([]string(nil), pathsSeen...)

	pathsSeenMu.Unlock()

	if len(seenPaths) != 1 {
		t.Fatalf("len(seenPaths) = %d, want %d", len(seenPaths), 1)
	}
}

func TestLinodeImageShareGroupMemberTokenDeleteToolDryRunStillValidatesTokenUuid(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(&config.Config{})

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
}

func imageShareGroupMemberTokenDeleteConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}
