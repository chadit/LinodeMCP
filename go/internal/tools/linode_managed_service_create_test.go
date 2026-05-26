package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestLinodeManagedServiceCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

		assert.Equal(t, managedServiceCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "managed service creation should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, managedServiceLabelParam, "schema should include label")
		assert.Contains(t, props, managedServiceTypeParam, "schema should include service_type")
		assert.Contains(t, props, managedServiceAddressParam, "schema should include address")
		assert.Contains(t, props, managedServiceTimeoutParam, "schema should include timeout")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, managedServiceLabelParam, "label must be marked required")
		assert.Contains(t, tool.InputSchema.Required, managedServiceTypeParam, "service_type must be marked required")
		assert.Contains(t, tool.InputSchema.Required, managedServiceAddressParam, "address must be marked required")
		assert.Contains(t, tool.InputSchema.Required, managedServiceTimeoutParam, "timeout must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
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

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
			})
		}
	})

	t.Run("invalid request rejected before client call", func(t *testing.T) {
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

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid request should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "request validation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedServiceCreateEndpoint, r.URL.Path, "request path should be /managed/services")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateManagedServiceRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, managedServiceLabelFixture, got.Label)
			assert.Equal(t, "url", got.ServiceType)
			assert.Equal(t, managedServiceAddressFixture, got.Address)
			assert.Equal(t, 30, got.Timeout)

			if got.Body == nil || got.ConsultationGroup == nil {
				t.Errorf("request body missing optional managed service fields: %#v", got)

				return
			}

			assert.Equal(t, managedServiceBodyFixture, *got.Body)
			assert.Equal(t, managedServiceConsultFixture, *got.ConsultationGroup)
			assert.Equal(t, []int{9991}, got.Credentials)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedService{ID: 9944, Label: got.Label, ServiceType: got.ServiceType, Address: got.Address, Timeout: got.Timeout, Body: got.Body, ConsultationGroup: *got.ConsultationGroup, Credentials: got.Credentials, Status: "managed-service-ok"}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedServiceCreateTool(cfg)

		req := createRequestWithArgs(t, validManagedServiceArgs())
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedServiceLabelFixture, "response should include label")
		assert.Contains(t, textContent.Text, "managed-service-ok", "response should include status")
	})
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
