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

const imageShareGroupImagesAddToolName = "linode_image_sharegroup_images_add"

func TestLinodeImageShareGroupImagesAddTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})

		assert.Equal(t, imageShareGroupImagesAddToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		assert.Contains(t, tool.InputSchema.Properties, keyImages, "schema should include images")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "mutating add tool must require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/images/sharegroups/54321/images", r.URL.Path, "request path should include share group ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			if !assert.Len(t, body[keyImages], 1) {
				return
			}

			image, ok := body[keyImages].([]any)[0].(map[string]any)
			if !assert.True(t, ok, "image payload should be an object") {
				return
			}

			assert.Equal(t, imagePrivate15Fixture, image[keyBetaID])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID:      "shared/1",
				keyLabel:       "Linux Debian",
				keyDescription: "Official Debian Linux image for server deployment",
				keyStatus:      statusAvailable,
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(imageShareGroupImagesAddConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 54321,
			keyImages:       `[{"id":" private/15 ","label":"Linux Debian"}]`,
			keyConfirm:      true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Added image", "response should include success message")
		assert.Contains(t, textContent.Text, "shared/1", "response should include image ID")
	})
}

func TestLinodeImageShareGroupImagesAddToolValidation(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		"missing confirm":  nil,
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	t.Run("invalid sharegroup id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 0,
			keyImages:       imagePrivate15JSON,
			keyConfirm:      true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "sharegroup_id must be a positive integer")
	})

	for name, images := range map[string]any{
		"missing images":        nil,
		"non-string images":     []any{map[string]any{keyBetaID: imagePrivate15Fixture}},
		"empty images":          `[]`,
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

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid images should be an error result")
			assert.Equal(t, int32(0), calls.Load(), "images validation must happen before client call")
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

		_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(imageShareGroupImagesAddConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 54321,
			keyImages:       imagePrivate15JSON,
			keyConfirm:      true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to add image to share group")
	})
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
