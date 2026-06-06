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
	imageShareGroupTokenUpdateToolName = "linode_image_sharegroup_token_update"
	updatedShareGroupTokenLabel        = "Backend Services - Engineering"
	shareGroupTokenUpdateCreated       = "2025-08-05T10:09:09"
	errTokenUUIDInvalid                = "token_uuid must be a UUID"
	errTokenUUIDUnsafe                 = "token_uuid must not contain path separators, query separators, fragments, or traversal segments"
	caseBlankLabelImageShareGroupToken = "blank label"
)

func TestLinodeImageShareGroupTokenUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(&config.Config{})

	shareGroupAssertEqual(t, imageShareGroupTokenUpdateToolName, tool.Name, "tool name should match")
	shareGroupAssertEqual(t, profiles.CapAdmin, capability, "token update should be admin capability")
	shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
	shareGroupRequireNotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	shareGroupAssertContains(t, props, keyTokenUUID, "schema should include token_uuid")
	shareGroupAssertContains(t, props, keyLabel, "schema should include label")
	shareGroupAssertContains(t, props, keyConfirm, "schema should include confirm")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be required")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyLabel, "label must be required")
	shareGroupAssertContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeImageShareGroupTokenUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageShareGroupTokenUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyLabel: updatedShareGroupTokenLabel}
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

func TestLinodeImageShareGroupTokenUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing token uuid", args: map[string]any{keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "empty token uuid", args: map[string]any{keyTokenUUID: blankString, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "path separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithSlash, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "query separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithQuery, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "fragment separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithFragment, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "encoded separator token uuid", args: map[string]any{keyTokenUUID: "token%2Fuuid", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: "traversal token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithDotdot, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "encoded traversal token uuid", args: map[string]any{keyTokenUUID: "token%2e%2euuid", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: "invalid uuid syntax", args: map[string]any{keyTokenUUID: "abc", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: caseMissingLabel, args: map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: errLabelRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupTokenUpdateHandler(t, &requestCount)
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

func TestLinodeImageShareGroupTokenUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shareGroupAssertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		shareGroupAssertEqual(t, "/images/sharegroups/tokens/"+shareGroupUUIDFixture, r.URL.Path, "request path should include token UUID")
		shareGroupAssertEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		shareGroupAssertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		shareGroupAssertEqual(t, updatedShareGroupTokenLabel, body[keyLabel], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		shareGroupAssertNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{
			TokenUUID:              shareGroupUUIDFixture,
			Status:                 statusActive,
			Label:                  updatedShareGroupTokenLabel,
			Created:                shareGroupTokenUpdateCreated,
			ValidForShareGroupUUID: shareGroupUUIDFixture,
			ShareGroupUUID:         shareGroupUUIDFixture,
			ShareGroupLabel:        shareGroupLabelFixture,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupUUIDFixture,
		keyLabel:     updatedShareGroupTokenLabel,
		keyConfirm:   true,
	}))

	shareGroupRequireNoError(t, err, "handler should not return Go error")
	shareGroupRequireNotNil(t, result, "handler should return a result")
	shareGroupAssertFalse(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	shareGroupRequireTrue(t, ok, "content should be TextContent")
	shareGroupAssertContains(t, textContent.Text, shareGroupUUIDFixture, "response should include token UUID")
	shareGroupAssertContains(t, textContent.Text, updatedShareGroupTokenLabel, "response should include updated label")
	shareGroupAssertContains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupTokenUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"token not found"}]}`))
		shareGroupAssertNoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupUUIDFixture,
		keyLabel:     updatedShareGroupTokenLabel,
		keyConfirm:   true,
	}))

	shareGroupRequireNoError(t, err, "handler should not return Go error")
	shareGroupRequireNotNil(t, result, "handler should return a result")
	shareGroupAssertTrue(t, result.IsError, "result should be a tool error")
	assertErrorContains(t, result, "Failed to update image share group token")
}

func newImageShareGroupTokenUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupTokenUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
