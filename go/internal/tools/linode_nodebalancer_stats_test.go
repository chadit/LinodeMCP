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

func writeToolNodeBalancerStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	_, err := w.Write([]byte(`{
		"title":"nodebalancer.example.com (nodebalancer123) - day (5 min avg)",
		"connections":[[1521483600000,12.5]],
		"traffic":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]]}
	}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLinodeNodeBalancerStatsGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerStatsGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_stats_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_stats_get")
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

	if _, ok := tool.InputSchema.Properties[keyNodeBalancerID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyNodeBalancerID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyNodeBalancerID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyNodeBalancerID)
	}
}

func TestLinodeNodeBalancerStatsGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerStatsGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1)}, wantContains: errNodeBalancerIDMin},
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeNodeBalancerStatsGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/444/stats" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/444/stats")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		writeToolNodeBalancerStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerStatsGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)}))
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

	if !strings.Contains(textContent.Text, "nodebalancer.example.com") {
		t.Errorf("textContent.Text does not contain %v", "nodebalancer.example.com")
	}

	if !strings.Contains(textContent.Text, "2004.36") {
		t.Errorf("textContent.Text does not contain %v", "2004.36")
	}
}

func TestLinodeNodeBalancerStatsGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/nodebalancers/444/stats" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers/444/stats")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeNodeBalancerStatsGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve NodeBalancer 444 stats") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve NodeBalancer 444 stats")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
