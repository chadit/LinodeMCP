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

func TestLinodeRegionAvailabilityListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

	if tool.Name != "linode_region_availability_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_region_availability_list")
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
	if !strings.Contains(rawSchema, canRunKeyEnv) {
		t.Errorf("RawInputSchema missing key %v", canRunKeyEnv)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeRegionAvailabilityListToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/regions/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions/availability")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyRegion: regionUSEast, "plan": "g6-standard-1", statusAvailable: true}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
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

	if !strings.Contains(textContent.Text, `"region_availabilities"`) {
		t.Errorf("textContent.Text does not contain %v", `"region_availabilities"`)
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
	}

	if !strings.Contains(textContent.Text, "g6-standard-1") {
		t.Errorf("textContent.Text does not contain %v", "g6-standard-1")
	}
}

func TestLinodeRegionAvailabilityListToolApiFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
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

	if !strings.Contains(textContent.Text, "Failed to retrieve") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeRegionAvailabilityGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

	if tool.Name != "linode_region_availability_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_region_availability_get")
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
	for _, key := range []string{keyRegionID, canRunKeyEnv} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}

	if !strings.Contains(rawSchema, `"required"`) || !strings.Contains(rawSchema, keyRegionID) {
		t.Errorf("RawInputSchema does not mark %v required", keyRegionID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeRegionAvailabilityGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/regions/us-east/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions/us-east/availability")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		// The per-region availability route answers with a bare array, unlike
		// the paginated cross-region list.
		if err := json.NewEncoder(w).Encode([]map[string]any{{keyRegion: regionUSEast, "plan": "g6-standard-1", statusAvailable: true, keyNotInProto: valNotInProto}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

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

	if !strings.Contains(textContent.Text, `"region_availabilities"`) {
		t.Errorf("textContent.Text does not contain %v", `"region_availabilities"`)
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
	}

	if !strings.Contains(textContent.Text, "g6-standard-1") {
		t.Errorf("textContent.Text does not contain %v", "g6-standard-1")
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if strings.Contains(textContent.Text, "not_in_proto") {
		t.Errorf("textContent.Text unexpectedly contains dropped unknown field: %v", textContent.Text)
	}
}

func TestLinodeRegionAvailabilityGetToolInvalidRegionId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingRegion, args: map[string]any{}, want: errRegionIDNonEmpty},
		{name: caseEmpty, args: map[string]any{keyRegionID: ""}, want: errRegionIDNonEmpty},
		{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue}, want: errRegionInvalid},
		{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue}, want: errRegionInvalid},
		{name: caseDotTraversal, args: map[string]any{keyRegionID: pathTraversalValue}, want: errRegionInvalid},
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeRegionAvailabilityGetToolApiFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
