package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	managedCredentialIDParam        = "credential_id"
	managedCredentialID             = 9991
	managedCredentialLabel          = "prod-password-1"
	managedCredentialLastDecrypted  = "2018-01-01T00:01:01"
	managedCredentialPath           = "/managed/credentials/9991"
	errManagedCredentialIDPositive  = "credential_id must be a positive integer"
	managedCredentialTemporaryError = "temporary failure"
)

func TestLinodeManagedCredentialGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

	if tool.Name != "linode_managed_credential_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_managed_credential_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[managedCredentialIDParam]; !ok {
		t.Errorf("props missing key %v", managedCredentialIDParam)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}
}

func TestLinodeManagedCredentialGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedCredentialPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel, LastDecrypted: managedCredentialLastDecrypted}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})

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

	if !strings.Contains(textContent.Text, managedCredentialLabel) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialLabel)
	}

	if !strings.Contains(textContent.Text, managedCredentialLastDecrypted) {
		t.Errorf("textContent.Text does not contain %v", managedCredentialLastDecrypted)
	}
}

func TestLinodeManagedCredentialGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedCredentialPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedCredentialPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: managedCredentialTemporaryError}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_managed_credential_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_managed_credential_get")
	}
}

func TestLinodeManagedCredentialGetToolClientConfigurationError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{managedCredentialIDParam: managedCredentialID})

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
}

func TestLinodeManagedCredentialGetToolInvalidCredentialId(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingCredentialID, args: map[string]any{}},
		{name: caseZeroCredentialID, args: map[string]any{managedCredentialIDParam: 0}},
		{name: "fractional credential id", args: map[string]any{managedCredentialIDParam: 1.5}},
		{name: "string separator credential id", args: map[string]any{managedCredentialIDParam: pathSeparatorValue}},
		{name: "query separator credential id", args: map[string]any{managedCredentialIDParam: querySeparatorValue}},
		{name: caseTraversalCredentialID, args: map[string]any{managedCredentialIDParam: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "https://example.invalid", Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedCredentialGetTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errManagedCredentialIDPositive) {
				t.Errorf("error text %q does not contain %q", text.Text, errManagedCredentialIDPositive)
			}
		})
	}
}
