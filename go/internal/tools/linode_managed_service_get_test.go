package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyManagedServiceID          = "service_id"
	managedServiceGetToolName    = "linode_managed_service_get"
	managedServiceToolIDValue    = 9944
	managedServiceToolPathValue  = "/managed/services/9944"
	managedServiceToolLabelValue = "prod-1"
	managedServiceToolAddress    = "https://example.org"
	managedServiceOversizedID    = 9007199254740992.0
)

func TestLinodeManagedServiceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedServiceGetTool(cfg)

	if tool.Name != managedServiceGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedServiceGetToolName)
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

	props := tool.InputSchema.Properties
	if _, ok := props[keyManagedServiceID]; !ok {
		t.Errorf("props missing key %v", keyManagedServiceID)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyManagedServiceID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyManagedServiceID)
	}
}

func TestLinodeManagedServiceGetToolInvalidServiceIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingServiceID, args: map[string]any{}},
		{name: caseZeroServiceID, args: map[string]any{keyManagedServiceID: 0}},
		{name: caseNegativeServiceID, args: map[string]any{keyManagedServiceID: -1}},
		{name: caseStringServiceID, args: map[string]any{keyManagedServiceID: "9944"}},
		{name: caseFractionalServiceID, args: map[string]any{keyManagedServiceID: 9944.5}},
		{name: caseOversizedServiceID, args: map[string]any{keyManagedServiceID: managedServiceOversizedID}},
		{name: caseSlashServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceSlashID}},
		{name: caseQueryServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceQueryID}},
		{name: caseTraversalServiceID, args: map[string]any{keyManagedServiceID: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "service_id") {
				t.Errorf("error text %q does not contain %q", text.Text, "service_id")
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedService{
			ID:          managedServiceToolIDValue,
			Label:       managedServiceToolLabelValue,
			ServiceType: managedServiceTypeURL,
			Status:      "ok",
			Address:     managedServiceToolAddress,
			Timeout:     30,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue}))
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

	if !strings.Contains(textContent.Text, managedServiceToolLabelValue) {
		t.Errorf("textContent.Text does not contain %v", managedServiceToolLabelValue)
	}

	if !strings.Contains(textContent.Text, managedServiceToolAddress) {
		t.Errorf("textContent.Text does not contain %v", managedServiceToolAddress)
	}
}

func TestLinodeManagedServiceGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceGetTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_managed_service_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_managed_service_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func managedServiceConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
