package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestLinodeFirewallDevicesListToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallDevicesListTool(&config.Config{})

	if tool.Name != "linode_firewall_devices_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_devices_list")
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

	if _, ok := tool.InputSchema.Properties[keyFirewallID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyFirewallID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyFirewallID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyFirewallID)
	}

	for _, key := range []string{keyPage, keyPageSize} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
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
	_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

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
			_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errFirewallIDPositive) {
				t.Errorf("error text %q does not contain %q", text.Text, errFirewallIDPositive)
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
	_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_devices_list") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_devices_list")
	}
}

func TestLinodeFirewallDeviceGetToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallDeviceGetTool(&config.Config{})

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

	if _, ok := tool.InputSchema.Properties[keyFirewallID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyFirewallID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyFirewallID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyFirewallID)
	}

	if _, ok := tool.InputSchema.Properties[keyFirewallDeviceID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyFirewallDeviceID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyFirewallDeviceID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyFirewallDeviceID)
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
	_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

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
		{name: caseMissingFirewallPathID, args: map[string]any{keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
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
			_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

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
	_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_firewall_device_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_firewall_device_get")
	}
}

func TestLinodeFirewallDeviceDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})

	if tool.Name != "linode_firewall_device_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_device_delete")
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

	for _, key := range []string{keyFirewallID, keyFirewallDeviceID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyFirewallID, keyFirewallDeviceID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeFirewallDeviceDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
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
	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "Firewall device removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "Firewall device removed successfully")
	}

	if !strings.Contains(textContent.Text, keyFirewallID) {
		t.Errorf("textContent.Text does not contain %v", keyFirewallID)
	}

	if !strings.Contains(textContent.Text, keyFirewallDeviceID) {
		t.Errorf("textContent.Text does not contain %v", keyFirewallDeviceID)
	}
}

func TestLinodeFirewallDeviceDeleteToolRejectsInvalidConfirmBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]any{
		caseMissingConfirm:         nil,
		caseConfirmFalse:           false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: float64(1),
	}

	for name, rawConfirm := range cases {
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
			_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

			args := map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)}
			if rawConfirm != nil {
				args[keyConfirm] = rawConfirm
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallDeviceDeleteToolRejectsInvalidIdsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingFirewallPathID, args: map[string]any{keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallIDPositive},
		{name: caseZeroFirewallPathID, args: map[string]any{keyFirewallID: float64(0), keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallIDPositive},
		{name: caseSlashFirewallPathID, args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallIDPositive},
		{name: caseQueryFirewallPathID, args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallIDPositive},
		{name: caseTraversalFirewallPathID, args: map[string]any{keyFirewallID: pathTraversalValue, keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallIDPositive},
		{name: caseMissingFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallDeviceIDPositive},
		{name: caseZeroFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(0), keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallDeviceIDPositive},
		{name: caseSlashFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: paymentMethodIDSlash, keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallDeviceIDPositive},
		{name: caseQueryFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: databaseInvalidInstanceIDQuery, keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallDeviceIDPositive},
		{name: caseTraversalFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errFirewallDeviceIDPositive},
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
			_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

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

func TestLinodeFirewallDeviceDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
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
	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_firewall_device_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_firewall_device_delete failed")
	}
}

// Dry-run coverage for firewall device delete. Kept in a sibling
// function so the main test's subtest count stays under maintidx's
// threshold.
func TestLinodeFirewallDeviceDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeFirewallDeviceDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	deviceBody := `{"id":456,"created":"2024-01-01T00:00:00","updated":"2024-01-01T00:00:00","entity":{"id":789,"type":"linode","label":"web-01","url":"/linode/instances/789"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcNetworkingFirewalls123Devices456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices456)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			if _, writeErr := w.Write([]byte(deviceBody)); writeErr != nil {
				t.Errorf("write response: %v", writeErr)
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
	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID:       float64(123),
		keyFirewallDeviceID: float64(456),
		keyDryRun:           true,
	})

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

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Error("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_device_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_device_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Error("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcNetworkingFirewalls123Devices456) {
		t.Errorf("got %v, want %v", would["path"], tcNetworkingFirewalls123Devices456)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeFirewallDeviceDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")

		if _, writeErr := w.Write([]byte(`{"id":456,"created":"2024-01-01T00:00:00","updated":"2024-01-01T00:00:00","entity":{"id":789,"type":"linode","label":"web-01","url":"/linode/instances/789"}}`)); writeErr != nil {
			t.Errorf("write response: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID:       float64(123),
		keyFirewallDeviceID: float64(456),
		keyDryRun:           true,
	})

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
}

func TestLinodeFirewallDeviceDeleteToolDryRunDryRunStillRejectsNonPositiveFirewallId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID:       float64(-1),
		keyFirewallDeviceID: float64(456),
		keyDryRun:           true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "firewall_id must be a positive integer") {
		t.Errorf("error text %q does not contain %q", text.Text, "firewall_id must be a positive integer")
	}
}

func TestLinodeFirewallDeviceDeleteToolDryRunDryRunStillRejectsNonPositiveDeviceId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID:       float64(123),
		keyFirewallDeviceID: float64(-1),
		keyDryRun:           true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "device id must be a positive integer") {
		t.Errorf("error text %q does not contain %q", text.Text, "device id must be a positive integer")
	}
}
