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
	lkeTierParam             = "tier"
	lkeTierSeparatorError    = "tier must not contain path separators"
	lkeVersionSeparatorError = "version must not contain path separators"
)

func TestLinodeLKETierVersionGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeLKETierVersionGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_lke_tier_version_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_lke_tier_version_get")
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
}

func TestLinodeLKETierVersionGetToolTestCase(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeLKETierVersionGetTool(cfg)

	for _, testCase := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing tier", args: map[string]any{databaseVersionParam: lkeVersion129}, want: "tier is required"},
		{name: "missing version", args: map[string]any{lkeTierParam: classStandard}, want: "version is required"},
		{name: "tier path separator", args: map[string]any{lkeTierParam: "standard/extra", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier query separator", args: map[string]any{lkeTierParam: "standard?debug=true", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier traversal", args: map[string]any{lkeTierParam: pathTraversalValue, databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "tier padded whitespace", args: map[string]any{lkeTierParam: " standard ", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "version path separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29/edge"}, want: lkeVersionSeparatorError},
		{name: "version query separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29?debug=true"}, want: lkeVersionSeparatorError},
		{name: "version traversal", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: pathTraversalValue}, want: lkeVersionSeparatorError},
		{name: "tier fragment separator", args: map[string]any{lkeTierParam: "standard#fragment", databaseVersionParam: lkeVersion129}, want: lkeTierSeparatorError},
		{name: "version fragment separator", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: "1.29#fragment"}, want: lkeVersionSeparatorError},
		{name: "version padded whitespace", args: map[string]any{lkeTierParam: classStandard, databaseVersionParam: " 1.29 "}, want: lkeVersionSeparatorError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, testCase.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeLKETierVersionGetToolSuccess(t *testing.T) {
	t.Parallel()

	tierVersion := linode.LKETierVersion{ID: lkeVersion129, Tier: classStandard}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/lke/tiers/standard/versions/1.29" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/lke/tiers/standard/versions/1.29")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(tierVersion); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeLKETierVersionGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{lkeTierParam: classStandard, databaseVersionParam: lkeVersion129})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, lkeVersion129) {
		t.Errorf("textContent.Text does not contain %v", lkeVersion129)
	}

	if !strings.Contains(textContent.Text, classStandard) {
		t.Errorf("textContent.Text does not contain %v", classStandard)
	}
}

func TestLinodeLKETierVersionGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
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
	_, _, srvHandler := tools.NewLinodeLKETierVersionGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{lkeTierParam: classStandard, databaseVersionParam: lkeVersion129})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve LKE tier version") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve LKE tier version")
	}
}
