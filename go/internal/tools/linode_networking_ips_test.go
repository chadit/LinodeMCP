package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	networkingIPAddressFixture   = "198.51.100.5"
	networkingIPv6AddressFixture = "2001:db8::1"
	networkingScopedIPv6Fixture  = "fe80::1%eth0"
	networkingZoneTraversalValue = "fe80::1%../../x?y=1"
	networkingIPAssignmentJSON   = `[{"address":"198.51.100.5","linode_id":123}]`
	networkingIPShareJSON        = `["198.51.100.5"]`
	keyIPs                       = "ips"
)

func TestLinodeNetworkingIPsListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

	if tool.Name != "linode_networking_ips_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ips_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if _, ok := tool.InputSchema.Properties["skip_ipv6_rdns"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "skip_ipv6_rdns")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPsListToolSuccess(t *testing.T) {
	t.Parallel()

	ips := linode.PaginatedResponse[linode.IPAddress]{
		Data: []linode.IPAddress{{
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
	_, _, handler := tools.NewLinodeNetworkingIPListTool(cfg)

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

	_, _, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

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

	tool, capability, handler := tools.NewLinodeNetworkingIPGetTool(&config.Config{})

	if tool.Name != "linode_networking_ip_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ip_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if _, ok := tool.InputSchema.Properties[keyAddress]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyAddress)
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

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
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
	_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

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
	_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

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
			_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

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

		if err := json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

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

func TestLinodeNetworkingIPUpdateRDNSToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(&config.Config{})

	if tool.Name != "linode_networking_ip_update_rdns" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ip_update_rdns")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	for _, key := range []string{keyAddress, keyRDNS, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPUpdateRDNSToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/networking/ips/"+networkingIPAddressFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ips/"+networkingIPAddressFixture)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.UpdateNetworkingIPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.RDNS != rdnsTestExampleOrg {
			t.Errorf("body.RDNS = %v, want %v", body.RDNS, rdnsTestExampleOrg)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
			Address: networkingIPAddressFixture,
			RDNS:    rdnsTestExampleOrg,
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
	_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: networkingIPAddressFixture,
		keyRDNS:    rdnsTestExampleOrg,
		keyConfirm: true,
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

	if !strings.Contains(textContent.Text, networkingIPAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", networkingIPAddressFixture)
	}

	if !strings.Contains(textContent.Text, rdnsTestExampleOrg) {
		t.Errorf("textContent.Text does not contain %v", rdnsTestExampleOrg)
	}
}

func TestLinodeNetworkingIPUpdateRDNSToolApiError(t *testing.T) {
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
	_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyAddress: networkingIPAddressFixture,
		keyRDNS:    rdnsTestExampleOrg,
		keyConfirm: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update networking IP") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update networking IP")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "not found") {
		t.Errorf("error text %q does not contain %q", text.Text, "not found")
	}
}

func TestLinodeNetworkingIPUpdateRDNSToolConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseMissingConfirm: nil,
		caseFalseConfirm:   false,
		caseStringConfirm:  boolStringTrue,
		caseNumericConfirm: float64(1),
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
			_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(cfg)

			args := map[string]any{keyAddress: networkingIPAddressFixture, keyRDNS: rdnsTestExampleOrg}
			if confirm != nil {
				args[keyConfirm] = confirm
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

func TestLinodeNetworkingIPUpdateRDNSToolArgs(t *testing.T) {
	t.Parallel()

	for name, args := range map[string]map[string]any{
		caseMissingAddress:        {keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"slash address":           {keyAddress: "198.51.100.5/24", keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"query separator address": {keyAddress: "198.51.100.5?bad=1", keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"traversal address":       {keyAddress: pathTraversalValue, keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"scoped IPv6 address":     {keyAddress: networkingScopedIPv6Fixture, keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"zone traversal address":  {keyAddress: networkingZoneTraversalValue, keyRDNS: rdnsTestExampleOrg, keyConfirm: true},
		"missing rdns":            {keyAddress: networkingIPAddressFixture, keyConfirm: true},
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
			_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(cfg)

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

func TestLinodeNetworkingIPAllocateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})

	if tool.Name != "linode_networking_ip_allocate" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ip_allocate")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if _, ok := tool.InputSchema.Properties["linode_id"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "linode_id")
	}

	if _, ok := tool.InputSchema.Properties["type"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "type")
	}

	if _, ok := tool.InputSchema.Properties["public"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "public")
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPAllocateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/ips" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ips")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AllocateNetworkingIPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if !body.Public {
			t.Error("body.Public = false, want true")
		}

		if body.Type != keyIPv4 {
			t.Errorf("body.Type = %v, want %v", body.Type, keyIPv4)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.IPAddress{
			Address:  networkingIPAddressFixture,
			Type:     keyIPv4,
			Public:   true,
			Region:   regionUSEast,
			LinodeID: 123,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID:   123,
		keyType:       keyIPv4,
		purposePublic: true,
		keyConfirm:    true,
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

	if !strings.Contains(textContent.Text, networkingIPAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", networkingIPAddressFixture)
	}

	if !strings.Contains(textContent.Text, "allocated") {
		t.Errorf("textContent.Text does not contain %v", "allocated")
	}
}

func TestLinodeNetworkingIPAllocateToolConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
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
			_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(cfg)

			args := map[string]any{keyLinodeID: 123, keyType: keyIPv4, purposePublic: true}
			if confirm != nil {
				args[keyConfirm] = confirm
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

func TestLinodeNetworkingIPAllocateToolArgs(t *testing.T) {
	t.Parallel()

	for name, args := range map[string]map[string]any{
		"missing linode_id": {keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"zero linode_id":    {keyLinodeID: 0, keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"decimal linode_id": {keyLinodeID: 12.5, keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"missing type":      {keyLinodeID: 123, purposePublic: true, keyConfirm: true},
		"blank type":        {keyLinodeID: 123, keyType: blankString, purposePublic: true, keyConfirm: true},
		"missing public":    {keyLinodeID: 123, keyType: keyIPv4, keyConfirm: true},
		"invalid public":    {keyLinodeID: 123, keyType: keyIPv4, purposePublic: boolStringTrue, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})

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
		})
	}
}

func TestLinodeNetworkingIPAssignToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})

	if tool.Name != "linode_networking_ips_assign" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ips_assign")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	for _, key := range []string{keyRegion, keyAssignments, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPAssignToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/ips/assign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ips/assign")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AssignNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.Region != regionUSEast {
			t.Errorf("body.Region = %v, want %v", body.Region, regionUSEast)
		}

		if len(body.Assignments) != 1 {
			t.Errorf("len(body.Assignments) = %d, want %d", len(body.Assignments), 1)
		}

		if body.Assignments[0].Address != networkingIPAddressFixture {
			t.Errorf("body.Assignments[0].Address = %v, want %v", body.Assignments[0].Address, networkingIPAddressFixture)
		}

		if body.Assignments[0].LinodeID != 123 {
			t.Errorf("body.Assignments[0].LinodeID = %v, want %v", body.Assignments[0].LinodeID, 123)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast,
		keyAssignments: networkingIPAssignmentJSON,
		keyConfirm:     true,
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

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

func TestLinodeNetworkingIPAssignToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast,
		keyAssignments: networkingIPAssignmentJSON,
		keyConfirm:     true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to assign networking IPs") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to assign networking IPs")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNetworkingIPAssignToolConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
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
			_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

			args := map[string]any{keyRegion: regionUSEast, keyAssignments: networkingIPAssignmentJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
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

func TestLinodeNetworkingIPAssignToolArgs(t *testing.T) {
	t.Parallel()

	for name, args := range map[string]map[string]any{
		caseMissingRegion:     {keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		caseBlankRegion:       {keyRegion: blankString, keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		"missing assignments": {keyRegion: regionUSEast, keyConfirm: true},
		"invalid assignments": {keyRegion: regionUSEast, keyAssignments: invalidJSON, keyConfirm: true},
		"empty assignments":   {keyRegion: regionUSEast, keyAssignments: databaseJSONArray, keyConfirm: true},
		"missing address":     {keyRegion: regionUSEast, keyAssignments: `[{"linode_id":123}]`, keyConfirm: true},
		"invalid linode_id":   {keyRegion: regionUSEast, keyAssignments: `[{"address":"198.51.100.5","linode_id":0}]`, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})

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
		})
	}
}

func TestLinodeNetworkingIPv4AssignToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})

	if tool.Name != "linode_networking_ipv4_assign" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ipv4_assign")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	for _, key := range []string{keyRegion, keyAssignments, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPv4AssignToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/ipv4/assign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ipv4/assign")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.AssignNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.Region != regionUSEast {
			t.Errorf("body.Region = %v, want %v", body.Region, regionUSEast)
		}

		if len(body.Assignments) != 1 {
			t.Errorf("len(body.Assignments) = %d, want %d", len(body.Assignments), 1)
		}

		if body.Assignments[0].Address != networkingIPAddressFixture {
			t.Errorf("body.Assignments[0].Address = %v, want %v", body.Assignments[0].Address, networkingIPAddressFixture)
		}

		if body.Assignments[0].LinodeID != 123 {
			t.Errorf("body.Assignments[0].LinodeID = %v, want %v", body.Assignments[0].LinodeID, 123)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast,
		keyAssignments: networkingIPAssignmentJSON,
		keyConfirm:     true,
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

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

func TestLinodeNetworkingIPv4AssignToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast,
		keyAssignments: networkingIPAssignmentJSON,
		keyConfirm:     true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to assign networking IPv4s") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to assign networking IPv4s")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNetworkingIPv4AssignToolConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
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
			_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(cfg)

			args := map[string]any{keyRegion: regionUSEast, keyAssignments: networkingIPAssignmentJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
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

func TestLinodeNetworkingIPv4AssignToolRejectsNonIpv4AssignmentsBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(cfg)

	for name, assignments := range map[string]string{
		"ipv6 assignment":   `[{"address":"2001:db8::1","linode_id":123}]`,
		"malformed address": `[{"address":"not-an-ip","linode_id":123}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
				keyRegion:      regionUSEast,
				keyAssignments: assignments,
				keyConfirm:     true,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "IP address must be a valid IPv4 address") {
				t.Errorf("error text %q does not contain %q", text.Text, "IP address must be a valid IPv4 address")
			}
		})
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestLinodeNetworkingIPv4AssignToolArgs(t *testing.T) {
	t.Parallel()

	for name, args := range map[string]map[string]any{
		caseMissingRegion:     {keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		caseBlankRegion:       {keyRegion: blankString, keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		"missing assignments": {keyRegion: regionUSEast, keyConfirm: true},
		"invalid assignments": {keyRegion: regionUSEast, keyAssignments: invalidJSON, keyConfirm: true},
		"empty assignments":   {keyRegion: regionUSEast, keyAssignments: databaseJSONArray, keyConfirm: true},
		"missing address":     {keyRegion: regionUSEast, keyAssignments: `[{"linode_id":123}]`, keyConfirm: true},
		"invalid linode_id":   {keyRegion: regionUSEast, keyAssignments: `[{"address":"198.51.100.5","linode_id":0}]`, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})

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
		})
	}
}

func TestLinodeNetworkingIPShareToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeNetworkingIPShareTool(&config.Config{})

	if tool.Name != "linode_networking_ips_share" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_networking_ips_share")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	for _, key := range []string{keyLinodeID, keyIPs, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNetworkingIPShareToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/ipv4/share" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ipv4/share")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body linode.ShareNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if len(body.IPs) != 1 {
			t.Errorf("len(body.IPs) = %d, want %d", len(body.IPs), 1)

			return
		}

		if body.IPs[0] != networkingIPAddressFixture {
			t.Errorf("body.IPs[0] = %v, want %v", body.IPs[0], networkingIPAddressFixture)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: 123,
		keyIPs:      networkingIPShareJSON,
		keyConfirm:  true,
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

	if !strings.Contains(textContent.Text, "updated") {
		t.Errorf("textContent.Text does not contain %v", "updated")
	}
}

func TestLinodeNetworkingIPShareToolEmptyIpsArray(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/ipv4/share" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/ipv4/share")
		}

		var body linode.ShareNetworkingIPsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if body.LinodeID != 123 {
			t.Errorf("body.LinodeID = %v, want %v", body.LinodeID, 123)
		}

		if len(body.IPs) != 0 {
			t.Errorf("body.IPs = %v, want empty", body.IPs)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: 123,
		keyIPs:      `[]`,
		keyConfirm:  true,
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
}

func TestLinodeNetworkingIPShareToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLinodeID: 123,
		keyIPs:      networkingIPShareJSON,
		keyConfirm:  true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to share networking IPs") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to share networking IPs")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeNetworkingIPShareToolConfirm(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
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
			_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

			args := map[string]any{keyLinodeID: 123, keyIPs: networkingIPShareJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
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

func TestLinodeNetworkingIPShareToolArgs(t *testing.T) {
	t.Parallel()

	for name, args := range map[string]map[string]any{
		"missing linode_id": {keyIPs: networkingIPShareJSON, keyConfirm: true},
		"zero linode_id":    {keyLinodeID: 0, keyIPs: networkingIPShareJSON, keyConfirm: true},
		"decimal linode_id": {keyLinodeID: 12.5, keyIPs: networkingIPShareJSON, keyConfirm: true},
		"missing ips":       {keyLinodeID: 123, keyConfirm: true},
		"invalid ips":       {keyLinodeID: 123, keyIPs: "not-json", keyConfirm: true},
		"null ips":          {keyLinodeID: 123, keyIPs: databaseJSONNull, keyConfirm: true},
		"blank ip":          {keyLinodeID: 123, keyIPs: `[""]`, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPShareTool(&config.Config{})

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
		})
	}
}
