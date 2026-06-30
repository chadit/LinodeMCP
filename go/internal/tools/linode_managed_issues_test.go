package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	managedIssueGetToolName    = "linode_managed_issue_get"
	managedIssueIDParam        = "issue_id"
	managedIssueIDValue        = 823
	managedIssueGetToolPath    = "/managed/issues/823"
	managedIssueOversizedID    = 9007199254740992.0
	managedIssueToolCreated    = "2018-01-01T00:01:01"
	managedIssuesToolPath      = "/managed/issues"
	managedIssuesToolLabel     = "Managed Issue opened!"
	managedIssuesToolEntityURL = "/support/tickets/98765"
)

func TestLinodeManagedIssueGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedIssueGetTool(cfg)

	if tool.Name != managedIssueGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedIssueGetToolName)
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
	if !strings.Contains(rawSchema, managedIssueIDParam) {
		t.Errorf("RawInputSchema missing key %v", managedIssueIDParam)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeManagedIssueGetToolInvalidIssueIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "missing issue id", args: map[string]any{}},
		{name: "zero issue id", args: map[string]any{managedIssueIDParam: 0}},
		{name: "negative issue id", args: map[string]any{managedIssueIDParam: -1}},
		{name: "string issue id", args: map[string]any{managedIssueIDParam: "823"}},
		{name: "fractional issue id", args: map[string]any{managedIssueIDParam: 823.5}},
		{name: "oversized issue id", args: map[string]any{managedIssueIDParam: managedIssueOversizedID}},
		{name: "slash issue id", args: map[string]any{managedIssueIDParam: pathSeparatorValue}},
		{name: "query issue id", args: map[string]any{managedIssueIDParam: querySeparatorValue}},
		{name: "traversal issue id", args: map[string]any{managedIssueIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			cfg := managedIssueConfig(srv.URL)
			_, _, handler := tools.NewLinodeManagedIssueGetTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "issue_id") {
				t.Errorf("error text %q does not contain %q", text.Text, "issue_id")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedIssueGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssueGetToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssueGetToolPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedIssue{
			ID:       managedIssueIDValue,
			Created:  managedIssueToolCreated,
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssuesToolLabel,
				Type:  "ticket",
				URL:   managedIssuesToolEntityURL,
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedIssueGetTool(managedIssueConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedIssueIDParam: managedIssueIDValue}))
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

	if !strings.Contains(textContent.Text, managedIssuesToolLabel) {
		t.Errorf("textContent.Text does not contain %v", managedIssuesToolLabel)
	}

	if !strings.Contains(textContent.Text, managedIssuesToolEntityURL) {
		t.Errorf("textContent.Text does not contain %v", managedIssuesToolEntityURL)
	}
}

func TestLinodeManagedIssueGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssueGetToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssueGetToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedIssueGetTool(managedIssueConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedIssueIDParam: managedIssueIDValue}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_managed_issue_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_managed_issue_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func managedIssueConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func TestLinodeManagedIssuesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedIssuesTool(cfg)

	if tool.Name != "linode_managed_issue_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_issue_list")
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
}

func TestLinodeManagedIssuesToolSuccess(t *testing.T) {
	t.Parallel()

	issues := linode.PaginatedResponse[linode.ManagedIssue]{
		Data: []linode.ManagedIssue{{
			ID:       823,
			Created:  "2018-01-01T00:01:01",
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssuesToolLabel,
				Type:  "ticket",
				URL:   managedIssuesToolEntityURL,
			},
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuesToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuesToolPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(issues); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, managedIssuesToolLabel) {
		t.Errorf("textContent.Text does not contain %v", managedIssuesToolLabel)
	}

	if !strings.Contains(textContent.Text, managedIssuesToolEntityURL) {
		t.Errorf("textContent.Text does not contain %v", managedIssuesToolEntityURL)
	}
}

func TestLinodeManagedIssuesToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

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

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeManagedIssuesToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuesToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuesToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedIssuesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}
