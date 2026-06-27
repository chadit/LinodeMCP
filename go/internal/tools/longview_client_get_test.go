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

func TestLinodeLongviewClientGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeLongviewClientGetTool(cfg)

	if tool.Name != "linode_longview_client_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_longview_client_get")
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

	if !strings.Contains(string(tool.RawInputSchema), keyLongviewClientID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLongviewClientID)
	}

	if strings.Contains(string(tool.RawInputSchema), keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeLongviewClientGetToolSuccessRedactsSecretFields(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID:              789,
			keyLabel:               longviewClientLabelFixture,
			keyLongviewAPIKey:      "secret-api-key",
			keyLongviewInstallCode: "secret-install-code",
			keyLongviewApps: map[string]bool{
				keyLongviewAppApache: true,
				databaseEngineName:   true,
				keyLongviewAppNginx:  false,
			},
			keyCreated: longviewClientCreatedFixture,
			keyUpdated: longviewClientUpdatedFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyLongviewClientID: float64(789)})

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

	if !strings.Contains(textContent.Text, longviewClientLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", longviewClientLabelFixture)
	}

	if !strings.Contains(textContent.Text, keyLongviewAppApache) {
		t.Errorf("textContent.Text does not contain %v", keyLongviewAppApache)
	}

	if strings.Contains(textContent.Text, "secret-api-key") {
		t.Errorf("textContent.Text should not contain %v", "secret-api-key")
	}

	if strings.Contains(textContent.Text, "secret-install-code") {
		t.Errorf("textContent.Text should not contain %v", "secret-install-code")
	}
}

func TestLinodeLongviewClientGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyLongviewClientID: float64(789)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_longview_client_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_longview_client_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errTemporaryFailure) {
		t.Errorf("error text %q does not contain %q", text.Text, errTemporaryFailure)
	}
}

func TestLinodeLongviewClientGetToolInvalidLongviewClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: "client_id is required"},
		{name: caseString, args: map[string]any{keyLongviewClientID: idAbc123}, want: "client_id must be an integer"},
		{name: caseZero, args: map[string]any{keyLongviewClientID: float64(0)}, want: "client_id must be an integer"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientGetTool(cfg)
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLongviewClientGetOmitsSecretFields(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(linode.LongviewClient{ID: 789, Label: longviewClientLabelFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(string(payload), keyLongviewAPIKey) {
		t.Errorf("string(payload) should not contain %v", keyLongviewAPIKey)
	}

	if strings.Contains(string(payload), keyLongviewInstallCode) {
		t.Errorf("string(payload) should not contain %v", keyLongviewInstallCode)
	}
}
