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

const managedServiceUpdateToolName = "linode_managed_service_update"

func TestLinodeManagedServiceUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedServiceUpdateTool(cfg)

		assert.Equal(t, managedServiceUpdateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "managed service update should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyManagedServiceID, "schema should include service_id")
		assert.Contains(t, props, managedServiceLabelParam, "schema should include label")
		assert.Contains(t, props, managedServiceTypeParam, "schema should include service_type")
		assert.Contains(t, props, managedServiceAddressParam, "schema should include address")
		assert.Contains(t, props, managedServiceTimeoutParam, "schema should include timeout")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyManagedServiceID, "service_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.NotContains(t, tool.InputSchema.Required, managedServiceLabelParam, "update fields should be optional")
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

				_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

				args := validManagedServiceUpdateArgs()
				if !testCase.set {
					delete(args, keyConfirm)
				}

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

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
			{name: "missing service id", mutate: func(args map[string]any) { delete(args, keyManagedServiceID) }, wantMessage: keyManagedServiceID},
			{name: "zero service id", mutate: func(args map[string]any) { args[keyManagedServiceID] = 0 }, wantMessage: keyManagedServiceID},
			{name: "slash service id", mutate: func(args map[string]any) { args[keyManagedServiceID] = "9944/9955" }, wantMessage: keyManagedServiceID},
			{name: "query service id", mutate: func(args map[string]any) { args[keyManagedServiceID] = "9944?x=1" }, wantMessage: keyManagedServiceID},
			{name: "traversal service id", mutate: func(args map[string]any) { args[keyManagedServiceID] = pathTraversalValue }, wantMessage: keyManagedServiceID},
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
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.UpdateManagedServiceRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))

			if got.Label == nil || got.ServiceType == nil || got.Address == nil || got.Timeout == nil || got.Credentials == nil {
				t.Errorf("request body missing managed service update fields: %#v", got)

				return
			}

			assert.Equal(t, managedServiceLabelFixture, *got.Label)
			assert.Equal(t, managedServiceTypeURL, *got.ServiceType)
			assert.Equal(t, managedServiceAddressFixture, *got.Address)
			assert.Equal(t, 30, *got.Timeout)
			assert.Equal(t, []int{9991}, *got.Credentials)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedService{ID: managedServiceToolIDValue, Label: *got.Label, ServiceType: *got.ServiceType, Address: *got.Address, Timeout: *got.Timeout, Credentials: *got.Credentials, Status: "managed-service-updated"}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, validManagedServiceUpdateArgs()))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedServiceLabelFixture, "response should include label")
		assert.Contains(t, textContent.Text, "managed-service-updated", "response should include status")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceUpdateTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, validManagedServiceUpdateArgs()))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_managed_service_update")
		assertErrorContains(t, result, errForbidden)
	})
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
