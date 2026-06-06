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

func TestLinodeImageShareGroupMemberTokenDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(cfg)

		shareGroupAssertEqual(t, "linode_image_sharegroup_member_token_delete", tool.Name, "tool name should match")
		shareGroupAssertEqual(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyTokenUUID, "schema should include token_uuid")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		shareGroupAssertContains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be marked required")
		shareGroupAssertContains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be marked required")
		shareGroupAssertContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		shareGroupRequireNotNil(t, handler, "handler should not be nil")
	})

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
			shareGroupAssertEqual(t, "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include share group ID and token UUID")
			shareGroupAssertEmpty(t, r.URL.RawQuery, "request query should be empty")
			shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))

		shareGroupRequireNoError(t, err, "handler should not return an error")
		shareGroupRequireNotNil(t, result, "result should not be nil")
		shareGroupAssertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, ok, "content should be TextContent")
		shareGroupAssertContains(t, textContent.Text, "revoked", "response should include success message")
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

		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true, keyConfirmedDryRun: true}))

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "linode_image_sharegroup_member_token_delete failed")
	})
}

// Dry-run coverage for image share group member token delete. CREDENTIAL
// SAFETY: the preview fetches the PARENT share group, never the member
// token entity, so the token secret is not surfaced to the model.
func TestLinodeImageShareGroupMemberTokenDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(&config.Config{})
		shareGroupAssertContains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview fetches parent group, never the token", func(t *testing.T) {
		t.Parallel()

		var pathsSeen []string

		var pathsSeenMu sync.Mutex

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathsSeenMu.Lock()

			pathsSeen = append(pathsSeen, r.Method+" "+r.URL.Path)

			pathsSeenMu.Unlock()
			shareGroupAssertEqual(t, "/images/sharegroups/1234", r.URL.Path,
				"dry_run must GET the parent group, not the member token")

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: 1234}))

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

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupRequireFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, isText)

		var body map[string]any
		shareGroupRequireNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		shareGroupAssertEqual(t, "linode_image_sharegroup_member_token_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		shareGroupAssertEqual(t, "DELETE", would["method"])
		shareGroupAssertEqual(t, "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID, would["path"])

		pathsSeenMu.Lock()

		seenPaths := append([]string(nil), pathsSeen...)

		pathsSeenMu.Unlock()

		shareGroupRequireLen(t, seenPaths, 1, "dry_run must issue exactly one GET")
	})

	t.Run("still validates token_uuid", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 1234,
			keyDryRun:       true,
		}))

		shareGroupRequireNoError(t, err)
		shareGroupAssertTrue(t, result.IsError)
	})
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
