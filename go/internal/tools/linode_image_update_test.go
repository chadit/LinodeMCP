package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

	if tool.Name != imageUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageUpdateToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyEnvironment, imageIDParam, keyLabel, keyDescription, keyTags, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantContains)
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageUpdateSuccess(t *testing.T) {
	t.Parallel()

	tags := []string{envProd, imageUploadTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/images/private%2F12345" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F12345")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			keyLabel:       imageUpdateLabel,
			keyDescription: imageUpdateDescription,
			keyTags:        []any{envProd, imageUploadTagWeb},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture, Label: imageUpdateLabel, Description: imageUpdateDescription, Tags: tags}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, imageIDFixture) {
		t.Errorf("textContent.Text does not contain %v", imageIDFixture)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeImageUpdateAcceptsDottedImageIDsAndDecodedTags(t *testing.T) {
	t.Parallel()

	tags := []string{envProd, imageUploadTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/private%2Fcustom%2Ev1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2Fcustom%2Ev1")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyTags], []any{envProd, imageUploadTagWeb}) {
			t.Errorf("body[keyTags] = %v, want %v", body[keyTags], []any{envProd, imageUploadTagWeb})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "private/custom.v1", Tags: tags}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: "private/custom.v1",
		keyTags:      []any{envProd, imageUploadTagWeb},
		keyConfirm:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeImageUpdateSendsEmptyTagsArray(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		tagsValue, tagsPresent := body[keyTags]
		if !tagsPresent {
			t.Error("tagsPresent = false, want true")
		}

		if v, ok := tagsValue.([]any); ok && len(v) != 0 {
			t.Errorf("tagsValue = %v, want empty", tagsValue)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture, Tags: []string{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: imageIDFixture,
		keyTags:      "[ ]",
		keyConfirm:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeImageUpdateDoesNotMutateArguments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Image{ID: imageIDFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))
	tags := []string{tcProd, imageUploadTagWeb}
	args := map[string]any{
		imageIDParam:      imageIDFixture,
		keyLabel:          imageUpdateLabel,
		keyTags:           tags,
		keyConfirm:        true,
		keyEnvironment:    envKeyDefault,
		"untouched_field": "untouched",
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if !reflect.DeepEqual(args[keyEnvironment], envKeyDefault) {
		t.Errorf("args[keyEnvironment] = %v, want %v", args[keyEnvironment], envKeyDefault)
	}

	if !reflect.DeepEqual(tags, []string{tcProd, imageUploadTagWeb}) {
		t.Errorf("tags = %v, want %v", tags, []string{tcProd, imageUploadTagWeb})
	}

	if !reflect.DeepEqual(args["untouched_field"], "untouched") {
		t.Errorf("got %v, want %v", args["untouched_field"], "untouched")
	}
}

func TestLinodeImageUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"image not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUpdateTool(imageUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageIDParam: imageIDFixture,
		keyLabel:     imageUpdateLabel,
		keyConfirm:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update image") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update image")
	}
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
