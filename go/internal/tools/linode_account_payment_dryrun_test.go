package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
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
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without paying", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountPaymentCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentUSD: float64(25),
			keyDryRun:     true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_payment_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_payment_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountPaymentsTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountPaymentsTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeAccountPaymentMethodCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, "4111111111111111") {
			t.Errorf("preview should not contain %v", "4111111111111111")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_account_payment_method_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_payment_method_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountPaymentMethodsTestPath) {
			t.Errorf("got %v, want %v", would["path"], accountPaymentMethodsTestPath)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})
}

func TestLinodeAccountPaymentMethodDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountPaymentMethodGetPath, linode.AccountPaymentMethod{Type: paymentMethodCreditCard})
		_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentMethodID: accountPaymentMethodTestID,
			keyDryRun:          true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_payment_method_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_payment_method_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], accountPaymentMethodGetPath) {
			t.Errorf("got %v, want %v", would["path"], accountPaymentMethodGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeAccountPaymentMethodMakeDefaultToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without changing default", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, accountPaymentMethodGetPath, linode.AccountPaymentMethod{Type: paymentMethodCreditCard})
		_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPaymentMethodID: accountPaymentMethodTestID,
			keyDryRun:          true,
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

		if !reflect.DeepEqual(body["tool"], "linode_account_payment_method_make_default") {
			t.Errorf("got %v, want %v", body["tool"], "linode_account_payment_method_make_default")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], accountPaymentMethodGetPath+"/make-default") {
			t.Errorf("got %v, want %v", would["path"], accountPaymentMethodGetPath+"/make-default")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
