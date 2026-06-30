package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeIPv6RangesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeIPv6RangesListTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyPage, keyPageSize} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeIPv6RangesListToolSuccessWithPagination(t *testing.T) {
	t.Parallel()

	ranges := linode.PaginatedResponse[linode.IPv6Range]{
		Data: []linode.IPv6Range{{
			Range:  ipv6RangeFixture,
			Region: regionUSEast,
			Prefix: 124,
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingIpv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingIpv6Ranges)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(ranges); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25}))
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

	if !strings.Contains(textContent.Text, ipv6RangeFixture) {
		t.Errorf("textContent.Text does not contain %v", ipv6RangeFixture)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeIPv6RangesListToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingIpv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingIpv6Ranges)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}
}

func TestLinodeIPv6RangesListToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangesListTool(cfg)

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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeIPv6RangeCreateToolCreateDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyPrefixLength, keyLinodeID, keyRouteTarget, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeIPv6RangeCreateToolCreateSuccess(t *testing.T) {
	t.Parallel()

	createdRange := linode.IPv6Range{Range: ipv6RangeFixture, Region: regionUSEast, Prefix: 124, RouteTarget: ipv6RouteTarget}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNetworkingIpv6Ranges {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingIpv6Ranges)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.CreateIPv6RangeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID == nil {
			t.Fatal("linode_id should be sent")
		}

		if *body.LinodeID != 12345 {
			t.Errorf("*body.LinodeID = %v, want %v", *body.LinodeID, 12345)
		}

		if body.PrefixLength != 124 {
			t.Errorf("body.PrefixLength = %v, want %v", body.PrefixLength, 124)
		}

		if body.RouteTarget != ipv6RouteTarget {
			t.Errorf("body.RouteTarget = %v, want %v", body.RouteTarget, ipv6RouteTarget)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(createdRange); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyPrefixLength: 124,
		keyLinodeID:     12345,
		keyRouteTarget:  ipv6RouteTarget,
		keyConfirm:      true,
	}))
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

	if !strings.Contains(textContent.Text, "IPv6 range created") {
		t.Errorf("textContent.Text does not contain %v", "IPv6 range created")
	}

	if !strings.Contains(textContent.Text, ipv6RangeFixture) {
		t.Errorf("textContent.Text does not contain %v", ipv6RangeFixture)
	}
}

func TestLinodeIPv6RangeCreateToolCreateConfirmRequired(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})
	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyPrefixLength: 124}},
		{name: caseConfirmFalse, args: map[string]any{keyPrefixLength: 124, keyConfirm: false}},
		{name: caseString, args: map[string]any{keyPrefixLength: 124, keyConfirm: boolStringTrue}},
		{name: caseNumeric, args: map[string]any{keyPrefixLength: 124, keyConfirm: 1}},
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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeIPv6RangeCreateToolCreateValidationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeCreateTool(cfg)
	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing prefix length", args: map[string]any{keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
		{name: "string prefix length", args: map[string]any{keyPrefixLength: "124", keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
		{name: "large prefix length", args: map[string]any{keyPrefixLength: 129, keyConfirm: true}, wantMessage: errIPv6RangePrefixRange},
		{name: caseZeroLinodeID, args: map[string]any{keyPrefixLength: 124, keyLinodeID: 0, keyConfirm: true}, wantMessage: errLinodeIDPositive},
		{name: "string route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: 123, keyConfirm: true}, wantMessage: "route_target must be a non-empty string"},
		{name: "malformed route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "not-an-ip", keyConfirm: true}, wantMessage: "route_target must be a valid IPv6 address"},
		{name: "ipv4 route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "192.0.2.1", keyConfirm: true}, wantMessage: "route_target must be a valid IPv6 address"},
		{name: "empty route target", args: map[string]any{keyPrefixLength: 124, keyRouteTarget: "", keyConfirm: true}, wantMessage: "route_target must be a non-empty string"},
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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeIPv6RangeGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeIPv6RangeGetTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyIPv6Range]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyIPv6Range)
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeIPv6RangeGetToolSuccess(t *testing.T) {
	t.Parallel()

	rangeResult := linode.IPv6Range{
		Range:       ipv6RangeCIDR,
		Region:      regionUSEast,
		Prefix:      64,
		RouteTarget: ipv6RangeRouteTarget,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != tcNetworkingIpv6Ranges20010db82F64 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcNetworkingIpv6Ranges20010db82F64)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(rangeResult); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyIPv6Range: ipv6RangeCIDR}))
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

	if !strings.Contains(textContent.Text, ipv6RangeCIDR) {
		t.Errorf("textContent.Text does not contain %v", ipv6RangeCIDR)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeIPv6RangeGetToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != tcNetworkingIpv6Ranges20010db82F64 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcNetworkingIpv6Ranges20010db82F64)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyIPv6Range: ipv6RangeCIDR}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve IPv6 range") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve IPv6 range")
	}
}

func TestLinodeIPv6RangeGetToolInvalidRangeRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "missing range", args: map[string]any{}},
		{name: "non-string range", args: map[string]any{keyIPv6Range: 123}},
		{name: "slash only", args: map[string]any{keyIPv6Range: "/"}},
		{name: "query separator", args: map[string]any{keyIPv6Range: "2001:0db8::/64?x=1"}},
		{name: "traversal", args: map[string]any{keyIPv6Range: pathTraversalValue}},
		{name: "ipv4 prefix", args: map[string]any{keyIPv6Range: "192.0.2.0/24"}},
		{name: "host bits set", args: map[string]any{keyIPv6Range: "2001:0db8::1/64"}},
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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, keyIPv6Range) {
				t.Errorf("textContent.Text does not contain %v", keyIPv6Range)
			}
		})
	}
}

func TestLinodeIPv6RangeDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeIPv6RangeDeleteTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyIPv6Range, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeIPv6RangeDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != tcNetworkingIpv6Ranges20010db82F64 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcNetworkingIpv6Ranges20010db82F64)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyIPv6Range: ipv6RangeCIDR, keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "IPv6 range deleted") {
		t.Errorf("textContent.Text does not contain %v", "IPv6 range deleted")
	}

	if !strings.Contains(textContent.Text, ipv6RangeCIDR) {
		t.Errorf("textContent.Text does not contain %v", ipv6RangeCIDR)
	}
}

func TestLinodeIPv6RangeDeleteToolConfirmRequired(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(&config.Config{})
	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{keyIPv6Range: ipv6RangeCIDR}},
		{name: caseConfirmFalse, args: map[string]any{keyIPv6Range: ipv6RangeCIDR, keyConfirm: false}},
		{name: caseString, args: map[string]any{keyIPv6Range: ipv6RangeCIDR, keyConfirm: boolStringTrue}},
		{name: caseNumeric, args: map[string]any{keyIPv6Range: ipv6RangeCIDR, keyConfirm: 1}},
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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "confirm=true") {
				t.Errorf("textContent.Text does not contain %v", "confirm=true")
			}
		})
	}
}

func TestLinodeIPv6RangeDeleteToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != tcNetworkingIpv6Ranges20010db82F64 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcNetworkingIpv6Ranges20010db82F64)
		}

		http.Error(w, `{"errors":[{"reason":"forbidden"}]}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyIPv6Range: ipv6RangeCIDR, keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode_ipv6_range_delete failed") {
		t.Errorf("textContent.Text does not contain %v", "linode_ipv6_range_delete failed")
	}
}

func TestLinodeIPv6RangeDeleteToolInvalidRangeRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(cfg)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "missing range", args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "non-string range", args: map[string]any{keyIPv6Range: 123, keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "slash only", args: map[string]any{keyIPv6Range: "/", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "query separator", args: map[string]any{keyIPv6Range: "2001:0db8::/64?x=1", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "traversal", args: map[string]any{keyIPv6Range: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "ipv4 prefix", args: map[string]any{keyIPv6Range: "192.0.2.0/24", keyConfirm: true, keyConfirmedDryRun: true}},
		{name: "host bits set", args: map[string]any{keyIPv6Range: "2001:0db8::1/64", keyConfirm: true, keyConfirmedDryRun: true}},
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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, keyIPv6Range) {
				t.Errorf("textContent.Text does not contain %v", keyIPv6Range)
			}
		})
	}
}

// Dry-run coverage for IPv6 range delete. Sibling function keeps the
// main test's subtest count below maintidx's threshold.
func TestLinodeIPv6RangeDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeIPv6RangeDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeIPv6RangeDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	rangeBody := `{"range":"2001:0db8::","region":"us-east","prefix":64}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.EscapedPath() != tcNetworkingIpv6Ranges20010db82F64 {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcNetworkingIpv6Ranges20010db82F64)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(rangeBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyIPv6Range: ipv6RangeCIDR,
		keyDryRun:    true,
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

	if !reflect.DeepEqual(body["tool"], "linode_ipv6_range_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_ipv6_range_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/networking/ipv6/ranges/"+ipv6RangeCIDR) {
		t.Errorf("got %v, want %v", would["path"], "/networking/ipv6/ranges/"+ipv6RangeCIDR)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeIPv6RangeDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"range":"2001:0db8::","region":"us-east"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyIPv6Range: ipv6RangeCIDR,
		keyDryRun:    true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeIPv6RangeDeleteToolDryRunDryRunStillValidatesRange(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeIPv6RangeDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, keyIPv6Range) {
		t.Errorf("error text %q does not contain %q", text.Text, keyIPv6Range)
	}
}
