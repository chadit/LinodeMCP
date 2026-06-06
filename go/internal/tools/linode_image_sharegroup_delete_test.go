package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeImageShareGroupDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupDeleteTool(cfg)

		shareGroupAssertEqual(t, "linode_image_sharegroup_delete", tool.Name, "tool name should match")
		shareGroupAssertEqual(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		shareGroupAssertContains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be marked required")
		shareGroupAssertContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		shareGroupRequireNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyShareGroupID: 1234}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingShareGroupID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: "negative sharegroup id", args: map[string]any{keyShareGroupID: -1, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseSlashShareGroupID, args: map[string]any{keyShareGroupID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseQueryShareGroupID, args: map[string]any{keyShareGroupID: shareGroupIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errShareGroupIDPositive},
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
			_, _, handler := tools.NewLinodeImageShareGroupDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			shareGroupRequireNoError(t, err)
			shareGroupRequireNotNil(t, result)
			shareGroupAssertTrue(t, result.IsError, "invalid delete request should be an error result")
			assertErrorContains(t, result, tt.wantContains)
			shareGroupAssertFalse(t, called.Load(), "validation should reject before client call")
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			shareGroupAssertEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			shareGroupAssertEqual(t, "/images/sharegroups/1234", r.URL.Path, "request path should include share group ID")
			shareGroupAssertEmpty(t, r.URL.RawQuery, "request query should be empty")
			shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageShareGroupDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyConfirm: true, keyConfirmedDryRun: true}))

		shareGroupRequireNoError(t, err, "handler should not return an error")
		shareGroupRequireNotNil(t, result, "result should not be nil")
		shareGroupAssertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, ok, "content should be TextContent")
		shareGroupAssertContains(t, textContent.Text, "removed successfully", "response should include success message")
		shareGroupAssertEqual(t, int32(1), requestCount.Load(), "delete should make one request")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageShareGroupDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyConfirm: true, keyConfirmedDryRun: true}))

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "linode_image_sharegroup_delete failed")
	})
}

// Dry-run coverage for image share group delete.
func TestLinodeImageShareGroupDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupDeleteTool(&config.Config{})
		shareGroupAssertContains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		var methodsSeenMu sync.Mutex

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeenMu.Lock()

			methodsSeen = append(methodsSeen, r.Method)

			methodsSeenMu.Unlock()
			shareGroupAssertEqual(t, "/images/sharegroups/1234", r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: 1234}))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageShareGroupDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 1234,
			keyDryRun:       true,
		}))

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupRequireFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, isText)

		var body map[string]any
		shareGroupRequireNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		shareGroupAssertEqual(t, "linode_image_sharegroup_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		shareGroupAssertEqual(t, "DELETE", would["method"])
		shareGroupAssertEqual(t, "/images/sharegroups/1234", would["path"])

		methodsSeenMu.Lock()

		seenMethods := append([]string(nil), methodsSeen...)

		methodsSeenMu.Unlock()

		shareGroupAssertEqual(t, []string{http.MethodGet}, seenMethods,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("still validates sharegroup_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))

		shareGroupRequireNoError(t, err)
		shareGroupAssertTrue(t, result.IsError)
		assertErrorContains(t, result, errShareGroupIDPositive)
	})
}
