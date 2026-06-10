package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeInstanceInterfaceGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_interface_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_interface_get")
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

	for _, key := range []string{keyLinodeID, keyInterfaceID} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeInstanceInterfaceGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfaceGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123)}, wantContains: "interface_id is required"},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue}, wantContains: errInterfaceIDInteger},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue}, wantContains: errInterfaceIDInteger},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue}, wantContains: errInterfaceIDInteger},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(-1)}, wantContains: errInterfaceIDMinOne},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceInterfaceGetToolSuccess(t *testing.T) {
	t.Parallel()

	instanceInterface := linode.InstanceInterface{ID: 456, MACAddress: "22:00:AB:CD:EF:02"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(instanceInterface); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "22:00:AB:CD:EF:02") {
		t.Errorf("textContent.Text does not contain %v", "22:00:AB:CD:EF:02")
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}
}

func TestLinodeInstanceInterfaceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve interface 456 for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve interface 456 for instance 123")
	}
}

func TestLinodeInstanceInterfacesListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfacesListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_interface_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_interface_list")
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

	if _, ok := tool.InputSchema.Properties[keyLinodeID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyLinodeID)
	}
}

func TestLinodeInstanceInterfacesListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfacesListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceInterfacesListToolSuccess(t *testing.T) {
	t.Parallel()

	interfaces := []linode.InstanceInterface{{ID: 1234, MACAddress: macAddressFixture}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{"interfaces": interfaces}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfacesListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, macAddressFixture) {
		t.Errorf("textContent.Text does not contain %v", macAddressFixture)
	}

	if !strings.Contains(textContent.Text, "1234") {
		t.Errorf("textContent.Text does not contain %v", "1234")
	}
}

func TestLinodeInstanceInterfacesListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfacesListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list interfaces for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list interfaces for instance 123")
	}
}

func TestLinodeInstanceInterfaceDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_interface_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_interface_delete")
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

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	for _, key := range []string{keyLinodeID, keyInterfaceID, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if !slices.Contains(tool.InputSchema.Required, keyConfirm) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyConfirm)
	}
}

func TestLinodeInstanceInterfaceDeleteToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfaceDeleteTool(cfg)

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
		{name: caseNumericConfirmRejected, value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)}
			if tt.set {
				args[keyConfirm] = tt.value
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

func TestLinodeInstanceInterfaceDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfaceDeleteTool(cfg)

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errLinodeIDInteger},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, want: tools.ErrInterfaceIDRequired.Error()},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errInterfaceIDInteger},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: interfaceIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errInterfaceIDInteger},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errInterfaceIDInteger},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(-1), keyConfirm: true, keyConfirmedDryRun: true}, want: errInterfaceIDMinOne},
		{name: caseZeroInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(0), keyConfirm: true, keyConfirmedDryRun: true}, want: errInterfaceIDMinOne},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.want) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.want)
			}
		})
	}
}

func TestLinodeInstanceInterfaceDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "deleted") {
		t.Errorf("error text %q does not contain %q", text.Text, "deleted")
	}
}

func TestLinodeInstanceInterfaceDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete interface 456 from instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete interface 456 from instance 123")
	}
}
