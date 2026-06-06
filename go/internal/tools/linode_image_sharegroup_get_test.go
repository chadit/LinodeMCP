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

func TestLinodeImageShareGroupGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

		assertEqual(t, "linode_image_sharegroup_get", tool.Name, "tool name should match")
		assertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		assertNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := shareGroupDescription
		updated := shareGroupUpdated
		shareGroup := linode.ImageShareGroup{
			ID:           123,
			UUID:         shareGroupUUIDFixture,
			Label:        shareGroupLabelFixture,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreated,
			Updated:      &updated,
			ImagesCount:  2,
			MembersCount: 3,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(shareGroup))
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
		_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, shareGroupLabelFixture, "response should contain share group label")
		assertContains(t, textContent.Text, shareGroupUUIDFixture, "response should contain share group UUID")
	})

	t.Run("client failure returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, "/images/sharegroups/123", r.URL.Path, "request path should include share group ID")
			w.WriteHeader(http.StatusInternalServerError)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
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
		_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123})
		result, err := handler(t.Context(), req)

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "client failure should return a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to retrieve image share group")
		assertContains(t, textContent.Text, errTemporaryFailure)
	})

	t.Run("rejects invalid sharegroup_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			caseMissing:  nil,
			caseZero:     0,
			"negative":   -1,
			"fractional": 1.5,
			caseSlash:    "123/456",
			caseQuery:    "1?foo=bar",
			caseDotdot:   pathTraversalValue,
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
				_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

				args := map[string]any{}
				if name != caseMissing {
					args[keyShareGroupID] = value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				requireNoError(t, err)
				requireNotNil(t, result)
				assertTrue(t, result.IsError, "invalid sharegroup_id should return a tool error")
				assertFalse(t, called.Load(), "invalid sharegroup_id should be rejected before client call")
			})
		}
	})
}
