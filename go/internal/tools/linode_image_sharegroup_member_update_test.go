package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	imageShareGroupMemberUpdateToolName = "linode_image_sharegroup_member_update"
	updatedShareGroupMemberLabel        = "Engineering - Backend"
)

func TestLinodeImageShareGroupMemberUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(&config.Config{})

	assert.Equal(t, imageShareGroupMemberUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapWrite, capability, "member update should be write capability")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, keyShareGroupID, "schema should include sharegroup_id")
	assert.Contains(t, props, keyTokenUUID, "schema should include token_uuid")
	assert.Contains(t, props, keyLabel, "schema should include label")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be required")
	assert.Contains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be required")
	assert.Contains(t, tool.InputSchema.Required, keyLabel, "label must be required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeImageShareGroupMemberUpdateRequiresConfirm(t *testing.T) {
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

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupMemberUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Equal(t, int32(0), requestCount.Load(), "confirm failure must happen before client call")
		})
	}
}

func TestLinodeImageShareGroupMemberUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingShareGroupID, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "path separator sharegroup id", args: map[string]any{keyShareGroupID: paymentMethodIDSlash, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "query separator sharegroup id", args: map[string]any{keyShareGroupID: shareGroupIDQueryValue, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "missing token uuid", args: map[string]any{keyShareGroupID: 123, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "empty token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: blankString, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "path separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithSlash, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "query separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithQuery, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "fragment separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithFragment, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "traversal token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithDotdot, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "invalid uuid syntax", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: invalidTokenUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: caseMissingLabel, args: map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: blankString, keyConfirm: true}, wantContains: errLabelRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupMemberUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantContains)
			assert.Equal(t, int32(0), requestCount.Load(), "validation must happen before client call")
		})
	}
}

func TestLinodeImageShareGroupMemberUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/123/members/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include share group ID and token UUID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, updatedShareGroupMemberLabel, body[keyLabel], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{
			TokenUUID: shareGroupTokenGetUUID,
			Status:    statusActive,
			Label:     updatedShareGroupMemberLabel,
			Created:   imageShareGroupTokenCreated,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 123,
		keyTokenUUID:    shareGroupTokenGetUUID,
		keyLabel:        updatedShareGroupMemberLabel,
		keyConfirm:      true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.False(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, shareGroupTokenGetUUID, "response should include token UUID")
	assert.Contains(t, textContent.Text, updatedShareGroupMemberLabel, "response should include updated label")
	assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupMemberUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"member not found"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 123,
		keyTokenUUID:    shareGroupTokenGetUUID,
		keyLabel:        updatedShareGroupMemberLabel,
		keyConfirm:      true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.True(t, result.IsError, "result should be a tool error")
	assertErrorContains(t, result, "Failed to update image share group member token")
}

func newImageShareGroupMemberUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupMemberUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
