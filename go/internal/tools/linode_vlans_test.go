package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeVLANsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeVLANsListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vlans_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vlans_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyPage, keyPageSize} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}
}

func TestLinodeVLANsListToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingVlans {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingVlans)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyLabel: vlanLabelApp, keySupportTicketRegion: regionUSEast, "linodes": []int{123}}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVLANsListTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, vlanLabelApp) {
		t.Errorf("textContent.Text does not contain %v", vlanLabelApp)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeVLANsListToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingVlans {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingVlans)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVLANsListTool(cfg)

	result, err := handler(t.Context(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_vlans_list") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_vlans_list")
	}
}

func TestLinodeVLANsListToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}
		})
	}
}

func TestLinodeVLANDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeVLANDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vlan_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vlan_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyRegionID, keyLabel, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeVLANDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/networking/vlans/us-east/app-vlan" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/vlans/us-east/app-vlan")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, vlanLabelApp) {
		t.Errorf("textContent.Text does not contain %v", vlanLabelApp)
	}
}

func TestLinodeVLANDeleteToolConfirmRejectsBeforeClient(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeVLANDeleteToolInvalidPathParamsRejectBeforeClient(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}
		})
	}
}

func TestLinodeVLANDeleteToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/networking/vlans/us-east/app-vlan" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/vlans/us-east/app-vlan")
		}

		http.Error(w, `{\"errors\":[{\"reason\":\"forbidden\"}]}`, http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVLANDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast, keyLabel: vlanLabelApp, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_vlan_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_vlan_delete failed")
	}
}

// Dry-run coverage for VLAN delete. VLANs have no single-GET endpoint,
// so dry-run lists and filters to the matching region+label. Sibling
// function keeps the main test's subtest count below maintidx.
func TestLinodeVLANDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVLANDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeVLANDeleteToolDryRunPreviewListsAndFiltersWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcNetworkingVlans {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingVlans)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{
					{keyLabel: "other-vlan", keyRegion: regionUSEast},
					{keyLabel: vlanLabelApp, keyRegion: regionUSEast},
				},
				keyPage: 1, keyPages: 1, keyResults: 2,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vlan_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vlan_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/networking/vlans/"+regionUSEast+"/"+vlanLabelApp) {
		t.Errorf("got %v, want %v", would["path"], "/networking/vlans/"+regionUSEast+"/"+vlanLabelApp)
	}

	// current_state must be the matched VLAN, not the whole list.
	state, stateIsObject := body["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	if !reflect.DeepEqual(state[keyLabel], vlanLabelApp) {
		t.Errorf("state[keyLabel] = %v, want %v", state[keyLabel], vlanLabelApp)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeVLANDeleteToolDryRunPreviewErrorsWhenVLANNotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{}, keyPage: 1, keyPages: 1, keyResults: 0,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "VLAN not found") {
		t.Errorf("error text %q does not contain %q", text.Text, "VLAN not found")
	}
}

func TestLinodeVLANDeleteToolDryRunDryRunStillValidatesRegionAndLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVLANDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyLabel:  vlanLabelApp,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, keyRegionID) {
		t.Errorf("error text %q does not contain %q", text.Text, keyRegionID)
	}
}
