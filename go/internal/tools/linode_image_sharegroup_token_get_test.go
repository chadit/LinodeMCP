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

func TestLinodeImageShareGroupTokenGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupTokenGetTool(cfg)

		assertEqual(t, "linode_image_sharegroup_token_get", tool.Name, "tool name should match")
		assertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Properties, keyTokenUUID, "schema should include token_uuid")
		assertNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		updated := "2025-08-04T11:09:09"
		expiry := "2025-09-04T10:09:09"
		token := linode.ImageShareGroupToken{
			TokenUUID:              shareGroupTokenGetUUID,
			Status:                 statusActive,
			Label:                  "Backend Services - Engineering",
			Created:                imageShareGroupTokenCreated,
			Updated:                &updated,
			Expiry:                 &expiry,
			ValidForShareGroupUUID: shareGroupUUIDFixture,
			ShareGroupUUID:         shareGroupUUIDFixture,
			ShareGroupLabel:        shareGroupLabelFixture,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			assertEqual(t, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include token UUID")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(token))
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
		_, _, handler := tools.NewLinodeImageShareGroupTokenGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Backend Services - Engineering", "response should contain token label")
		assertContains(t, textContent.Text, shareGroupTokenGetUUID, "response should contain token UUID")
	})

	t.Run("rejects invalid token_uuid before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			caseSlash:    tokenUUIDWithSlash,
			caseQuery:    tokenUUIDWithQuery,
			caseFragment: tokenUUIDWithFragment,
			caseDotdot:   tokenUUIDWithDotdot,
			caseNotUUID:  invalidTokenUUID,
			caseEmpty:    blankString,
			caseNumeric:  123,
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
				_, _, handler := tools.NewLinodeImageShareGroupTokenGetTool(cfg)

				req := createRequestWithArgs(t, map[string]any{keyTokenUUID: value})
				result, err := handler(t.Context(), req)

				requireNoError(t, err)
				requireNotNil(t, result)
				assertTrue(t, result.IsError, "invalid token_uuid should be an error result")
				assertFalse(t, called.Load(), "invalid token_uuid must be rejected before the client call")
			})
		}
	})

	t.Run("missing token_uuid", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupTokenGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "missing token_uuid should be an error result")
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
		_, _, handler := tools.NewLinodeImageShareGroupTokenGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID})
		result, err := handler(t.Context(), req)

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "Failed to retrieve image share group token")
	})
}
