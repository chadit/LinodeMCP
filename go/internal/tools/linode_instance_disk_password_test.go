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

const keyDiskPassword = "password"

// TestLinodeInstanceDiskPasswordResetTool verifies the instance disk password reset tool
// registers correctly, validates confirm, and resets disk root passwords.
func TestLinodeInstanceDiskPasswordResetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceDiskPasswordResetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_disk_password_reset" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_disk_password_reset")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyDiskID]; !ok {
		t.Errorf("props missing key %v", keyDiskID)
	}

	if _, ok := props[keyDiskPassword]; !ok {
		t.Errorf("props missing key %v", keyDiskPassword)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceDiskPasswordResetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceDiskPasswordResetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: pathQueryValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errLinodeIDRequired},
		{name: "missing disk identifier", args: map[string]any{keyLinodeID: float64(123), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseSlash, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathSeparatorValue, keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseQuery, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathQueryValue, keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseDotTraversal, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathTraversalValue, keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "missing password", args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: managedCredentialsToolPasswordReq},
		{name: "weak password", args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: "weak", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: "root_pass must be at least 12 characters"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodeInstanceDiskPasswordResetToolClientErrorMapsToToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/disks/10/password" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/10/password")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid password"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceDiskPasswordResetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to reset password") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to reset password")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "invalid password") {
		t.Errorf("error text %q does not contain %q", text.Text, "invalid password")
	}
}

func TestLinodeInstanceDiskPasswordResetToolSuccessfulPasswordReset(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/disks/10/password" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/10/password")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if body[keyDiskPassword] != rootPassStrong {
			t.Errorf("body[keyDiskPassword] = %v, want %v", body[keyDiskPassword], rootPassStrong)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceDiskPasswordResetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "Password reset") {
		t.Errorf("textContent.Text does not contain %v", "Password reset")
	}

	if !strings.Contains(textContent.Text, "10") {
		t.Errorf("textContent.Text does not contain %v", "10")
	}
}
