package tools_test

import (
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

func TestLinodeImageShareGroupsByImageListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		assertEqual(t, "linode_image_sharegroups_by_image_list", tool.Name, "tool name should match")
		assertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Properties, keyImageID, "schema should include image_id")
		assertContains(t, tool.InputSchema.Required, keyImageID, "image_id must be marked required")
		assertNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		requireNotNil(t, handler, "handler should not be nil")
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
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, "/images/private%2F12345/sharegroups", r.URL.EscapedPath(), "request path should include escaped image ID and sharegroups suffix")
			assertEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "base-images", "response should contain share group label")
		assertContains(t, textContent.Text, shareGroupUUIDExample, "response should contain share group UUID")
	})

	t.Run("rejects invalid image_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			"missing slash":       "private12345",
			caseExtraSeparator:    "private/12345/extra",
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

				requireNoError(t, err)
				requireNotNil(t, result)
				assertTrue(t, result.IsError, "invalid image_id should be an error result")
				assertFalse(t, called.Load(), "invalid image_id must be rejected before the client call")
			})
		}
	})

	t.Run("missing image_id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "missing image_id should be an error result")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to retrieve image share groups by image")
	})
}
