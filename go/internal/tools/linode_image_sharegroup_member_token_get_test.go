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

func TestLinodeImageShareGroupMemberTokenGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

		assert.Equal(t, "linode_image_sharegroup_member_token_get", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		assert.Contains(t, tool.InputSchema.Properties, keyTokenUUID, "schema should include token_uuid")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only get tool must not require confirm")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		updated := "2025-08-05T10:09:09"
		member := linode.ImageShareGroupMember{
			TokenUUID: shareGroupTokenGetUUID,
			Status:    statusActive,
			Label:     "Engineering - Backend",
			Created:   imageShareGroupTokenCreated,
			Updated:   &updated,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/images/sharegroups/123/members/"+shareGroupTokenGetUUID, r.URL.Path, "request path should include share group ID and token UUID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(member))
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
		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Engineering - Backend", "response should contain member label")
		assert.Contains(t, textContent.Text, shareGroupTokenGetUUID, "response should contain token UUID")
	})

	t.Run("rejects invalid path params before client call", func(t *testing.T) {
		t.Parallel()

		invalidArgs := map[string]map[string]any{
			"slash sharegroup_id":  {keyShareGroupID: paymentMethodIDSlash, keyTokenUUID: shareGroupTokenGetUUID},
			"query sharegroup_id":  {keyShareGroupID: pathQueryValue, keyTokenUUID: shareGroupTokenGetUUID},
			"dotdot sharegroup_id": {keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID},
			"zero sharegroup_id":   {keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID},
			"slash token_uuid":     {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithSlash},
			"query token_uuid":     {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithQuery},
			"dotdot token_uuid":    {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithDotdot},
			"numeric token_uuid":   {keyShareGroupID: 123, keyTokenUUID: 123},
		}

		for name, args := range invalidArgs {
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
				_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError, "invalid path params should be an error result")
				assert.False(t, called.Load(), "invalid path params must be rejected before the client call")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
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
		_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should be an error result")
	})
}
