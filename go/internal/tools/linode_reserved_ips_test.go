package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const reservedIPListPaginationQuery = "page=2&page_size=50"

func TestLinodeReservedIPListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPListTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_list")
	}

	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(string(tool.RawInputSchema), key) {
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

func TestLinodeReservedIPListToolSuccess(t *testing.T) {
	t.Parallel()

	const responseBody = `{"data":[{"address":"192.0.2.10","assigned_entity":null,"gateway":null,"interface_id":null,"linode_id":null,"prefix":24,"public":true,"rdns":null,"region":"us-east","reserved":true,"subnet_mask":"255.255.255.0","tags":["prod"],"type":"ipv4","vpc_nat_1_1":null}],"page":2,"pages":3,"results":1}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/reserved/ips" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/reserved/ips")
		}

		if r.URL.RawQuery != reservedIPListPaginationQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, reservedIPListPaginationQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeReservedIPListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 50}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.IsError {
		t.Fatalf("result = %+v, want successful result", result)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("result.Content[0] is not mcp.TextContent")
	}

	var body struct {
		Count       int              `json:"count"`
		ReservedIPs []map[string]any `json:"reserved_ips"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}

	if body.Count != 1 || len(body.ReservedIPs) != 1 {
		t.Fatalf("response envelope = %+v, want count 1 and one reserved IP", body)
	}

	item := body.ReservedIPs[0]
	for _, key := range []string{"assigned_entity", "gateway", "interface_id", "linode_id", "rdns", "vpc_nat_1_1"} {
		value, exists := item[key]
		if !exists || value != nil {
			t.Errorf("item[%q] = %#v, exists %v; want explicit null", key, value, exists)
		}
	}
}

func TestLinodeReservedIPListToolInvalidPagination(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPListTool(&config.Config{})
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, want: paginationMessagePageMustBe},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, want: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, want: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !result.IsError || !ok || !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("result = %+v, want tool error containing %q", result, testCase.want)
			}
		})
	}
}

func TestLinodeReservedIPListToolAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeReservedIPListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !result.IsError || !ok || !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("result = %+v, want tool error containing Failed to retrieve items", result)
	}
}

func TestLinodeReservedIPListToolConfigError(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeReservedIPListTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !result.IsError || !ok || textContent.Text == "" {
		t.Errorf("result = %+v, want non-empty configuration error", result)
	}
}
