package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// expect* helpers are fatal package-local checks from linode_assertions_test.go; check* helpers are nonfatal.

func TestLinodeNodeBalancerConfigDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_config_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_config_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyNodeBalancerID, keyConfigID, keyConfirm, keyDryRun} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeNodeBalancerConfigDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerConfigDeleteTool(cfg)

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}, want: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: false}, want: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: boolStringTrue}, want: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: float64(1)}, want: errConfirmEqualsTrue},
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(-1), keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
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

func TestLinodeNodeBalancerConfigDeleteToolDryRunReturnsPreviewWithoutDeleting(t *testing.T) {
	t.Parallel()

	var methods []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		// The Phase 2 dependency walk also reads the config's backend nodes,
		// so the preview issues a second GET beyond the config-list fetch.
		if r.URL.Path == tcNodebalancers123Configs456Nodes {
			if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != tcNodebalancers123Configs {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456, keyPort: 80}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyDryRun: true}))
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

	body := decodeBody(t, textContent.Text)
	if body["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", body["dry_run"])
	}

	assertDryRunRequest(t, body, "DELETE", "/nodebalancers/123/configs/456")

	if slices.Contains(methods, http.MethodDelete) {
		t.Errorf("methods should not contain %v", http.MethodDelete)
	}
}

func TestLinodeNodeBalancerConfigDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcNodebalancers123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "removed") {
		t.Errorf("textContent.Text does not contain %v", "removed")
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}
}

func TestLinodeNodeBalancerConfigDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete config 456 from NodeBalancer 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete config 456 from NodeBalancer 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
