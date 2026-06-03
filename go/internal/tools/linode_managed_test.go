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
	managedContactsToolPath              = "/managed/contacts"
	managedContactsToolName              = "John Doe"
	managedContactsToolEmail             = "john.doe@example.org"
	managedContactDeleteIDKey            = "contact_id"
	managedContactDeleteID               = float64(567)
	managedContactUpdateIDKey            = "contact_id"
	managedContactUpdateIDMessage        = "contact_id must be a positive integer"
	managedContactUpdateEmptyCase        = "empty update"
	managedContactUpdateMutableRequired  = "at least one mutable contact field is required"
	managedContactUpdateGroupKey         = "group"
	managedContactUpdateCaseNumericEmail = "numeric email"
)

func TestLinodeManagedContactDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

		assert.Equal(t, "linode_managed_contact_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destructive")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "delete tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, managedContactsToolPath+"/567", r.URL.Path, "request path should include contact ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedContactDeleteIDKey: managedContactDeleteID, keyConfirm: true, keyConfirmedDryRun: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted successfully", "response should confirm deletion")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		cases := map[string]any{
			caseMissingConfirm:         nil,
			caseFalseConfirmRejected:   false,
			caseStringConfirmRejected:  boolStringTrue,
			caseNumericConfirmRejected: float64(1),
		}

		t.Cleanup(func() {
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})

		for name, confirm := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

				args := map[string]any{managedContactDeleteIDKey: managedContactDeleteID}
				if confirm != nil {
					args[keyConfirm] = confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				assertErrorContains(t, result, errConfirmEqualsTrue)
			})
		}
	})

	t.Run("invalid contact id rejects before client", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		cases := map[string]any{
			caseMissing:             nil,
			caseSlash:               "56/7",
			caseQuery:               "567?x=1",
			caseDotTraversal:        pathTraversalValue,
			"oversized contact id":  float64(9007199254740992),
			caseZeroContactID:       float64(0),
			"negative contact id":   float64(-1),
			"fractional contact id": float64(567.5),
		}

		t.Cleanup(func() {
			assert.Equal(t, int32(0), calls.Load(), "contact ID rejection must happen before client call")
		})

		for name, contactID := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

				args := map[string]any{keyConfirm: true, keyConfirmedDryRun: true}
				if contactID != nil {
					args[managedContactDeleteIDKey] = contactID
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				assert.True(t, result.IsError, "invalid contact ID should be an error result")
				assertErrorContains(t, result, "contact_id must be a positive integer")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, managedContactsToolPath+"/567", r.URL.Path, "request path should include contact ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedContactDeleteIDKey: managedContactDeleteID, keyConfirm: true, keyConfirmedDryRun: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to delete linode_managed_contact_delete")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeManagedContactsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedContactsTool(cfg)

		assert.Equal(t, "linode_managed_contacts", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		contacts := linode.PaginatedResponse[linode.ManagedContact]{
			Data: []linode.ManagedContact{{
				ID:    567,
				Name:  managedContactsToolName,
				Email: managedContactsToolEmail,
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedContactsToolPath, r.URL.Path, "request path should be /managed/contacts")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(contacts))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedContactsToolName, "response should contain contact name")
		assert.Contains(t, textContent.Text, managedContactsToolEmail, "response should contain contact email")
	})

	t.Run("invalid pagination rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeManagedContactsTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "validation message should explain the bad argument")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, managedContactsToolPath, r.URL.Path, "request path should be /managed/contacts")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_managed_contacts", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeManagedContactUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

		assert.Equal(t, "linode_managed_contact_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.Contains(t, tool.InputSchema.Required, managedContactUpdateIDKey, "contact_id must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		contact := linode.ManagedContact{ID: 567, Name: managedContactsToolName, Email: managedContactsToolEmail}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)

			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedContactsToolPath+"/567", r.URL.Path, "request path should include contact ID")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Equal(t, managedContactsToolName, got[keyName])
			assert.Equal(t, managedContactsToolEmail, got[keyEmail])
			assert.Equal(t, "on-call", got[managedContactUpdateGroupKey])

			phone, ok := got["phone"].(map[string]any)
			if assert.True(t, ok, "phone should be object") {
				assert.Equal(t, "123-456-7890", phone["primary"])
				assert.Equal(t, "555-1212", phone["secondary"])
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(contact))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			managedContactUpdateIDKey:    567,
			keyName:                      managedContactsToolName,
			keyEmail:                     managedContactsToolEmail,
			managedContactUpdateGroupKey: "on-call",
			"phone_primary":              "123-456-7890",
			"phone_secondary":            "555-1212",
			keyConfirm:                   true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		assert.Equal(t, int32(1), calls.Load(), "client should be called once")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedContactsToolName, "response should contain contact name")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			confirm    any
			setConfirm bool
		}{
			{name: "missing confirm"},
			{name: caseFalseConfirm, confirm: false, setConfirm: true},
			{name: caseStringConfirm, confirm: boolStringTrue, setConfirm: true},
			{name: caseNumericConfirm, confirm: 1, setConfirm: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

				args := map[string]any{managedContactUpdateIDKey: 567, keyName: managedContactsToolName}
				if testCase.setConfirm {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm=true", "response should require confirmation")
			})
		}
	})

	t.Run("invalid input rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing contact id", args: map[string]any{keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
			{name: "zero contact id", args: map[string]any{managedContactUpdateIDKey: 0, keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
			{name: "slash contact id", args: map[string]any{managedContactUpdateIDKey: "5/67", keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
			{name: "query contact id", args: map[string]any{managedContactUpdateIDKey: "567?x=1", keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
			{name: "traversal contact id", args: map[string]any{managedContactUpdateIDKey: pathTraversalValue, keyName: managedContactsToolName, keyConfirm: true}, wantMessage: managedContactUpdateIDMessage},
			{name: managedContactUpdateEmptyCase, args: map[string]any{managedContactUpdateIDKey: 567, keyConfirm: true}, wantMessage: managedContactUpdateMutableRequired},
			{name: "numeric name", args: map[string]any{managedContactUpdateIDKey: 567, keyName: 123, keyConfirm: true}, wantMessage: "name must be a string"},
			{name: managedContactUpdateCaseNumericEmail, args: map[string]any{managedContactUpdateIDKey: 567, keyEmail: 123, keyConfirm: true}, wantMessage: "email must be a string"},
			{name: "numeric group", args: map[string]any{managedContactUpdateIDKey: 567, managedContactUpdateGroupKey: 123, keyConfirm: true}, wantMessage: "group must be a string"},
			{name: "numeric primary phone", args: map[string]any{managedContactUpdateIDKey: 567, "phone_primary": 123, keyConfirm: true}, wantMessage: "phone_primary must be a string"},
			{name: "numeric secondary phone", args: map[string]any{managedContactUpdateIDKey: 567, "phone_secondary": 123, keyConfirm: true}, wantMessage: "phone_secondary must be a string"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "validation message should explain the bad argument")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedContactsToolPath+"/567", r.URL.Path, "request path should include contact ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedContactUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{managedContactUpdateIDKey: 567, keyName: managedContactsToolName, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to update linode_managed_contact", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}
