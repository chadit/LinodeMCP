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

const (
	placementGroupCreateToolName = "linode_placement_group_create"
	placementGroupCreateLabel    = "pg-test"
	placementGroupCreateRegion   = "us-east"
	placementGroupType           = "anti_affinity:local"
	errPlacementGroupRegionBlank = "region must be a non-empty string"
	caseNumericLabel             = "numeric label"
	placementGroupCreatePolicy   = "strict"
)

func TestLinodePlacementGroupCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

		assert.Equal(t, placementGroupCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "placement group creation should be CapWrite")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "region", "schema should include region")
		assert.Contains(t, props, keyPlacementGroupTypeJSON, "schema should include placement_group_type")
		assert.Contains(t, props, keyPlacementGroupPolicyJSON, "schema should include placement_group_policy")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, "label", "label must be marked required")
		assert.Contains(t, tool.InputSchema.Required, "region", "region must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyPlacementGroupTypeJSON, "placement_group_type must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyPlacementGroupPolicyJSON, "placement_group_policy must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissingConfirm, set: false},
			{name: caseRequiresConfirm, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

				args := placementGroupCreateArgs()
				delete(args, keyConfirm)

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
			})
		}
	})

	t.Run("invalid request rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			update      func(map[string]any)
			wantMessage string
		}{
			{name: caseMissingLabel, update: func(args map[string]any) { delete(args, "label") }, wantMessage: errLabelRequired},
			{name: caseBlankLabelImageShareGroupToken, update: func(args map[string]any) { args["label"] = blankString }, wantMessage: errLabelNonEmpty},
			{name: caseNumericLabel, update: func(args map[string]any) { args["label"] = 123 }, wantMessage: errLabelNonEmpty},
			{name: caseMissingRegion, update: func(args map[string]any) { delete(args, "region") }, wantMessage: "region is required"},
			{name: "blank region", update: func(args map[string]any) { args["region"] = blankString }, wantMessage: errPlacementGroupRegionBlank},
			{name: caseMissingType, update: func(args map[string]any) { delete(args, keyPlacementGroupTypeJSON) }, wantMessage: "placement_group_type is required"},
			{name: "numeric type", update: func(args map[string]any) { args[keyPlacementGroupTypeJSON] = 123 }, wantMessage: "placement_group_type must be a non-empty string"},
			{name: caseInvalidType, update: func(args map[string]any) { args[keyPlacementGroupTypeJSON] = "affinity:local" }, wantMessage: "placement_group_type must be anti_affinity:local"},
			{name: "missing policy", update: func(args map[string]any) { delete(args, keyPlacementGroupPolicyJSON) }, wantMessage: "placement_group_policy is required"},
			{name: "blank policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = blankString }, wantMessage: "placement_group_policy must be a non-empty string"},
			{name: "numeric policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = 123 }, wantMessage: "placement_group_policy must be a non-empty string"},
			{name: "invalid policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = "eventual" }, wantMessage: "placement_group_policy must be strict or flexible"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

				args := placementGroupCreateArgs()
				testCase.update(args)

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("dry_run returns preview without client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

		args := placementGroupCreateArgs()
		delete(args, keyConfirm)
		args[keyDryRun] = true

		result, err := handler(t.Context(), createRequestWithArgs(t, args))

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry_run should return a preview")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"dry_run": true`)
		assert.Contains(t, textContent.Text, `"method": "POST"`)
		assert.Contains(t, textContent.Text, `"path": "/placement/groups"`)
		assert.Contains(t, textContent.Text, "side_effects")
		assert.Contains(t, textContent.Text, "will be created in region")
		assert.Equal(t, int32(0), calls, "create dry_run must not call the Linode API")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupCreateArgs()))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create placement group")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreatePlacementGroupRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, placementGroupCreateLabel, got.Label)
			assert.Equal(t, placementGroupCreateRegion, got.Region)
			assert.Equal(t, placementGroupType, got.PlacementGroupType)
			assert.Equal(t, placementGroupCreatePolicy, got.PlacementGroupPolicy)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 123, Label: placementGroupCreateLabel, Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupCreateArgs()))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, placementGroupCreateLabel, "response should include label")
		assert.Contains(t, textContent.Text, placementGroupCreateRegion, "response should include region")
	})
}

func placementGroupCreateArgs() map[string]any {
	return map[string]any{
		"label":                     placementGroupCreateLabel,
		"region":                    placementGroupCreateRegion,
		keyPlacementGroupTypeJSON:   placementGroupType,
		keyPlacementGroupPolicyJSON: placementGroupCreatePolicy,
		keyConfirm:                  true,
	}
}
