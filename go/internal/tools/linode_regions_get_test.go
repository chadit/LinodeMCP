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

func TestLinodeRegionGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeRegionGetTool(cfg)

	if tool.Name != "linode_region_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_region_get")
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

	if _, ok := tool.InputSchema.Properties[keyRegionID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyRegionID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyRegionID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyRegionID)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}
}

func TestLinodeRegionGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/regions/us-east" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions/us-east")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: regionUSEast, keyLabel: regionLabelNewark, "country": countryUS, keyStatus: statusOK}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))
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

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if !strings.Contains(textContent.Text, regionLabelNewark) {
		t.Errorf("textContent.Text does not contain %v", regionLabelNewark)
	}
}

func TestLinodeRegionGetToolApiFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_region_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_region_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeRegionGetToolInvalidRegionId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeRegionGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingRegion, args: map[string]any{}, want: keyRegionID + " must be a non-empty string"},
		{name: caseEmpty, args: map[string]any{keyRegionID: ""}, want: keyRegionID + " must be a non-empty string"},
		{name: caseNumber, args: map[string]any{keyRegionID: 123}, want: keyRegionID + " must be a non-empty string"},
		{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue}, want: errRegionIDSlug},
		{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue}, want: errRegionIDSlug},
		{name: "path traversal", args: map[string]any{keyRegionID: pathTraversalValue}, want: errRegionIDSlug},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}
