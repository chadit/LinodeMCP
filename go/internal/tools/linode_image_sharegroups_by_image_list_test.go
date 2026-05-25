package tools_test

import (
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

func TestLinodeImageShareGroupsByImageListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		assert.Equal(t, "linode_image_sharegroups_by_image_list", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyImageID, "schema should include image_id")
		assert.Contains(t, tool.InputSchema.Required, keyImageID, "image_id must be marked required")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := shareGroupDescription
		shareGroups := []linode.ImageShareGroup{{
			ID:           1,
			UUID:         shareGroupUUIDExample,
			Label:        "base-images",
			Description:  &description,
			IsSuspended:  false,
			Created:      "2025-04-14T22:44:02",
			Updated:      nil,
			Expiry:       nil,
			ImagesCount:  2,
			MembersCount: 3,
		}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/images/private%2F12345/sharegroups", r.URL.EscapedPath(), "request path should include escaped image ID and sharegroups suffix")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    shareGroups,
				keyPage:    2,
				keyPages:   3,
				keyResults: 51,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture, keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "base-images", "response should contain share group label")
		assert.Contains(t, textContent.Text, shareGroupUUIDExample, "response should contain share group UUID")
	})

	t.Run("rejects invalid image_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			"missing slash":       "private12345",
			"extra separator":     "private/12345/extra",
			"unsupported shared":  "shared/12345",
			"unsupported linode":  "linode/ubuntu24.04",
			"non-numeric private": "private/abc",
			"zero private":        privateImageZeroFixture,
			caseQuery:             "private/12345?query",
			caseDotdot:            privateImageTraversalFixture,
			caseEmpty:             blankString,
			caseNumeric:           12345,
		}

		for name, value := range invalidValues {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}))
				defer srv.Close()

				cfg := &config.Config{
					Environments: map[string]config.EnvironmentConfig{
						envKeyDefault: {
							Label:  envLabelDefault,
							Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
						},
					},
				}
				_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

				req := createRequestWithArgs(t, map[string]any{keyImageID: value})
				result, err := handler(t.Context(), req)

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "invalid image_id should be an error result")
				assert.False(t, called.Load(), "invalid image_id must be rejected before the client call")
			})
		}
	})

	t.Run("missing image_id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "missing image_id should be an error result")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve image share groups by image")
	})
}
