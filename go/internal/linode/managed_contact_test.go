package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedContactPath, r.URL.Path, "request path should include encoded contact ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "reading request body should not fail")
		assert.Empty(t, body, "request body should be empty")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(contact))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedContact(t.Context(), managedContactID)

	require.NoError(t, err, "GetManagedContact should succeed")
	require.NotNil(t, result, "contact should be returned")
	assert.Equal(t, managedContactID, result.ID)
	assert.Equal(t, managedContactGetName, result.Name)
	assert.Equal(t, managedContactGetEmail, result.Email)
	require.NotNil(t, result.Phone.Primary)
	assert.Equal(t, managedContactGetPhone, *result.Phone.Primary)
}

func TestClientGetManagedContactRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	contact := linode.ManagedContact{ID: managedContactID, Name: managedContactGetName, Email: managedContactGetEmail}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedContactPath, r.URL.Path, "request path should include contact ID")

		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "temporary failure"}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(contact))
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

	require.NoError(t, err, "GetManagedContact should retry transient failures")
	require.NotNil(t, result, "contact should be returned after retry")
	assert.Equal(t, int32(2), calls.Load(), "one retry should be attempted")
	assert.Equal(t, managedContactID, result.ID)
}

func TestClientGetManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedContactPath, r.URL.EscapedPath(), "request path should include escaped contact ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedContact(t.Context(), managedContactID)

	require.Error(t, err, "API error should be returned")
	assert.Nil(t, result, "contact should be nil on API error")
	assert.ErrorContains(t, err, "not found")
}
