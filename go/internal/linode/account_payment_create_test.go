package linode_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const temporaryPaymentError = "temporary"

func accountCheckInDelta(t *testing.T, want, got any, message string) {
	t.Helper()

	wantFloat, wantOK := accountFloatValue(want)
	gotFloat, gotOK := accountFloatValue(got)

	if !wantOK || !gotOK {
		t.Errorf("%s: unsupported numeric comparison types %T and %T", message, want, got)

		return
	}

	const delta = 0.001
	if math.Abs(wantFloat-gotFloat) > delta {
		t.Errorf("%s: want %v within %v, got %v", message, wantFloat, delta, gotFloat)
	}
}

func accountFloatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func accountCheckContains(t *testing.T, collection map[string]any, key, message string) {
	t.Helper()

	if _, ok := collection[key]; !ok {
		t.Errorf("%s: expected key %q in map", message, key)
	}
}

func accountCheckNotContains(t *testing.T, collection map[string]any, key, message string) {
	t.Helper()

	if _, ok := collection[key]; ok {
		t.Errorf("%s: unexpected key %q in map", message, key)
	}
}

func accountCheckForbiddenError(t *testing.T, err error) {
	t.Helper()

	apiErr := requireAPIError(t, err, "error should be an API error")
	checkEqual(t, "forbidden", apiErr.Message, "error reason should match")
}

func TestClientCreateAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5}
	created := linode.AccountPayment{ID: 456, Date: "2026-05-22T00:00:00", USD: 25.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		checkEmpty(t, r.URL.RawQuery, "create request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "request should include bearer token")

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		checkNoError(t, decodeErr, "decode request body")

		if decodeErr != nil {
			return
		}

		accountCheckInDelta(t, 123, body["payment_method_id"], "payment_method_id should be serialized")
		accountCheckInDelta(t, 25.5, body["usd"], "usd should be serialized")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(created), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)

	requireNoError(t, err, "CreateAccountPayment should succeed on 200 response")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, 456, result.ID, "payment ID should match")
	accountCheckInDelta(t, 25.5, result.USD, "result USD should match")
}

func TestClientCreateAccountPaymentWithDefaultMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{USD: 25.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		checkNoError(t, decodeErr, "decode request body")

		if decodeErr != nil {
			return
		}

		accountCheckNotContains(t, body, "payment_method_id", "payment_method_id should be omitted when not supplied")
		accountCheckInDelta(t, 25.5, body["usd"], "usd should be serialized")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 457, Date: "2026-05-22T00:00:00", USD: 25.5}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)

	requireNoError(t, err, "CreateAccountPayment should succeed with the default payment method")
	requireNotNil(t, result, "result should not be nil")
	checkEqual(t, 457, result.ID, "payment ID should match")
}

func TestClientCreateAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "forbidden"}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})

	requireError(t, err, "CreateAccountPayment should propagate API errors")
	checkNil(t, result, "result should be nil")
	accountCheckForbiddenError(t, err)
}

func TestClientCreateAccountPaymentDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})

	requireError(t, err, "CreateAccountPayment should return the transient error")
	checkEqual(t, int32(1), calls.Load(), "mutating payment request must not be retried")
}
