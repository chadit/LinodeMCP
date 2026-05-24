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

func TestLinodeImageUploadTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeImageUploadTool(&config.Config{})

		assert.Equal(t, imageUploadToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyRegion, "schema should include region")
		assert.Contains(t, props, keyDescription, "schema should include description")
		assert.Contains(t, props, "cloud_init", "schema should include cloud_init")
		assert.Contains(t, props, keyTags, "schema should include tags")
		assert.Contains(t, props, keyConfirm, "mutating upload tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyLabel, "label must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyRegion, "region must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/images/upload", r.URL.Path, "request path should be /images/upload")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, imageUploadLabelFixture, body[keyLabel])
			assert.Equal(t, regionUSEast, body["region"])
			assert.Equal(t, "custom upload", body[keyDescription])
			assert.Equal(t, true, body["cloud_init"])
			assert.Equal(t, []any{envProd, imageUploadTagWeb}, body[keyTags])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"image": map[string]any{
					keyBetaID:      "private/99",
					keyLabel:       imageUploadLabelFixture,
					keyDescription: "custom upload",
					keyStatus:      imageUploadStatusFixture,
					keyRegion:      regionUSEast,
					keyTags:        []string{envProd, imageUploadTagWeb},
				},
				"upload_to": imageUploadTargetFixture,
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:       imageUploadLabelFixture,
			keyRegion:      regionUSEast,
			keyDescription: "custom upload",
			"cloud_init":   true,
			keyTags:        `["prod","web"]`,
			keyConfirm:     true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Image upload", "response should include success message")
		assert.Contains(t, textContent.Text, "private/99", "response should include image ID")
		assert.Contains(t, textContent.Text, imageUploadTargetFixture, "response should include upload target")
	})
}

func TestLinodeImageUploadToolValidation(t *testing.T) {
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast, keyConfirm: true}, want: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyLabel: "  ", keyRegion: regionUSEast, keyConfirm: true}, want: errLabelRequired},
		{name: caseMissingRegion, args: map[string]any{keyLabel: imageUploadLabelFixture, keyConfirm: true}, want: errRegionRequired},
		{name: "blank region", args: map[string]any{keyLabel: imageUploadLabelFixture, keyRegion: "  ", keyConfirm: true}, want: errRegionRequired},
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid input should be an error result")
			assertErrorContains(t, result, tt.want)
			assert.Equal(t, int32(0), calls.Load(), "validation must happen before client call")
		})
	}

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:   imageUploadLabelFixture,
			keyRegion:  regionUSEast,
			keyConfirm: true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to upload image")
	})
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
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"image":     map[string]any{keyBetaID: "private/99"},
			"upload_to": imageUploadTargetFixture,
		}))
	}))

	_, _, handler := tools.NewLinodeImageUploadTool(imageUploadConfig(srv.URL))

	return srv.Close, handler
}
