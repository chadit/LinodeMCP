package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestClientDeleteManagedContactSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteManagedContact(t.Context(), 567)

	requireNoError(t, err, "DeleteManagedContact should succeed on 200 response")
}

func TestClientDeleteManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteManagedContact(t.Context(), 567)

	requireError(t, err, "DeleteManagedContact should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientDeleteManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteManagedContact(t.Context(), 567)

	requireError(t, err, "DeleteManagedContact should fail when the server is unreachable")
	netErr := requireNetworkError(t, err, "error should be a NetworkError")
	checkEqual(t, "DeleteManagedContact", netErr.Operation)
}

func TestClientDeleteManagedContactDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	err := client.DeleteManagedContact(t.Context(), 567)

	requireError(t, err, "DeleteManagedContact should return the transient failure")
	checkEqual(t, int32(1), calls.Load(), "destructive DELETE should not be retried")
}

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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(contacts))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedContacts(t.Context(), 2, 25)

	requireNoError(t, err, "ListManagedContacts should succeed on 200 response")
	requireNotNil(t, result)
	requireLenOne(t, result.Data)
	checkEqual(t, managedContactName, result.Data[0].Name)
	checkEqual(t, managedContactEmail, result.Data[0].Email)
	checkEqual(t, managedContactGroup, *result.Data[0].Group)
	checkEqual(t, managedContactPhone, *result.Data[0].Phone.Primary)
}

func TestClientListManagedContactsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedContact]{
			Data:    []linode.ManagedContact{{Name: managedContactName}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedContacts(t.Context(), 0, 0)

	requireNoError(t, err, "read-only Managed contacts list should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedContactsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedContacts(t.Context(), 0, 0)

	requireError(t, err, "ListManagedContacts should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientCreateManagedContactSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, managedContactAuthHeader, r.Header.Get("Authorization"))

		var got linode.CreateManagedContactRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))

		if got.Name == nil || got.Email == nil || got.Group == nil || got.Phone == nil || got.Phone.Primary == nil {
			t.Errorf("request body missing managed contact fields: %#v", got)

			return
		}

		checkEqual(t, managedContactName, *got.Name)
		checkEqual(t, managedContactEmail, *got.Email)
		checkEqual(t, managedContactGroup, *got.Group)
		checkEqual(t, managedContactPhone, *got.Phone.Primary)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedContact{
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

	requireNoError(t, err, "CreateManagedContact should succeed on 200 response")
	requireNotNil(t, contact)
	checkEqual(t, 567, contact.ID)
	checkEqual(t, managedContactName, contact.Name)
	checkEqual(t, managedContactEmail, contact.Email)
	checkEqual(t, managedContactUpdated, contact.Updated)
}

func TestClientCreateManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	requireError(t, err, "CreateManagedContact should fail when the server is unreachable")
	netErr := requireNetworkError(t, err, "error should be a NetworkError")
	checkEqual(t, "CreateManagedContact", netErr.Operation)
}

func TestClientCreateManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed contact could not be created"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	requireError(t, err, "CreateManagedContact should fail on 400 response")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientCreateManagedContactDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedContactsPath, r.URL.Path, "request path should be /managed/contacts")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})

	requireError(t, err, "CreateManagedContact should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CreateManagedContact must not retry and replay a mutating request")
}

func TestClientUpdateManagedContactSuccess(t *testing.T) {
	t.Parallel()

	group := managedContactGroup
	primaryPhone := managedContactPhone
	updatedContact := linode.ManagedContact{
		ID:      567,
		Name:    managedContactName,
		Email:   managedContactEmail,
		Group:   &group,
		Phone:   linode.ManagedContactPhone{Primary: &primaryPhone},
		Updated: managedContactUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.UpdateManagedContactRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if checkNotNil(t, got.Name) {
			checkEqual(t, managedContactName, *got.Name)
		}

		if checkNotNil(t, got.Email) {
			checkEqual(t, managedContactEmail, *got.Email)
		}

		if checkNotNil(t, got.Group) {
			checkEqual(t, managedContactGroup, *got.Group)
		}

		if checkNotNil(t, got.Phone) && checkNotNil(t, got.Phone.Primary) {
			checkEqual(t, managedContactPhone, *got.Phone.Primary)
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(updatedContact))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{
		Name:  &updatedContact.Name,
		Email: &updatedContact.Email,
		Group: updatedContact.Group,
		Phone: &linode.UpdateManagedContactPhone{Primary: &primaryPhone},
	})

	requireNoError(t, err, "UpdateManagedContact should succeed on 200 response")
	requireNotNil(t, result)
	checkEqual(t, managedContactName, result.Name)
	checkEqual(t, managedContactEmail, result.Email)
}

func TestClientUpdateManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	name := managedContactName
	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})

	requireError(t, err, "UpdateManagedContact should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientUpdateManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	name := managedContactName
	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})

	requireError(t, err, "UpdateManagedContact should fail when the server is unreachable")
	netErr := requireNetworkError(t, err, "error should be a NetworkError")
	checkEqual(t, "UpdateManagedContact", netErr.Operation)
}

func TestClientUpdateManagedContactDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, managedContactsPath+"/567", r.URL.Path, "request path should include contact ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	t.Cleanup(srv.Close)

	name := managedContactName
	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})

	requireError(t, err, "mutating Managed contact update should not retry transient failures")
	checkEqual(t, int32(1), calls.Load(), "client should call update exactly once")
}
