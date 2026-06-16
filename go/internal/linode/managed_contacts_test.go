package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	err := client.DeleteManagedContact(t.Context(), 567)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	err := client.DeleteManagedContact(t.Context(), 567)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientDeleteManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))

	err := client.DeleteManagedContact(t.Context(), 567)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &netErr)
	}

	if netErr.Operation != "DeleteManagedContact" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "DeleteManagedContact")
	}
}

func TestClientDeleteManagedContactDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	err := client.DeleteManagedContact(t.Context(), 567)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(contacts); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedContacts(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].Name != managedContactName {
		t.Errorf("result.Data[0].Name = %v, want %v", result.Data[0].Name, managedContactName)
	}

	if result.Data[0].Email != managedContactEmail {
		t.Errorf("result.Data[0].Email = %v, want %v", result.Data[0].Email, managedContactEmail)
	}

	if *result.Data[0].Group != managedContactGroup {
		t.Errorf("*result.Data[0].Group = %v, want %v", *result.Data[0].Group, managedContactGroup)
	}

	if *result.Data[0].Phone.Primary != managedContactPhone {
		t.Errorf("*result.Data[0].Phone.Primary = %v, want %v", *result.Data[0].Phone.Primary, managedContactPhone)
	}
}

func TestClientListManagedContactsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedContact]{
			Data:    []linode.ManagedContact{{Name: managedContactName}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.ListManagedContacts(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}

func TestClientListManagedContactsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListManagedContacts(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientCreateManagedContactSuccess(t *testing.T) {
	t.Parallel()

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
	wantContact := linode.ManagedContact{
		ID:      567,
		Name:    managedContactName,
		Email:   managedContactEmail,
		Group:   &contactGroup,
		Phone:   linode.ManagedContactPhone{Primary: &contactPhone},
		Updated: managedContactUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreateManagedContactRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, *req) {
			t.Errorf("got = %+v, want %+v", got, *req)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(wantContact); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contact, err := client.CreateManagedContact(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if contact == nil {
		t.Fatal("contact is nil")
	}

	if !reflect.DeepEqual(*contact, wantContact) {
		t.Errorf("contact = %+v, want %+v", *contact, wantContact)
	}
}

func TestClientCreateManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Errorf("error = %v, want %v", err, &netErr)
	}
}

func TestClientCreateManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"managed contact could not be created"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}
}

func TestClientCreateManagedContactDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != managedContactsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	contactName := managedContactName

	_, err := client.CreateManagedContact(t.Context(), &linode.CreateManagedContactRequest{Name: &contactName})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
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
	wantReq := linode.UpdateManagedContactRequest{
		Name:  &updatedContact.Name,
		Email: &updatedContact.Email,
		Group: updatedContact.Group,
		Phone: &linode.UpdateManagedContactPhone{Primary: &primaryPhone},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		var got linode.UpdateManagedContactRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, wantReq) {
			t.Errorf("got = %+v, want %+v", got, wantReq)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updatedContact); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateManagedContact(t.Context(), 567, wantReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, updatedContact) {
		t.Errorf("result = %+v, want %+v", *result, updatedContact)
	}
}

func TestClientUpdateManagedContactAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedContactsForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	name := managedContactName
	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientUpdateManagedContactNetworkError(t *testing.T) {
	t.Parallel()

	name := managedContactName
	client := linode.NewClient("http://127.0.0.1:1", "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &netErr)
	}

	if netErr.Operation != "UpdateManagedContact" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "UpdateManagedContact")
	}
}

func TestClientUpdateManagedContactDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != managedContactsPath+"/567" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedContactsPath+"/567")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	name := managedContactName
	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	_, err := client.UpdateManagedContact(t.Context(), 567, linode.UpdateManagedContactRequest{Name: &name})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
