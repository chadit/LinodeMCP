package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeVLANsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeVLANsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vlans_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, profiles.CapRead, capability, "tool should be read capability")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		expectContains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		expectNotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only schema should not include confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/vlans", r.URL.Path, "request path should match")
			checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{keyLabel: vlanLabelApp, "region": regionUSEast, "linodes": []int{123}}},
				keyPage: 1, keyPages: 1, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeVLANsListTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, vlanLabelApp, "response should contain VLAN label")
		expectContains(t, textContent.Text, regionUSEast, "response should contain VLAN region")
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/networking/vlans", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANsListTool(cfg)

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		expectNoError(t, err, "handler should return tool errors without Go errors")
		expectNotNil(t, result, "result should not be nil")
		expectTrue(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve linode_vlans_list")
	})

	t.Run("invalid pagination rejects before client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANsListTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				expectNoError(t, err, "handler should return tool errors without Go errors")
				expectNotNil(t, result, "result should not be nil")
				expectTrue(t, result.IsError, "invalid pagination should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
			})
		}
	})
}

func TestLinodeVLANDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeVLANDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vlan_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.InputSchema.Properties, keyRegionID, "schema should include region_id")
		expectContains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		expectContains(t, tool.InputSchema.Properties, keyConfirm, "mutating schema should include confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			checkEqual(t, "/networking/vlans/us-east/app-vlan", r.URL.Path, "request path should match")
			checkEmpty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, vlanLabelApp, "response should include VLAN label")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		cases := []struct {
			name  string
			value any
		}{
			{name: caseMissing},
			{name: caseFalseConfirmRejected, value: false},
			{name: caseStringConfirmRejected, value: boolStringTrue},
			{name: caseNumericConfirmRejected, value: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp}
				if testCase.value != nil {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should return tool error without Go error")
				expectNotNil(t, result, "result should not be nil")
				expectTrue(t, result.IsError, "missing or invalid confirm should be an error result")
				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("invalid path params reject before client", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingRegion, args: map[string]any{keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDRequired},
			{name: caseMissingLabel, args: map[string]any{keyRegionID: regionUSEast, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errLabelRequired},
			{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDSlug},
			{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDSlug},
			{name: caseFragment, args: map[string]any{keyRegionID: "us-east#frag", keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDSlug},
			{name: caseDotTraversal, args: map[string]any{keyRegionID: pathTraversalValue, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDSlug},
			{name: "region uppercase", args: map[string]any{keyRegionID: "US EAST", keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errRegionIDSlug},
			{name: "label slash", args: map[string]any{keyRegionID: regionUSEast, keyLabel: "app/vlan", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errLabelNoSeparators},
			{name: "label query", args: map[string]any{keyRegionID: regionUSEast, keyLabel: "app?vlan", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errLabelNoSeparators},
			{name: "label fragment", args: map[string]any{keyRegionID: regionUSEast, keyLabel: "app#vlan", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errLabelNoSeparators},
			{name: "label traversal", args: map[string]any{keyRegionID: regionUSEast, keyLabel: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errLabelNoSeparators},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				expectNoError(t, err, "handler should return tool error without Go error")
				expectNotNil(t, result, "result should not be nil")
				expectTrue(t, result.IsError, "invalid path params should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
			})
		}
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
			checkEqual(t, "/networking/vlans/us-east/app-vlan", r.URL.Path, "request path should match")
			http.Error(w, `{\"errors\":[{\"reason\":\"forbidden\"}]}`, http.StatusForbidden)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err, "handler should return tool errors without Go errors")
		expectNotNil(t, result, "result should not be nil")
		expectTrue(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "linode_vlan_delete failed")
	})
}

// Dry-run coverage for VLAN delete. VLANs have no single-GET endpoint,
// so dry-run lists and filters to the matching region+label. Sibling
// function keeps the main test's subtest count below maintidx.
func TestLinodeVLANDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVLANDeleteTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview lists and filters without mutating", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		var sawDelete atomic.Bool

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)

			if r.Method == http.MethodDelete {
				sawDelete.Store(true)
			}

			checkEqual(t, "/networking/vlans", r.URL.Path, "dry_run must hit the VLAN list endpoint")

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
					keyData: []map[string]any{
						{keyLabel: "other-vlan", keyRegion: regionUSEast},
						{keyLabel: vlanLabelApp, keyRegion: regionUSEast},
					},
					keyPage: 1, keyPages: 1, keyResults: 2,
				}))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegionID: regionUSEast,
			keyLabel:    vlanLabelApp,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, true, body[keyDryRun])
		checkEqual(t, "linode_vlan_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		expectTrue(t, isWouldObject)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/networking/vlans/"+regionUSEast+"/"+vlanLabelApp, would["path"])

		// current_state must be the matched VLAN, not the whole list.
		state, stateIsObject := body["current_state"].(map[string]any)
		expectTrue(t, stateIsObject)
		checkEqual(t, vlanLabelApp, state[keyLabel])

		checkEqual(t, int32(1), requestCount.Load(),
			"dry_run must only issue one GET list request")
		expectFalse(t, sawDelete.Load(), "dry_run must never issue DELETE")
	})

	t.Run("preview errors when VLAN not found", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{}, keyPage: 1, keyPages: 1, keyResults: 0,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegionID: regionUSEast,
			keyLabel:    vlanLabelApp,
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError,
			"dry_run on a non-existent VLAN must surface a not-found error, like the real delete would")
		assertErrorContains(t, result, "VLAN not found")
	})

	t.Run("dry_run still validates region and label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVLANDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyLabel:  vlanLabelApp,
			keyDryRun: true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, keyRegionID)
	})
}
