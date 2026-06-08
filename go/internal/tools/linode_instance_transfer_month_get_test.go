package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	toolLinodeInstanceTransferMonthGet = "linode_instance_transfer_month_get"
	transferKeyYear                    = "year"
	transferKeyMonth                   = "month"
)

func TestLinodeInstanceTransferMonthGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceTransferMonthGetTool(cfg)

	t.Parallel()

	if tool.Name != toolLinodeInstanceTransferMonthGet {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeInstanceTransferMonthGet)
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

	for _, key := range []string{keyLinodeID, transferKeyYear, transferKeyMonth} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}
}

func TestLinodeInstanceTransferMonthGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceTransferMonthGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, transferKeyYear: 2024, transferKeyMonth: 1}, wantContains: errLinodeIDInteger},
		{name: "missing year", args: map[string]any{keyLinodeID: 123, transferKeyMonth: 1}, wantContains: "year is required"},
		{name: "traversal year", args: map[string]any{keyLinodeID: 123, transferKeyYear: pathTraversalValue, transferKeyMonth: 1}, wantContains: "year must be an integer"},
		{name: "query month", args: map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: "1?query"}, wantContains: "month must be an integer"},
		{name: "month too large", args: map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: 13}, wantContains: "month must be"},
	}
	for _, validationTest := range validationTests {
		t.Run(validationTest.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, validationTest.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, validationTest.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, validationTest.wantContains)
			}
		})
	}
}

func TestLinodeInstanceTransferMonthGetToolSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.Transfer{In: 1.5, Out: 2.5, Total: 4}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/transfer/2024/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/transfer/2024/1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceTransferMonthGetTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: 123, transferKeyYear: 2024, transferKeyMonth: 1}))
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

	if !strings.Contains(textContent.Text, `"total": 4`) {
		t.Errorf("textContent.Text does not contain %v", `"total": 4`)
	}
}
