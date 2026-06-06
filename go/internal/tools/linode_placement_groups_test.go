package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	placementGroupIDKey             = "group_id"
	placementGroupUpdatedLabel      = "pg-renamed"
	placementGroupIDIntegerMessage  = "group_id must be an integer"
	placementGroupLabelBlankMessage = "label must be a non-empty string"
	placementGroupSlashID           = "12/34"
	caseNumericPlacementGroupLabel  = "numeric label"
)

func TestLinodePlacementGroupListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodePlacementGroupListTool(cfg)

		checkEqual(t, "linode_placement_groups_list", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "tool should be read-only")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		expectContains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		expectNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only list tool must not require confirm")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		groups := []linode.PlacementGroup{{ID: 123, Label: "pg-east", Region: regionUSEast, PlacementGroupType: "anti_affinity:local", PlacementGroupPolicy: "strict", IsCompliant: true}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
			checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    groups,
				keyPage:    2,
				keyPages:   3,
				keyResults: 51,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		expectFalse(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "pg-east", "response should contain placement group label")
		expectContains(t, textContent.Text, "us-east", "response should contain region")
	})

	t.Run("invalid page_size", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError, "invalid page_size should be an error result")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "Failed to retrieve placement groups")
	})
}

func TestLinodePlacementGroupUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

		checkEqual(t, "linode_placement_group_update", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapWrite, capability, "placement group update should be CapWrite")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectContains(t, tool.InputSchema.Properties, placementGroupIDKey, "schema should include group_id")
		expectContains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		expectContains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		expectContains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		expectContains(t, tool.InputSchema.Required, placementGroupIDKey, "group_id must be required")
		expectContains(t, tool.InputSchema.Required, keyLabel, "label must be required")
		expectContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("requires confirm", func(t *testing.T) {
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

				srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					t.Fatalf("confirm failure must happen before client call")
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

				args := map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err)
				expectNotNil(t, result)
				expectTrue(t, result.IsError, "confirm failure should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing group_id", args: map[string]any{keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDRequired},
			{name: "zero group_id", args: map[string]any{placementGroupIDKey: 0, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: "group_id must be an integer greater than or equal to 1"},
			{name: "string group_id", args: map[string]any{placementGroupIDKey: "123", keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
			{name: "fractional group_id", args: map[string]any{placementGroupIDKey: 123.5, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
			{name: "slash group_id", args: map[string]any{placementGroupIDKey: placementGroupSlashID, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
			{name: "query group_id", args: map[string]any{placementGroupIDKey: "12?x=1", keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
			{name: "dotdot group_id", args: map[string]any{placementGroupIDKey: pathTraversalValue, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
			{name: caseEmptyLabel, args: map[string]any{placementGroupIDKey: 123, keyLabel: "", keyConfirm: true}, wantMessage: placementGroupLabelBlankMessage},
			{name: caseNumericPlacementGroupLabel, args: map[string]any{placementGroupIDKey: 123, keyLabel: 123, keyConfirm: true}, wantMessage: placementGroupLabelBlankMessage},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					t.Fatalf("request validation must happen before client call")
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				expectNoError(t, err)
				expectNotNil(t, result)
				expectTrue(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
			})
		}
	})

	t.Run("dry run", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyDryRun: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError, "dry run should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "linode_placement_group_update")
		expectContains(t, textContent.Text, "PUT")
		expectContains(t, textContent.Text, "/placement/groups/123")
		expectContains(t, textContent.Text, "side_effects")
		expectContains(t, textContent.Text, "label is set to")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			checkEqual(t, "/placement/groups/123", r.URL.Path, "request path should include group ID")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
			checkEqual(t, placementGroupUpdatedLabel, body[keyLabel])

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 123, Label: placementGroupUpdatedLabel, Region: regionUSEast}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, placementGroupUpdatedLabel, "response should include updated label")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			checkEqual(t, "/placement/groups/123", r.URL.Path, "request path should include group ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to update linode_placement_group_update")
		assertErrorContains(t, result, errForbidden)
	})
}
