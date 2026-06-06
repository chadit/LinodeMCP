package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyUSD                       = "usd"
	accountPaymentCreatedMessage = "Account payment created successfully"
	errUSDPositive               = "usd must be a positive number"
)

func TestLinodeAccountPaymentCreateTool(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

		assert.Equal(t, "linode_account_payment_create", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPaymentMethodID, "schema should include payment_method_id")
		assert.Contains(t, props, keyUSD, "schema should include usd")
		assert.Contains(t, props, keyConfirm, "mutating payment tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyUSD, "usd must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.NotContains(t, tool.InputSchema.Required, keyPaymentMethodID, "payment_method_id should remain optional")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr)

			if decodeErr != nil {
				return
			}

			assert.InDelta(t, 123, body[keyPaymentMethodID], 0.001)
			assert.InDelta(t, 25.5, body[keyUSD], 0.001)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 456, Date: "2026-05-22T00:00:00", USD: 25.5}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: "123", keyUSD: 25.5, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, accountPaymentCreatedMessage)
		assert.Contains(t, textContent.Text, "25.5")
	})

	t.Run("success with default payment method", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")

			var body map[string]any

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr)

			if decodeErr != nil {
				return
			}

			assert.NotContains(t, body, keyPaymentMethodID, "payment_method_id should be omitted when not supplied")
			assert.InDelta(t, 25.5, body[keyUSD], 0.001)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 457, Date: "2026-05-22T00:00:00", USD: 25.5}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyUSD: 25.5, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"reason": errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: "123", keyUSD: 25.5, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create linode_account_payment_create", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumeric, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

				args := map[string]any{keyPaymentMethodID: "123", keyUSD: 25.5}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "response should require confirmation")
			})
		}
	})

	t.Run("invalid inputs reject before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing usd", args: map[string]any{keyPaymentMethodID: "123", keyConfirm: true}, want: "usd is required"},
			{name: "empty payment method id", args: map[string]any{keyPaymentMethodID: "", keyUSD: 25.5, keyConfirm: true}, want: "payment_method_id must be a non-empty string"},
			{name: "numeric payment method id", args: map[string]any{keyPaymentMethodID: 123, keyUSD: 25.5, keyConfirm: true}, want: "payment_method_id must be a non-empty string"},
			{name: "slash payment method id", args: map[string]any{keyPaymentMethodID: "123/456", keyUSD: 25.5, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: "query payment method id", args: map[string]any{keyPaymentMethodID: "123?456", keyUSD: 25.5, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: "traversal payment method id", args: map[string]any{keyPaymentMethodID: pathTraversalValue, keyUSD: 25.5, keyConfirm: true}, want: errPaymentMethodIDNoSeparators},
			{name: "alpha payment method id", args: map[string]any{keyPaymentMethodID: "abc", keyUSD: 25.5, keyConfirm: true}, want: "payment_method_id must be a positive integer"},
			{name: "zero payment method id", args: map[string]any{keyPaymentMethodID: "0", keyUSD: 25.5, keyConfirm: true}, want: "payment_method_id must be a positive integer"},
			{name: "zero usd", args: map[string]any{keyUSD: 0, keyConfirm: true}, want: errUSDPositive},
			{name: "negative usd", args: map[string]any{keyUSD: -1, keyConfirm: true}, want: errUSDPositive},
			{name: "string usd", args: map[string]any{keyUSD: "25.5", keyConfirm: true}, want: errUSDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})
}
