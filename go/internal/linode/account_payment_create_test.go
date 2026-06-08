package linode_test

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const temporaryPaymentError = "temporary"

func TestClientCreateAccountPaymentSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5}
	created := linode.AccountPayment{ID: 456, Date: "2026-05-22T00:00:00", USD: 25.5}

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

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if numVal, numOK := body["payment_method_id"].(float64); !numOK || math.Abs(numVal-float64(123)) > 0.001 {
			t.Errorf("body[payment_method_id] = %v, want %v", body["payment_method_id"], 123)
		}

		if numVal, numOK := body["usd"].(float64); !numOK || math.Abs(numVal-float64(25.5)) > 0.001 {
			t.Errorf("body[usd] = %v, want %v", body["usd"], 25.5)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 456 {
		t.Errorf("result.ID = %v, want %v", result.ID, 456)
	}

	if math.Abs(result.USD-25.5) > 0.001 {
		t.Errorf("result.USD = %v, want %v", result.USD, 25.5)
	}
}

func TestClientCreateAccountPaymentWithDefaultMethodSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountPaymentRequest{USD: 25.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		if _, ok := body["payment_method_id"]; ok {
			t.Errorf("body has unexpected key %v", "payment_method_id")
		}

		if numVal, numOK := body["usd"].(float64); !numOK || math.Abs(numVal-float64(25.5)) > 0.001 {
			t.Errorf("body[usd] = %v, want %v", body["usd"], 25.5)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.AccountPayment{ID: 457, Date: "2026-05-22T00:00:00", USD: 25.5}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 457 {
		t.Errorf("result.ID = %v, want %v", result.ID, 457)
	}
}

func TestClientCreateAccountPaymentAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "forbidden"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientCreateAccountPaymentDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountPayments {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPayments)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	_, err := client.CreateAccountPayment(t.Context(), &linode.CreateAccountPaymentRequest{PaymentMethodID: 123, USD: 25.5})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
