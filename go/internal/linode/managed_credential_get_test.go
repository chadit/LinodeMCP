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
	managedCredentialID            = 9991
	managedCredentialLabel         = "prod-password-1"
	managedCredentialLastDecrypted = "2018-01-01T00:01:01"
	managedCredentialPath          = "/managed/credentials/9991"
)

func TestClientGetManagedCredentialSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedCredentialPath, r.URL.Path, "request path should include credential ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel, LastDecrypted: managedCredentialLastDecrypted}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.GetManagedCredential(t.Context(), managedCredentialID)

	requireNoError(t, err, "GetManagedCredential should succeed on 200 response")
	requireNotNil(t, got)
	checkEqual(t, managedCredentialID, got.ID)
	checkEqual(t, managedCredentialLabel, got.Label)
	checkEqual(t, managedCredentialLastDecrypted, got.LastDecrypted)
}

func TestClientGetManagedCredentialAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedCredentialPath, r.URL.Path, "request path should include credential ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "access denied"}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedCredential(t.Context(), managedCredentialID)

	requireError(t, err)
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetManagedCredentialRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, managedCredentialPath, r.URL.Path, "request path should include credential ID")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedCredential{ID: managedCredentialID, Label: managedCredentialLabel}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
	got, err := client.GetManagedCredential(t.Context(), managedCredentialID)

	requireNoError(t, err, "safe GET should retry after transient API error")
	requireNotNil(t, got)
	checkEqual(t, int32(2), attempts.Load(), "safe GET should retry once")
}
