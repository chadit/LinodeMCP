package linode_test

import (
	"encoding/json"
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
	managedContactsPath      = "/managed/contacts"
	managedContactName       = "John Doe"
	managedContactEmail      = "john.doe@example.org"
	managedContactGroup      = "on-call"
	managedContactPhone      = "123-456-7890"
	managedContactUpdated    = "2018-01-01T00:01:01"
	managedContactsForbidden = "Forbidden"
)

func TestClientListManagedContactsSuccess(t *testing.T) {
	t.Parallel()

	group := managedContactGroup
	primaryPhone := managedContactPhone
	contacts := linode.PaginatedResponse[linode.ManagedContact]{
		Data: []linode.ManagedContact{{
			ID:      567,
			Name:    managedContactName,
			Email:   managedContactEmail,
			Group:   &group,
			Phone:   linode.ManagedContactPhone{Primary: &primaryPhone},
			Updated: managedContactUpdated,
		}},
		Page:    2,
		Pages:   3,
		Results: 51,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(contacts))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedContacts(t.Context(), 2, 25)

	require.NoError(t, err, "ListManagedContacts should succeed on 200 response")
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, managedContactName, result.Data[0].Name)
	assert.Equal(t, managedContactEmail, result.Data[0].Email)
	assert.Equal(t, managedContactGroup, *result.Data[0].Group)
	assert.Equal(t, managedContactPhone, *result.Data[0].Phone.Primary)
}

func TestClientListManagedContactsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedContact]{
			Data:    []linode.ManagedContact{{Name: managedContactName}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedContacts(t.Context(), 0, 0)

	require.NoError(t, err, "read-only Managed contacts list should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedContactsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedContacts(t.Context(), 0, 0)

	require.Error(t, err, "ListManagedContacts should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
