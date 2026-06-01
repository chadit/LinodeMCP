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
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	supportTicketReplyCreateToolName = "linode_account_support_ticket_reply_create"
	supportTicketReplyDescription    = "Thanks, here is more detail."
	supportTicketReplyCreatedBy      = "adevi"
)

func TestLinodeAccountSupportTicketReplyCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(&config.Config{})

		assert.Equal(t, supportTicketReplyCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "support ticket reply creation should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, supportTicketAttachmentTicketID, "schema should include ticket_id")
		assert.Contains(t, props, keyDescription, "schema should include description")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, supportTicketAttachmentTicketID, "ticket_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyDescription, "description must be marked required")
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
				_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

				args := map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: supportTicketReplyDescription}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or false confirm should be a tool error")
				assertErrorContains(t, result, "confirm=true")
				assert.Equal(t, int32(0), calls, "confirmation must fail before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/support/tickets/123/replies", r.URL.Path, "request path should include ticket ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, supportTicketReplyDescription, got[keyDescription])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: 456, keyDescription: supportTicketReplyDescription, "created_by": supportTicketReplyCreatedBy}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			supportTicketAttachmentTicketID: float64(123),
			keyDescription:                  supportTicketReplyDescription,
			keyConfirm:                      true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, supportTicketReplyDescription, "response should include description")
		assert.Contains(t, textContent.Text, supportTicketReplyCreatedBy, "response should include creator")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/support/tickets/123/replies", r.URL.Path, "request path should include ticket ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: supportTicketReplyDescription, keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create support ticket reply")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeAccountSupportTicketReplyCreateToolRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingTicketID, args: map[string]any{keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketIDRequired},
		{name: caseZeroTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(0), keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: caseFractionalTicketID, args: map[string]any{supportTicketAttachmentTicketID: float64(1.5), keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id separator", args: map[string]any{supportTicketAttachmentTicketID: "123/replies", keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id query", args: map[string]any{supportTicketAttachmentTicketID: databaseInvalidInstanceIDQuery, keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "string ticket id traversal", args: map[string]any{supportTicketAttachmentTicketID: "..", keyDescription: supportTicketReplyDescription, keyConfirm: true}, wantMessage: errSupportTicketAttachmentIDPositive},
		{name: "missing description", args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyConfirm: true}, wantMessage: "description is required"},
		{name: caseBlankDescription, args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: blankString, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
		{name: "numeric description", args: map[string]any{supportTicketAttachmentTicketID: float64(123), keyDescription: float64(1), keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			require.NoError(t, err, "handler should not return transport error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid request should be a tool error")
			assertErrorContains(t, result, testCase.wantMessage)
			assert.Equal(t, int32(0), calls, "request validation must fail before client call")
		})
	}
}

func TestLinodeAccountSupportTicketReplyCreateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeAccountSupportTicketReplyCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		supportTicketAttachmentTicketID: float64(123),
		keyDescription:                  supportTicketReplyDescription,
		keyDryRun:                       true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, supportTicketReplyCreateToolName, body["tool"])

	would, _ := body["would_execute"].(map[string]any)
	assert.Equal(t, "POST", would["method"])
	assert.Equal(t, "/support/tickets/123/replies", would["path"])
	bodyPreview, _ := would["body"].(map[string]any)
	assert.Equal(t, supportTicketReplyDescription, bodyPreview[keyDescription])

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "reply create surfaces a side effect")

	effect, gotString := sideEffects[0].(string)
	require.True(t, gotString)
	assert.Contains(t, effect, "ticket 123", "side effect should name the ticket")
}
