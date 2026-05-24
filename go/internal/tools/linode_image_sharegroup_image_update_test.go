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
	imageShareGroupImageUpdateToolName = "linode_image_sharegroup_image_update"
	imageShareGroupImageIDParam        = "image_id"
	imageShareGroupImageIDFixture      = "shared/1"
	updatedSharedImageLabel            = "Updated Shared Debian"
	updatedSharedImageDesc             = "Updated shared image description"
	errImageShareGroupImageIDInvalid   = "image_id must match shared/<positive integer>"
)

func TestLinodeImageShareGroupImageUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupImageUpdateTool(&config.Config{})

	assert.Equal(t, imageShareGroupImageUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapWrite, capability, "shared image update should be write capability")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, imageShareGroupIDParam, "schema should include sharegroup_id")
	assert.Contains(t, props, imageShareGroupImageIDParam, "schema should include image_id")
	assert.Contains(t, props, keyLabel, "schema should include label")
	assert.Contains(t, props, keyDescription, "schema should include description")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, tool.InputSchema.Required, imageShareGroupIDParam, "sharegroup_id must be required")
	assert.Contains(t, tool.InputSchema.Required, imageShareGroupImageIDParam, "image_id must be required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeImageShareGroupImageUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageShareGroupImageUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{
				imageShareGroupIDParam:      imageShareGroupIDFixture,
				imageShareGroupImageIDParam: imageShareGroupImageIDFixture,
				keyLabel:                    updatedSharedImageLabel,
			}
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

func TestLinodeImageShareGroupImageUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing share group id", args: map[string]any{imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "invalid share group id", args: map[string]any{imageShareGroupIDParam: 0, imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "missing image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: "image_id must be a non-empty string"},
		{name: "private source image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: imagePrivate15Fixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "query separator image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1?query", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "fragment separator image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1#frag", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "extra path segment image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1/2", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "traversal image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/../1", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "missing update fields", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyConfirm: true}, wantContains: errImageShareGroupUpdateRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: "label must be a non-empty string"},
		{name: "blank description", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyDescription: blankString, keyConfirm: true}, wantContains: "description must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupImageUpdateHandler(t, &requestCount)
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

func TestLinodeImageShareGroupImageUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/54321/images/shared%2F1", r.URL.EscapedPath(), "request path should include escaped shared image ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, updatedSharedImageLabel, body[keyLabel], "label should be sent")
		assert.Equal(t, updatedSharedImageDesc, body[keyDescription], "description should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Image{ID: imageShareGroupImageIDFixture, Label: updatedSharedImageLabel, Description: updatedSharedImageDesc}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(imageShareGroupImageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam:      imageShareGroupIDFixture,
		imageShareGroupImageIDParam: imageShareGroupImageIDFixture,
		keyLabel:                    updatedSharedImageLabel,
		keyDescription:              updatedSharedImageDesc,
		keyConfirm:                  true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.False(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, updatedSharedImageLabel, "response should include updated label")
	assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupImageUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"shared image not found"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(imageShareGroupImageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam:      imageShareGroupIDFixture,
		imageShareGroupImageIDParam: imageShareGroupImageIDFixture,
		keyLabel:                    updatedSharedImageLabel,
		keyConfirm:                  true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.True(t, result.IsError, "result should be a tool error")
	assertErrorContains(t, result, "Failed to update shared image")
}

func newImageShareGroupImageUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(imageShareGroupImageUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupImageUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
