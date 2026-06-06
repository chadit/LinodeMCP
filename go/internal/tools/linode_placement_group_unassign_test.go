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

const placementGroupUnassignToolName = "linode_placement_group_unassign"

func TestLinodePlacementGroupUnassignTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

		checkEqual(t, placementGroupUnassignToolName, tool.Name, "tool name should match")
		checkEqual(t, profiles.CapWrite, capability, "placement group unassignment should be CapWrite")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContains(t, props, keyPlacementGroupID, "schema should include group_id")
		expectContains(t, props, "linodes", "schema should include linodes")
		expectContains(t, props, keyDryRun, "schema should include dry_run")
		expectContains(t, props, keyConfirm, "schema should include confirm")
		expectContains(t, tool.InputSchema.Required, keyPlacementGroupID, "group_id must be marked required")
		expectContains(t, tool.InputSchema.Required, "linodes", "linodes must be marked required")
		expectContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
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
				_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

				args := placementGroupUnassignArgs()
				delete(args, keyConfirm)

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should not return transport error")
				expectNotNil(t, result, "result should not be nil")
				expectTrue(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				checkEqual(t, int32(0), calls, "confirm failure must happen before client call")
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
			{name: "missing group_id", update: func(args map[string]any) { delete(args, keyPlacementGroupID) }, wantMessage: "group_id is required"},
			{name: "slash group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = pathSeparatorValue }, wantMessage: placementGroupIDError},
			{name: "query group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = "12?x=1" }, wantMessage: placementGroupIDError},
			{name: "traversal group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = pathTraversalValue }, wantMessage: placementGroupIDError},
			{name: "missing linodes", update: func(args map[string]any) { delete(args, "linodes") }, wantMessage: tools.ErrPlacementGroupLinodesRequired.Error()},
			{name: "empty linodes", update: func(args map[string]any) { args["linodes"] = []any{} }, wantMessage: tools.ErrPlacementGroupLinodesEmpty.Error()},
			{name: "string linodes", update: func(args map[string]any) { args["linodes"] = []any{"123"} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
			{name: "non-array linodes", update: func(args map[string]any) { args["linodes"] = "123" }, wantMessage: tools.ErrPlacementGroupLinodesJSON.Error()},
			{name: "fractional linode", update: func(args map[string]any) { args["linodes"] = []any{123.5} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
			{name: "duplicate linode", update: func(args map[string]any) { args["linodes"] = []any{float64(123), float64(123)} }, wantMessage: tools.ErrPlacementGroupLinodesDuplicate.Error()},
			{name: "zero linode", update: func(args map[string]any) { args["linodes"] = []any{float64(0)} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
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
				_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

				args := placementGroupUnassignArgs()
				testCase.update(args)

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should not return transport error")
				expectNotNil(t, result, "result should not be nil")
				expectTrue(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				checkEqual(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/placement/groups/789/unassign", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupUnassignArgs()))

		expectNoError(t, err, "handler should return API failures as tool errors")
		expectNotNil(t, result, "result should not be nil")
		expectTrue(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to unassign placement group")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/placement/groups/789/unassign", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.PlacementGroupUnassignRequest
			checkNoError(t, json.NewDecoder(r.Body).Decode(&got))
			checkEqual(t, []int{123, 456}, got.Linodes)

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 789, Label: "pg-test", Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupUnassignArgs()))

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		expectFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "pg-test", "response should include label")
		expectContains(t, textContent.Text, "unassigned", "response should describe unassignment")
	})
}

func placementGroupUnassignArgs() map[string]any {
	return map[string]any{
		keyPlacementGroupID: "789",
		"linodes":           []any{float64(123), float64(456)},
		keyConfirm:          true,
	}
}

func TestLinodePlacementGroupUnassignToolDryRun(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		checkEqual(t, http.MethodGet, r.Method, "dry_run should fetch current placement group state")
		checkEqual(t, "/placement/groups/789", r.URL.Path, "dry_run state path should match")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 789, Label: "pg-test", Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

	args := placementGroupUnassignArgs()
	delete(args, keyConfirm)
	args[keyDryRun] = true

	result, err := handler(t.Context(), createRequestWithArgs(t, args))

	expectNoError(t, err, "handler should not return transport error")
	expectNotNil(t, result, "result should not be nil")
	expectFalse(t, result.IsError, "dry_run should return a preview")
	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok, "content should be TextContent")
	expectContains(t, textContent.Text, `"dry_run": true`)
	expectContains(t, textContent.Text, `"method": "POST"`)
	expectContains(t, textContent.Text, `"path": "/placement/groups/789/unassign"`)
	expectContains(t, textContent.Text, `"body": {`)
	expectContains(t, textContent.Text, `"linodes": [`)
	expectContains(t, textContent.Text, `123`)
	expectContains(t, textContent.Text, `456`)
	expectContains(t, textContent.Text, "side_effects", "dry_run should surface side effects")
	expectContains(t, textContent.Text, "removed from placement group 789", "side effect should describe the unassignment")
	checkEqual(t, int32(1), calls, "unassign dry_run should only fetch current placement group state")
}
