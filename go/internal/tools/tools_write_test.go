package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// validTestSSHKey is a fake but valid-looking SSH key for testing purposes.
// It has the correct prefix and length to pass validation.
const validTestSSHKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 user@example.com"

// End-to-end verification of the SSH key creation workflow.
func TestLinodeSSHKeyCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_sshkey_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_sshkey_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, monitorAlertDefinitionLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", managedServiceLabelParam)
	}

	if !strings.Contains(raw, "ssh_key") {
		t.Errorf("tool.RawInputSchema missing key %v", "ssh_key")
	}

	if !strings.Contains(raw, canRunKeyEnv) {
		t.Errorf("tool.RawInputSchema missing key %v", canRunKeyEnv)
	}
}

func TestLinodeSSHKeyCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeSSHKeyCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLabel, args: map[string]any{keySSHKey: validTestSSHKey, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "missing ssh key", args: map[string]any{keyLabel: keyNameTest, keyConfirm: true}, wantContains: "ssh_key is required"},
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

func TestLinodeSSHKeyCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	createdKey := linode.SSHKey{
		ID:    123,
		Label: keyNameTest,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/sshkeys" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/sshkeys")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(createdKey); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeSSHKeyCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   keyNameTest,
		keySSHKey:  validTestSSHKey,
		keyConfirm: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, keyNameTest) {
		t.Errorf("textContent.Text does not contain %v", keyNameTest)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the SSH key update workflow.
func TestLinodeSSHKeyUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_sshkey_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_sshkey_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keySSHKeyID) {
		t.Errorf("tool.RawInputSchema missing key %v", keySSHKeyID)
	}

	if !strings.Contains(raw, keyLabel) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeSSHKeyUpdateToolConfirmMustBeLiteralTrueBeforeClientCall(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeSSHKeyUpdateTool(successCfg)

	tests := []struct {
		name    string
		confirm any
		set     bool
	}{
		{name: "missing"},
		{name: "false", confirm: false, set: true},
		{name: caseStringConfirmRejected, confirm: boolStringTrue, set: true},
		{name: caseNumericConfirmRejected, confirm: 1, set: true},
	}

	for _, tt := range tests {
		args := map[string]any{keySSHKeyID: float64(123), keyLabel: keyNameTest}
		if tt.set {
			args[keyConfirm] = tt.confirm
		}

		req := createRequestWithArgs(t, args)

		result, err := successHandler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
			t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
		}

		if callCount.Load() != 0 {
			t.Errorf("callCount.Load() = %v, want zero", callCount.Load())
		}
	}
}

func TestLinodeSSHKeyUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeSSHKeyUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing sshkey id", args: map[string]any{keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "zero sshkey id", args: map[string]any{keySSHKeyID: float64(0), keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "negative sshkey id", args: map[string]any{keySSHKeyID: float64(-1), keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id with slash", args: map[string]any{keySSHKeyID: "123/45", keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id with query", args: map[string]any{keySSHKeyID: "123?x=1", keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: "malformed sshkey id traversal", args: map[string]any{keySSHKeyID: pathTraversalValue, keyLabel: keyNameTest, keyConfirm: true}, wantContains: errSSHKeyIDPositive},
		{name: caseMissingLabel, args: map[string]any{keySSHKeyID: float64(123), keyConfirm: true}, wantContains: errLabelRequired},
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

func TestLinodeSSHKeyUpdateToolApiFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileSshkeys123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSshkeys123)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	failureCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, failureHandler := tools.NewLinodeSSHKeyUpdateTool(failureCfg)

	req := createRequestWithArgs(t, map[string]any{
		keySSHKeyID: float64(123),
		keyLabel:    keyNameTest,
		keyConfirm:  true,
	})

	result, err := failureHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "failed to change label") {
		t.Errorf("error text %q does not contain %q", text.Text, "failed to change label")
	}
}

func TestLinodeSSHKeyUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	updatedKey := linode.SSHKey{
		ID:    123,
		Label: keyNameTest,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileSshkeys123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSshkeys123)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var req linode.UpdateSSHKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if req.Label != keyNameTest {
			t.Errorf("req.Label = %v, want %v", req.Label, keyNameTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updatedKey); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeSSHKeyUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keySSHKeyID: float64(123),
		keyLabel:    keyNameTest,
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}

	if !strings.Contains(textContent.Text, keyNameTest) {
		t.Errorf("textContent.Text does not contain %v", keyNameTest)
	}
}

// End-to-end verification of the SSH key deletion workflow.
func TestLinodeSSHKeyDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_sshkey_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_sshkey_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keySSHKeyID) {
		t.Errorf("tool.RawInputSchema missing key %v", keySSHKeyID)
	}
}

func TestLinodeSSHKeyDeleteToolMissingSshkeyId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeSSHKeyDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyConfirmedDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "ssh_key_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "ssh_key_id is required")
	}
}

func TestLinodeSSHKeyDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileSshkeys123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSshkeys123)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeSSHKeyDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// End-to-end verification of the instance boot workflow.
func TestLinodeInstanceBootToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceBootTool(cfg)

	t.Parallel()

	if tool.Name != toolInstanceBoot {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolInstanceBoot)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "instance_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "instance_id")
	}

	if !strings.Contains(rawSchema, "config_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "config_id")
	}
}

func TestLinodeInstanceBootToolMissingInstanceId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceBootTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "instance_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "instance_id is required")
	}
}

func TestLinodeInstanceBootToolSuccessfulBoot(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/boot" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/boot")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceBootTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "boot initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "boot initiated successfully")
	}
}

// End-to-end verification of the instance reboot workflow.
func TestLinodeInstanceRebootToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceRebootTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_reboot" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_reboot")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "instance_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "instance_id")
	}

	if !strings.Contains(rawSchema, "config_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "config_id")
	}
}

func TestLinodeInstanceRebootToolMissingInstanceId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceRebootTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "instance_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "instance_id is required")
	}
}

func TestLinodeInstanceRebootToolSuccessfulReboot(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/reboot" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/reboot")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceRebootTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "reboot initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "reboot initiated successfully")
	}
}

// End-to-end verification of the instance shutdown workflow.
func TestLinodeInstanceShutdownToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	t.Parallel()

	if tool.Name != tcLinodeInstanceShutdown {
		t.Errorf("tool.Name = %v, want %v", tool.Name, tcLinodeInstanceShutdown)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "instance_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "instance_id")
	}
}

func TestLinodeInstanceShutdownToolMissingInstanceId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceShutdownTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "instance_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "instance_id is required")
	}
}

func TestLinodeInstanceShutdownToolSuccessfulShutdown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/shutdown" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/shutdown")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceShutdownTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: float64(123), keyConfirm: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "shutdown initiated successfully") {
		t.Errorf("textContent.Text does not contain %v", "shutdown initiated successfully")
	}
}

// End-to-end verification of the instance creation workflow under the current
// Linode Interfaces generation. The wire shape matches BIMHelperScripts
// linode_add_network at api-common.sh:378 exactly.
func TestLinodeInstanceCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keySupportTicketRegion) {
		t.Errorf("tool.RawInputSchema missing key %v", keySupportTicketRegion)
	}

	if !strings.Contains(rawSchema, "type") {
		t.Errorf("tool.RawInputSchema missing key %v", "type")
	}

	if !strings.Contains(rawSchema, monitorAlertDefinitionLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", managedServiceLabelParam)
	}

	if !strings.Contains(rawSchema, "image") {
		t.Errorf("tool.RawInputSchema missing key %v", "image")
	}

	if !strings.Contains(rawSchema, keyFirewallID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyFirewallID)
	}

	if !strings.Contains(rawSchema, "route_ipv4") {
		t.Errorf("tool.RawInputSchema missing key %v", "route_ipv4")
	}

	if !strings.Contains(rawSchema, "route_ipv6") {
		t.Errorf("tool.RawInputSchema missing key %v", "route_ipv6")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}

	// private_ip is replaced by interface-level VPC routing in the current
	// API and must not be a tool parameter.
	props := tool.InputSchema.Properties
	if _, ok := props["private_ip"]; ok {
		t.Errorf("props has unexpected key %v", "private_ip")
	}

	// firewall_id is a hard requirement of the current API.
	if !strings.Contains(rawSchema, keyFirewallID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyFirewallID)
	}
}

func TestLinodeInstanceCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyRegion: regionUSEast, keyType: typeG6Nanode1, keyFirewallID: 12345},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyType: typeG6Nanode1, keyFirewallID: 12345, keyConfirm: true},
			wantContains: errRegionRequired,
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyRegion: regionUSEast, keyFirewallID: 12345, keyConfirm: true},
			wantContains: errTypeRequired,
		},
		{
			name:         caseMissingFirewallID,
			args:         map[string]any{keyRegion: regionUSEast, keyType: typeG6Nanode1, keyConfirm: true},
			wantContains: errFirewallIDRequired,
		},
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

func TestLinodeInstanceCreateToolBodyShapeMatchesBIMHelperScriptsReference(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Instance{ID: 456, Label: "web-server", Region: regionUSEast, Status: "provisioning"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:     regionUSEast,
		keyType:       typeG6Nanode1,
		keyLabel:      "web-server",
		keyFirewallID: 12345,
		keyRootPass:   rootPassStrong,
		keyConfirm:    true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	// Top-level wire fields per linode_add_network at api-common.sh:378.
	if !reflect.DeepEqual(capturedBody["interface_generation"], monitorAlertDefinitionToolServiceType) {
		t.Errorf("got %v, want %v", capturedBody["interface_generation"], monitorAlertDefinitionToolServiceType)
	}

	interfaces, interfacesOK := capturedBody["interfaces"].([]any)
	if !interfacesOK {
		t.Fatal("interfacesOK = false, want true")
	}

	if len(interfaces) != 1 {
		t.Fatalf("len(interfaces) = %d, want %d", len(interfaces), 1)
	}

	iface, ifaceOK := interfaces[0].(map[string]any)
	if !ifaceOK {
		t.Fatal("ifaceOK = false, want true")
	}

	// public: {} is sent so the API uses defaults; no nested fields under it.
	if !reflect.DeepEqual(iface["public"], map[string]any{}) {
		t.Errorf("iface[public] = %v, want empty", iface["public"])
	}

	// default_route: both families default to true.
	if !reflect.DeepEqual(iface["default_route"], map[string]any{"ipv4": true, tcIpv6: true}) {
		t.Errorf("iface[default_route] = %v, want %v", iface["default_route"], map[string]any{"ipv4": true, tcIpv6: true})
	}

	// firewall_id at interface level (not top-level).
	if iface["firewall_id"] != float64(12345) {
		t.Errorf("value = %v, want %v", iface["firewall_id"], float64(12345))
	}

	if _, ok := capturedBody["firewall_id"]; ok {
		t.Errorf("capturedBody has unexpected key %v", "firewall_id")
	}

	textContent, textOK := result.Content[0].(mcp.TextContent)
	if !textOK {
		t.Fatal("textOK = false, want true")
	}

	if !strings.Contains(textContent.Text, "web-server") {
		t.Errorf("textContent.Text does not contain %v", "web-server")
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

func TestLinodeInstanceCreateToolSendsAuthorizedKeysAndBooted(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Instance{ID: 456, Region: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

	const exampleKey = "ssh-ed25519 AAAAexamplekey"

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:         regionUSEast,
		keyType:           typeG6Nanode1,
		keyFirewallID:     12345,
		"authorized_keys": []any{exampleKey},
		"booted":          false,
		keyConfirm:        true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.IsError {
		t.Fatalf("result = %v, want success", result)
	}

	if !reflect.DeepEqual(capturedBody["authorized_keys"], []any{exampleKey}) {
		t.Errorf("capturedBody[authorized_keys] = %v, want %v", capturedBody["authorized_keys"], []any{exampleKey})
	}

	if capturedBody["booted"] != false {
		t.Errorf("capturedBody[booted] = %v, want %v", capturedBody["booted"], false)
	}
}

func TestLinodeInstanceCreateToolRouteFlagsOmitIpv4KeyWhenFalse(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Instance{ID: 789, Label: "v6-only", Region: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:     regionUSEast,
		keyType:       typeG6Nanode1,
		keyFirewallID: 12345,
		keyRootPass:   rootPassStrong,
		"route_ipv4":  false,
		"route_ipv6":  true,
		keyConfirm:    true,
	})

	_, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	interfaces, interfacesOK := capturedBody["interfaces"].([]any)
	if !interfacesOK {
		t.Fatal("interfacesOK = false, want true")
	}

	iface, ifaceOK := interfaces[0].(map[string]any)
	if !ifaceOK {
		t.Fatal("ifaceOK = false, want true")
	}

	route, routeOK := iface["default_route"].(map[string]any)
	if !routeOK {
		t.Fatal("routeOK = false, want true")
	}

	// The wire shape must omit the ipv4 key entirely when false, not send
	// "ipv4": false. The API treats absence as "not the default route" for
	// that family.
	_, hasIPv4 := route["ipv4"]
	if hasIPv4 {
		t.Error("hasIPv4 = true, want false")
	}

	if !reflect.DeepEqual(route[tcIpv6], true) {
		t.Errorf("got %v, want %v", route[tcIpv6], true)
	}
}

// Instance GET response parsing under the current Interfaces generation must
// surface interface_generation and interfaces[] on the returned struct.
func TestLinodeInstanceGetParsesInterfaces(t *testing.T) {
	t.Parallel()

	firewallID := 12345
	respBody := linode.Instance{
		ID:                  321,
		Label:               firewallDeviceLabelFixture,
		Status:              statusRunning,
		Region:              regionUSEast,
		InterfaceGeneration: "linode",
		Interfaces: []linode.InstanceInterface{
			{
				ID:           1,
				Public:       &linode.InterfacePublicConfig{},
				DefaultRoute: &linode.InterfaceDefaultRoute{IPv4: true, IPv6: true},
				FirewallID:   &firewallID,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/321" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/321")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(respBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: "321"})

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

	textContent, textOK := result.Content[0].(mcp.TextContent)
	if !textOK {
		t.Fatal("textOK = false, want true")
	}

	// Parse the JSON response and assert structurally so the test does not
	// depend on the marshaler's whitespace choices. The GET handler returns
	// the Instance unwrapped at the top level.
	var parsed linode.Instance

	if err := json.Unmarshal([]byte(textContent.Text), &parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.InterfaceGeneration != monitorAlertDefinitionToolServiceType {
		t.Errorf("parsed.InterfaceGeneration = %v, want %v", parsed.InterfaceGeneration, monitorAlertDefinitionToolServiceType)
	}

	if len(parsed.Interfaces) != 1 {
		t.Fatalf("len(parsed.Interfaces) = %d, want %d", len(parsed.Interfaces), 1)
	}

	if parsed.Interfaces[0].ID != 1 {
		t.Errorf("parsed.Interfaces[0].ID = %v, want %v", parsed.Interfaces[0].ID, 1)
	}

	if parsed.Interfaces[0].FirewallID == nil {
		t.Fatal("parsed.Interfaces[0].FirewallID is nil")
	}

	if *parsed.Interfaces[0].FirewallID != 12345 {
		t.Errorf("*parsed.Interfaces[0].FirewallID = %v, want %v", *parsed.Interfaces[0].FirewallID, 12345)
	}
}

// End-to-end verification of the instance deletion workflow.
func TestLinodeInstanceDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	t.Parallel()

	if tool.Name != canRunDestroyTool {
		t.Errorf("tool.Name = %v, want %v", tool.Name, canRunDestroyTool)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "instance_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "instance_id")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyInstanceID: float64(123)},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingInstanceID,
			args:         map[string]any{keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: "instance_id is required",
		},
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

func TestLinodeInstanceDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != instanceGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceGetPath)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyConfirm:    true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

func TestLinodeInstanceDeleteToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, _ := tools.NewLinodeInstanceDeleteTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), "dry_run") {
		t.Errorf("tool.RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeInstanceDeleteToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	instanceBody := `{"id":456,"label":"web-test","type":"g6-standard-1","region":"us-east","status":"running"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/linode/instances/456" {
			_, _ = w.Write([]byte(instanceBody))

			return
		}

		// The Tier A dependency walk also fetches volumes, IPs,
		// firewalls, and the type. An empty body decodes to zero
		// dependencies, keeping this subtest on the no-mutation and
		// preview-shape contract; the rich walk has its own test.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeInstanceDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(456),
		keyDryRun:     true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], canRunDestroyTool) {
		t.Errorf("got %v, want %v", body["tool"], canRunDestroyTool)
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances/456") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances/456")
	}

	state, stateIsObject := body["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	if state[keyBetaID] != float64(456) {
		t.Errorf("value = %v, want %v", state[keyBetaID], float64(456))
	}

	if !reflect.DeepEqual(state[keyLabel], "web-test") {
		t.Errorf("state[keyLabel] = %v, want %v", state[keyLabel], "web-test")
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeInstanceDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":789,"label":"no-confirm","status":"running"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeInstanceDeleteTool(dryRunCfg)

	// Intentionally omit confirm; the dry-run path must not gate on it.
	req := createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(789),
		keyDryRun:     true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeInstanceDeleteToolDryRunStillValidatesInstanceId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "instance_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "instance_id is required")
	}
}

// End-to-end verification of the instance resize workflow.
func TestLinodeInstanceResizeToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_resize" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_resize")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "instance_id") {
		t.Errorf("tool.RawInputSchema missing key %v", "instance_id")
	}

	if !strings.Contains(rawSchema, "type") {
		t.Errorf("tool.RawInputSchema missing key %v", "type")
	}

	if !strings.Contains(rawSchema, "confirm") {
		t.Errorf("tool.RawInputSchema missing key %v", "confirm")
	}
}

func TestLinodeInstanceResizeToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyInstanceID: float64(123), keyType: typeG6Standard1},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         "missing instance id",
			args:         map[string]any{keyType: typeG6Standard1, keyConfirm: true},
			wantContains: "instance_id is required",
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyInstanceID: float64(123), keyConfirm: true},
			wantContains: errTypeRequired,
		},
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

func TestLinodeInstanceResizeToolSuccessfulResize(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/instances/123/resize" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/resize")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeInstanceResizeTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyType:       typeG6Standard1,
		keyConfirm:    true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "resize") {
		t.Errorf("textContent.Text does not contain %v", "resize")
	}

	if !strings.Contains(textContent.Text, typeG6Standard1) {
		t.Errorf("textContent.Text does not contain %v", typeG6Standard1)
	}
}

// End-to-end verification of the firewall creation workflow.
func TestLinodeFirewallCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_firewall_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, monitorAlertDefinitionLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", monitorAlertDefinitionLabelParam)
	}

	if !strings.Contains(raw, "inbound_policy") {
		t.Errorf("tool.RawInputSchema missing key %v", "inbound_policy")
	}

	if !strings.Contains(raw, "outbound_policy") {
		t.Errorf("tool.RawInputSchema missing key %v", "outbound_policy")
	}
}

func TestLinodeFirewallCreateToolCaseMissingLabel(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallCreateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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
}

func TestLinodeFirewallCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     789,
		Label:  labelWebFirewall,
		Status: statusEnabled,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/networking/firewalls" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeFirewallCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:         labelWebFirewall,
		"inbound_policy": "DROP",
		keyConfirm:       true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, labelWebFirewall) {
		t.Errorf("textContent.Text does not contain %v", labelWebFirewall)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the firewall update workflow.
func TestLinodeFirewallUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_firewall_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyFirewallID, monitorAlertDefinitionLabelParam, keyStatus} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallUpdateToolMissingFirewallId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "firewall_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "firewall_id is required")
	}
}

func TestLinodeFirewallUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     789,
		Label:  "updated-firewall",
		Status: statusEnabled,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNetworkingFirewalls789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls789)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeFirewallUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyLabel:      "updated-firewall",
		keyConfirm:    true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// End-to-end verification of the firewall deletion workflow.
func TestLinodeFirewallDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_firewall_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_firewall_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyFirewallID, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeFirewallDeleteToolCaseRequiresConfirm(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(789)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeFirewallDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNetworkingFirewalls789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNetworkingFirewalls789)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeFirewallDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyConfirm:    true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// Dry-run coverage for firewall delete. Kept in a sibling function so
// the main test's subtest count stays under maintidx's threshold.
func TestLinodeFirewallDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeFirewallDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeFirewallDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	firewallBody := `{"id":789,"label":"prod-fw","status":"enabled"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == tcNetworkingFirewalls789 {
			_, _ = w.Write([]byte(firewallBody))

			return
		}

		// The Tier A walk also lists firewall devices; an empty page
		// keeps this subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyDryRun:     true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcNetworkingFirewalls789) {
		t.Errorf("got %v, want %v", would["path"], tcNetworkingFirewalls789)
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeFirewallDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":789,"label":"prod-fw"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyDryRun:     true,
	})

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
}

func TestLinodeFirewallDeleteToolDryRunStillValidatesFirewallId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "firewall_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "firewall_id is required")
	}
}

// End-to-end verification of the domain import workflow.
func TestLinodeDomainImportToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainImportTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_import" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_import")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomain, keyRemoteNameserver, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainImportToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainImportTool(cfg)

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: "numeric", value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run("confirm "+tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyDomain: domainExample, keyRemoteNameserver: remoteNameserverExample}
			if tt.set {
				args[keyConfirm] = tt.value
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeDomainImportToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainImportTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomain, args: map[string]any{keyRemoteNameserver: remoteNameserverExample, keyConfirm: true}, wantContains: errDomainRequired},
		{name: "missing remote nameserver", args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: "remote_nameserver is required"},
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

func TestLinodeDomainImportToolSuccessfulImport(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{
		ID:     111,
		Domain: domainExample,
		Type:   keyMaster,
		Status: statusActive,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/import" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/import")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["domain"], domainExample) {
			t.Errorf("got %v, want %v", body["domain"], domainExample)
		}

		if !reflect.DeepEqual(body[keyRemoteNameserver], remoteNameserverExample) {
			t.Errorf("body[keyRemoteNameserver] = %v, want %v", body[keyRemoteNameserver], remoteNameserverExample)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(domain); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainImportTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomain:           domainExample,
		keyRemoteNameserver: remoteNameserverExample,
		keyConfirm:          true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, domainExample) {
		t.Errorf("textContent.Text does not contain %v", domainExample)
	}

	if !strings.Contains(textContent.Text, "imported successfully") {
		t.Errorf("textContent.Text does not contain %v", "imported successfully")
	}
}

func TestLinodeDomainImportToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"invalid domain"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errorHandler := tools.NewLinodeDomainImportTool(errorCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomain:           domainExample,
		keyRemoteNameserver: remoteNameserverExample,
		keyConfirm:          true,
	})

	result, err := errorHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to import domain") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to import domain")
	}
}

// End-to-end verification of the domain clone workflow.
func TestLinodeDomainCloneToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainCloneTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_clone" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_clone")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keyDomain, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainCloneToolConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainCloneTool(cfg)

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: "numeric", value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run("confirm "+tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyDomainID: float64(111), keyDomain: domainExample}
			if tt.set {
				args[keyConfirm] = tt.value
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeDomainCloneToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainCloneTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomainID, args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: "zero domain id", args: map[string]any{keyDomainID: 0, keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: "negative domain id", args: map[string]any{keyDomainID: -1, keyDomain: domainExample, keyConfirm: true}, wantContains: errDomainIDPositive},
		{name: caseMissingDomain, args: map[string]any{keyDomainID: float64(111), keyConfirm: true}, wantContains: errDomainRequired},
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

func TestLinodeDomainCloneToolSuccessfulClone(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{ID: 222, Domain: domainExample, Type: keyMaster, Status: statusActive}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111/clone" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111/clone")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyDomain], domainExample) {
			t.Errorf("body[keyDomain] = %v, want %v", body[keyDomain], domainExample)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(domain); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainCloneTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111), keyDomain: domainExample, keyConfirm: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, domainExample) {
		t.Errorf("textContent.Text does not contain %v", domainExample)
	}

	if !strings.Contains(textContent.Text, "cloned") {
		t.Errorf("textContent.Text does not contain %v", "cloned")
	}
}

func TestLinodeDomainCloneToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"invalid domain"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errorHandler := tools.NewLinodeDomainCloneTool(errorCfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111), keyDomain: domainExample, keyConfirm: true})

	result, err := errorHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to clone domain") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to clone domain")
	}
}

// End-to-end verification of the domain creation workflow.
func TestLinodeDomainCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomain, keyType, keySoaEmail} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingDomain, args: map[string]any{keyType: keyMaster, keyConfirm: true}, wantContains: errDomainRequired},
		{name: caseMissingType, args: map[string]any{keyDomain: domainExample, keyConfirm: true}, wantContains: errTypeRequired},
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

func TestLinodeDomainCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{
		ID:     111,
		Domain: domainExample,
		Type:   keyMaster,
		Status: statusActive,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(domain); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomain:   domainExample,
		keyType:     keyMaster,
		keySoaEmail: "admin@example.com",
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, domainExample) {
		t.Errorf("textContent.Text does not contain %v", domainExample)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the domain update workflow.
func TestLinodeDomainUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keySoaEmail, keyStatus} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainUpdateToolCaseMissingDomainID(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keySoaEmail: "new@example.com", keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errDomainIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errDomainIDRequired)
	}
}

func TestLinodeDomainUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	domain := linode.Domain{
		ID:     111,
		Domain: domainExample,
		Type:   keyMaster,
		Status: statusActive,
	}

	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(domain); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(111),
		keyDomain:   domainExample,
		keySoaEmail: "new@example.com",
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if capturedBody["domain"] != domainExample {
		t.Errorf("capturedBody[domain] = %v, want %v", capturedBody["domain"], domainExample)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

func TestLinodeDomainUpdateToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, _ := tools.NewLinodeDomainUpdateTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeDomainUpdateToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	domainBody := `{"id":222,"domain":"dry.example.com","type":"master","status":"active","soa_email":"existing@example.com"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/domains/222" {
			_, _ = w.Write([]byte(domainBody))

			return
		}

		// The Tier A walk also lists DNS records; an empty page keeps
		// this subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainUpdateTool(dryRunCfg)

	// Intentionally omit optional args; dry_run path returns current
	// state via GET regardless of what update fields would be sent.
	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(222),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_domain_update") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_update")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "PUT") {
		t.Errorf("got %v, want %v", would["method"], "PUT")
	}

	if !reflect.DeepEqual(would["path"], "/domains/222") {
		t.Errorf("got %v, want %v", would["path"], "/domains/222")
	}

	state, stateIsObject := body["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	if state[keyBetaID] != float64(222) {
		t.Errorf("value = %v, want %v", state[keyBetaID], float64(222))
	}

	if !reflect.DeepEqual(state["domain"], "dry.example.com") {
		t.Errorf("got %v, want %v", state["domain"], "dry.example.com")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeDomainUpdateToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":333,"domain":"no-confirm.example.com","status":"active"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainUpdateTool(dryRunCfg)

	// Intentionally omit confirm; the dry-run path must not gate on it.
	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(333),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeDomainUpdateToolDryRunStillValidatesDomainId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainUpdateTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errDomainIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errDomainIDRequired)
	}
}

// End-to-end verification of the domain deletion workflow.
func TestLinodeDomainDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainDeleteToolCaseRequiresConfirm(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyDomainID: float64(111)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeDomainDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(111),
		keyConfirm:  true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

func TestLinodeDomainDeleteToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, _ := tools.NewLinodeDomainDeleteTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeDomainDeleteToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	domainBody := `{"id":222,"domain":"dry-delete.example.com","type":"master","status":"active"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/domains/222" {
			_, _ = w.Write([]byte(domainBody))

			return
		}

		// The Tier A walk also lists DNS records; an empty page keeps
		// this subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(222),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_domain_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/domains/222") {
		t.Errorf("got %v, want %v", would["path"], "/domains/222")
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeDomainDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":333,"domain":"no-confirm-delete.example.com","status":"active"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(333),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeDomainDeleteToolDryRunStillValidatesDomainId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainDeleteTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errDomainIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errDomainIDRequired)
	}
}

// End-to-end verification of the domain record creation workflow.
func TestLinodeDomainRecordCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_record_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_record_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keyType, keyTarget, keyName} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainRecordCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainRecordCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyType: "A", keyTarget: privateIPv4AddressOne, keyConfirm: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         caseMissingType,
			args:         map[string]any{keyDomainID: float64(111), keyTarget: privateIPv4AddressOne, keyConfirm: true},
			wantContains: errTypeRequired,
		},
		{
			name:         "missing target",
			args:         map[string]any{keyDomainID: float64(111), keyType: "A", keyConfirm: true},
			wantContains: "target is required",
		},
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

func TestLinodeDomainRecordCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	record := linode.DomainRecord{
		ID:     222,
		Type:   "A",
		Name:   hostWWW,
		Target: "8.8.8.8",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111/records" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111/records")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(record); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainRecordCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(111),
		keyType:     "A",
		keyName:     hostWWW,
		keyTarget:   "8.8.8.8",
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the domain record update workflow.
func TestLinodeDomainRecordUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_record_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_record_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keyRecordID, keyTarget} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainRecordUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainRecordUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyRecordID: float64(222), keyTarget: privateIPv4AddressTwo, keyConfirm: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111), keyTarget: privateIPv4AddressTwo, keyConfirm: true},
			wantContains: "record_id is required",
		},
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

func TestLinodeDomainRecordUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	record := linode.DomainRecord{
		ID:     222,
		Type:   "A",
		Name:   hostWWW,
		Target: privateIPv4AddressTwo,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111/records/222" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111/records/222")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(record); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainRecordUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(111),
		keyRecordID: float64(222),
		keyTarget:   privateIPv4AddressTwo,
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// End-to-end verification of the domain record deletion workflow.
func TestLinodeDomainRecordDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_domain_record_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_record_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDomainID, keyRecordID} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeDomainRecordDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingDomainID,
			args:         map[string]any{keyRecordID: float64(222), keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: errDomainIDRequired,
		},
		{
			name:         "missing record id",
			args:         map[string]any{keyDomainID: float64(111), keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: "record_id is required",
		},
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

func TestLinodeDomainRecordDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/111/records/222" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111/records/222")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeDomainRecordDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(111),
		keyRecordID: float64(222),
		keyConfirm:  true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

func TestLinodeDomainRecordDeleteToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, _ := tools.NewLinodeDomainRecordDeleteTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeDomainRecordDeleteToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	recordBody := `{"id":444,"type":"A","name":"www","target":"192.0.2.1","ttl_sec":3600}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != "/domains/333/records/444" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/333/records/444")
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(recordBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainRecordDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(333),
		keyRecordID: float64(444),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_domain_record_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_domain_record_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/domains/333/records/444") {
		t.Errorf("got %v, want %v", would["path"], "/domains/333/records/444")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeDomainRecordDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":555,"type":"CNAME","name":"alias","target":"www.example.com"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeDomainRecordDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDomainID: float64(333),
		keyRecordID: float64(555),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeDomainRecordDeleteToolDryRunStillValidatesBothIDs(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeDomainRecordDeleteTool(cfg)

	t.Parallel()

	t.Run("missing domain_id", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyRecordID: float64(444),
			keyDryRun:   true,
		})

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

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "domain_id is required")
		}
	})

	t.Run("missing record_id", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyDomainID: float64(333),
			keyDryRun:   true,
		})

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

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "record_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "record_id is required")
		}
	})
}

// End-to-end verification of the volume creation workflow.
func TestLinodeVolumeCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{monitorAlertDefinitionLabelParam, keySupportTicketRegion, keySize, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeVolumeCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyLabel: labelDataVol, keyRegion: regionUSEast},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingLabel,
			args:         map[string]any{keyRegion: regionUSEast, keyConfirm: true},
			wantContains: errLabelRequired,
		},
		{
			name:         "requires region or linode id",
			args:         map[string]any{keyLabel: labelDataVol, keyConfirm: true},
			wantContains: "either region or linode_id is required",
		},
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

func TestLinodeVolumeCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	volume := linode.Volume{
		ID:     333,
		Label:  labelDataVol,
		Region: regionUSEast,
		Size:   50,
		Status: "creating",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(volume); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   labelDataVol,
		keyRegion:  regionUSEast,
		keySize:    float64(50),
		keyConfirm: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, labelDataVol) {
		t.Errorf("textContent.Text does not contain %v", labelDataVol)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the volume attach workflow.
func TestLinodeVolumeAttachToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeAttachTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_attach" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_attach")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyVolumeID, keyLinodeID, keyConfigID} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeVolumeAttachToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeAttachTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseMissingVolumeID,
			args:         map[string]any{keyLinodeID: float64(123), keyConfirm: true},
			wantContains: errVolumeIDRequired,
		},
		{
			name:         caseMissingLinodeID,
			args:         map[string]any{keyVolumeID: float64(333), keyConfirm: true},
			wantContains: errLinodeIDRequired,
		},
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

func TestLinodeVolumeAttachToolSuccessfulAttachment(t *testing.T) {
	t.Parallel()

	linodeID := 123
	volume := linode.Volume{
		ID:       333,
		Label:    labelDataVol,
		Region:   regionUSEast,
		LinodeID: &linodeID,
		Status:   statusActive,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/333/attach" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/333/attach")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(volume); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeAttachTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyLinodeID: float64(123),
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "attached") {
		t.Errorf("textContent.Text does not contain %v", "attached")
	}
}

// End-to-end verification of the volume detach workflow.
func TestLinodeVolumeDetachToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_detach" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_detach")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyVolumeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyVolumeID)
	}
}

func TestLinodeVolumeDetachToolCaseMissingVolumeID(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeDetachTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVolumeIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVolumeIDRequired)
	}
}

func TestLinodeVolumeDetachToolSuccessfulDetachment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/333/detach" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/333/detach")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeDetachTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333), keyConfirm: true})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "detached successfully") {
		t.Errorf("textContent.Text does not contain %v", "detached successfully")
	}
}

// End-to-end verification of the volume resize workflow.
func TestLinodeVolumeResizeToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeResizeTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_resize" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_resize")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyVolumeID, keySize, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeVolumeResizeToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeResizeTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyVolumeID: float64(333), keySize: float64(100)},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingVolumeID,
			args:         map[string]any{keySize: float64(100), keyConfirm: true},
			wantContains: errVolumeIDRequired,
		},
		{
			name: "missing size",
			args: map[string]any{keyVolumeID: float64(333), keyConfirm: true},
			// When size is 0 or missing, validation returns "size is required" or min size error.
			wantContains: "size",
		},
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

func TestLinodeVolumeResizeToolSuccessfulResize(t *testing.T) {
	t.Parallel()

	volume := linode.Volume{
		ID:     333,
		Label:  labelDataVol,
		Region: regionUSEast,
		Size:   100,
		Status: "resizing",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/333/resize" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/333/resize")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(volume); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeResizeTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keySize:     float64(100),
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "resize") {
		t.Errorf("textContent.Text does not contain %v", "resize")
	}
}

// End-to-end verification of the volume deletion workflow.
func TestLinodeVolumeDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyVolumeID, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeVolumeDeleteToolCaseRequiresConfirm(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeVolumeDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVolumes333 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyConfirm:  true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

func TestLinodeVolumeDeleteToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, _ := tools.NewLinodeVolumeDeleteTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeVolumeDeleteToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	volumeBody := `{"id":444,"label":"data-vol","size":50,"region":"us-east","status":"active"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != "/volumes/444" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/444")
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(volumeBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeVolumeDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(444),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_volume_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_volume_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/volumes/444") {
		t.Errorf("got %v, want %v", would["path"], "/volumes/444")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeVolumeDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":555,"label":"unconfirmed","size":20,"region":"us-east"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeVolumeDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(555),
		keyDryRun:   true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeVolumeDeleteToolDryRunStillValidatesVolumeId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVolumeIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVolumeIDRequired)
	}
}

// End-to-end verification of the NodeBalancer creation workflow.
func TestLinodeNodeBalancerCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyRegion, keyLabel, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeNodeBalancerCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNodeBalancerCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyRegion: regionUSEast},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseMissingRegion,
			args:         map[string]any{keyConfirm: true},
			wantContains: errRegionRequired,
		},
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

func TestLinodeNodeBalancerCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	nodeBalancer := linode.NodeBalancer{
		ID:     444,
		Label:  "web-lb",
		Region: regionUSEast,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nodebalancers" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/nodebalancers")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(nodeBalancer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeNodeBalancerCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:  regionUSEast,
		keyLabel:   "web-lb",
		keyConfirm: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "web-lb") {
		t.Errorf("textContent.Text does not contain %v", "web-lb")
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of the NodeBalancer update workflow.
func TestLinodeNodeBalancerUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyNodeBalancerID, keyLabel, "client_conn_throttle"} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeNodeBalancerUpdateToolMissingNodebalancerId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyLabel: labelNew, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "nodebalancer_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "nodebalancer_id is required")
	}
}

func TestLinodeNodeBalancerUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	nodeBalancer := linode.NodeBalancer{
		ID:     444,
		Label:  "updated-lb",
		Region: regionUSEast,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNodebalancers444 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers444)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(nodeBalancer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeNodeBalancerUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(444),
		keyLabel:          "updated-lb",
		keyConfirm:        true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// End-to-end verification of the NodeBalancer deletion workflow.
func TestLinodeNodeBalancerDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_nodebalancer_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_nodebalancer_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyNodeBalancerID, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeNodeBalancerDeleteToolCaseRequiresConfirm(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(444)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeNodeBalancerDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcNodebalancers444 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcNodebalancers444)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeNodeBalancerDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(444),
		keyConfirm:        true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// Dry-run coverage for NodeBalancer delete. Kept in a sibling function
// so the main test's subtest count stays under maintidx's threshold.
func TestLinodeNodeBalancerDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeNodeBalancerDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeNodeBalancerDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	nbBody := `{"id":444,"label":"prod-lb","region":"us-east"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == tcNodebalancers444 {
			_, _ = w.Write([]byte(nbBody))

			return
		}

		// The Tier A walk also lists configs; an empty page keeps this
		// subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(444),
		keyDryRun:         true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcNodebalancers444) {
		t.Errorf("got %v, want %v", would["path"], tcNodebalancers444)
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeNodeBalancerDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":444,"label":"prod-lb"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(444),
		keyDryRun:         true,
	})

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
}

func TestLinodeNodeBalancerDeleteToolDryRunStillValidatesNodebalancerId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeNodeBalancerDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "nodebalancer_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "nodebalancer_id is required")
	}
}

// End-to-end verification of the volume update workflow.
func TestLinodeVolumeUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_volume_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyVolumeID, monitorAlertDefinitionLabelParam, keyTags, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeVolumeUpdateToolCaseRequiresConfirm(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
		t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
	}
}

func TestLinodeVolumeUpdateToolMissingVolumeId(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVolumeIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVolumeIDRequired)
	}
}

func TestLinodeVolumeUpdateToolMissingLabelAndTags(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVolumeUpdateTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{keyVolumeID: float64(333), keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one of label or tags is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "at least one of label or tags is required")
	}
}

func TestLinodeVolumeUpdateToolSuccessfulUpdateWithLabel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVolumes333 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Volume{ID: 333, Label: "updated-volume", Size: 20, Region: "us-east", Status: "active"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyLabel:    "updated-volume",
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeVolumeUpdateToolSuccessfulUpdateWithTags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes/444" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/444")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.Volume{ID: 444, Label: "tagged-volume", Size: 50, Region: "us-west", Status: "active", Tags: []string{"production", "db"}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeVolumeUpdateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(444),
		keyTags:     []any{"production", "db"},
		keyConfirm:  true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeVolumeUpdateToolUpdaterError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors": [{"reason": "internal server error"}]}`)) // errcheck: test mock; write failure is acceptable
	}))
	defer srv.Close()

	errorCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errorHandler := tools.NewLinodeVolumeUpdateTool(errorCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(333),
		keyLabel:    "new-label",
		keyConfirm:  true,
	})

	result, err := errorHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(tc.Text, "update failed") {
		t.Errorf("tc.Text does not contain %v", "update failed")
	}
}

// End-to-end verification of the image creation workflow.
func TestLinodeImageCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeImageCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_image_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_create")
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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyDiskID, keyLabel, keyDescription, tcCloudInit, keyTags, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeImageCreateToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyDiskID: 123}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyDiskID: 123, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing disk id", args: map[string]any{keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "zero disk id", args: map[string]any{keyDiskID: 0, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "negative disk id", args: map[string]any{keyDiskID: -1, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requestCount.Add(1)
			}))
			t.Cleanup(srv.Close)

			validationCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, validationHandler := tools.NewLinodeImageCreateTool(validationCfg)

			req := createRequestWithArgs(t, tt.args)

			result, err := validationHandler(t.Context(), req)
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

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	created := linode.Image{ID: "private/15", Label: imageUploadLabelFixture, Status: "creating", CreatedBy: "tester"}

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if body[keyDiskID] != float64(123) {
			t.Errorf("value = %v, want %v", body[keyDiskID], float64(123))
		}

		for key, want := range map[string]any{
			monitorAlertDefinitionLabelParam: imageUploadLabelFixture,
			keyDescription:                   "test image",
			tcCloudInit:                      true,
			keyTags:                          []any{"blue", "green"},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeImageCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDiskID:      123,
		keyLabel:       imageUploadLabelFixture,
		keyDescription: "test image",
		tcCloudInit:    true,
		keyTags:        []any{"blue", "green"},
		keyConfirm:     true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "private/15") {
		t.Errorf("textContent.Text does not contain %v", "private/15")
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

func TestLinodeImageCreateToolClientErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"disk not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errHandler := tools.NewLinodeImageCreateTool(errCfg)

	req := createRequestWithArgs(t, map[string]any{keyDiskID: 123, keyConfirm: true})

	result, err := errHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create image") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create image")
	}
}

func TestLinodeImageShareGroupTokenCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeImageShareGroupTokenCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_image_sharegroup_token_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_token_create")
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

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyValidForShareGroupUUID, keyLabel, keyConfirm} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeImageShareGroupTokenCreateToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing share group uuid", args: map[string]any{keyConfirm: true}, wantContains: errValidForShareGroupUUID},
		{name: "empty share group uuid", args: map[string]any{keyValidForShareGroupUUID: blankString, keyConfirm: true}, wantContains: errValidForShareGroupUUID},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requestCount.Add(1)
			}))
			t.Cleanup(srv.Close)

			validationCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, validationHandler := tools.NewLinodeImageShareGroupTokenCreateTool(validationCfg)

			req := createRequestWithArgs(t, tt.args)

			result, err := validationHandler(t.Context(), req)
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

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageShareGroupTokenCreateToolSuccessfulTokenCreation(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/images/sharegroups/tokens" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], "release-token") {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], "release-token")
		}

		if !reflect.DeepEqual(body[keyValidForShareGroupUUID], shareGroupUUIDFixture) {
			t.Errorf("body[keyValidForShareGroupUUID] = %v, want %v", body[keyValidForShareGroupUUID], shareGroupUUIDFixture)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyToken:                  shareGroupTokenValueFixture,
			keyTokenUUID:              shareGroupTokenUUIDFixture,
			keyStatus:                 statusActive,
			keyLabel:                  "release-token",
			"created":                 imageShareGroupTokenCreated,
			"updated":                 nil,
			"expiry":                  nil,
			keyValidForShareGroupUUID: shareGroupUUIDFixture,
			"sharegroup_uuid":         shareGroupUUIDFixture,
			"sharegroup_label":        shareGroupLabelFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeImageShareGroupTokenCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyValidForShareGroupUUID: shareGroupUUIDFixture,
		keyLabel:                  "release-token",
		keyConfirm:                true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, shareGroupTokenUUIDFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupTokenUUIDFixture)
	}

	if !strings.Contains(textContent.Text, shareGroupTokenValueFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupTokenValueFixture)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

func TestLinodeImageShareGroupTokenCreateToolClientErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"share group not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errHandler := tools.NewLinodeImageShareGroupTokenCreateTool(errCfg)

	req := createRequestWithArgs(t, map[string]any{keyValidForShareGroupUUID: shareGroupUUIDFixture, keyConfirm: true})

	result, err := errHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create image share group token") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create image share group token")
	}
}

// End-to-end verification of the StackScript creation workflow.
func TestLinodeStackScriptCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, _, handler := tools.NewLinodeStackScriptCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_stackscript_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_stackscript_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyLabel, keyScript, keyImages, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeStackScriptCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeStackScriptCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: []any{testDebian12Image}},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         "missing label",
			args:         map[string]any{keyScript: testStackScript, keyImages: []any{testDebian12Image}, keyConfirm: true},
			wantContains: "label is required",
		},
		{
			name:         "missing script",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyImages: []any{testDebian12Image}, keyConfirm: true},
			wantContains: "script is required",
		},
		{
			name:         caseBlankLabelImageShareGroupToken,
			args:         map[string]any{keyLabel: blankString, keyScript: testStackScript, keyImages: []any{testDebian12Image}, keyConfirm: true},
			wantContains: "label is required",
		},
		{
			name:         "blank script",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: blankString, keyImages: []any{testDebian12Image}, keyConfirm: true},
			wantContains: "script is required",
		},
		{
			name:         "missing images",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyConfirm: true},
			wantContains: "images is required",
		},
		{
			name:         "query image",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: []any{configIDQueryValue}, keyConfirm: true},
			wantContains: errStackScriptImagesValid,
		},
		{
			name:         "fragment image",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: []any{"linode/debian12#fragment"}, keyConfirm: true},
			wantContains: errStackScriptImagesValid,
		},
		{
			name:         "extra separator image",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: []any{"private/15/extra"}, keyConfirm: true},
			wantContains: errStackScriptImagesValid,
		},
		{
			name:         "traversal image",
			args:         map[string]any{keyLabel: testStackScriptLabel, keyScript: testStackScript, keyImages: []any{privateImageTraversalFixture}, keyConfirm: true},
			wantContains: errStackScriptImagesValid,
		},
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

func TestLinodeStackScriptCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	created := linode.StackScript{
		ID:       456,
		Label:    testStackScriptLabel,
		Script:   testStackScriptWithWhitespace,
		Images:   []string{testDebian12Image},
		IsPublic: false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/stackscripts" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyScript], testStackScriptWithWhitespace) {
			t.Errorf("body[keyScript] = %v, want %v", body[keyScript], testStackScriptWithWhitespace)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeStackScriptCreateTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   testStackScriptLabel,
		keyScript:  testStackScriptWithWhitespace,
		keyImages:  []any{testDebian12Image},
		keyConfirm: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, testStackScriptLabel) {
		t.Errorf("textContent.Text does not contain %v", testStackScriptLabel)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

func TestLinodeStackScriptCreateToolClientErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"label is not unique"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errHandler := tools.NewLinodeStackScriptCreateTool(errCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   testStackScriptLabel,
		keyScript:  testStackScript,
		keyImages:  []any{testDebian12Image},
		keyConfirm: true,
	})

	result, err := errHandler(t.Context(), req)
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

func TestLinodeStackScriptCreateToolEmptyImagesAfterTrimRejected(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeStackScriptCreateTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   testStackScriptLabel,
		keyScript:  testStackScript,
		keyImages:  []any{" "},
		keyConfirm: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "images is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "images is required")
	}
}

// End-to-end verification of the StackScript deletion workflow.
func TestLinodeStackScriptDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeStackScriptDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_stackscript_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_stackscript_delete")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyStackScriptID, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeStackScriptDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeStackScriptDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{
			name:         caseRequiresConfirm,
			args:         map[string]any{keyStackScriptID: testStackScriptID},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseFalseConfirmRejected,
			args:         map[string]any{keyStackScriptID: testStackScriptID, keyConfirm: false},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseStringConfirmRejected,
			args:         map[string]any{keyStackScriptID: testStackScriptID, keyConfirm: boolStringTrue},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseNumericConfirmRejected,
			args:         map[string]any{keyStackScriptID: testStackScriptID, keyConfirm: float64(1)},
			wantContains: errConfirmEqualsTrue,
		},
		{
			name:         caseZero,
			args:         map[string]any{keyStackScriptID: float64(0), keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: "stackscript_id must be a positive integer",
		},
		{
			name:         "fractional stackscript id",
			args:         map[string]any{keyStackScriptID: 456.5, keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: errStackScriptIDInteger,
		},
		{
			name:         "separator stackscript id",
			args:         map[string]any{keyStackScriptID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: errStackScriptIDInteger,
		},
		{
			name:         "query stackscript id",
			args:         map[string]any{keyStackScriptID: configIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: errStackScriptIDInteger,
		},
		{
			name:         "traversal stackscript id",
			args:         map[string]any{keyStackScriptID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true},
			wantContains: errStackScriptIDInteger,
		},
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

func TestLinodeStackScriptDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/stackscripts/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts/456")
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, successHandler := tools.NewLinodeStackScriptDeleteTool(successCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyStackScriptID: testStackScriptID,
		keyConfirm:       true, keyConfirmedDryRun: true,
	})

	result, err := successHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "deleted successfully")
	}
}

func TestLinodeStackScriptDeleteToolClientErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errHandler := tools.NewLinodeStackScriptDeleteTool(errCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyStackScriptID: testStackScriptID,
		keyConfirm:       true, keyConfirmedDryRun: true,
	})

	result, err := errHandler(t.Context(), req)
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
