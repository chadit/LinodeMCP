package tools_test

import (
	"encoding/json"
	"math"
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

const (
	managedLinodeSettingsUpdateToolName  = "linode_managed_linode_settings_update"
	managedLinodeSettingsUpdateIDKey     = "linode_id"
	managedLinodeSettingsUpdateSSHKey    = "ssh"
	managedLinodeSettingsUpdateAccessKey = "access"
	managedLinodeSettingsUpdateIPKey     = "ip"
	managedLinodeSettingsUpdatePortKey   = "port"
	managedLinodeSettingsUpdateUserKey   = "user"
	managedLinodeSettingsUpdatePortError = "ssh.port must be an integer between 1 and 65535"
)

// managedSettingsUpdateArgs builds tool args for the nested ssh object. A nil
// linodeID omits the linode_id key so the missing-id case can be exercised.
func managedSettingsUpdateArgs(linodeID any, ssh map[string]any) map[string]any {
	args := map[string]any{
		managedLinodeSettingsUpdateSSHKey: ssh,
		keyConfirm:                        true,
	}
	if linodeID != nil {
		args[managedLinodeSettingsUpdateIDKey] = linodeID
	}

	return args
}

func TestLinodeManagedLinodeSettingsUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

	if tool.Name != managedLinodeSettingsUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedLinodeSettingsUpdateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	for _, key := range []string{keyConfirm, managedLinodeSettingsUpdateSSHKey} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyConfirm, managedLinodeSettingsUpdateIDKey, managedLinodeSettingsUpdateSSHKey} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeManagedLinodeSettingsUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	port := 2222
	user := keyGrantLinode
	settings := linode.ManagedLinodeSettings{
		ID:    123,
		Label: managedLinodeSettingsLabel,
		Group: managedLinodeSettingsGroup,
		SSH:   linode.ManagedLinodeSettingsSSH{Access: true, IP: testNetIPv4AddressOne, Port: &port, User: &user},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedLinodeSettingsPath+"/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsPath+"/123")
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		ssh, ok := got["ssh"].(map[string]any)
		if !ok {
			t.Error("ssh should be an object")
		}

		for key, want := range map[string]any{
			"access": true,
			"ip":     testNetIPv4AddressOne,
			"port":   float64(port),
			"user":   user,
		} {
			if !reflect.DeepEqual(ssh[key], want) {
				t.Errorf("ssh[%v] = %v, want %v", key, ssh[key], want)
			}
		}

		if _, ok := got["group"]; ok {
			t.Errorf("got has unexpected key %v", "group")
		}

		if _, ok := got[keySupportTicketID]; ok {
			t.Errorf("got has unexpected key %v", keySupportTicketID)
		}

		if _, ok := got[monitorAlertDefinitionLabelParam]; ok {
			t.Errorf("got has unexpected key %v", managedServiceLabelParam)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		managedLinodeSettingsUpdateIDKey: 123,
		managedLinodeSettingsUpdateSSHKey: map[string]any{
			managedLinodeSettingsUpdateAccessKey: true,
			managedLinodeSettingsUpdateIPKey:     testNetIPv4AddressOne,
			managedLinodeSettingsUpdatePortKey:   port,
			managedLinodeSettingsUpdateUserKey:   user,
		},
		keyConfirm: true,
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

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, managedLinodeSettingsLabel) {
		t.Errorf("textContent.Text does not contain %v", managedLinodeSettingsLabel)
	}
}

func TestLinodeManagedLinodeSettingsUpdateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		confirm    any
		setConfirm bool
	}{
		{name: caseMissingConfirm},
		{name: caseFalseConfirm, confirm: false, setConfirm: true},
		{name: caseStringConfirm, confirm: boolStringTrue, setConfirm: true},
		{name: caseNumericConfirm, confirm: 1, setConfirm: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

			args := map[string]any{
				managedLinodeSettingsUpdateIDKey:  123,
				managedLinodeSettingsUpdateSSHKey: map[string]any{managedLinodeSettingsUpdateAccessKey: true},
			}
			if testCase.setConfirm {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

func TestLinodeManagedLinodeSettingsUpdateToolInvalidInputRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingLinodeID, args: managedSettingsUpdateArgs(nil, map[string]any{managedLinodeSettingsUpdateAccessKey: true}), wantMessage: errLinodeIDPositive},
		{name: "zero linode id", args: managedSettingsUpdateArgs(0, map[string]any{managedLinodeSettingsUpdateAccessKey: true}), wantMessage: errLinodeIDPositive},
		{name: caseSlashLinodeID, args: managedSettingsUpdateArgs(pathSeparatorValue, map[string]any{managedLinodeSettingsUpdateAccessKey: true}), wantMessage: errLinodeIDPositive},
		{name: caseQueryLinodeID, args: managedSettingsUpdateArgs("123?x=1", map[string]any{managedLinodeSettingsUpdateAccessKey: true}), wantMessage: errLinodeIDPositive},
		{name: caseTraversalLinodeID, args: managedSettingsUpdateArgs(pathTraversalValue, map[string]any{managedLinodeSettingsUpdateAccessKey: true}), wantMessage: errLinodeIDPositive},
		{name: "missing ssh", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, keyConfirm: true}, wantMessage: "ssh is required and must be an object"},
		{name: "ssh wrong type", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateSSHKey: "nope", keyConfirm: true}, wantMessage: "ssh must be an object"},
		{name: managedContactUpdateEmptyCase, args: managedSettingsUpdateArgs(123, map[string]any{}), wantMessage: "at least one mutable SSH setting is required"},
		{name: "numeric ssh ip", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdateIPKey: 123}), wantMessage: "ssh.ip must be a string"},
		{name: "string ssh access", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdateAccessKey: boolStringTrue}), wantMessage: "ssh.access must be a boolean"},
		{name: "zero ssh port", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdatePortKey: 0}), wantMessage: managedLinodeSettingsUpdatePortError},
		{name: "large ssh port", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdatePortKey: 65536}), wantMessage: managedLinodeSettingsUpdatePortError},
		{name: "fractional ssh port", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdatePortKey: 22.5}), wantMessage: managedLinodeSettingsUpdatePortError},
		{name: "infinite ssh port", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdatePortKey: math.Inf(1)}), wantMessage: managedLinodeSettingsUpdatePortError},
		{name: "nan ssh port", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdatePortKey: math.NaN()}), wantMessage: managedLinodeSettingsUpdatePortError},
		{name: "numeric ssh user", args: managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdateUserKey: 123}), wantMessage: "ssh.user must be a string"},
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

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeManagedLinodeSettingsUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedLinodeSettingsPath+"/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedLinodeSettingsPath+"/123")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, managedSettingsUpdateArgs(123, map[string]any{managedLinodeSettingsUpdateAccessKey: true})))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_managed_linode_settings_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_managed_linode_settings_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
