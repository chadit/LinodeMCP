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
	keyTFAConfirmExpiry = "expiry"
	tfaConfirmExpiry    = "2026-01-01T00:00:00"
)

func TestClientConfirmProfileTFAEnableSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")

		var body map[string]string
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "expected no error")
		checkEqual(t, map[string]string{"tfa_code": "123456"}, body, "values differ")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{"scratch": "setup-token", keyTFAConfirmExpiry: tfaConfirmExpiry}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	requireNoError(t, err, "ConfirmProfileTFAEnable should succeed on 200 response")
	checkEqual(t, "setup-token", result["scratch"], "values differ")
	checkEqual(t, tfaConfirmExpiry, result[keyTFAConfirmExpiry], "values differ")
}

func TestClientConfirmProfileTFAEnableAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	requireError(t, err, "ConfirmProfileTFAEnable should fail on 403 response")

	_ = requireAPIError(t, err, "ConfirmProfileTFAEnable should return APIError")
}

func TestClientConfirmProfileTFAEnableNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	requireError(t, err, "ConfirmProfileTFAEnable should return the transient error")
	checkEqual(t, int32(1), calls.Load(), "security-state-changing POST must not be retried")
}
