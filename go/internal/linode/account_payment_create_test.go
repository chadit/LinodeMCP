package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const temporaryPaymentError = "temporary"

func TestClientCreateAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5}
	created := linode.AccountPayment{ID: 456, Date: "2026-05-22T00:00:00", USD: 25.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		assert.Empty(t, r.URL.RawQuery, "create request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.InDelta(t, 123, body["payment_method_id"], 0.001)
		assert.InDelta(t, 25.5, body["usd"], 0.001)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)

	require.NoError(t, err, "CreateAccountPayment should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 456, result.ID)
	assert.InDelta(t, 25.5, result.USD, 0.001)
}

func TestClientCreateAccountPaymentWithDefaultMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{USD: 25.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.NotContains(t, body, "payment_method_id", "payment_method_id should be omitted when not supplied")
		assert.InDelta(t, 25.5, body["usd"], 0.001)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.AccountPayment{ID: 457, Date: "2026-05-22T00:00:00", USD: 25.5}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)

	require.NoError(t, err, "CreateAccountPayment should succeed with the default payment method")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 457, result.ID)
}

func TestClientCreateAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "forbidden"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})

	require.Error(t, err, "CreateAccountPayment should propagate API errors")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "forbidden")
}

func TestClientCreateAccountPaymentDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/payments", r.URL.Path, "request path should be /account/payments")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})

	require.Error(t, err, "CreateAccountPayment should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "mutating payment request must not be retried")
}
