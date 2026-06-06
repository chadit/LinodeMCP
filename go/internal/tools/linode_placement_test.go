package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyPlacementGroupID         = "group_id"
	placementGroupIDError       = "group_id must be a positive integer"
	placementGroupIDRequired    = "group_id is required"
	placementGroupLabel         = "PG_Miami_failover"
	placementGroupRegion        = "us-mia"
	placementGroupTypeLocal     = "anti_affinity:local"
	placementGroupPolicy        = "strict"
	keyPlacementIsCompliant     = "is_compliant"
	keyPlacementGroupTypeJSON   = "placement_group_type"
	keyPlacementGroupPolicyJSON = "placement_group_policy"
	caseMissingGroupID          = "missing group id"
	caseSlashGroupID            = "slash group id"
	caseQueryGroupID            = "query group id"
	caseTraversalGroupID        = "traversal group id"
	placementGroupSlashValue    = "528/529"
	placementGroupQueryValue    = "528?x=1"
	keyPlacementMembers         = "members"
)

func TestLinodePlacementGroupGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodePlacementGroupGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_placement_group_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, "CapRead", capability.String(), "tool should be read-only")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingGroupID, args: map[string]any{}, wantContains: placementGroupIDRequired},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber}, wantContains: placementGroupIDError},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0"}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should match")
			checkEqual(t, "/placement/groups/528", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementIsCompliant: true, keyPlacementMembers: []map[string]any{{keyLinodeID: 123, keyPlacementIsCompliant: true}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, placementGroupLabel, "response should contain placement group label")
		expectContains(t, textContent.Text, placementGroupRegion, "response should contain placement group region")
	})
}

func TestLinodePlacementGroupDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodePlacementGroupDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_placement_group_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, "CapDestroy", capability.String(), "tool should be destroy capability")
		expectContains(t, tool.InputSchema.Properties, keyPlacementGroupID, "schema should include group_id")
		expectContains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		expectContains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		expectContains(t, tool.InputSchema.Required, keyPlacementGroupID, "group_id must be marked required")
		expectContains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		expectNotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyPlacementGroupID: "528"}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingGroupID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDRequired},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			srvCfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				},
			}
			_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

			result, err := srvHandler(t.Context(), createRequestWithArgs(t, tt.args))
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
			expectFalse(t, called.Load(), "validation should reject before client call")
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			checkEqual(t, http.MethodDelete, r.Method, "request method should match")
			checkEqual(t, "/placement/groups/528", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"), "authorization header should match")
			checkEqual(t, http.NoBody, r.Body, "delete request should not include a body")
			w.WriteHeader(http.StatusOK)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "deleted successfully", "response should confirm deletion")
		checkEqual(t, int32(1), requestCount.Load(), "delete should make one request")
	})

	t.Run("dry run previews without deleting", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32
		var sawDelete atomic.Bool

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			if r.Method == http.MethodDelete {
				sawDelete.Store(true)
			}
			checkEqual(t, http.MethodGet, r.Method, "dry_run should fetch state with GET")
			checkEqual(t, "/placement/groups/528", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementIsCompliant: true, keyPlacementMembers: []map[string]any{},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyDryRun: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "dry_run should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "dry_run", "response should be a dry-run preview")
		checkEqual(t, int32(1), requestCount.Load(), "dry_run should make one GET request")
		expectFalse(t, sawDelete.Load(), "dry_run must not issue DELETE")
	})

	t.Run("dry run surfaces member Linodes as detached dependencies", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementIsCompliant: true,
				keyPlacementMembers:     []map[string]any{{keyLinodeID: 111}, {keyLinodeID: 222}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyDryRun: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectFalse(t, result.IsError, "dry_run should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "detached", "each member Linode should be a detached dependency")
		expectContains(t, textContent.Text, "111", "member Linode IDs should be named")
		expectContains(t, textContent.Text, "222", "member Linode IDs should be named")
		expectContains(t, textContent.Text, "detaches 2 Linode", "preview should warn about detached members")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "client failure should be a tool error")
		assertErrorContains(t, result, "linode_placement_group_delete failed")
	})
}
