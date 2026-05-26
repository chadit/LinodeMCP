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
	managedContactsPath       = "/managed/contacts"
	managedContactName        = "John Doe"
	managedContactEmail       = "john.doe@example.org"
	managedContactGroup       = "on-call"
	managedContactPhone       = "123-456-7890"
	managedContactUpdated     = "2018-01-01T00:01:01"
	managedContactsForbidden  = "Forbidden"
	managedContactAuthHeader  = "Bearer my-token"
	managedContactCreateError = "managed contact could not be created"
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

func TestClientCreateManagedContactSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, managedContactAuthHeader, r.Header.Get("Authorization"))

		var got linode.CreateManagedContactRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		if got.Name == nil || got.Email == nil || got.Group == nil || got.Phone == nil || got.Phone.Primary == nil {
			t.Errorf("request body missing managed contact fields: %#v", got)

			return
		}

		assert.Equal(t, managedContactName, *got.Name)
		assert.Equal(t, managedContactEmail, *got.Email)
		assert.Equal(t, managedContactGroup, *got.Group)
		assert.Equal(t, managedContactPhone, *got.Phone.Primary)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ManagedContact{
			ID:      567,
			Name:    managedContactName,
			Email:   managedContactEmail,
			Group:   got.Group,
			Phone:   linode.ManagedContactPhone{Primary: got.Phone.Primary},
			Updated: managedContactUpdated,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	contactName := managedContactName
	contactEmail := managedContactEmail
	contactGroup := managedContactGroup
	contactPhone := managedContactPhone
	req := &linode.CreateManagedContactRequest{
		Name:  &contactName,
		Email: &contactEmail,
		Group: &contactGroup,
		Phone: &linode.CreateManagedContactPhoneRequest{Primary: &contactPhone},
	}

	contact, err := client.CreateManagedContact(t.Context(), req)

	require.NoError(t, err, "CreateManagedContact should succeed on 200 response")
	require.NotNil(t, contact)
	assert.Equal(t, 567, contact.ID)
	assert.Equal(t, managedContactName, contact.Name)
	assert.Equal(t, managedContactEmail, contact.Email)
	assert.Equal(t, managedContactUpdated, contact.Updated)
}

func TestClientCreateManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	require.Error(t, err, "CreateManagedContact should fail when the server is unreachable")

	var netErr *linode.NetworkError
	assert.ErrorAs(t, err, &netErr, "error should be a NetworkError")
}

func TestClientCreateManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed contact could not be created"}]}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	require.Error(t, err, "CreateManagedContact should fail on 400 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientCreateManagedContactDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	require.Error(t, err, "CreateManagedContact should fail on 500 response")
	assert.Equal(t, int32(1), calls.Load(), "CreateManagedContact must not retry and replay a mutating request")
}
