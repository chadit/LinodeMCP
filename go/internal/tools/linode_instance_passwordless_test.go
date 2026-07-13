package tools_test

import (
	"context"
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
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	passwordlessCreateTool          = "linode_instance_create"
	passwordlessRebuildTool         = "linode_instance_rebuild"
	passwordlessDiskTool            = "linode_instance_disk_create"
	passwordlessRootPass            = "StrongPass123!"
	passwordlessSSHKey              = "ssh-ed25519 AAAA-passwordless-test"
	passwordlessUser                = "passwordless-user"
	passwordlessSecondKey           = "second-key"
	passwordlessSecondUser          = "second-user"
	passwordlessStatusProvisioning  = "provisioning"
	passwordlessCreateLabel         = "passwordless-create"
	passwordlessDiskLabel           = "passwordless-disk"
	passwordlessAuthorizedKeys      = "authorized_keys"
	passwordlessAuthorizedUsers     = "authorized_users"
	passwordlessSchemaArray         = "array"
	passwordlessSchemaBoolean       = "boolean"
	passwordlessCreatePath          = "/linode/instances"
	passwordlessRebuildPath         = "/linode/instances/123/rebuild"
	passwordlessDiskPath            = "/linode/instances/123/disks"
	passwordlessInterfaceGeneration = "linode"
	passwordlessIPv6Key             = "ipv6"
	focusedPasswordlessAuthError    = "at least one authentication method is required"
)

type passwordlessToolHandler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

type passwordlessAuthCase struct {
	toolName string
	authName string
}

type passwordlessConfirmCase struct {
	name    string
	present bool
	value   any
}

func newPasswordlessTool(t *testing.T, name string, cfg *config.Config) (mcp.Tool, passwordlessToolHandler) {
	t.Helper()

	switch name {
	case passwordlessCreateTool:
		tool, _, handler := tools.NewLinodeInstanceCreateTool(cfg)

		return tool, handler
	case passwordlessRebuildTool:
		tool, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

		return tool, handler
	case passwordlessDiskTool:
		tool, _, handler := tools.NewLinodeInstanceDiskCreateTool(cfg)

		return tool, handler
	default:
		t.Fatalf("unknown tool %q", name)

		return mcp.Tool{}, nil
	}
}

func passwordlessToolNames() []string {
	return []string{passwordlessCreateTool, passwordlessRebuildTool, passwordlessDiskTool}
}

func passwordlessAuthCases() []passwordlessAuthCase {
	cases := make([]passwordlessAuthCase, 0, 9)

	for _, toolName := range passwordlessToolNames() {
		for _, authName := range []string{keyRootPass, passwordlessAuthorizedKeys, passwordlessAuthorizedUsers} {
			cases = append(cases, passwordlessAuthCase{toolName: toolName, authName: authName})
		}
	}

	return cases
}

func passwordlessToolArgs(name string) map[string]any {
	switch name {
	case passwordlessCreateTool:
		return map[string]any{
			keyRegion: regionUSEast, keyType: typeG6Nanode1, keyLabel: passwordlessCreateLabel,
			keyImage: imageIDUbuntu2404, keyFirewallID: float64(99), keyConfirm: true,
		}
	case passwordlessRebuildTool:
		return map[string]any{
			keyLinodeID: float64(123), keyImage: imageIDUbuntu2404, keyConfirm: true,
			keyConfirmedDryRun: true,
		}
	case passwordlessDiskTool:
		return map[string]any{
			keyLinodeID: float64(123), keyLabel: passwordlessDiskLabel, keySize: float64(1024),
			keyImage: imageIDUbuntu2404, keyConfirm: true,
		}
	default:
		return nil
	}
}

func passwordlessExpectedBody(name string) map[string]any {
	switch name {
	case passwordlessCreateTool:
		return map[string]any{
			keyRegion:              regionUSEast,
			keyType:                typeG6Nanode1,
			keyLabel:               passwordlessCreateLabel,
			keyImage:               imageIDUbuntu2404,
			"interface_generation": passwordlessInterfaceGeneration,
			"interfaces": []any{map[string]any{
				keyInterfacePublic: map[string]any{},
				"default_route":    map[string]any{keyIPv4: true, passwordlessIPv6Key: true},
				keyFirewallID:      float64(99),
			}},
		}
	case passwordlessRebuildTool:
		return map[string]any{keyImage: imageIDUbuntu2404}
	case passwordlessDiskTool:
		return map[string]any{keyLabel: passwordlessDiskLabel, keySize: float64(1024), keyImage: imageIDUbuntu2404}
	default:
		return nil
	}
}

func passwordlessPath(name string) string {
	switch name {
	case passwordlessCreateTool:
		return passwordlessCreatePath
	case passwordlessRebuildTool:
		return passwordlessRebuildPath
	case passwordlessDiskTool:
		return passwordlessDiskPath
	default:
		return ""
	}
}

func addPasswordlessAuth(args, body map[string]any, toolName, authName string) {
	switch authName {
	case keyRootPass:
		args[keyRootPass] = passwordlessRootPass
		body[keyRootPass] = passwordlessRootPass
	case passwordlessAuthorizedKeys:
		if toolName == passwordlessDiskTool {
			args[passwordlessAuthorizedKeys] = passwordlessSSHKey + "," + passwordlessSecondKey
			body[passwordlessAuthorizedKeys] = []any{passwordlessSSHKey, passwordlessSecondKey}

			return
		}

		args[passwordlessAuthorizedKeys] = []any{passwordlessSSHKey, passwordlessSecondKey}
		body[passwordlessAuthorizedKeys] = []any{passwordlessSSHKey, passwordlessSecondKey}
	case passwordlessAuthorizedUsers:
		if toolName == passwordlessDiskTool {
			args[passwordlessAuthorizedUsers] = passwordlessUser + "," + passwordlessSecondUser
			body[passwordlessAuthorizedUsers] = []any{passwordlessUser, passwordlessSecondUser}

			return
		}

		args[passwordlessAuthorizedUsers] = []any{passwordlessUser, passwordlessSecondUser}
		body[passwordlessAuthorizedUsers] = []any{passwordlessUser, passwordlessSecondUser}
	}
}

func passwordlessConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}

func passwordlessSuccessServer(
	t *testing.T,
	toolName string,
	wantBody map[string]any,
	calls *atomic.Int32,
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %q, want %q", r.Method, http.MethodPost)
		}

		if r.URL.Path != passwordlessPath(toolName) {
			t.Errorf("r.URL.Path = %q, want %q", r.URL.Path, passwordlessPath(toolName))
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %q, want empty", r.URL.RawQuery)
		}

		var gotBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)

			return
		}

		if !reflect.DeepEqual(gotBody, wantBody) {
			t.Errorf("request body = %#v, want %#v", gotBody, wantBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if toolName == passwordlessDiskTool {
			if err := json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 50, keyLabel: passwordlessDiskLabel, keySize: 1024, keyStatus: statusReady,
			}); err != nil {
				t.Errorf("encode response: %v", err)
			}

			return
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: 123, keyLabel: passwordlessCreateLabel, keyRegion: regionUSEast, keyStatus: passwordlessStatusProvisioning,
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

func runPasswordlessForwardingCase(t *testing.T, testCase passwordlessAuthCase) {
	t.Helper()

	args := passwordlessToolArgs(testCase.toolName)
	wantBody := passwordlessExpectedBody(testCase.toolName)
	addPasswordlessAuth(args, wantBody, testCase.toolName, testCase.authName)

	var calls atomic.Int32

	srv := passwordlessSuccessServer(t, testCase.toolName, wantBody, &calls)
	t.Cleanup(srv.Close)

	_, handler := newPasswordlessTool(t, testCase.toolName, passwordlessConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want success", result)
	}

	if calls.Load() != 1 {
		t.Errorf("HTTP calls = %d, want 1", calls.Load())
	}
}

func TestPasswordlessProvisioningToolsForwardExactlyOneAuthenticationMethod(t *testing.T) {
	t.Parallel()

	for _, testCase := range passwordlessAuthCases() {
		t.Run(testCase.toolName+"/"+testCase.authName, func(t *testing.T) {
			t.Parallel()
			runPasswordlessForwardingCase(t, testCase)
		})
	}
}

func TestPasswordlessProvisioningToolsRejectMissingAuthBeforeClientTraffic(t *testing.T) {
	t.Parallel()

	for _, toolName := range passwordlessToolNames() {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(srv.Close)

			_, handler := newPasswordlessTool(t, toolName, passwordlessConfig(srv.URL))

			args := passwordlessToolArgs(toolName)
			if toolName != passwordlessRebuildTool {
				delete(args, keyImage)
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			if result == nil || !result.IsError {
				t.Fatalf("result = %#v, want tool error", result)
			}

			text, ok := result.Content[0].(mcp.TextContent)
			if !ok || !strings.Contains(text.Text, focusedPasswordlessAuthError) {
				t.Errorf("error text = %q, want substring %q", text.Text, focusedPasswordlessAuthError)
			}

			if calls.Load() != 0 {
				t.Errorf("HTTP calls = %d, want 0", calls.Load())
			}
		})
	}
}

func passwordlessConfirmCases() []passwordlessConfirmCase {
	return []passwordlessConfirmCase{
		{name: caseMissing},
		{name: caseFalse, present: true, value: false},
		{name: caseString, present: true, value: boolStringTrue},
		{name: caseNumeric, present: true, value: float64(1)},
	}
}

func runPasswordlessConfirmCase(t *testing.T, toolName string, confirmCase passwordlessConfirmCase) {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, handler := newPasswordlessTool(t, toolName, passwordlessConfig(srv.URL))
	args := passwordlessToolArgs(toolName)
	args[keyRootPass] = passwordlessRootPass
	delete(args, keyConfirm)

	if confirmCase.present {
		args[keyConfirm] = confirmCase.value
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatalf("result = %#v, want tool error", result)
	}

	if calls.Load() != 0 {
		t.Errorf("HTTP calls = %d, want 0", calls.Load())
	}
}

func TestPasswordlessProvisioningToolsRequireLiteralTrueBeforeClientTraffic(t *testing.T) {
	t.Parallel()

	for _, toolName := range passwordlessToolNames() {
		for _, confirmCase := range passwordlessConfirmCases() {
			t.Run(toolName+"/"+confirmCase.name, func(t *testing.T) {
				t.Parallel()
				runPasswordlessConfirmCase(t, toolName, confirmCase)
			})
		}
	}
}

func TestPasswordlessProvisioningToolSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		toolName      string
		propertyTypes map[string]string
		notRequired   []string
	}{
		{
			toolName: passwordlessCreateTool,
			propertyTypes: map[string]string{
				passwordlessAuthorizedKeys: passwordlessSchemaArray, passwordlessAuthorizedUsers: passwordlessSchemaArray,
				keyConfirm: passwordlessSchemaBoolean,
			},
		},
		{
			toolName: passwordlessRebuildTool,
			propertyTypes: map[string]string{
				passwordlessAuthorizedKeys: passwordlessSchemaArray, passwordlessAuthorizedUsers: passwordlessSchemaArray,
				keyRootPass: caseString, keyConfirm: passwordlessSchemaBoolean,
			},
			notRequired: []string{keyRootPass},
		},
		{
			toolName: passwordlessDiskTool,
			propertyTypes: map[string]string{
				passwordlessAuthorizedKeys: caseString, passwordlessAuthorizedUsers: caseString,
				keyConfirm: passwordlessSchemaBoolean,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			t.Parallel()

			tool, _ := newPasswordlessTool(t, tt.toolName, &config.Config{})

			var schema struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
				t.Fatalf("decode schema: %v", err)
			}

			for property, wantType := range tt.propertyTypes {
				got, ok := schema.Properties[property]
				if !ok {
					t.Errorf("schema missing property %q", property)

					continue
				}

				if got.Type != wantType {
					t.Errorf("schema property %q type = %q, want %q", property, got.Type, wantType)
				}
			}

			for _, property := range tt.notRequired {
				if slices.Contains(schema.Required, property) {
					t.Errorf("schema required = %v, must not contain %q", schema.Required, property)
				}
			}

			if !slices.Contains(schema.Required, keyConfirm) {
				t.Errorf("schema required = %v, must contain %q", schema.Required, keyConfirm)
			}
		})
	}
}

func TestPasswordlessProvisioningToolsRejectInvalidAuthorizedArrays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value any
	}{
		{name: "blank authorized key", field: passwordlessAuthorizedKeys, value: []any{passwordlessSSHKey, blankWhitespace}},
		{name: "invalid authorized keys type", field: passwordlessAuthorizedKeys, value: float64(1)},
		{name: "blank authorized user", field: passwordlessAuthorizedUsers, value: []any{passwordlessUser, blankWhitespace}},
		{name: "invalid authorized users type", field: passwordlessAuthorizedUsers, value: float64(1)},
	}

	for _, toolName := range []string{passwordlessCreateTool, passwordlessRebuildTool} {
		for _, test := range tests {
			t.Run(toolName+"/"+test.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				}))
				t.Cleanup(srv.Close)

				args := passwordlessToolArgs(toolName)
				args[test.field] = test.value
				_, handler := newPasswordlessTool(t, toolName, passwordlessConfig(srv.URL))

				result, err := handler(t.Context(), createRequestWithArgs(t, args))
				if err != nil {
					t.Fatalf("handler error: %v", err)
				}

				if result == nil || !result.IsError {
					t.Fatalf("result = %#v, want tool error", result)
				}

				if calls.Load() != 0 {
					t.Errorf("HTTP calls = %d, want 0", calls.Load())
				}
			})
		}
	}
}

func TestPasswordlessProvisioningToolsRejectWrongAuthTypesWithValidAlternative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		toolName         string
		invalidField     string
		invalidValue     any
		alternativeField string
		alternativeValue any
	}{
		{name: "create root pass", toolName: passwordlessCreateTool, invalidField: keyRootPass, invalidValue: float64(1), alternativeField: passwordlessAuthorizedUsers, alternativeValue: []any{passwordlessUser}},
		{name: "rebuild root pass", toolName: passwordlessRebuildTool, invalidField: keyRootPass, invalidValue: float64(1), alternativeField: passwordlessAuthorizedUsers, alternativeValue: []any{passwordlessUser}},
		{name: "disk root pass", toolName: passwordlessDiskTool, invalidField: keyRootPass, invalidValue: float64(1), alternativeField: passwordlessAuthorizedUsers, alternativeValue: passwordlessUser},
		{name: "disk authorized keys", toolName: passwordlessDiskTool, invalidField: passwordlessAuthorizedKeys, invalidValue: []any{passwordlessSSHKey}, alternativeField: keyRootPass, alternativeValue: passwordlessRootPass},
		{name: "disk authorized users", toolName: passwordlessDiskTool, invalidField: passwordlessAuthorizedUsers, invalidValue: []any{passwordlessUser}, alternativeField: keyRootPass, alternativeValue: passwordlessRootPass},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(srv.Close)

			args := passwordlessToolArgs(test.toolName)
			args[test.invalidField] = test.invalidValue
			args[test.alternativeField] = test.alternativeValue
			_, handler := newPasswordlessTool(t, test.toolName, passwordlessConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			if result == nil || !result.IsError {
				t.Fatalf("result = %#v, want tool error", result)
			}

			if calls.Load() != 0 {
				t.Errorf("HTTP calls = %d, want 0", calls.Load())
			}
		})
	}
}
