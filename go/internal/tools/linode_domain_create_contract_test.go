package tools_test

import (
	"encoding/json"
	"math"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestLinodeDomainCreateContract freezes what POST /v4/domains advertises:
// capability, scope, the full documented property set, which of those are
// required, and the two closed enums.
func TestLinodeDomainCreateContract(t *testing.T) {
	t.Parallel()

	tool, capability, _ := tools.NewLinodeDomainCreateTool(&config.Config{})
	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	wantScopes := []profiles.Scope{profiles.ScopeDomainsReadWrite}
	if got := profiles.RequiredScopes(tool.Name, capability); !reflect.DeepEqual(got, wantScopes) {
		t.Errorf("RequiredScopes() = %v, want %v", got, wantScopes)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}

	if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, field := range []string{
		keyEnvironment, keyDomain, keyType, keyAXFRIPs, keyDescription, keyExpireSec,
		keyGroup, keyMasterIPs, keyRefreshSec, keyRetrySec, keySoaEmail, keyStatus,
		keyTags, keyTTLSec, keyConfirm, keyDryRun,
	} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("tool.RawInputSchema missing property %q", field)
		}
	}

	for _, field := range []string{keyDomain, keyType, keyConfirm} {
		if !slices.Contains(schema.Required, field) {
			t.Errorf("tool.RawInputSchema required missing %q", field)
		}
	}

	for _, field := range []string{
		keyAXFRIPs, keyDescription, keyExpireSec, keyGroup, keyMasterIPs,
		keyRefreshSec, keyRetrySec, keySoaEmail, keyStatus, keyTags, keyTTLSec,
	} {
		if slices.Contains(schema.Required, field) {
			t.Errorf("tool.RawInputSchema unexpectedly requires %q", field)
		}
	}

	for field, want := range map[string][]string{
		keyType:   {keyMaster, domainSlave},
		keyStatus: {statusActive, statusDisabled},
	} {
		var property struct {
			Enum []string `json:"enum"`
		}

		if err := json.Unmarshal(schema.Properties[field], &property); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(property.Enum, want) {
			t.Errorf("%s enum = %v, want %v", field, property.Enum, want)
		}
	}
}

func TestLinodeDomainCreateValidationContract(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainCreateTool(&config.Config{})

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "domain too long",
			args: map[string]any{keyDomain: strings.Repeat("a", 254), keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyConfirm: true},
			want: "domain must be between 1 and 253 characters",
		},
		{
			name: "invalid domain pattern",
			args: map[string]any{keyDomain: "bad domain", keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyConfirm: true},
			want: "domain must match the documented domain-name pattern",
		},
		{
			name: caseMissingType,
			args: map[string]any{keyDomain: domainExample, keyConfirm: true},
			want: errTypeRequired,
		},
		{
			name: caseInvalidType,
			args: map[string]any{keyDomain: domainExample, keyType: "primary", keyConfirm: true},
			want: "type must be one of: master, slave",
		},
		{
			name: "master requires soa email",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keyConfirm: true},
			want: "soa_email is required for master domains",
		},
		{
			name: "slave requires master ips",
			args: map[string]any{keyDomain: domainExample, keyType: domainSlave, keyConfirm: true},
			want: "master_ips must include at least one value for slave domains",
		},
		{
			name: "invalid status",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyStatus: "edit_mode", keyConfirm: true},
			want: "status must be one of: active, disabled",
		},
		{
			name: "soa email type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: 1, keyConfirm: true},
			want: "soa_email must be a string",
		},
		{
			name: "description type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyDescription: 1, keyConfirm: true},
			want: "description must be a string",
		},
		{
			name: "group type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyGroup: 1, keyConfirm: true},
			want: "group must be a string",
		},
		{
			name: "status type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyStatus: 1, keyConfirm: true},
			want: "status must be a string",
		},
		{
			name: "axfr ips type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyAXFRIPs: 1, keyConfirm: true},
			want: "axfr_ips must be an array of strings",
		},
		{
			name: "master ips type",
			args: map[string]any{keyDomain: domainExample, keyType: domainSlave, keyMasterIPs: 1, keyConfirm: true},
			want: "master_ips must be an array of strings",
		},
		{
			name: "master ips item type",
			args: map[string]any{keyDomain: domainExample, keyType: domainSlave, keyMasterIPs: []any{domainMasterIPExample, 1}, keyConfirm: true},
			want: "master_ips must be an array of strings",
		},
		{
			name: "tags type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyTags: 1, keyConfirm: true},
			want: "tags must be an array of strings",
		},
		{
			name: "expire sec type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyExpireSec: "1", keyConfirm: true},
			want: "expire_sec must be an integer",
		},
		{
			name: "refresh sec type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyRefreshSec: "1", keyConfirm: true},
			want: "refresh_sec must be an integer",
		},
		{
			name: "retry sec type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyRetrySec: "1", keyConfirm: true},
			want: errRetrySecInteger,
		},
		{
			name: "retry sec nan",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyRetrySec: math.NaN(), keyConfirm: true},
			want: errRetrySecInteger,
		},
		{
			name: "retry sec infinity",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyRetrySec: math.Inf(1), keyConfirm: true},
			want: errRetrySecInteger,
		},
		{
			name: "retry sec fractional",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyRetrySec: 1.5, keyConfirm: true},
			want: errRetrySecInteger,
		},
		{
			name: "ttl sec type",
			args: map[string]any{keyDomain: domainExample, keyType: keyMaster, keySoaEmail: domainSOAEmailExample, keyTTLSec: "1", keyConfirm: true},
			want: "ttl_sec must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatal("result.IsError = false, want true")
			}

			if got := domainCreateResultText(t, result); !strings.Contains(got, tt.want) {
				t.Errorf("error text %q does not contain %q", got, tt.want)
			}
		})
	}
}

// TestLinodeDomainCreateForwardsOptionalBody proves an explicitly supplied
// optional field survives to the wire body, including the zero values a
// value-typed request struct with omitempty used to drop.
func TestLinodeDomainCreateForwardsOptionalBody(t *testing.T) {
	t.Parallel()

	wantBody := map[string]any{
		keyDomain:      domainExample,
		keyType:        domainSlave,
		keyAXFRIPs:     []any{"192.0.2.10"},
		keyDescription: "",
		keyExpireSec:   float64(604800),
		keyGroup:       "",
		keyMasterIPs:   []any{domainMasterIPExample},
		keyRefreshSec:  float64(14400),
		keyRetrySec:    float64(0),
		keyStatus:      statusDisabled,
		keySoaEmail:    "",
		keyTags:        []any{},
		keyTTLSec:      float64(300),
	}

	_, _, handler := tools.NewLinodeDomainCreateTool(&config.Config{})

	args := map[string]any{
		keyDomain:      domainExample,
		keyType:        domainSlave,
		keyAXFRIPs:     []any{"192.0.2.10"},
		keyDescription: "",
		keyExpireSec:   604800,
		keyGroup:       "",
		keyMasterIPs:   []any{domainMasterIPExample},
		keyRefreshSec:  float64(14400),
		keyRetrySec:    float64(0),
		keyStatus:      statusDisabled,
		keySoaEmail:    "",
		keyTags:        []string{},
		keyTTLSec:      float64(300),
		keyDryRun:      true,
	}

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("result.IsError = true: %s", domainCreateResultText(t, result))
	}

	wouldExecute := domainCreateWouldExecute(t, result)

	if got := wouldExecute["method"]; got != http.MethodPost {
		t.Errorf("would_execute.method = %v, want %v", got, http.MethodPost)
	}

	if got := wouldExecute["path"]; got != "/domains" {
		t.Errorf("would_execute.path = %v, want /domains", got)
	}

	if got := wouldExecute["body"]; !reflect.DeepEqual(got, wantBody) {
		t.Errorf("would_execute.body = %#v, want %#v", got, wantBody)
	}
}

// TestLinodeDomainCreateOmitsAbsentOptionalBody is the other half of the
// presence contract: a field nobody supplied must not appear in the body at
// all, so the API applies its own documented default.
func TestLinodeDomainCreateOmitsAbsentOptionalBody(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeDomainCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyDomain:   domainExample,
		keyType:     keyMaster,
		keySoaEmail: domainSOAEmailExample,
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		keyDomain:   domainExample,
		keyType:     keyMaster,
		keySoaEmail: domainSOAEmailExample,
	}

	if got := domainCreateWouldExecute(t, result)["body"]; !reflect.DeepEqual(got, want) {
		t.Errorf("would_execute.body = %#v, want %#v", got, want)
	}
}

// domainCreateWouldExecute decodes the dry-run preview and returns its
// would_execute object.
func domainCreateWouldExecute(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	var preview map[string]any
	if err := json.Unmarshal([]byte(domainCreateResultText(t, result)), &preview); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wouldExecute, isObject := preview["would_execute"].(map[string]any)
	if !isObject {
		t.Fatalf("would_execute = %T, want object", preview["would_execute"])
	}

	return wouldExecute
}

func domainCreateResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result.Content[0] = %T, want mcp.TextContent", result.Content[0])
	}

	return text.Text
}
