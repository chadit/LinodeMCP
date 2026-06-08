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

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedIssuesPath      = "/managed/issues"
	managedIssueCreated    = "2018-01-01T00:01:01"
	managedIssuePath       = "/managed/issues/823"
	managedIssueLabel      = "Managed Issue opened!"
	managedIssueEntityType = "ticket"
	managedIssueEntityURL  = "/support/tickets/98765"
	managedIssuesForbidden = "Forbidden"
	managedIssueAuthHeader = "Bearer token"
)

func TestClientGetManagedIssueSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedIssue{
			ID:       823,
			Created:  managedIssueCreated,
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssueLabel,
				Type:  managedIssueEntityType,
				URL:   managedIssueEntityURL,
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.GetManagedIssue(t.Context(), 823)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 823 {
		t.Errorf("result.ID = %v, want %v", result.ID, 823)
	}

	if result.Created != managedIssueCreated {
		t.Errorf("result.Created = %v, want %v", result.Created, managedIssueCreated)
	}

	if !reflect.DeepEqual(result.Services, []int{654}) {
		t.Errorf("result.Services = %v, want %v", result.Services, []int{654})
	}

	if result.Entity.ID != 98765 {
		t.Errorf("result.Entity.ID = %v, want %v", result.Entity.ID, 98765)
	}

	if result.Entity.Label != managedIssueLabel {
		t.Errorf("result.Entity.Label = %v, want %v", result.Entity.Label, managedIssueLabel)
	}

	if result.Entity.Type != managedIssueEntityType {
		t.Errorf("result.Entity.Type = %v, want %v", result.Entity.Type, managedIssueEntityType)
	}

	if result.Entity.URL != managedIssueEntityURL {
		t.Errorf("result.Entity.URL = %v, want %v", result.Entity.URL, managedIssueEntityURL)
	}
}

func TestClientGetManagedIssueRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedIssuePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuePath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ManagedIssue{ID: 823}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.GetManagedIssue(t.Context(), 823)
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

func TestClientGetManagedIssueAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuePath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetManagedIssue(t.Context(), 823)
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

func TestClientListManagedIssuesSuccess(t *testing.T) {
	t.Parallel()

	issues := linode.PaginatedResponse[linode.ManagedIssue]{
		Data: []linode.ManagedIssue{{
			ID:       823,
			Created:  managedIssueCreated,
			Services: []int{654},
			Entity: linode.ManagedIssueEntity{
				ID:    98765,
				Label: managedIssueLabel,
				Type:  managedIssueEntityType,
				URL:   managedIssueEntityURL,
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 51,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(issues); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	result, err := client.ListManagedIssues(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want %d", len(result.Data), 1)
	}

	if result.Data[0].ID != 823 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 823)
	}

	if result.Data[0].Created != managedIssueCreated {
		t.Errorf("result.Data[0].Created = %v, want %v", result.Data[0].Created, managedIssueCreated)
	}

	if !reflect.DeepEqual(result.Data[0].Services, []int{654}) {
		t.Errorf("result.Data[0].Services = %v, want %v", result.Data[0].Services, []int{654})
	}

	if result.Data[0].Entity.ID != 98765 {
		t.Errorf("result.Data[0].Entity.ID = %v, want %v", result.Data[0].Entity.ID, 98765)
	}

	if result.Data[0].Entity.Label != managedIssueLabel {
		t.Errorf("result.Data[0].Entity.Label = %v, want %v", result.Data[0].Entity.Label, managedIssueLabel)
	}

	if result.Data[0].Entity.Type != managedIssueEntityType {
		t.Errorf("result.Data[0].Entity.Type = %v, want %v", result.Data[0].Entity.Type, managedIssueEntityType)
	}

	if result.Data[0].Entity.URL != managedIssueEntityURL {
		t.Errorf("result.Data[0].Entity.URL = %v, want %v", result.Data[0].Entity.URL, managedIssueEntityURL)
	}
}

func TestClientListManagedIssuesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != managedIssuesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuesPath)
		}

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedIssue]{
			Data:    []linode.ManagedIssue{{ID: 823}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))

	result, err := client.ListManagedIssues(t.Context(), 0, 0)
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

func TestClientListManagedIssuesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != managedIssuesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, managedIssuesPath)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedIssuesForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListManagedIssues(t.Context(), 0, 0)
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
