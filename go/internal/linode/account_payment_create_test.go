package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	temporaryPaymentError = "temporary"
	paymentUSDString      = "25.5"

	accountPaymentProtoID       = 4242
	accountPaymentProtoMethodID = 7
	accountPaymentProtoDate     = "2026-07-29T18:04:01"
	accountPaymentProtoUSD      = 25.5

	accountPaymentProtoBody = `{"id":4242,"date":"2026-07-29T18:04:01","usd":25.5}`

	accountPaymentMethodProtoID   = 991
	accountPaymentMethodProtoBody = `{"id":991,"type":"credit_card","is_default":true,` +
		`"data":{"card_type":"Visa","last_four":"1111","expiry":"12/2030"}}`
	accountPaymentMethodProtoCardType = "Visa"
	accountPaymentMethodProtoLastFour = "1111"
)

// TestClientCreateAccountPaymentProtoSuccess verifies CreateAccountPaymentProto
// sends a POST to /account/payments carrying the amount and payment method, then
// decodes the response into the proto AccountPayment element.
func TestClientCreateAccountPaymentProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var sent map[string]any
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if sent["usd"] != paymentUSDString {
			t.Errorf("sent[usd] = %v, want %v", sent["usd"], paymentUSDString)
		}

		if sent["payment_method_id"] != float64(accountPaymentProtoMethodID) {
			t.Errorf("sent[payment_method_id] = %v, want %v", sent["payment_method_id"], accountPaymentProtoMethodID)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountPaymentProtoBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountPaymentProto(t.Context(), &linode.CreateAccountPaymentRequest{
		USD:             paymentUSDString,
		PaymentMethodID: accountPaymentProtoMethodID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != accountPaymentProtoID {
		t.Errorf("got.GetId() = %v, want %v", got.GetId(), accountPaymentProtoID)
	}

	if got.GetDate() != accountPaymentProtoDate {
		t.Errorf("got.GetDate() = %v, want %v", got.GetDate(), accountPaymentProtoDate)
	}

	if got.GetUsd() != accountPaymentProtoUSD {
		t.Errorf("got.GetUsd() = %v, want %v", got.GetUsd(), accountPaymentProtoUSD)
	}
}

// TestClientCreateAccountPaymentProtoDoesNotRetryTransientError verifies the
// payment POST is sent exactly once so a transient failure cannot charge the
// account twice.
func TestClientCreateAccountPaymentProtoDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: temporaryPaymentError}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	got, err := client.CreateAccountPaymentProto(t.Context(), &linode.CreateAccountPaymentRequest{
		USD:             paymentUSDString,
		PaymentMethodID: accountPaymentProtoMethodID,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// TestClientCreateAccountPaymentMethodProtoSuccess verifies
// CreateAccountPaymentMethodProto sends a POST to /account/payment-methods and
// decodes the response into the proto AccountPaymentMethod element, including
// the polymorphic data object that rides through the Struct field.
func TestClientCreateAccountPaymentMethodProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		assertAccountPaymentMethodProtoRequest(t, body)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(accountPaymentMethodProtoBody)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountPaymentMethodProto(t.Context(), newAccountPaymentMethodProtoRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != accountPaymentMethodProtoID {
		t.Errorf("got.GetId() = %v, want %v", got.GetId(), accountPaymentMethodProtoID)
	}

	if got.GetType() != paymentMethodCreditCard {
		t.Errorf("got.GetType() = %v, want %v", got.GetType(), paymentMethodCreditCard)
	}

	if !got.GetIsDefault() {
		t.Error("got.GetIsDefault() = false, want true")
	}

	lastFour := got.GetData().GetFields()[keyLastFour].GetStringValue()
	if lastFour != accountPaymentMethodProtoLastFour {
		t.Errorf("lastFour = %v, want %v", lastFour, accountPaymentMethodProtoLastFour)
	}

	// The data object is polymorphic per payment method type, so keys the
	// caller never named still have to survive the Struct round trip.
	cardType := got.GetData().GetFields()["card_type"].GetStringValue()
	if cardType != accountPaymentMethodProtoCardType {
		t.Errorf("cardType = %v, want %v", cardType, accountPaymentMethodProtoCardType)
	}
}

// TestClientCreateAccountPaymentMethodProtoAPIError verifies
// CreateAccountPaymentMethodProto propagates API errors instead of returning a
// zero-valued proto element.
func TestClientCreateAccountPaymentMethodProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcAccountPaymentMethods {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountPaymentMethodProto(t.Context(), newAccountPaymentMethodProtoRequest())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

// newAccountPaymentMethodProtoRequest builds the payment method both proto
// create tests send, so the request assertion and the call site cannot drift
// apart.
func newAccountPaymentMethodProtoRequest() *linode.CreateAccountPaymentMethodRequest {
	return &linode.CreateAccountPaymentMethodRequest{
		Type:      paymentMethodCreditCard,
		IsDefault: true,
		Data:      map[string]any{"card_token": paymentMethodToken},
	}
}

// assertAccountPaymentMethodProtoRequest checks the body the server received
// carries the method type, the default flag, and the nested data object.
func assertAccountPaymentMethodProtoRequest(t *testing.T, body []byte) {
	t.Helper()

	var sent struct {
		Data map[string]any `json:"data"`
		Type string         `json:"type"`
		// IsDefault is read back explicitly because a dropped flag would
		// silently leave the new method non-default.
		IsDefault bool `json:"is_default"`
	}

	if err := json.Unmarshal(body, &sent); err != nil {
		t.Errorf("unexpected error: %v", err)

		return
	}

	if sent.Type != paymentMethodCreditCard {
		t.Errorf("sent.Type = %v, want %v", sent.Type, paymentMethodCreditCard)
	}

	if !sent.IsDefault {
		t.Error("sent.IsDefault = false, want true")
	}

	if sent.Data["card_token"] != paymentMethodToken {
		t.Errorf("sent.Data[card_token] = %v, want %v", sent.Data["card_token"], paymentMethodToken)
	}
}
