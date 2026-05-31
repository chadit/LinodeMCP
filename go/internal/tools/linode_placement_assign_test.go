package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyPlacementGroupLinodes   = "linodes"
	placementGroupIDFixture    = "528"
	placementGroupLinodeSingle = float64(123)
	errConfirmTrue             = "confirm=true"
	errLinodesRequired         = "linodes is required"
	keyPlacementGroupCompliant = "is_compliant"
	keyPlacementGroupMembers   = "members"
)

func TestLinodePlacementGroupAssignTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

		assert.Equal(t, "linode_placement_group_assign", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write-capable")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyPlacementGroupID, "schema should include group_id")
		assert.Contains(t, tool.InputSchema.Properties, keyPlacementGroupLinodes, "schema should include linodes")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should require confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}}, wantContains: errConfirmTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: false}, wantContains: errConfirmTrue},
		{name: "string confirm", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: boolStringTrue}, wantContains: errConfirmTrue},
		{name: "numeric confirm", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: 1}, wantContains: errConfirmTrue},
		{name: caseMissingGroupID, args: map[string]any{keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDRequired},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "missing linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyConfirm: true}, wantContains: errLinodesRequired},
		{name: "dry run still validates linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyDryRun: true}, wantContains: errLinodesRequired},
		{name: "string linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: "[123]", keyConfirm: true}, wantContains: "linodes must be a JSON array"},
		{name: "invalid linode element", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{"123"}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "empty linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{}, keyConfirm: true}, wantContains: "at least one"},
		{name: "zero linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(0)}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "fractional linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123.5)}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "duplicate linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123), float64(123)}, keyConfirm: true}, wantContains: "unique"},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should match")
			assert.Equal(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"), "authorization header should match")

			var body map[string][]int

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr, "request body should decode")
			assert.Equal(t, []int{123, 456}, body[keyPlacementGroupLinodes], "linodes body should match")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementGroupCompliant: true,
				keyPlacementGroupMembers:   []map[string]any{{keyLinodeID: 123, keyPlacementGroupCompliant: true}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123), float64(456)}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Assigned 2 Linode", "response should include success message")
		assert.Contains(t, textContent.Text, placementGroupLabel, "response should include placement group")
	})

	t.Run("api error includes group id and reason", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should match")
			assert.Equal(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "placement group 528")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("dry run state fetch error is reported", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "dry_run should fetch current state")
			assert.Equal(t, "/placement/groups/528", r.URL.Path, "dry_run should fetch target group")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "dry run state fetch failure should be a tool error")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("dry run skips confirm and does not post", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "dry_run should fetch current state")
			assert.Equal(t, "/placement/groups/528", r.URL.Path, "dry_run should fetch target group")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: 528, keyLabel: placementGroupLabel}), "encoding state should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "dry run should not require confirm")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "/placement/groups/528/assign", "dry run should show target route")
		assert.Contains(t, textContent.Text, "123", "dry run should show Linode IDs")
		assert.Contains(t, textContent.Text, placementGroupLabel, "dry run should show current state")
	})
}
