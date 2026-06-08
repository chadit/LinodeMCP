package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
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
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPromoCreditTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without applying", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountPromoCreditTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPromoCode: "PROMO2026",
			keyDryRun:    true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_promo_credit") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_promo_credit")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountPromoCodesTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountPromoCodesTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeAccountAgreementsAcknowledgeToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountAgreementsAcknowledgeTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without acknowledging", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyBillingAgreement: true,
			keyDryRun:           true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_agreements_acknowledge") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_agreements_acknowledge")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountAgreementsTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountAgreementsTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}
	})
}

func TestLinodeAccountBetaEnrollToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountBetaEnrollTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without enrolling", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountBetaEnrollTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyBetaID: "beta-1",
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_beta_enroll") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_beta_enroll")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountBetasTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountBetasTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		if s, ok := sideEffects[0].(string); !ok || !strings.Contains(s, "beta-1") {
			t.Errorf("%q does not contain %q", s, "beta-1")
		}
	})
}

func TestLinodeAccountCancelToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountCancelTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without canceling", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountCancelTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_cancel") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_cancel")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountCancelTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountCancelTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		warnings, _ := body["warnings"].([]any)
		if len(warnings) != 1 {
			t.Fatalf("len(warnings) = %d, want %d", len(warnings), 1)
		}

		if s, ok := warnings[0].(string); !ok || !strings.Contains(s, "irreversible") {
			t.Errorf("%q does not contain %q", s, "irreversible")
		}
	})
}

func TestLinodeAccountEventSeenToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountEventSeenTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads event then would POST seen", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountEventGetPath, map[string]any{})
		_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyEventID: float64(accountEventTestID),
			keyDryRun:  true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_event_seen") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_event_seen")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountEventGetPath+"/seen") {
			t.Errorf("got %v, want %v", would["path"], accountEventGetPath+"/seen")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		if s, ok := sideEffects[0].(string); !ok || !strings.Contains(s, "earlier events") {
			t.Errorf("%q does not contain %q", s, "earlier events")
		}
	})
}

func TestLinodeAccountChildAccountTokenToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountChildAccountTokenTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview reads child account not the token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountChildGetPath, map[string]any{})
		_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyEUUID:  accountChildEUUID,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_child_account_token") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_child_account_token")
		}

		state, _ := body["current_state"].(map[string]any)
		if _, ok := state["token"]; ok {
			t.Errorf("state has unexpected key %v", "token")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountChildGetPath+"/token") {
			t.Errorf("got %v, want %v", would["path"], accountChildGetPath+"/token")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		if s, ok := sideEffects[0].(string); ok && strings.Contains(s, "token=") {
			t.Errorf("%q unexpectedly contains %q", s, "token=")
		}
	})
}
