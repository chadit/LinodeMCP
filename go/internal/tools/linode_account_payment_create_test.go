package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyUSD                       = "usd"
	accountPaymentCreatedMessage = "Account payment created successfully"
	errUSDPositive               = "usd must be a positive number"
	errUSDNonEmptyString         = "usd must be a non-empty string"
	paymentUSDString             = "25.5"
)

func TestLinodeAccountPaymentCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

	if tool.Name != "linode_account_payment_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyPaymentMethodID]; !ok {
		t.Errorf("props missing key %v", keyPaymentMethodID)
	}

	if _, ok := props[keyUSD]; !ok {
		t.Errorf("props missing key %v", keyUSD)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyUSD, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if slices.Contains(tool.InputSchema.Required, keyPaymentMethodID) {
		t.Errorf("tool.InputSchema.Required should not contain %v", keyPaymentMethodID)
	}
}

func TestLinodeAccountPaymentCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountPaymentsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentsTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if body[keyPaymentMethodID] != float64(123) {
			t.Errorf("value = %v, want %v", body[keyPaymentMethodID], float64(123))
		}

		if body[keyUSD] != paymentUSDString {
			t.Errorf("value = %v, want %v", body[keyUSD], paymentUSDString)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountPayment{ID: 456, Date: "2026-05-22T00:00:00", USD: 25.5}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: float64(123), keyUSD: paymentUSDString, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountPaymentCreatedMessage) {
		t.Errorf("textContent.Text does not contain %v", accountPaymentCreatedMessage)
	}

	if !strings.Contains(textContent.Text, "25.5") {
		t.Errorf("textContent.Text does not contain %v", "25.5")
	}
}

func TestLinodeAccountPaymentCreateToolSuccessWithDefaultPaymentMethod(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountPaymentsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentsTestPath)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if _, ok := body[keyPaymentMethodID]; ok {
			t.Errorf("body has unexpected key %v", keyPaymentMethodID)
		}

		if body[keyUSD] != paymentUSDString {
			t.Errorf("value = %v, want %v", body[keyUSD], paymentUSDString)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountPayment{ID: 457, Date: "2026-05-22T00:00:00", USD: 25.5}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyUSD: paymentUSDString, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeAccountPaymentCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountPaymentsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"reason": errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: float64(123), keyUSD: paymentUSDString, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to create linode_account_payment_create") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create linode_account_payment_create")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountPaymentCreateToolConfirmRejectsBeforeClient(t *testing.T) {
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

			args := map[string]any{keyPaymentMethodID: float64(123), keyUSD: paymentUSDString}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
				t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeAccountPaymentCreateToolInvalidInputsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing usd", args: map[string]any{keyPaymentMethodID: float64(123), keyConfirm: true}, want: "usd is required"},
		{name: "string payment method id", args: map[string]any{keyPaymentMethodID: "123", keyUSD: paymentUSDString, keyConfirm: true}, want: errPaymentMethodIDPositive},
		{name: "zero payment method id", args: map[string]any{keyPaymentMethodID: float64(0), keyUSD: paymentUSDString, keyConfirm: true}, want: errPaymentMethodIDPositive},
		{name: "negative payment method id", args: map[string]any{keyPaymentMethodID: float64(-1), keyUSD: paymentUSDString, keyConfirm: true}, want: errPaymentMethodIDPositive},
		{name: "empty usd", args: map[string]any{keyUSD: "", keyConfirm: true}, want: errUSDNonEmptyString},
		{name: "numeric usd type", args: map[string]any{keyUSD: float64(25.5), keyConfirm: true}, want: errUSDNonEmptyString},
		{name: "alpha usd", args: map[string]any{keyUSD: "abc", keyConfirm: true}, want: errUSDPositive},
		{name: "zero usd", args: map[string]any{keyUSD: "0", keyConfirm: true}, want: errUSDPositive},
		{name: "negative usd", args: map[string]any{keyUSD: "-1", keyConfirm: true}, want: errUSDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentCreateTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}
