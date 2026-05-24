package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

		assert.Equal(t, "linode_image_sharegroup_member_token_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		assert.Contains(t, tool.InputSchema.Properties, keyTokenUUID, "schema should include token_uuid")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
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
		{name: caseMissingShareGroupID, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: caseSlashShareGroupID, args: map[string]any{keyShareGroupID: pathSeparatorValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: caseQueryShareGroupID, args: map[string]any{keyShareGroupID: pathQueryValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "slash token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token/uuid", keyConfirm: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "query token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token?uuid", keyConfirm: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "fragment token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token#uuid", keyConfirm: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "traversal token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: "token..uuid", keyConfirm: true}, wantContains: errImageShareTokenNoSeparators},
		{name: "empty token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: blankString, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "numeric token rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: 123, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "invalid uuid rejected", args: map[string]any{keyShareGroupID: 1234, keyTokenUUID: invalidTokenUUID, keyConfirm: true}, wantContains: "token_uuid must be a UUID"},
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid delete request should be an error result")
			assertErrorContains(t, result, tt.wantContains)
			assert.False(t, called.Load(), "validation should reject before client call")
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/images/sharegroups/1234/members/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include share group ID and token UUID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "revoked", "response should include success message")
		assert.Equal(t, int32(1), requestCount.Load(), "delete should make one request")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenDeleteTool(imageShareGroupMemberTokenDeleteConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "Failed to revoke image share group member token")
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
