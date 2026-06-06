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

func TestLinodeImageShareGroupMembersListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupMembersListTool(cfg)

		shareGroupAssertEqual(t, "linode_image_sharegroup_members_list", tool.Name, "tool name should match")
		shareGroupAssertEqual(t, profiles.CapRead, capability, "tool should be read-only")
		shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		shareGroupAssertNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		shareGroupRequireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		updated := "2025-08-05T10:09:09"
		members := []linode.ImageShareGroupMember{
			{TokenUUID: shareGroupTokenGetUUID, Status: statusActive, Label: "Engineering - Backend", Created: "2025-08-04T10:07:59", Updated: &updated},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			shareGroupAssertEqual(t, http.MethodGet, r.Method, "request method should be GET")
			shareGroupAssertEqual(t, "/images/sharegroups/123/members", r.URL.Path, "request path should include share group ID and members suffix")
			shareGroupAssertEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    members,
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
		_, _, handler := tools.NewLinodeImageShareGroupMembersListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123, keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		shareGroupRequireNoError(t, err, "handler should not return an error")
		shareGroupRequireNotNil(t, result, "result should not be nil")
		shareGroupAssertFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, ok, "content should be TextContent")
		shareGroupAssertContains(t, textContent.Text, "Engineering - Backend", "response should contain member label")
		shareGroupAssertContains(t, textContent.Text, shareGroupTokenGetUUID, "response should contain token UUID")
	})

	t.Run("rejects invalid sharegroup_id before client call", func(t *testing.T) {
		t.Parallel()

		invalidValues := map[string]any{
			caseSlash: paymentMethodIDSlash,
			caseQuery: shareGroupIDQueryValue,

			caseDotdot:  pathTraversalValue,
			caseEmpty:   blankString,
			caseNumeric: 0,
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
				_, _, handler := tools.NewLinodeImageShareGroupMembersListTool(cfg)

				req := createRequestWithArgs(t, map[string]any{keyShareGroupID: value})
				result, err := handler(t.Context(), req)

				shareGroupRequireNoError(t, err)
				shareGroupRequireNotNil(t, result)
				shareGroupAssertTrue(t, result.IsError, "invalid sharegroup_id should be an error result")
				shareGroupAssertFalse(t, called.Load(), "invalid sharegroup_id must be rejected before the client call")
			})
		}
	})

	t.Run("missing sharegroup_id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupMembersListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "missing sharegroup_id should be an error result")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{
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
		_, _, handler := tools.NewLinodeImageShareGroupMembersListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123})
		result, err := handler(t.Context(), req)

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, ok, "content should be TextContent")
		shareGroupAssertContains(t, textContent.Text, "Failed to retrieve image share group members")
	})
}
