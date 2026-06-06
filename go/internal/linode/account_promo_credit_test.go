package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	temporaryPromoCreditError = "temporary"
	promoCodeFixture          = "PROMO123"
)

func TestClientAddAccountPromoCreditSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		checkEmpty(t, r.URL.RawQuery, "promo credit request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "request should include bearer token")

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		checkNoError(t, decodeErr, "decode request body")

		if decodeErr != nil {
			return
		}

		checkEqual(t, promoCodeFixture, body["promo_code"], "promo code should be serialized")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encode response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	requireNoError(t, err, "AddAccountPromoCredit should succeed on 200 response")
}

func TestClientAddAccountPromoCreditAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	requireError(t, err, "AddAccountPromoCredit should propagate API errors")
	accountCheckForbiddenError(t, err)
}

func TestClientAddAccountPromoCreditDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPromoCreditError}}}), "encode error response body")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	requireError(t, err, "AddAccountPromoCredit should return the transient error")
	checkEqual(t, int32(1), calls.Load(), "mutating promo credit request must not be retried")
}
