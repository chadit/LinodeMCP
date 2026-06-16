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

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyManagedLinodeSettingsLinodeID    = "linode_id"
	managedLinodeSettingsGetToolName    = "linode_managed_linode_settings_get"
	managedLinodeSettingsToolIDValue    = 234
	managedLinodeSettingsOversizedID    = 9007199254740992.0
	managedLinodeSettingsToolPathValue  = "/managed/linode-settings/234"
	managedLinodeSettingsToolLabelValue = "linode123"
)

func TestLinodeManagedLinodeSettingsGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedLinodeSettingsGetTool(cfg)

	if tool.Name != managedLinodeSettingsGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedLinodeSettingsGetToolName)
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
	if _, ok := props[keyManagedLinodeSettingsLinodeID]; !ok {
		t.Errorf("props missing key %v", keyManagedLinodeSettingsLinodeID)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyManagedLinodeSettingsLinodeID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyManagedLinodeSettingsLinodeID)
	}
}

func TestLinodeManagedLinodeSettingsGetToolInvalidLinodeIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingLinodeID, args: map[string]any{}},
		{name: "zero linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: 0}},
		{name: "negative linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: -1}},
		{name: "string linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: "234"}},
		{name: "fractional linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: 234.5}},
		{name: "oversized linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsOversizedID}},
		{name: caseSlashLinodeID, args: map[string]any{keyManagedLinodeSettingsLinodeID: "234/235"}},
		{name: "query linode id", args: map[string]any{keyManagedLinodeSettingsLinodeID: "234?x=1"}},
		{name: caseTraversalLinodeID, args: map[string]any{keyManagedLinodeSettingsLinodeID: pathTraversalValue}},
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

			cfg := managedLinodeSettingsConfig(srv.URL)
			_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_id") {
				t.Errorf("error text %q does not contain %q", text.Text, "linode_id")
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedLinodeSettingsGetToolSuccess(t *testing.T) {
	t.Parallel()

	sshUser := keyGrantLinode
	sshPort := 22

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedLinodeSettingsToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsToolPathValue)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedLinodeSettings{
			ID:    managedLinodeSettingsToolIDValue,
			Label: managedLinodeSettingsToolLabelValue,
			Group: managedLinodeSettingsGroup,
			SSH: linode.ManagedLinodeSettingsSSH{
				Access: true,
				IP:     "203.0.113.1",
				Port:   &sshPort,
				User:   &sshUser,
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(managedLinodeSettingsConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsToolIDValue}))
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

	if !strings.Contains(textContent.Text, managedLinodeSettingsToolLabelValue) {
		t.Errorf("textContent.Text does not contain %v", managedLinodeSettingsToolLabelValue)
	}

	if !strings.Contains(textContent.Text, "203.0.113.1") {
		t.Errorf("textContent.Text does not contain %v", "203.0.113.1")
	}
}

func TestLinodeManagedLinodeSettingsGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedLinodeSettingsToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsToolPathValue)
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

	_, _, handler := tools.NewLinodeManagedLinodeSettingsGetTool(managedLinodeSettingsConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedLinodeSettingsLinodeID: managedLinodeSettingsToolIDValue}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_managed_linode_settings_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_managed_linode_settings_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func managedLinodeSettingsConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
