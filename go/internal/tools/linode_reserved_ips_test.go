package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const reservedIPListPaginationQuery = "page=2&page_size=50"

func TestLinodeReservedIPDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeReservedIPDeleteTool(&config.Config{})
	if tool.Name != "linode_networking_reserved_ip_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_reserved_ip_delete")
	}

	for _, key := range []string{keyAddress, keyConfirm, "dry_run", "mode", "plan_id"} {
		if !strings.Contains(string(tool.RawInputSchema), key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeReservedIPDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != "/networking/reserved/ips/"+reservedIPAddressFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/reserved/ips/"+reservedIPAddressFixture)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("request body = %q, want empty", body)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeReservedIPDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: reservedIPAddressFixture, keyConfirm: true, keyConfirmBypassDryRun: true,
	}))
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

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}

	if body["message"] != "Reserved IP 192.0.2.10 unreserved successfully" || body[keyAddress] != reservedIPAddressFixture {
		t.Errorf("response = %v, want stable message and address", body)
	}
}

func TestLinodeReservedIPDeleteToolDryRunFetchesRawStateWithoutDeleting(t *testing.T) {
	t.Parallel()

	const responseBody = `{"address":"192.0.2.10","assigned_entity":null,"gateway":null,"interface_id":null,"linode_id":null,"prefix":24,"public":true,"rdns":null,"region":"us-east","reserved":true,"subnet_mask":"255.255.255.0","tags":[],"type":"ipv4","vpc_nat_1_1":null}`

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("dry-run request method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/networking/reserved/ips/"+reservedIPAddressFixture {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/networking/reserved/ips/"+reservedIPAddressFixture)
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
	_, _, handler := tools.NewLinodeReservedIPDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: reservedIPAddressFixture,
		keyDryRun:  true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.IsError {
		t.Fatalf("result = %+v, want successful dry-run", result)
	}

	textContent, contentOK := result.Content[0].(mcp.TextContent)
	if !contentOK {
		t.Fatal("result.Content[0] is not mcp.TextContent")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal dry-run response: %v", err)
	}

	assertDryRunRequest(t, body, http.MethodDelete, "/networking/reserved/ips/"+reservedIPAddressFixture)

	currentState, currentStateOK := body["current_state"].(map[string]any)
	if !currentStateOK {
		t.Fatalf("current_state = %T, want object", body["current_state"])
	}

	if assignedEntity, present := currentState["assigned_entity"]; !present || assignedEntity != nil {
		t.Errorf("current_state.assigned_entity = %v (present %v), want explicit null", assignedEntity, present)
	}

	tags, ok := currentState["tags"].([]any)
	if !ok || len(tags) != 0 {
		t.Errorf("current_state.tags = %#v, want explicit empty array", currentState["tags"])
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("API calls = %d, want one GET and no DELETE", got)
	}
}

func TestLinodeReservedIPDeleteToolRejectsInvalidInputBeforeClient(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLoopbackClosed, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeReservedIPDeleteTool(cfg)

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingAddress, args: map[string]any{keyConfirm: true}, want: errAddressRequired},
		{name: "IPv6 address", args: map[string]any{keyAddress: networkingIPv6AddressFixture, keyConfirm: true}, want: "valid IPv4 address"},
		{name: "path separator", args: map[string]any{keyAddress: "192.0.2.10/other", keyConfirm: true}, want: "valid IPv4 address"},
		{name: "string confirm", args: map[string]any{keyAddress: reservedIPAddressFixture, keyConfirm: boolStringTrue}, want: "confirm=true"},
	}

	for _, testCase := range tests {
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
