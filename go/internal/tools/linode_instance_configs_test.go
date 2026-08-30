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

type instanceConfigCreateValidationCase struct {
	name         string
	args         map[string]any
	wantContains string
}

func TestLinodeInstanceConfigInterfaceGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceConfigInterfaceGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_config_interface_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_config_interface_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyLinodeID, keyConfigID, keyInterfaceID} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeInstanceConfigInterfaceGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeInstanceConfigInterfaceGetTool(cfg)

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingLinodeID, args: map[string]any{keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfigID: float64(789), keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)}, wantContains: errConfigIDRequired},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: shareGroupIDQueryValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyInterfaceID: float64(456)}, wantContains: errConfigIDPositive},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789)}, wantContains: errInterfaceIDRequired},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: float64(-1)}, wantContains: errInterfaceIDPositive},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathSeparatorValue}, wantContains: errInterfaceIDPositive},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: shareGroupIDQueryValue}, wantContains: errInterfaceIDPositive},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(789), keyInterfaceID: pathTraversalValue}, wantContains: errInterfaceIDPositive},
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

func TestLinodeInstanceConfigInterfaceGetToolSuccessfulGet(t *testing.T) {
	t.Parallel()

	primary := true
	gotInterface := ConfigInterfaceResponse{ID: 456, Active: true, Purpose: purposeVPC, Primary: primary}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(gotInterface); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigInterfaceGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID:    float64(123),
		keyConfigID:    float64(789),
		keyInterfaceID: float64(456),
	})

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

	if !strings.Contains(textContent.Text, purposeVPC) {
		t.Errorf("textContent.Text does not contain %v", purposeVPC)
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}

	if !strings.Contains(textContent.Text, boolStringTrue) {
		t.Errorf("textContent.Text does not contain %v", boolStringTrue)
	}
}

func TestLinodeInstanceConfigInterfaceGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs789Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs789Interfaces456)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodeInstanceConfigInterfaceGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID:    float64(123),
		keyConfigID:    float64(789),
		keyInterfaceID: float64(456),
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve configuration profile interface") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve configuration profile interface")
	}
}
