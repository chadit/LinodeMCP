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

const managedServiceUpdateToolName = "linode_managed_service_update"

func TestLinodeManagedServiceUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeManagedServiceUpdateTool(cfg)

	if tool.Name != managedServiceUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, managedServiceUpdateToolName)
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyManagedServiceID]; !ok {
		t.Errorf("props missing key %v", keyManagedServiceID)
	}

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

	for _, key := range []string{keyManagedServiceID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if slices.Contains(tool.InputSchema.Required, managedServiceLabelParam) {
		t.Errorf("tool.InputSchema.Required should not contain %v", managedServiceLabelParam)
	}
}

func TestLinodeManagedServiceUpdateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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

			_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

			args := validManagedServiceUpdateArgs()
			if !testCase.set {
				delete(args, keyConfirm)
			}

			if testCase.set {
				args[keyConfirm] = testCase.value
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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceUpdateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		mutate      func(map[string]any)
		wantMessage string
	}{
		{name: caseMissingServiceID, mutate: func(args map[string]any) { delete(args, keyManagedServiceID) }, wantMessage: keyManagedServiceID},
		{name: caseZeroServiceID, mutate: func(args map[string]any) { args[keyManagedServiceID] = 0 }, wantMessage: keyManagedServiceID},
		{name: caseSlashServiceID, mutate: func(args map[string]any) { args[keyManagedServiceID] = invalidManagedServiceSlashID }, wantMessage: keyManagedServiceID},
		{name: caseQueryServiceID, mutate: func(args map[string]any) { args[keyManagedServiceID] = invalidManagedServiceQueryID }, wantMessage: keyManagedServiceID},
		{name: caseTraversalServiceID, mutate: func(args map[string]any) { args[keyManagedServiceID] = pathTraversalValue }, wantMessage: keyManagedServiceID},
		{name: caseNoUpdateFields, mutate: func(args map[string]any) { keepOnlyServiceIDAndConfirm(args) }, wantMessage: "at least one managed service field is required"},
		{name: "bad label type", mutate: func(args map[string]any) { args[managedServiceLabelParam] = 42 }, wantMessage: errLabelString},
		{name: "invalid type", mutate: func(args map[string]any) { args[managedServiceTypeParam] = "udp" }, wantMessage: errManagedServiceTypeInvalid},
		{name: "bad timeout", mutate: func(args map[string]any) { args[managedServiceTimeoutParam] = 0 }, wantMessage: errManagedServiceTimeoutInvalid},
		{name: "bad credentials", mutate: func(args map[string]any) { args[managedServiceCredentialsParam] = []any{float64(-1)} }, wantMessage: "credentials must be an array of positive integers"},
		{name: "empty credentials", mutate: func(args map[string]any) { args[managedServiceCredentialsParam] = []any{} }, wantMessage: "credentials must include at least one ID"},
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

			_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

			args := validManagedServiceUpdateArgs()
			testCase.mutate(args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeManagedServiceUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	label := managedServiceLabelFixture
	serviceType := managedServiceTypeURL
	address := managedServiceAddressFixture
	timeout := 30
	body := managedServiceBodyFixture
	consult := managedServiceConsultFixture
	want := linode.UpdateManagedServiceRequest{
		Label:             &label,
		ServiceType:       &serviceType,
		Address:           &address,
		Timeout:           &timeout,
		Body:              &body,
		ConsultationGroup: &consult,
		Credentials:       &[]int{9991},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.UpdateManagedServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %+v, want %+v", got, want)
		}

		if got.Label == nil || got.ServiceType == nil || got.Address == nil || got.Timeout == nil || got.Credentials == nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ManagedService{ID: managedServiceToolIDValue, Label: *got.Label, ServiceType: *got.ServiceType, Address: *got.Address, Timeout: *got.Timeout, Credentials: *got.Credentials, Status: "managed-service-updated"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, validManagedServiceUpdateArgs()))
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

	if !strings.Contains(textContent.Text, "managed-service-updated") {
		t.Errorf("textContent.Text does not contain %v", "managed-service-updated")
	}
}

func TestLinodeManagedServiceUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedServiceToolPathValue {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServiceToolPathValue)
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

	_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, validManagedServiceUpdateArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_managed_service_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_managed_service_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func validManagedServiceUpdateArgs() map[string]any {
	return map[string]any{
		keyManagedServiceID:            managedServiceToolIDValue,
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

func keepOnlyServiceIDAndConfirm(args map[string]any) {
	for key := range args {
		if key != keyManagedServiceID && key != keyConfirm {
			delete(args, key)
		}
	}
}
