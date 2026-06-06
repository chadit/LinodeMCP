package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	accountAgreementsTestPath    = "/account/agreements"
	accountBetasTestPath         = "/account/betas"
	accountCancelTestPath        = "/account/cancel"
	accountPromoCodesTestPath    = "/account/promo-codes"
	accountEventsTestPath        = "/account/events"
	accountChildAccountsTestPath = "/account/child-accounts"

	accountEventTestID  = 123
	accountChildEUUID   = "euuid-1"
	accountEventGetPath = accountEventsTestPath + "/123"
	accountChildGetPath = accountChildAccountsTestPath + "/" + accountChildEUUID
	keyBillingAgreement = "billing_agreement"
)

func TestLinodeAccountPromoCreditToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPromoCreditTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without applying", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountPromoCreditTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPromoCode: "PROMO2026",
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_promo_credit", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountPromoCodesTestPath, would["path"])
		assert.Nil(t, body["current_state"], "no existing resource to preview")
	})
}

func TestLinodeAccountAgreementsAcknowledgeToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountAgreementsAcknowledgeTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without acknowledging", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyBillingAgreement: true,
			keyDryRun:           true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_agreements_acknowledge", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountAgreementsTestPath, would["path"])
		assert.Nil(t, body["current_state"], "no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "acknowledge surfaces a side effect")
	})
}

func TestLinodeAccountBetaEnrollToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountBetaEnrollTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without enrolling", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountBetaEnrollTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyBetaID: "beta-1",
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_beta_enroll", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountBetasTestPath, would["path"])
		assert.Nil(t, body["current_state"], "no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "beta enroll surfaces a side effect")
		assert.Contains(t, sideEffects[0], "beta-1", "side effect names the beta program")
	})
}

func TestLinodeAccountCancelToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountCancelTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without canceling", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountCancelTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_cancel", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountCancelTestPath, would["path"])
		assert.Nil(t, body["current_state"], "no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "cancel surfaces a side effect")

		warnings, _ := body["warnings"].([]any)
		require.Len(t, warnings, 1, "cancel surfaces an irreversibility warning")
		assert.Contains(t, warnings[0], "irreversible", "warning flags the permanence")
	})
}

func TestLinodeAccountEventSeenToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountEventSeenTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads event then would POST seen", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountEventGetPath, map[string]any{})
		_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyEventID: float64(accountEventTestID),
			keyDryRun:  true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_event_seen", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountEventGetPath+"/seen", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "event seen surfaces a side effect")
		assert.Contains(t, sideEffects[0], "earlier events", "side effect notes the wider mark-seen behavior")
	})
}

func TestLinodeAccountChildAccountTokenToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountChildAccountTokenTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview reads child account not the token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountChildGetPath, map[string]any{})
		_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyEUUID:  accountChildEUUID,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_child_account_token", body["tool"])

		state, _ := body["current_state"].(map[string]any)
		assert.NotContains(t, state, "token", "dry_run current_state must not surface the proxy token")

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountChildGetPath+"/token", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run reads the child account metadata only")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "token create surfaces a side effect")
		assert.NotContains(t, sideEffects[0], "token=", "side effect must not echo a token value")
	})
}
