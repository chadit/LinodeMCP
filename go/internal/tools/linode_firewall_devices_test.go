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
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

func TestLinodeFirewallDevicesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeFirewallDeviceListTool(&config.Config{})

	if tool.Name != "linode_firewall_device_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_device_list")
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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyFirewallID, keyPage, keyPageSize} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallDevicesListToolSuccess(t *testing.T) {
	t.Parallel()

	devices := linode.PaginatedResponse[linode.FirewallDevice]{
		Data: []linode.FirewallDevice{{
			ID: 456,
			Entity: linode.FirewallDeviceEntity{
				ID:    123,
				Label: firewallDeviceLabelFixture,
				Type:  monitorAlertDefinitionToolServiceType,
				URL:   "/v4/linode/instances/123",
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices)
		}

		if r.URL.Query().Get(keyPage) != "2" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "50" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(devices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeFirewallDeviceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, firewallDeviceLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", firewallDeviceLabelFixture)
	}

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolServiceType) {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolServiceType)
	}
}

func TestLinodeFirewallDevicesListToolRejectsInvalidFirewallIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		caseMissingFirewallPathID:   nil,
		caseZeroFirewallPathID:      float64(0),
		caseSlashFirewallPathID:     paymentMethodIDSlash,
		caseQueryFirewallPathID:     databaseInvalidInstanceIDQuery,
		caseTraversalFirewallPathID: pathTraversalValue,
	}

	for name, rawID := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := gentools.NewLinodeFirewallDeviceListTool(cfg)

			args := map[string]any{}
			if rawID != nil {
				args[keyFirewallID] = rawID
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			wantFirewallID := errFirewallIDPositive
			if name == caseMissingFirewallPathID {
				wantFirewallID = errFirewallIDRequired
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, wantFirewallID) {
				t.Errorf("error text %q does not contain %q", text.Text, wantFirewallID)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallDevicesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices)
		}

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
	_, _, handler := gentools.NewLinodeFirewallDeviceListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "forbidden") {
		t.Errorf("error text %q does not contain %q", text.Text, "forbidden")
	}
}

func TestLinodeFirewallDeviceGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeFirewallDeviceGetTool(&config.Config{})

	if tool.Name != "linode_firewall_device_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_device_get")
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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyFirewallID, keyFirewallDeviceID} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallDeviceGetToolSuccess(t *testing.T) {
	t.Parallel()

	device := linode.FirewallDevice{
		ID: 456,
		Entity: linode.FirewallDeviceEntity{
			ID:    123,
			Label: firewallDeviceLabelFixture,
			Type:  monitorAlertDefinitionToolServiceType,
			URL:   "/v4/linode/instances/123",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(device); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := gentools.NewLinodeFirewallDeviceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, firewallDeviceLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", firewallDeviceLabelFixture)
	}

	if !strings.Contains(textContent.Text, monitorAlertDefinitionToolServiceType) {
		t.Errorf("textContent.Text does not contain %v", monitorAlertDefinitionToolServiceType)
	}
}

func TestLinodeFirewallDeviceGetToolRejectsInvalidIdsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingFirewallPathID, args: map[string]any{keyFirewallDeviceID: float64(456)}, want: errFirewallIDRequired},
		{name: caseZeroFirewallPathID, args: map[string]any{keyFirewallID: float64(0), keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
		{name: caseSlashFirewallPathID, args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
		{name: caseQueryFirewallPathID, args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
		{name: caseTraversalFirewallPathID, args: map[string]any{keyFirewallID: pathTraversalValue, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
		{name: caseMissingFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123)}, want: errFirewallDeviceIDPositive},
		{name: caseZeroFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(0)}, want: errFirewallDeviceIDPositive},
		{name: caseSlashFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: paymentMethodIDSlash}, want: errFirewallDeviceIDPositive},
		{name: caseQueryFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: databaseInvalidInstanceIDQuery}, want: errFirewallDeviceIDPositive},
		{name: caseTraversalFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: pathTraversalValue}, want: errFirewallDeviceIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := gentools.NewLinodeFirewallDeviceGetTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallDeviceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices456)
		}

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
	_, _, handler := gentools.NewLinodeFirewallDeviceGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve device") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve device")
	}
}
