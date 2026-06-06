package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	supportTicketCreateToolName = "linode_account_support_ticket_create"
	supportTicketCreateSummary  = "Need help"
	supportTicketCreateBody     = "Instance is unreachable"
	errSummaryRequired          = "summary is required"
	errSummaryNonEmpty          = "summary must be a non-empty string"
	errDescriptionRequired      = "description is required"
	errDescriptionNonEmpty      = "description must be a non-empty string"
	errSupportTicketIDPositive  = "linode_id must be a positive integer"
	errSupportTicketRegion      = "region must be a non-empty string"
	keySupportTicketLinodeID    = "linode_id"
	keySupportTicketRegion      = "region"
	keySupportTicketID          = "id"
	keySupportTicketSummary     = "summary"
	caseMissingSummary          = "missing summary"
	caseEmptySummary            = "empty summary"
	caseBlankSummary            = "blank summary"
	caseBlankDescription        = "blank description"
	caseNumericSummary          = "numeric summary"
	supportTicketStatusOpen     = "open"
	supportTicketSeverity       = "major"
)

func TestLinodeAccountSupportTicketCreateRejectsInvalidOptionalFields(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	cases := []struct {
		name        string
		field       string
		value       any
		wantMessage string
	}{
		{name: "zero id", field: keySupportTicketLinodeID, value: float64(0), wantMessage: errSupportTicketIDPositive},
		{name: "negative id", field: keySupportTicketLinodeID, value: float64(-1), wantMessage: errSupportTicketIDPositive},
		{name: "fractional id", field: keySupportTicketLinodeID, value: float64(1.5), wantMessage: errSupportTicketIDPositive},
		{name: "oversized id", field: keySupportTicketLinodeID, value: float64(9007199254740992), wantMessage: errSupportTicketIDPositive},
		{name: "string id", field: keySupportTicketLinodeID, value: "12345", wantMessage: errSupportTicketIDPositive},
		{name: "object id", field: keySupportTicketLinodeID, value: map[string]any{keySupportTicketID: float64(12345)}, wantMessage: errSupportTicketIDPositive},
		{name: "blank region", field: keySupportTicketRegion, value: blankString, wantMessage: errSupportTicketRegion},
		{name: "numeric region", field: keySupportTicketRegion, value: float64(1), wantMessage: errSupportTicketRegion},
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
			_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

			args := map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody, keyConfirm: true}
			args[testCase.field] = testCase.value

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return transport error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid optional field should be a tool error")
			assertErrorContains(t, result, testCase.wantMessage)
			assert.Equal(t, int32(0), calls, "request validation must fail before client call")
		})
	}
}

func TestLinodeAccountSupportTicketCreateTool(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeAccountSupportTicketCreateTool(&config.Config{})

		assert.Equal(t, supportTicketCreateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "support ticket creation should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keySupportTicketSummary, "schema should include summary")
		assert.Contains(t, props, keyDescription, "schema should include description")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, props, keySupportTicketLinodeID, "schema should include optional linode_id")
		assert.Contains(t, tool.InputSchema.Required, keySupportTicketSummary, "summary must be marked required")
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
				_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

				args := map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody}
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
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingSummary, args: map[string]any{keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryRequired},
			{name: caseEmptySummary, args: map[string]any{keySupportTicketSummary: "", keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
			{name: caseBlankSummary, args: map[string]any{keySupportTicketSummary: blankString, keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
			{name: caseNumericSummary, args: map[string]any{keySupportTicketSummary: 123, keyDescription: supportTicketCreateBody, keyConfirm: true}, wantMessage: errSummaryNonEmpty},
			{name: "missing description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyConfirm: true}, wantMessage: errDescriptionRequired},
			{name: "empty description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: "", keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
			{name: caseBlankDescription, args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: blankString, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
			{name: "numeric description", args: map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: 123, keyConfirm: true}, wantMessage: errDescriptionNonEmpty},
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
				_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

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
			assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, supportTicketCreateSummary, got["summary"])
			assert.Equal(t, supportTicketCreateBody, got["description"])
			assert.Equal(t, "backups", got["bucket"])
			assert.InDelta(t, float64(23456), got["database_id"], 0)
			assert.InDelta(t, float64(34567), got["domain_id"], 0)
			assert.InDelta(t, float64(45678), got["firewall_id"], 0)
			assert.InDelta(t, float64(12345), got[keySupportTicketLinodeID], 0)
			assert.InDelta(t, float64(56789), got["lkecluster_id"], 0)
			assert.InDelta(t, float64(67890), got["longviewclient_id"], 0)
			assert.Equal(t, "managed", got["managed_issue"])
			assert.InDelta(t, float64(78901), got["nodebalancer_id"], 0)
			assert.Equal(t, placementGroupCreateRegion, got[keySupportTicketRegion])
			assert.Equal(t, supportTicketSeverity, got["severity"])
			assert.Equal(t, "vlan-a", got["vlan"])
			assert.InDelta(t, float64(89012), got["volume_id"], 0)
			assert.InDelta(t, float64(90123), got["vpc_id"], 0)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.SupportTicket{ID: 987, Summary: supportTicketCreateSummary, Description: supportTicketCreateBody, Status: supportTicketStatusOpen}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keySupportTicketSummary:  supportTicketCreateSummary,
			keyDescription:           supportTicketCreateBody,
			"bucket":                 "backups",
			"database_id":            float64(23456),
			"domain_id":              float64(34567),
			"firewall_id":            float64(45678),
			keySupportTicketLinodeID: float64(12345),
			"lkecluster_id":          float64(56789),
			"longviewclient_id":      float64(67890),
			"managed_issue":          "managed",
			"nodebalancer_id":        float64(78901),
			keySupportTicketRegion:   placementGroupCreateRegion,
			"severity":               supportTicketSeverity,
			"vlan":                   "vlan-a",
			"volume_id":              float64(89012),
			keyVPCID:                 float64(90123),
			keyConfirm:               true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, supportTicketCreateSummary, "response should include summary")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/support/tickets", r.URL.Path, "request path should be /support/tickets")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keySupportTicketSummary: supportTicketCreateSummary, keyDescription: supportTicketCreateBody, keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_account_support_ticket_create")
		assertErrorContains(t, result, errForbidden)
	})
}

func TestLinodeAccountSupportTicketCreateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	_, _, handler := tools.NewLinodeAccountSupportTicketCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keySupportTicketSummary:  supportTicketCreateSummary,
		keyDescription:           supportTicketCreateBody,
		keySupportTicketLinodeID: float64(12345),
		"severity":               supportTicketSeverity,
		keyDryRun:                true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, supportTicketCreateToolName, body["tool"])

	would, _ := body["would_execute"].(map[string]any)
	assert.Equal(t, "POST", would["method"])
	assert.Equal(t, "/support/tickets", would["path"])
	bodyPreview, _ := would["body"].(map[string]any)
	assert.Equal(t, supportTicketCreateSummary, bodyPreview[keySupportTicketSummary])
	assert.Equal(t, supportTicketCreateBody, bodyPreview[keyDescription])
	assert.InDelta(t, float64(12345), bodyPreview[keySupportTicketLinodeID], 0)
	assert.Equal(t, supportTicketSeverity, bodyPreview["severity"])
	assert.Nil(t, body["current_state"], "create has no existing resource to preview")

	sideEffects, _ := body["side_effects"].([]any)
	require.Len(t, sideEffects, 1, "create surfaces the new-ticket side effect")

	effect, gotString := sideEffects[0].(string)
	require.True(t, gotString)
	assert.Contains(t, effect, supportTicketCreateSummary, "side effect should name the ticket summary")
}
