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

const (
	temporaryPromoCreditError = "temporary"
	promoCodeFixture          = "PROMO123"
)

func TestClientAddAccountPromoCreditSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		assert.Empty(t, r.URL.RawQuery, "promo credit request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr)

		if decodeErr != nil {
			return
		}

		assert.Equal(t, promoCodeFixture, body["promo_code"])
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	require.NoError(t, err, "AddAccountPromoCredit should succeed on 200 response")
}

func TestClientAddAccountPromoCreditAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	require.Error(t, err, "AddAccountPromoCredit should propagate API errors")
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientAddAccountPromoCreditDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/account/promo-codes", r.URL.Path, "request path should be /account/promo-codes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPromoCreditError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(2))

	err := client.AddAccountPromoCredit(t.Context(), &linode.AddAccountPromoCreditRequest{PromoCode: promoCodeFixture})

	require.Error(t, err, "AddAccountPromoCredit should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "mutating promo credit request must not be retried")
}
