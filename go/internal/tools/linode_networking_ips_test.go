package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	networkingIPAddressFixture   = "198.51.100.5"
	networkingIPv6AddressFixture = "2001:db8::1"
	networkingScopedIPv6Fixture  = "fe80::1%eth0"
	networkingZoneTraversalValue = "fe80::1%../../x?y=1"
)

func TestLinodeNetworkingIPsListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeNetworkingIPListTool(&config.Config{})

	if tool.Name != "linode_networking_ip_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ip_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if !strings.Contains(string(tool.RawInputSchema), "skip_ipv6_rdns") {
		t.Errorf("tool.RawInputSchema missing key %v", "skip_ipv6_rdns")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPsListToolSuccess(t *testing.T) {
	t.Parallel()

	ips := PaginatedResponse[IPAddress]{
		Data: []IPAddress{{
			Address: networkingIPAddressFixture,
			Type:    keyIPv4,
			Public:  true,
			Region:  regionUSEast,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/ips" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ips")
		}

		if r.URL.Query().Get("skip_ipv6_rdns") != boolStringTrue {
			t.Errorf("got %v, want %v", r.URL.Query().Get("skip_ipv6_rdns"), boolStringTrue)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(ips); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeNetworkingIPListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": true}))
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

	if !strings.Contains(textContent.Text, networkingIPAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", networkingIPAddressFixture)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeNetworkingIPsListToolInvalidSkipIpv6Rdns(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeNetworkingIPListTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": boolStringTrue}))
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

	if !strings.Contains(textContent.Text, "skip_ipv6_rdns must be a boolean") {
		t.Errorf("textContent.Text does not contain %v", "skip_ipv6_rdns must be a boolean")
	}
}

func TestLinodeNetworkingIPGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeNetworkingIPGetTool(&config.Config{})

	if tool.Name != "linode_networking_ip_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ip_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if !strings.Contains(string(tool.RawInputSchema), keyAddress) {
		t.Errorf("tool.RawInputSchema missing key %v", keyAddress)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/ips/"+networkingIPAddressFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ips/"+networkingIPAddressFixture)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(IPAddress{
			Address: networkingIPAddressFixture,
			Type:    keyIPv4,
			Public:  true,
			Region:  regionUSEast,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeNetworkingIPGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPAddressFixture}))
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

	if !strings.Contains(textContent.Text, networkingIPAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", networkingIPAddressFixture)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

func TestLinodeNetworkingIPGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeNetworkingIPGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPAddressFixture}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve networking IP") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve networking IP")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "not found") {
		t.Errorf("error text %q does not contain %q", text.Text, "not found")
	}
}

func TestLinodeNetworkingIPGetToolAddress(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]any{
		caseMissingAddress:        nil,
		"non-string address":      123,
		"slash address":           "198.51.100.5/24",
		"query separator address": "198.51.100.5?bad=1",
		"traversal address":       pathTraversalValue,
		"scoped IPv6 address":     networkingScopedIPv6Fixture,
		"zone traversal address":  networkingZoneTraversalValue,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := gentools.NewLinodeNetworkingIPGetTool(cfg)

			args := map[string]any{}
			if address != nil {
				args[keyAddress] = address
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

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeNetworkingIPGetToolIPv6Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/networking/ips/2001:db8::1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/networking/ips/2001:db8::1")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(IPAddress{Address: networkingIPv6AddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeNetworkingIPGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPv6AddressFixture}))
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
