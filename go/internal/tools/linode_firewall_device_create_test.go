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

func TestLinodeFirewallDeviceCreateToolCreateDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})

	if tool.Name != "linode_firewall_device_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_device_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyFirewallID, keyBetaID, keyType, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyFirewallID, keyBetaID, keyType, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeFirewallDeviceCreateToolCreateSuccess(t *testing.T) {
	t.Parallel()

	device := linode.FirewallDevice{
		ID: 789,
		Entity: linode.FirewallDeviceEntity{
			ID:    456,
			Label: firewallDeviceLabelFixture,
			Type:  monitorAlertDefinitionToolServiceType,
			URL:   "/v4/linode/instances/456",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcNetworkingFirewalls123Devices {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls123Devices)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var got linode.CreateFirewallDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, linode.CreateFirewallDeviceRequest{ID: 456, Type: monitorAlertDefinitionToolServiceType}) {
			t.Errorf("got = %v, want %v", got, linode.CreateFirewallDeviceRequest{ID: 456, Type: monitorAlertDefinitionToolServiceType})
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
	_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Firewall device assigned successfully") {
		t.Errorf("textContent.Text does not contain %v", "Firewall device assigned successfully")
	}

	if !strings.Contains(textContent.Text, firewallDeviceLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", firewallDeviceLabelFixture)
	}
}

func TestLinodeFirewallDeviceCreateToolCreateRejectsInvalidConfirmBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

			args := map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType}
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

func TestLinodeFirewallDeviceCreateToolCreateRejectsInvalidInputBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]any{
		caseMissingFirewallPathID:   {keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		caseZeroFirewallPathID:      {keyFirewallID: float64(0), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		caseSlashFirewallPathID:     {keyFirewallID: paymentMethodIDSlash, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		caseQueryFirewallPathID:     {keyFirewallID: databaseInvalidInstanceIDQuery, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		caseTraversalFirewallPathID: {keyFirewallID: pathTraversalValue, keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		"missing device id":         {keyFirewallID: float64(123), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		"zero device id":            {keyFirewallID: float64(123), keyBetaID: float64(0), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true},
		caseInvalidType:             {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: "linode/123", keyConfirm: true},
		"query type":                {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: "linode?x=1", keyConfirm: true},
		"traversal type":            {keyFirewallID: float64(123), keyBetaID: float64(456), keyType: pathTraversalValue, keyConfirm: true},
	}

	for name, args := range cases {
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
			_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

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

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodeFirewallDeviceCreateToolCreateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
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
	_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyBetaID: float64(456), keyType: monitorAlertDefinitionToolServiceType, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_firewall_device_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_firewall_device_create")
	}
}
