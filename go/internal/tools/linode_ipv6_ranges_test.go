package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestLinodeIPv6RangesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeIPv6RangeListTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
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

	ranges := PaginatedResponse[IPv6Range]{
		Data: []IPv6Range{{
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
	_, _, handler := gentools.NewLinodeIPv6RangeListTool(cfg)

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
	_, _, handler := gentools.NewLinodeIPv6RangeListTool(cfg)

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
	_, _, handler := gentools.NewLinodeIPv6RangeListTool(cfg)

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

func TestLinodeIPv6RangeGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeIPv6RangeGetTool(&config.Config{})

	if tool.Name != "linode_ipv6_range_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_ipv6_range_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyIPv6Range) {
		t.Errorf("tool.RawInputSchema missing key %v", keyIPv6Range)
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

	// The single-range detail endpoint returns is_bgp and the bound Linode IDs
	// and drops route_target. The extra field the proto does not model must be
	// dropped by the DiscardUnknown decode, proving the output routes through the
	// proto serializer.
	rangeResult := struct {
		IPv6Range

		NotInProto string `json:"not_in_proto"`
		Linodes    []int  `json:"linodes"`
		IsBgp      bool   `json:"is_bgp"`
	}{
		Range:      ipv6RangeCIDR,
		Region:     regionUSEast,
		Prefix:     64,
		IsBgp:      false,
		Linodes:    []int{12345, 12346},
		NotInProto: valNotInProto,
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
	_, _, handler := gentools.NewLinodeIPv6RangeGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "12345") {
		t.Errorf("textContent.Text does not contain the bound Linode ID 12345")
	}

	if strings.Contains(textContent.Text, valNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
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
	_, _, handler := gentools.NewLinodeIPv6RangeGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeIPv6RangeGetTool(cfg)

	cases := []struct {
		args map[string]any
		name string
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
