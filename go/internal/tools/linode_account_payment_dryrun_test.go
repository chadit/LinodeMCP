package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	accountPaymentsTestPath       = "/account/payments"
	accountPaymentMethodsTestPath = "/account/payment-methods"
	accountPaymentMethodTestID    = "123"
	accountPaymentMethodGetPath   = accountPaymentMethodsTestPath + "/" + accountPaymentMethodTestID
	keyPaymentType                = "type"
	keyPaymentData                = "data"
	keyPaymentIsDefault           = "is_default"
	keyPaymentUSD                 = "usd"
)

func TestLinodeAccountPaymentCreateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without paying", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountPaymentCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentUSD: float64(25),
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_payment_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountPaymentsTestPath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeAccountPaymentMethodCreateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview never echoes the card data", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentType:      paymentMethodCreditCard,
			keyPaymentData:      map[string]any{"card_number": "4111111111111111"},
			keyPaymentIsDefault: true,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, "4111111111111111", "dry_run must not echo card data")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_account_payment_method_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountPaymentMethodsTestPath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeAccountPaymentMethodDeleteToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountPaymentMethodGetPath, linode.AccountPaymentMethod{Type: paymentMethodCreditCard})
		_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentMethodID: accountPaymentMethodTestID,
			keyDryRun:          true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_payment_method_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, accountPaymentMethodGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeAccountPaymentMethodMakeDefaultToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without changing default", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountPaymentMethodGetPath, linode.AccountPaymentMethod{Type: paymentMethodCreditCard})
		_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentMethodID: accountPaymentMethodTestID,
			keyDryRun:          true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_payment_method_make_default", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountPaymentMethodGetPath+"/make-default", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}
