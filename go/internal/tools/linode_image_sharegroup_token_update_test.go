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

	assert.Equal(t, imageShareGroupTokenUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapAdmin, capability, "token update should be admin capability")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, keyTokenUUID, "schema should include token_uuid")
	assert.Contains(t, props, keyLabel, "schema should include label")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, tool.InputSchema.Required, keyTokenUUID, "token_uuid must be required")
	assert.Contains(t, tool.InputSchema.Required, keyLabel, "label must be required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
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

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Equal(t, int32(0), requestCount.Load(), "confirm failure must happen before client call")
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
		{name: "missing token uuid", args: map[string]any{keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: "token_uuid must be a non-empty string"},
		{name: "empty token uuid", args: map[string]any{keyTokenUUID: blankString, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: "token_uuid must be a non-empty string"},
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

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantContains)
			assert.Equal(t, int32(0), requestCount.Load(), "validation must happen before client call")
		})
	}
}

func TestLinodeImageShareGroupTokenUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/tokens/"+shareGroupUUIDFixture, r.URL.Path, "request path should include token UUID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, updatedShareGroupTokenLabel, body[keyLabel], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{
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

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.False(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, shareGroupUUIDFixture, "response should include token UUID")
	assert.Contains(t, textContent.Text, updatedShareGroupTokenLabel, "response should include updated label")
	assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupTokenUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"token not found"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupUUIDFixture,
		keyLabel:     updatedShareGroupTokenLabel,
		keyConfirm:   true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.True(t, result.IsError, "result should be a tool error")
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
