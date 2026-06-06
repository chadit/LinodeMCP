package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedContactID         = 174
	managedContactPath       = "/managed/contacts/174"
	managedContactGetName    = "John Doe"
	managedContactGetEmail   = "john.doe@example.org"
	managedContactGetPhone   = "123-456-7890"
	managedContactGetUpdated = "2018-01-01T00:01:01"
)

func TestClientGetManagedContactSuccess(t *testing.T) {
	t.Parallel()

	phone := managedContactGetPhone
	contact := linode.ManagedContact{
		ID:      managedContactID,
		Name:    managedContactGetName,
		Email:   managedContactGetEmail,
		Phone:   linode.ManagedContactPhone{Primary: &phone},
		Updated: managedContactGetUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedContactPath, r.URL.Path, "request path should include encoded contact ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err, "reading request body should not fail")
		checkEmpty(t, body, "request body should be empty")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(contact))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedContact(t.Context(), managedContactID)

	requireNoError(t, err, "GetManagedContact should succeed")
	requireNotNil(t, result, "contact should be returned")
	checkEqual(t, managedContactID, result.ID)
	checkEqual(t, managedContactGetName, result.Name)
	checkEqual(t, managedContactGetEmail, result.Email)
	requireNotNil(t, result.Phone.Primary)
	checkEqual(t, managedContactGetPhone, *result.Phone.Primary)
}

func TestClientGetManagedContactRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	contact := linode.ManagedContact{ID: managedContactID, Name: managedContactGetName, Email: managedContactGetEmail}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedContactPath, r.URL.Path, "request path should include contact ID")

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(contact))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"my-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithJitter(false),
	)
	result, err := client.GetManagedContact(t.Context(), managedContactID)

	requireNoError(t, err, "GetManagedContact should retry transient failures")
	requireNotNil(t, result, "contact should be returned after retry")
	checkEqual(t, int32(2), calls.Load(), "one retry should be attempted")
	checkEqual(t, managedContactID, result.ID)
}

func TestClientGetManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedContactPath, r.URL.EscapedPath(), "request path should include escaped contact ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedContact(t.Context(), managedContactID)

	requireError(t, err, "API error should be returned")
	checkNil(t, result, "contact should be nil on API error")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusNotFound, apiErr.StatusCode)
}
