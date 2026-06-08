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

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	managedServiceCreateToolName    = "linode_managed_service_create"
	managedServiceLabelParam        = "label"
	managedServiceTypeParam         = "service_type"
	managedServiceAddressParam      = "address"
	managedServiceTimeoutParam      = "timeout"
	managedServiceBodyParam         = "body"
	managedServiceConsultParam      = "consultation_group"
	managedServiceCredentialsParam  = "credentials"
	managedServiceCreateEndpoint    = "/managed/services"
	managedServiceLabelFixture      = "prod-1"
	managedServiceAddressFixture    = "https://example.org"
	managedServiceBodyFixture       = "it worked"
	managedServiceConsultFixture    = "on-call"
	errManagedServiceTypeInvalid    = "service_type must be url or tcp"
	errManagedServiceTimeoutInvalid = "timeout must be an integer between 1 and 255"
)

func TestLinodeManagedServiceCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

	if tool.Name != managedServiceCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedServiceCreateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[managedServiceLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props[managedServiceTypeParam]; !ok {
		t.Errorf("props missing key %v", managedServiceTypeParam)
	}

	if _, ok := props[managedServiceAddressParam]; !ok {
		t.Errorf("props missing key %v", managedServiceAddressParam)
	}

	if _, ok := props[managedServiceTimeoutParam]; !ok {
		t.Errorf("props missing key %v", managedServiceTimeoutParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{managedServiceLabelParam, managedServiceTypeParam, managedServiceAddressParam, managedServiceTimeoutParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeManagedServiceCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
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

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

			args := validManagedServiceArgs()
			if !testCase.set {
				delete(args, keyConfirm)
			}

			if testCase.set {
				args[keyConfirm] = testCase.value
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

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceCreateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		mutate      func(map[string]any)
		wantMessage string
	}{
		{name: caseMissingLabel, mutate: func(args map[string]any) { delete(args, managedServiceLabelParam) }, wantMessage: errLabelRequired},
		{name: "invalid type", mutate: func(args map[string]any) { args[managedServiceTypeParam] = "udp" }, wantMessage: errManagedServiceTypeInvalid},
		{name: "bad timeout", mutate: func(args map[string]any) { args[managedServiceTimeoutParam] = 0 }, wantMessage: errManagedServiceTimeoutInvalid},
		{name: "bad credentials", mutate: func(args map[string]any) { args[managedServiceCredentialsParam] = []any{float64(-1)} }, wantMessage: "credentials must be an array of positive integers"},
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

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

			args := validManagedServiceArgs()
			testCase.mutate(args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceCreateToolSuccess(t *testing.T) {
	t.Parallel()

	body := managedServiceBodyFixture
	consult := managedServiceConsultFixture
	want := linode.CreateManagedServiceRequest{
		Label:             managedServiceLabelFixture,
		ServiceType:       managedServiceTypeURL,
		Address:           managedServiceAddressFixture,
		Timeout:           30,
		Body:              &body,
		ConsultationGroup: &consult,
		Credentials:       []int{9991},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedServiceCreateEndpoint {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceCreateEndpoint)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreateManagedServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %+v, want %+v", got, want)
		}

		if got.Body == nil || got.ConsultationGroup == nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedService{ID: 9944, Label: got.Label, ServiceType: got.ServiceType, Address: got.Address, Timeout: got.Timeout, Body: got.Body, ConsultationGroup: *got.ConsultationGroup, Credentials: got.Credentials, Status: "managed-service-ok"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

	req := createRequestWithArgs(t, validManagedServiceArgs())

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

	if !strings.Contains(textContent.Text, managedServiceLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", managedServiceLabelFixture)
	}

	if !strings.Contains(textContent.Text, "managed-service-ok") {
		t.Errorf("textContent.Text does not contain %v", "managed-service-ok")
	}
}

func validManagedServiceArgs() map[string]any {
	return map[string]any{
		managedServiceLabelParam:       managedServiceLabelFixture,
		managedServiceTypeParam:        managedServiceTypeURL,
		managedServiceAddressParam:     managedServiceAddressFixture,
		managedServiceTimeoutParam:     30,
		managedServiceBodyParam:        managedServiceBodyFixture,
		managedServiceConsultParam:     managedServiceConsultFixture,
		managedServiceCredentialsParam: []any{float64(9991)},
		keyConfirm:                     true,
	}
}
