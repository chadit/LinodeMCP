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
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	managedContactsToolPath  = "/managed/contacts"
	managedContactsToolName  = "John Doe"
	managedContactsToolEmail = "john.doe@example.org"

	// Argument names and fixtures the rest of the package reads. They outlived
	// the per-tool files they were declared in, which went away when those
	// tools became generated.
	managedServiceLabelParam           = "label"
	managedServiceAddressParam         = "address"
	managedContactNameParam            = "name"
	managedLinodeSettingsUpdatePortKey = "port"
)

func TestLinodeManagedContactsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeManagedContactListTool(cfg)

	if tool.Name != "linode_managed_contact_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_contact_list")
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

func TestLinodeManagedContactsToolSuccess(t *testing.T) {
	t.Parallel()

	contacts := linode.PaginatedResponse[linode.ManagedContact]{
		Data: []linode.ManagedContact{{
			ID:    567,
			Name:  managedContactsToolName,
			Email: managedContactsToolEmail,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(contacts); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeManagedContactListTool(cfg)

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

	if !strings.Contains(textContent.Text, managedContactsToolName) {
		t.Errorf("textContent.Text does not contain %v", managedContactsToolName)
	}

	if !strings.Contains(textContent.Text, managedContactsToolEmail) {
		t.Errorf("textContent.Text does not contain %v", managedContactsToolEmail)
	}
}

func TestLinodeManagedContactsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
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
			_, _, handler := gentools.NewLinodeManagedContactListTool(cfg)
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

func TestLinodeManagedContactsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsToolPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsToolPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeManagedContactListTool(cfg)

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
