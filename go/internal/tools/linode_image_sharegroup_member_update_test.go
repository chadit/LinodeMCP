package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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

	shareGroupAssertEqual(t, imageShareGroupMemberUpdateToolName, tool.Name, "tool name should match")
	shareGroupAssertEqual(t, profiles.CapWrite, capability, "member update should be write capability")
	shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
	shareGroupRequireNotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	shareGroupAssertContains(t, props, keyShareGroupID, "schema should include sharegroup_id")
	shareGroupAssertContains(t, props, keyTokenUUID, "schema should include token_uuid")
	shareGroupAssertContains(t, props, keyLabel, "schema should include label")
	shareGroupAssertContains(t, props, keyConfirm, "schema should include confirm")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be required")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be required")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyLabel, "label must be required")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
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

			shareGroupRequireNoError(t, err, "handler should not return Go error")
			shareGroupRequireNotNil(t, result, "handler should return a result")
			shareGroupAssertTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			shareGroupAssertEqual(t, int32(0), requestCount.Load(), "confirm failure must happen before client call")
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

			shareGroupRequireNoError(t, err, "handler should not return Go error")
			shareGroupRequireNotNil(t, result, "handler should return a result")
			shareGroupAssertTrue(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantContains)
			shareGroupAssertEqual(t, int32(0), requestCount.Load(), "validation must happen before client call")
		})
	}
}

func TestLinodeImageShareGroupMemberUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shareGroupAssertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		shareGroupAssertEqual(t, "/images/sharegroups/123/members/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include share group ID and token UUID")
		shareGroupAssertEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		shareGroupAssertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		shareGroupAssertEqual(t, updatedShareGroupMemberLabel, body[keyLabel], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		shareGroupAssertNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupMember{
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

	shareGroupRequireNoError(t, err, "handler should not return Go error")
	shareGroupRequireNotNil(t, result, "handler should return a result")
	shareGroupAssertFalse(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	shareGroupRequireTrue(t, ok, "content should be TextContent")
	shareGroupAssertContains(t, textContent.Text, shareGroupTokenGetUUID, "response should include token UUID")
	shareGroupAssertContains(t, textContent.Text, updatedShareGroupMemberLabel, "response should include updated label")
	shareGroupAssertContains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupMemberUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"member not found"}]}`))
		shareGroupAssertNoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 123,
		keyTokenUUID:    shareGroupTokenGetUUID,
		keyLabel:        updatedShareGroupMemberLabel,
		keyConfirm:      true,
	}))

	shareGroupRequireNoError(t, err, "handler should not return Go error")
	shareGroupRequireNotNil(t, result, "handler should return a result")
	shareGroupAssertTrue(t, result.IsError, "result should be a tool error")
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
