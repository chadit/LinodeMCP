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
	managedContactCreateToolName      = "linode_managed_contact_create"
	managedContactNameParam           = "contact_name"
	managedContactEmailParam          = "contact_email"
	managedContactNameFixture         = "John Doe"
	managedContactEmailFixture        = "john.doe@example.org"
	managedContactGroupFixture        = "on-call"
	managedContactPhoneFixture        = "123-456-7890"
	managedContactPhonePrimaryParam   = "phone_primary"
	managedContactPhoneSecondaryParam = "phone_secondary"
	managedContactCreateEndpoint      = "/managed/contacts"
	errManagedContactFieldRequired    = "at least one managed contact field is required"
	errManagedContactReadOnlyField    = "id and updated are read-only"
	errManagedContactNameNonEmpty     = "name must be a non-empty string"
	errManagedContactEmailNonEmpty    = "email must be a non-empty string"
	errManagedContactPhoneNonEmpty    = "phone_primary must be a non-empty string"
)

func TestLinodeManagedContactCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedContactCreateTool(cfg)

		assert.Equal(t, managedContactCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "managed contact creation should be CapAdmin")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, managedContactNameParam, "schema should include contact_name")
		assert.Contains(t, props, managedContactEmailParam, "schema should include contact_email")
		assert.Contains(t, props, managedContactPhonePrimaryParam, "schema should include phone_primary")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
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
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

				args := map[string]any{managedContactNameParam: managedContactNameFixture}
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
			args        map[string]any
			wantMessage string
		}{
			{name: "empty request", args: map[string]any{keyConfirm: true}, wantMessage: errManagedContactFieldRequired},
			{name: "managed contact read-only id", args: map[string]any{keyBetaID: 123, managedContactNameParam: managedContactNameFixture, keyConfirm: true}, wantMessage: errManagedContactReadOnlyField},
			{name: "read only updated", args: map[string]any{keyUpdated: "2018-01-01T00:01:01", managedContactNameParam: managedContactNameFixture, keyConfirm: true}, wantMessage: errManagedContactReadOnlyField},
			{name: "managed contact empty name", args: map[string]any{managedContactNameParam: "", keyConfirm: true}, wantMessage: errManagedContactNameNonEmpty},
			{name: "managed contact numeric email", args: map[string]any{managedContactEmailParam: 123, keyConfirm: true}, wantMessage: errManagedContactEmailNonEmpty},
			{name: "blank phone", args: map[string]any{managedContactPhonePrimaryParam: blankString, keyConfirm: true}, wantMessage: errManagedContactPhoneNonEmpty},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
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
			assert.Equal(t, managedContactCreateEndpoint, r.URL.Path, "request path should be /managed/contacts")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateManagedContactRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))

			if got.Name == nil || got.Email == nil || got.Group == nil || got.Phone == nil || got.Phone.Primary == nil {
				t.Errorf("request body missing managed contact fields: %#v", got)

				return
			}

			assert.Equal(t, managedContactNameFixture, *got.Name)
			assert.Equal(t, managedContactEmailFixture, *got.Email)
			assert.Equal(t, managedContactGroupFixture, *got.Group)
			assert.Equal(t, managedContactPhoneFixture, *got.Phone.Primary)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedContact{ID: 567, Name: managedContactNameFixture, Email: managedContactEmailFixture, Group: got.Group, Phone: linode.ManagedContactPhone{Primary: got.Phone.Primary}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedContactNameParam: managedContactNameFixture, managedContactEmailParam: managedContactEmailFixture, "group": managedContactGroupFixture, managedContactPhonePrimaryParam: managedContactPhoneFixture, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedContactNameFixture, "response should include name")
		assert.Contains(t, textContent.Text, managedContactEmailFixture, "response should include email")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, managedContactCreateEndpoint, r.URL.Path, "request path should be /managed/contacts")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedContactNameParam: managedContactNameFixture, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_managed_contact_create")
		assertErrorContains(t, result, errForbidden)
	})
}
