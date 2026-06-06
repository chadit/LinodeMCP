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
	imageUpdateToolName    = "linode_image_update"
	imageIDParam           = "image_id"
	imageIDFixture         = "private/12345"
	imageUpdateLabel       = "Updated Debian 12"
	imageUpdateDescription = "Updated image description"
	errImageUpdateRequired = "at least one of label, description, or tags is required"
	errImageIDPrefixed     = "image_id must be a prefixed image ID"
	errImageIDPrefix       = "image_id prefix must be linode, private, or shared"
	errImageIDNotEditable  = "image_id must reference an editable private or shared image"
	errImageIDUnsafe       = "image_id must not contain query separators, fragments, or traversal segments"
)

func TestLinodeImageUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageUpdateTool(&config.Config{})

	assertEqual(t, imageUpdateToolName, tool.Name, "tool name should match")
	assertEqual(t, profiles.CapWrite, capability, "image update should be write capability")
	assertNotEmpty(t, tool.Description, "tool should have a description")
	requireNotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties
	assertContains(t, props, keyEnvironment, "schema should include environment")
	assertContains(t, props, imageIDParam, "schema should include image_id")
	assertContains(t, props, keyLabel, "schema should include label")
	assertContains(t, props, keyDescription, "schema should include description")
	assertContains(t, props, keyTags, "schema should include tags")
	assertContains(t, props, keyConfirm, "schema should include confirm")
	assertContains(t, tool.InputSchema.Required, imageIDParam, "image_id must be required")
	assertContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeImageUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{imageIDParam: imageIDFixture, keyLabel: imageUpdateLabel}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			requireNoError(t, err, "handler should not return Go error")
			requireNotNil(t, result, "handler should return a result")
			assertTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assertEqual(t, int32(0), requestCount.Load(), "confirm failure must happen before client call")
		})
	}
}

func TestLinodeImageUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingImageID, args: map[string]any{keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: "image_id must be a non-empty string"},
		{name: "extra path separator image id", args: map[string]any{imageIDParam: "private/12345/extra", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDPrefixed},
		{name: "query separator image id", args: map[string]any{imageIDParam: "private/12345?x=1", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDUnsafe},
		{name: "fragment separator image id", args: map[string]any{imageIDParam: "private/12345#frag", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDUnsafe},
		{name: "unknown image id prefix", args: map[string]any{imageIDParam: "custom/12345", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDPrefix},
		{name: "read-only image id", args: map[string]any{imageIDParam: "linode/debian12", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDNotEditable},
		{name: caseTraversalImageID, args: map[string]any{imageIDParam: "shared/..", keyLabel: imageUpdateLabel, keyConfirm: true}, wantContains: errImageIDUnsafe},
		{name: "missing image update fields", args: map[string]any{imageIDParam: imageIDFixture, keyConfirm: true}, wantContains: errImageUpdateRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{imageIDParam: imageIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: "label must"},
		{name: "blank image description", args: map[string]any{imageIDParam: imageIDFixture, keyDescription: blankString, keyConfirm: true}, wantContains: "description must"},
		{name: "non-string tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: []any{1}, keyConfirm: true}, wantContains: errTagsMust},
		{name: "malformed tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: `[`, keyConfirm: true}, wantContains: errTagsMust},
		{name: "null tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: `null`, keyConfirm: true}, wantContains: errTagsMust},
		{name: "padded null tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: ` null `, keyConfirm: true}, wantContains: errTagsMust},
		{name: "object tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: jsonObjectEmpty, keyConfirm: true}, wantContains: errTagsMust},
		{name: "numeric tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: `123`, keyConfirm: true}, wantContains: errTagsMust},
		{name: "boolean tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: boolStringTrue, keyConfirm: true}, wantContains: errTagsMust},
		{name: "quoted string tags", args: map[string]any{imageIDParam: imageIDFixture, keyTags: `"tag"`, keyConfirm: true}, wantContains: errTagsMust},
		{name: "empty tag", args: map[string]any{imageIDParam: imageIDFixture, keyTags: `["prod",""]`, keyConfirm: true}, wantContains: "tags entries must be non-empty strings"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			requireNoError(t, err, "handler should not return Go error")
			requireNotNil(t, result, "handler should return a result")
			assertTrue(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantContains)
			assertEqual(t, int32(0), requestCount.Load(), "validation must happen before client call")
		})
	}
}

func TestLinodeImageUpdateSuccess(t *testing.T) {
	t.Parallel()

	tags := []string{envProd, imageUploadTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		assertEqual(t, "/images/private%2F12345", r.URL.EscapedPath(), "request path should include escaped image ID")
		assertEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

		var body map[string]any
		assertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assertEqual(t, imageUpdateLabel, body[keyLabel], "label should be sent")
		assertEqual(t, imageUpdateDescription, body[keyDescription], "description should be sent")
		assertEqual(t, []any{envProd, imageUploadTagWeb}, body[keyTags], "tags should be sent")

		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture, Label: imageUpdateLabel, Description: imageUpdateDescription, Tags: tags}))
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam:   imageIDFixture,
		keyLabel:       imageUpdateLabel,
		keyDescription: imageUpdateDescription,
		keyTags:        `["prod","web"]`,
		keyConfirm:     true,
	}))

	requireNoError(t, err, "handler should not return Go error")
	requireNotNil(t, result, "handler should return a result")
	assertFalse(t, result.IsError, "result should not be a tool error")

	textContent, ok := result.Content[0].(mcp.TextContent)
	requireTrue(t, ok, "content should be TextContent")
	assertContains(t, textContent.Text, imageIDFixture, "response should include updated image ID")
	assertContains(t, textContent.Text, "updated successfully", "response should confirm update")
}

func TestLinodeImageUpdateAcceptsDottedImageIDsAndDecodedTags(t *testing.T) {
	t.Parallel()

	tags := []string{envProd, imageUploadTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertEqual(t, "/images/private%2Fcustom%2Ev1", r.URL.EscapedPath(), "dotted image ID should be escaped as one segment")

		var body map[string]any
		assertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assertEqual(t, []any{envProd, imageUploadTagWeb}, body[keyTags], "decoded tags should be sent")

		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: "private/custom.v1", Tags: tags}))
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: "private/custom.v1",
		keyTags:      []any{envProd, imageUploadTagWeb},
		keyConfirm:   true,
	}))

	requireNoError(t, err, "handler should not return Go error")
	requireNotNil(t, result, "handler should return a result")
	assertFalse(t, result.IsError, "result should not be a tool error")
}

func TestLinodeImageUpdateSendsEmptyTagsArray(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		assertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		tagsValue, tagsPresent := body[keyTags]
		assertTrue(t, tagsPresent, "tags key should be present")
		assertEmpty(t, tagsValue, "empty tags array should be sent")

		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture, Tags: []string{}}))
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: imageIDFixture,
		keyTags:      "[ ]",
		keyConfirm:   true,
	}))

	requireNoError(t, err, "handler should not return Go error")
	requireNotNil(t, result, "handler should return a result")
	assertFalse(t, result.IsError, "result should not be a tool error")
}

func TestLinodeImageUpdateDoesNotMutateArguments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assertNoError(t, json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture}))
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	tags := []string{" prod ", imageUploadTagWeb}
	args := map[string]any{
		imageIDParam:      imageIDFixture,
		keyLabel:          imageUpdateLabel,
		keyTags:           tags,
		keyConfirm:        true,
		keyEnvironment:    envKeyDefault,
		"untouched_field": "untouched",
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))

	requireNoError(t, err, "handler should not return Go error")
	requireNotNil(t, result, "handler should return a result")
	assertFalse(t, result.IsError, "result should not be a tool error")
	assertEqual(t, envKeyDefault, args[keyEnvironment], "environment argument should not be rewritten")
	assertEqual(t, []string{" prod ", imageUploadTagWeb}, tags, "tag slice should not be mutated")
	assertEqual(t, "untouched", args["untouched_field"], "unrelated arguments should not be rewritten")
}

func TestLinodeImageUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"image not found"}]}`))
		assertNoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: imageIDFixture,
		keyLabel:     imageUpdateLabel,
		keyConfirm:   true,
	}))

	requireNoError(t, err, "handler should not return Go error")
	requireNotNil(t, result, "handler should return a result")
	assertTrue(t, result.IsError, "result should be a tool error")
	assertErrorContains(t, result, "Failed to update image")
}

func newImageUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
