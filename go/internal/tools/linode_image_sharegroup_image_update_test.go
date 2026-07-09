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

	if tool.Name != imageShareGroupImageUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupImageUpdateToolName)
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
	for _, key := range []string{imageShareGroupIDParam, imageShareGroupImageIDParam, keyLabel, keyDescription, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
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

func TestLinodeImageShareGroupImageUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing share group id", args: map[string]any{imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errShareGroupIDRequired},
		{name: "invalid share group id", args: map[string]any{imageShareGroupIDParam: 0, imageShareGroupImageIDParam: imageShareGroupImageIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: caseMissingImageID, args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageIDNonEmpty},
		{name: "private source image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: imagePrivate15Fixture, keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "query separator image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1?query", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "fragment separator image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1#frag", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: "extra path segment image id", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/1/2", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
		{name: caseTraversalImageID, args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, imageShareGroupImageIDParam: "shared/../1", keyLabel: updatedSharedImageLabel, keyConfirm: true}, wantContains: errImageShareGroupImageIDInvalid},
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

func TestLinodeImageShareGroupImageUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/images/sharegroups/54321/images/shared%2F1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/54321/images/shared%2F1")
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

		if !reflect.DeepEqual(body[keyLabel], updatedSharedImageLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], updatedSharedImageLabel)
		}

		if !reflect.DeepEqual(body[keyDescription], updatedSharedImageDesc) {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], updatedSharedImageDesc)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Image{ID: imageShareGroupImageIDFixture, Label: updatedSharedImageLabel, Description: updatedSharedImageDesc}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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

	if !strings.Contains(textContent.Text, updatedSharedImageLabel) {
		t.Errorf("textContent.Text does not contain %v", updatedSharedImageLabel)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeImageShareGroupImageUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"shared image not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(imageShareGroupImageUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam:      imageShareGroupIDFixture,
		imageShareGroupImageIDParam: imageShareGroupImageIDFixture,
		keyLabel:                    updatedSharedImageLabel,
		keyConfirm:                  true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update shared image") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update shared image")
	}
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
