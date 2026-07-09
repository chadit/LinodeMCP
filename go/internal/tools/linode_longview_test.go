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
	longviewToolName        = "linode_longview_client_create"
	longviewClientLabel     = "client789"
	longviewClientAPIKey    = "longview-api-key-test"
	longviewClientInstall   = "longview-install-code-test"
	longviewClientCreatedAt = "2018-01-01T00:01:01"
	longviewClientUpdatedAt = "2018-01-02T00:01:01"
	caseNumber              = "number"
)

func TestLinodeLongviewClientCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeLongviewClientCreateTool(cfg)

	if tool.Name != longviewToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, longviewToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyLabel, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeLongviewClientCreateToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: "string", confirm: boolStringTrue, include: true},
		{name: caseNumber, confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)

			args := map[string]any{keyLabel: longviewClientLabel}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeLongviewClientCreateToolMissingLabelRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}},
		{name: caseBlank, args: map[string]any{keyConfirm: true, keyLabel: blankString}},
		{name: caseNumeric, args: map[string]any{keyConfirm: true, keyLabel: 789}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errLabelRequired) {
				t.Errorf("error text %q does not contain %q", text.Text, errLabelRequired)
			}
		})
	}
}

func TestLinodeLongviewClientCreateToolSuccess(t *testing.T) {
	t.Parallel()

	apiClient := linode.CreatedLongviewClient{
		APIKey:      longviewClientAPIKey,
		Apps:        linode.LongviewApps{Apache: true, MySQL: true},
		Created:     longviewClientCreatedAt,
		ID:          789,
		InstallCode: longviewClientInstall,
		Label:       longviewClientLabel,
		Updated:     longviewClientUpdatedAt,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != longviewClientsBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewClientsBasePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var got linode.CreateLongviewClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label != longviewClientLabel {
			t.Errorf("got.Label = %v, want %v", got.Label, longviewClientLabel)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(apiClient); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyLabel: longviewClientLabel})

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

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Longview client created successfully") {
		t.Errorf("textContent.Text does not contain %v", "Longview client created successfully")
	}

	if !strings.Contains(textContent.Text, longviewClientAPIKey) {
		t.Errorf("textContent.Text does not contain %v", longviewClientAPIKey)
	}

	if !strings.Contains(textContent.Text, longviewClientInstall) {
		t.Errorf("textContent.Text does not contain %v", longviewClientInstall)
	}

	assertLongviewClientCreateResponse(t, textContent.Text)
}

// assertLongviewClientCreateResponse checks the create proto shape: the warning
// is kept, the secret-bearing element lives under longview_client (not the legacy
// client key), and it carries the one-time install secret plus metadata.
func assertLongviewClientCreateResponse(t *testing.T, text string) {
	t.Helper()

	var body struct {
		Warning        string `json:"warning"`
		LongviewClient struct {
			APIKey      string `json:"api_key"`
			ID          int    `json:"id"`
			InstallCode string `json:"install_code"`
			Label       string `json:"label"`
		} `json:"longview_client"`
		Client any `json:"client"`
	}
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}

	if body.Warning == "" {
		t.Error("body.Warning is empty, want the one-time-secret reminder")
	}

	if body.Client != nil {
		t.Errorf("body.Client = %v, want nil (legacy client key renamed to longview_client)", body.Client)
	}

	if body.LongviewClient.APIKey != longviewClientAPIKey {
		t.Errorf("body.LongviewClient.APIKey = %q, want %q", body.LongviewClient.APIKey, longviewClientAPIKey)
	}

	if body.LongviewClient.InstallCode != longviewClientInstall {
		t.Errorf("body.LongviewClient.InstallCode = %q, want %q", body.LongviewClient.InstallCode, longviewClientInstall)
	}

	if body.LongviewClient.ID != 789 {
		t.Errorf("body.LongviewClient.ID = %d, want 789", body.LongviewClient.ID)
	}

	if body.LongviewClient.Label != longviewClientLabel {
		t.Errorf("body.LongviewClient.Label = %q, want %q", body.LongviewClient.Label, longviewClientLabel)
	}
}

func TestLinodeLongviewClientCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != longviewClientsBasePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, longviewClientsBasePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyLabel: longviewClientLabel})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_longview_client_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_longview_client_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
