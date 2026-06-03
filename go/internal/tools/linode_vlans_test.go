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
		assert.Equal(t, "linode_vlans_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only schema should not include confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/vlans", r.URL.Path, "request path should match")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, vlanLabelApp, "response should contain VLAN label")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain VLAN region")
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/vlans", r.URL.Path, "request path should match")
			http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANsListTool(cfg)

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		require.NoError(t, err, "handler should return tool errors without Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
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

				require.NoError(t, err, "handler should return tool errors without Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
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
		assert.Equal(t, "linode_vlan_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyRegionID, "schema should include region_id")
		assert.Contains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "mutating schema should include confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/networking/vlans/us-east/app-vlan", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, vlanLabelApp, "response should include VLAN label")
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

				require.NoError(t, err, "handler should return tool error without Go error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
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

				require.NoError(t, err, "handler should return tool error without Go error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid path params should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
			})
		}
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/networking/vlans/us-east/app-vlan", r.URL.Path, "request path should match")
			http.Error(w, `{\"errors\":[{\"reason\":\"forbidden\"}]}`, http.StatusForbidden)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should return tool errors without Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
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
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview lists and filters without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, "/networking/vlans", r.URL.Path, "dry_run must hit the VLAN list endpoint")

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_vlan_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		require.True(t, isWouldObject)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/networking/vlans/"+regionUSEast+"/"+vlanLabelApp, would["path"])

		// current_state must be the matched VLAN, not the whole list.
		state, stateIsObject := body["current_state"].(map[string]any)
		require.True(t, stateIsObject)
		assert.Equal(t, vlanLabelApp, state[keyLabel])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue GET (list), never DELETE")
	})

	t.Run("preview errors when VLAN not found", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError,
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

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, keyRegionID)
	})
}
