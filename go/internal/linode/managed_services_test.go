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
	managedServiceID         = 9944
	managedServicePath       = "/managed/services/9944"
	managedServiceLabel      = "prod-1"
	managedServiceType       = "url"
	managedServiceStatus     = "ok"
	managedServiceAddress    = "https://example.org"
	managedServiceCreated    = "2018-01-01T00:01:01"
	managedServiceUpdated    = "2018-03-01T00:01:01"
	managedServiceBody       = "it worked"
	managedServiceGroup      = "on-call"
	managedServiceNotes      = "The service name is my-cool-application"
	managedServiceRegion     = "us-east"
	managedServicesForbidden = "Forbidden"
)

func TestClientGetManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	body := managedServiceBody
	notes := managedServiceNotes
	region := managedServiceRegion
	want := linode.ManagedService{
		ID:                managedServiceID,
		Label:             managedServiceLabel,
		ServiceType:       managedServiceType,
		Status:            managedServiceStatus,
		Address:           managedServiceAddress,
		Body:              &body,
		ConsultationGroup: managedServiceGroup,
		Created:           managedServiceCreated,
		Credentials:       []int{9991},
		Notes:             &notes,
		Region:            &region,
		Timeout:           30,
		Updated:           managedServiceUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedServicePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServicePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedService(t.Context(), managedServiceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !reflect.DeepEqual(*result, want) {
		t.Errorf("result = %+v, want %+v", *result, want)
	}
}

func TestClientGetManagedServiceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedServicePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServicePath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedService{ID: managedServiceID}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.GetManagedService(t.Context(), managedServiceID)
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

func TestClientGetManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedServicePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedServicePath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetManagedService(t.Context(), managedServiceID)
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
