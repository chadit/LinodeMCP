package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeImageShareGroupTokenDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

		shareGroupAssertEqual(t, "linode_image_sharegroup_token_delete", tool.Name, "tool name should match")
		shareGroupAssertEqual(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyTokenUUID, "schema should include token_uuid")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		shareGroupRequireNotNil(t, handler, "handler should not be nil")
	})

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
			shareGroupAssertEqual(t, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include token UUID")
			shareGroupAssertEmpty(t, r.URL.RawQuery, "request query should be empty")
			shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
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

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "linode_image_sharegroup_token_delete failed")
	})
}

// Dry-run coverage for image share group token delete. CREDENTIAL SAFETY:
// the preview resolves the token to its PARENT share group (GET
// .../tokens/{uuid}/sharegroup), never fetching the token entity itself,
// so the token secret is not surfaced to the model.
func TestLinodeImageShareGroupTokenDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupTokenDeleteTool(&config.Config{})
		shareGroupAssertContains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview resolves parent group, never the token", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sharegroup") {
				w.Header().Set("Content-Type", "application/json")
				shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: 1234}))

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

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupRequireFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, isText)

		var body map[string]any
		shareGroupRequireNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		shareGroupAssertEqual(t, "linode_image_sharegroup_token_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		shareGroupAssertEqual(t, "DELETE", would["method"])
		shareGroupAssertEqual(t, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID, would["path"])

		shareGroupRequireLen(t, pathsSeen, 1, "dry_run must issue exactly one GET")
		shareGroupAssertContains(t, pathsSeen[0], "/sharegroup",
			"dry_run must resolve the parent group, not fetch the token secret")
	})

	t.Run("still validates token_uuid", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupTokenDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))

		shareGroupRequireNoError(t, err)
		shareGroupAssertTrue(t, result.IsError)
	})
}
