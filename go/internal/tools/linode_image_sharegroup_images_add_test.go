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
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const imageShareGroupImagesAddToolName = "linode_image_sharegroup_image_add"

func TestLinodeImageShareGroupImagesAddToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})

	if tool.Name != imageShareGroupImagesAddToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupImagesAddToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyShareGroupID, keyImages, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupImagesAddToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/sharegroups/54321/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/54321/images")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		images, imagesOK := body[keyImages].([]any)
		if !imagesOK || len(images) != 1 {
			t.Errorf("len(body[keyImages]) = %d, want %d", len(images), 1)

			return
		}

		image, ok := images[0].(map[string]any)
		if !ok {
			t.Error("image payload should be an object")

			return
		}

		if !reflect.DeepEqual(image[keyBetaID], imagePrivate15Fixture) {
			t.Errorf("image[keyBetaID] = %v, want %v", image[keyBetaID], imagePrivate15Fixture)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID:      "shared/1",
			keyLabel:       "Linux Debian",
			keyDescription: "Official Debian Linux image for server deployment",
			keyStatus:      statusAvailable,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(imageShareGroupImagesAddConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 54321,
		keyImages:       `[{"id":" private/15 ","label":"Linux Debian"}]`,
		keyConfirm:      true,
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

	if !strings.Contains(textContent.Text, "Added image") {
		t.Errorf("textContent.Text does not contain %v", "Added image")
	}

	if !strings.Contains(textContent.Text, "shared/1") {
		t.Errorf("textContent.Text does not contain %v", "shared/1")
	}
}

func TestLinodeImageShareGroupImagesAddToolValidationConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseMissingConfirm: nil,
		caseFalseConfirm:   false,
		caseStringConfirm:  boolStringTrue,
		caseNumericConfirm: 1,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageShareGroupImagesAddHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			args := map[string]any{keyShareGroupID: 54321, keyImages: imagePrivate15JSON}
			if confirm != nil {
				args[keyConfirm] = confirm
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageShareGroupImagesAddToolValidationInvalidSharegroupId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 0,
		keyImages:       imagePrivate15JSON,
		keyConfirm:      true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "sharegroup_id must be a positive integer") {
		t.Errorf("error text %q does not contain %q", text.Text, "sharegroup_id must be a positive integer")
	}
}

func TestLinodeImageShareGroupImagesAddToolValidationImages(t *testing.T) {
	t.Parallel()

	for name, images := range map[string]any{
		"missing images":        nil,
		"non-string images":     []any{map[string]any{keyBetaID: imagePrivate15Fixture}},
		"empty images":          databaseJSONArray,
		"blank images":          `   `,
		"image missing id":      `[{"label":"missing id"}]`,
		"malformed images JSON": `[{`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageShareGroupImagesAddHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			args := map[string]any{keyShareGroupID: 54321, keyConfirm: true}
			if images != nil {
				args[keyImages] = images
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageShareGroupImagesAddToolValidationClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(imageShareGroupImagesAddConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 54321,
		keyImages:       imagePrivate15JSON,
		keyConfirm:      true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to add image to share group") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to add image to share group")
	}
}

func imageShareGroupImagesAddConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

func imageShareGroupImagesAddHandlerWithCallCounter(
	t *testing.T,
	calls *atomic.Int32,
) (func(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(imageShareGroupImagesAddConfig(srv.URL))

	return srv.Close, handler
}
