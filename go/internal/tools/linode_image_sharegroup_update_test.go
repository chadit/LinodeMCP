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
	imageShareGroupUpdateToolName    = "linode_image_sharegroup_update"
	imageShareGroupIDParam           = "sharegroup_id"
	imageShareGroupIDFixture         = 54321
	updatedImageShareGroupLabel      = "Engineering Base Images"
	updatedImageShareGroupDesc       = "Base images used by engineering teams"
	errImageShareGroupIDPositive     = "sharegroup_id must be a positive integer"
	errImageShareGroupUpdateRequired = "at least one of label or description is required"
)

func TestLinodeImageShareGroupUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupUpdateTool(&config.Config{})

	assert.Equal(t, imageShareGroupUpdateToolName, tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapWrite, capability, "share group update should be write capability")
	assert.NotEmpty(t, tool.Description, "tool should have a description")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, imageShareGroupIDParam, "schema should include sharegroup_id")
	assert.Contains(t, props, keyLabel, "schema should include label")
	assert.Contains(t, props, keyDescription, "schema should include description")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")
	assert.Contains(t, tool.InputSchema.Required, imageShareGroupIDParam, "sharegroup_id must be required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeImageShareGroupUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageShareGroupUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: updatedImageShareGroupLabel}
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

func TestLinodeImageShareGroupUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing share group id", args: map[string]any{keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "zero share group id", args: map[string]any{imageShareGroupIDParam: 0, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "negative share group id", args: map[string]any{imageShareGroupIDParam: -1, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "fractional share group id", args: map[string]any{imageShareGroupIDParam: 1.25, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "string share group id", args: map[string]any{imageShareGroupIDParam: "not-a-number", keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "path separator share group id", args: map[string]any{imageShareGroupIDParam: paymentMethodIDSlash, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "query separator share group id", args: map[string]any{imageShareGroupIDParam: paymentMethodIDQuery, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "traversal share group id", args: map[string]any{imageShareGroupIDParam: pathTraversalValue, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "missing update fields", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyConfirm: true}, wantContains: errImageShareGroupUpdateRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: "label must be a non-empty string"},
		{name: "blank description", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyDescription: blankString, keyConfirm: true}, wantContains: "description must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupUpdateHandler(t, &requestCount)
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

func TestLinodeImageShareGroupUpdateSuccess(t *testing.T) {
	t.Parallel()

	description := updatedImageShareGroupDesc

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/images/sharegroups/54321", r.URL.Path, "request path should include share group ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, updatedImageShareGroupLabel, body[keyLabel], "label should be sent")
		assert.Equal(t, description, body[keyDescription], "description should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           imageShareGroupIDFixture,
			UUID:         shareGroupUUIDFixture,
			Label:        updatedImageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      imageShareGroupCreated,
			Updated:      &description,
			ImagesCount:  2,
			MembersCount: 3,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam: imageShareGroupIDFixture,
		keyLabel:               updatedImageShareGroupLabel,
		keyDescription:         description,
		keyConfirm:             true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.False(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, updatedImageShareGroupLabel, "response should include updated label")
	assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageShareGroupUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"share group not found"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam: imageShareGroupIDFixture,
		keyLabel:               updatedImageShareGroupLabel,
		keyConfirm:             true,
	}))

	require.NoError(t, err, "handler should not return Go error")
	require.NotNil(t, result, "handler should return a result")
	assert.True(t, result.IsError, "result should be a tool error")
	assertErrorContains(t, result, "Failed to update image share group")
}

func newImageShareGroupUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
