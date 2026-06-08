package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const imageUploadToolName = "linode_image_upload"

const (
	imageUploadLabelFixture  = "custom-image"
	imageUploadStatusFixture = "creating"
	imageUploadTagWeb        = "web"
	imageUploadTargetFixture = "https://uploads.example.test/custom-image"
	keyTags                  = "tags"
)

func TestLinodeImageUploadToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageUploadTool(&config.Config{})

	if tool.Name != imageUploadToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageUploadToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyRegion]; !ok {
		t.Errorf("props missing key %v", keyRegion)
	}

	if _, ok := props[keyDescription]; !ok {
		t.Errorf("props missing key %v", keyDescription)
	}

	if _, ok := props[tcCloudInit]; !ok {
		t.Errorf("props missing key %v", tcCloudInit)
	}

	if _, ok := props[keyTags]; !ok {
		t.Errorf("props missing key %v", keyTags)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyLabel, keyRegion, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageUploadToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/upload" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/upload")
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

		for key, want := range map[string]any{
			keyLabel:               imageUploadLabelFixture,
			keySupportTicketRegion: regionUSEast,
			keyDescription:         tcCustomUpload,
			tcCloudInit:            true,
			keyTags:                []any{envProd, imageUploadTagWeb},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"image": map[string]any{
				keyBetaID:      "private/99",
				keyLabel:       imageUploadLabelFixture,
				keyDescription: tcCustomUpload,
				keyStatus:      imageUploadStatusFixture,
				keyRegion:      regionUSEast,
				keyTags:        []string{envProd, imageUploadTagWeb},
			},
			"upload_to": imageUploadTargetFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:       imageUploadLabelFixture,
		keyRegion:      regionUSEast,
		keyDescription: tcCustomUpload,
		tcCloudInit:    true,
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

	if !strings.Contains(textContent.Text, "Image upload") {
		t.Errorf("textContent.Text does not contain %v", "Image upload")
	}

	if !strings.Contains(textContent.Text, "private/99") {
		t.Errorf("textContent.Text does not contain %v", "private/99")
	}

	if !strings.Contains(textContent.Text, imageUploadTargetFixture) {
		t.Errorf("textContent.Text does not contain %v", imageUploadTargetFixture)
	}
}

func TestLinodeImageUploadToolValidationClientErrorConfirm(t *testing.T) {
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

			closeServer, handler := imageUploadHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			args := map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: regionUSEast}
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

func TestLinodeImageUploadToolValidationClientErrorTt(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast, keyConfirm: true}, want: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLabel: blankString, keyRegion: regionUSEast, keyConfirm: true}, want: errLabelRequired},
		{name: caseMissingRegion, args: map[string]any{keyLabel: imageUploadLabelFixture, keyConfirm: true}, want: errRegionRequired},
		{name: caseBlankRegion, args: map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: blankString, keyConfirm: true}, want: errRegionRequired},
		{name: "non-string tags", args: map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: regionUSEast, keyTags: []any{envProd}, keyConfirm: true}, want: "tags must be a JSON string array"},
		{name: "malformed tags", args: map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: regionUSEast, keyTags: `[`, keyConfirm: true}, want: "tags must be a JSON string array"},
		{name: "empty tag", args: map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: regionUSEast, keyTags: `["prod",""]`, keyConfirm: true}, want: "tags entries must be non-empty strings"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageUploadHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.want) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageUploadToolValidationClientErrorDirect(t *testing.T) {
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
	_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:   imageUploadLabelFixture,
		keyRegion:  regionUSEast,
		keyConfirm: true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to upload image") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to upload image")
	}
}

func imageUploadConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

func imageUploadHandlerWithCallCounter(
	t *testing.T,
	calls *atomic.Int32,
) (func(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"image":     map[string]any{keyBetaID: "private/99"},
			"upload_to": imageUploadTargetFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))

	_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))

	return srv.Close, handler
}
