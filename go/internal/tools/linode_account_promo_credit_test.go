package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyPromoCode                     = "promo_code"
	accountPromoCreditAppliedMessage = "Account promo credit applied successfully"
	promoCodeFixture                 = "PROMO123"
)

func TestLinodeAccountPromoCreditToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPromoCreditTool(cfg)

	if tool.Name != "linode_account_promo_credit_add" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_promo_credit_add")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPromoCode) {
		t.Errorf("RawInputSchema missing key %v", keyPromoCode)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	for _, key := range []string{keyPromoCode, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing required key %v", key)
		}
	}
}

func TestLinodeAccountPromoCreditToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/promo-codes" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/promo-codes")
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

		if !reflect.DeepEqual(body[keyPromoCode], promoCodeFixture) {
			t.Errorf("body[keyPromoCode] = %v, want %v", body[keyPromoCode], promoCodeFixture)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPromoCreditTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPromoCode: promoCodeFixture, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, accountPromoCreditAppliedMessage) {
		t.Errorf("textContent.Text does not contain %v", accountPromoCreditAppliedMessage)
	}

	if !strings.Contains(textContent.Text, promoCodeFixture) {
		t.Errorf("textContent.Text does not contain %v", promoCodeFixture)
	}
}

func TestLinodeAccountPromoCreditToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/promo-codes" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/promo-codes")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPromoCreditTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPromoCode: promoCodeFixture, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Failed to apply linode_account_promo_credit_add") {
		t.Errorf("textContent.Text does not contain %v", "Failed to apply linode_account_promo_credit_add")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountPromoCreditToolConfirmRejectsBeforeClient(t *testing.T) {
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
			_, _, handler := tools.NewLinodeAccountPromoCreditTool(cfg)

			args := map[string]any{keyPromoCode: promoCodeFixture}
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

func TestLinodeAccountPromoCreditToolInvalidInputsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing promo code", args: map[string]any{keyConfirm: true}, want: "promo_code is required"},
		{name: "empty promo code", args: map[string]any{keyPromoCode: "", keyConfirm: true}, want: "promo_code must be a non-empty string"},
		{name: "numeric promo code", args: map[string]any{keyPromoCode: 123, keyConfirm: true}, want: "promo_code must be a non-empty string"},
		{name: "leading whitespace promo code", args: map[string]any{keyPromoCode: " PROMO123", keyConfirm: true}, want: "promo_code must not include leading or trailing whitespace"},
		{name: "trailing whitespace promo code", args: map[string]any{keyPromoCode: "PROMO123 ", keyConfirm: true}, want: "promo_code must not include leading or trailing whitespace"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPromoCreditTool(cfg)
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
