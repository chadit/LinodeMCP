package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func writeToolInstanceStatsFixture(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	// not_in_proto is a field the InstanceStats proto does not model; the
	// DiscardUnknown decode must drop it so it never reaches the output.
	_, err := w.Write([]byte(`{
		"title":"linode.com - my-linode (linode123456) - day (5 min avg)",
		"not_in_proto":"dropped",
		"data":{
			"cpu":[[1521483600000,0.42]],
			"io":{"io":[[1521484800000,0.19]],"swap":[[1521484800000,0]]},
			"netv4":{"in":[[1521484800000,2004.36]],"out":[[1521484800000,3928.91]],"private_in":[[1521484800000,0]],"private_out":[[1521484800000,5.6]]},
			"netv6":{"in":[[1521484800000,0]],"out":[[1521484800000,0]],"private_in":[[1521484800000,195.18]],"private_out":[[1521484800000,5.6]]}
		}
	}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLinodeInstanceStatsGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceStatsGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_stats_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_stats_get")
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

	if !strings.Contains(string(tool.RawInputSchema), keyLinodeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeID)
	}
}

func TestLinodeInstanceStatsGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceStatsGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
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

func TestLinodeInstanceStatsGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/stats" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/stats")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		writeToolInstanceStatsFixture(t, w)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceStatsGetTool(srvCfg)

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

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "linode.com - my-linode") {
		t.Errorf("textContent.Text does not contain %v", "linode.com - my-linode")
	}

	if !strings.Contains(textContent.Text, "0.42") {
		t.Errorf("textContent.Text does not contain %v", "0.42")
	}

	if strings.Contains(textContent.Text, keyNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeInstanceStatsGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/stats" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/stats")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceStatsGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to get stats for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to get stats for instance 123")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

const (
	toolLinodeInstanceStatsMonthGet = "linode_instance_stats_month_get"
	keyStatsYear                    = "year"
	keyStatsMonth                   = "month"
	errStatsYearRange               = "year must be an integer between 2000 and 2037"
	errStatsMonthRange              = "month must be an integer between 1 and 12"
)

func TestLinodeInstanceStatsByYearMonthToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)

	t.Parallel()

	if tool.Name != toolLinodeInstanceStatsMonthGet {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeInstanceStatsMonthGet)
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

	for _, key := range []string{keyLinodeID, keyStatsYear, keyStatsMonth} {
		if !strings.Contains(string(tool.RawInputSchema), key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeInstanceStatsByYearMonthToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyStatsYear: float64(2024), keyStatsMonth: float64(8)}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyStatsYear: float64(2024), keyStatsMonth: float64(8)}, wantContains: "linode_id must be a positive integer"},
		{name: "missing year", args: map[string]any{keyLinodeID: float64(123), keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "year too low", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(1999), keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "year separator", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: "2024/..", keyStatsMonth: float64(8)}, wantContains: errStatsYearRange},
		{name: "missing month", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024)}, wantContains: errStatsMonthRange},
		{name: "month too high", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(13)}, wantContains: errStatsMonthRange},
		{name: "month query", args: map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: "8?query"}, wantContains: errStatsMonthRange},
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceStatsByYearMonthToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/stats/2024/8" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/stats/2024/8")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"title":       "linode123 stats",
			keyNotInProto: valNotInProto,
			"data": map[string]any{
				"cpu":   [][]float64{{1521483600000, 0.42}},
				"io":    map[string]any{"io": [][]float64{{1521484800000, 0.19}}, "swap": [][]float64{{1521484800000, 0}}},
				"netv4": map[string]any{"in": [][]float64{{1521484800000, 2004.36}}, "out": [][]float64{{1521484800000, 3928.91}}, "private_in": [][]float64{{1521484800000, 0}}, "private_out": [][]float64{{1521484800000, 5.6}}},
				"netv6": map[string]any{"in": [][]float64{{1521484800000, 10}}, "out": [][]float64{{1521484800000, 20}}, "private_in": [][]float64{{1521484800000, 0}}, "private_out": [][]float64{{1521484800000, 0}}},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(8)}))
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

	if !strings.Contains(textContent.Text, "linode123 stats") {
		t.Errorf("textContent.Text does not contain %v", "linode123 stats")
	}

	if !strings.Contains(textContent.Text, "2004.36") {
		t.Errorf("textContent.Text does not contain %v", "2004.36")
	}

	if strings.Contains(textContent.Text, keyNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeInstanceStatsByYearMonthToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/stats/2024/8" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/stats/2024/8")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeInstanceStatsByYearMonthTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyStatsYear: float64(2024), keyStatsMonth: float64(8)}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve statistics for instance 123 in 2024-08") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve statistics for instance 123 in 2024-08")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
