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
	keyContactID              = "contact_id"
	managedContactGetToolName = "linode_managed_contact_get"
	managedContactIDValue     = 174
	managedContactOversizedID = 9007199254740992.0
	managedContactPathValue   = "/managed/contacts/174"
	managedContactEmailValue  = "john.doe@example.org"
)

func TestLinodeManagedContactGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedContactGetTool(cfg)

	if tool.Name != managedContactGetToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedContactGetToolName)
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
	if _, ok := props[keyContactID]; !ok {
		t.Errorf("props missing key %v", keyContactID)
	}

	if _, ok := props[keyConfirm]; ok {
		t.Errorf("props has unexpected key %v", keyConfirm)
	}

	if !slices.Contains(tool.InputSchema.Required, keyContactID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyContactID)
	}
}

func TestLinodeManagedContactGetToolInvalidContactIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "missing contact id", args: map[string]any{}},
		{name: caseZeroContactID, args: map[string]any{keyContactID: 0}},
		{name: "negative contact id", args: map[string]any{keyContactID: -1}},
		{name: "string contact id", args: map[string]any{keyContactID: "174"}},
		{name: "fractional contact id", args: map[string]any{keyContactID: 174.5}},
		{name: "oversized contact id", args: map[string]any{keyContactID: managedContactOversizedID}},
		{name: "slash contact id", args: map[string]any{keyContactID: "174/175"}},
		{name: "query contact id", args: map[string]any{keyContactID: "174?x=1"}},
		{name: "traversal contact id", args: map[string]any{keyContactID: pathTraversalValue}},
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

			cfg := managedContactConfig(srv.URL)
			_, _, handler := tools.NewLinodeManagedContactGetTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "contact_id") {
				t.Errorf("error text %q does not contain %q", text.Text, "contact_id")
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedContactGetToolSuccess(t *testing.T) {
	t.Parallel()

	phone := "123-456-7890"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactPathValue)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedContact{
			ID:    managedContactIDValue,
			Name:  "John Doe",
			Email: managedContactEmailValue,
			Phone: linode.ManagedContactPhone{Primary: &phone},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedContactGetTool(managedContactConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyContactID: managedContactIDValue}))
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

	if !strings.Contains(textContent.Text, managedContactEmailValue) {
		t.Errorf("textContent.Text does not contain %v", managedContactEmailValue)
	}
}

func TestLinodeManagedContactGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactPathValue)
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

	_, _, handler := tools.NewLinodeManagedContactGetTool(managedContactConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyContactID: managedContactIDValue}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_managed_contact_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_managed_contact_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func managedContactConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
