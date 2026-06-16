package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListProfileLoginsSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data: []linode.AccountLogin{{
			Datetime:   accountUserPasswordCreated,
			ID:         321,
			IP:         "203.0.113.20",
			Restricted: false,
			Status:     statusSuccessful,
			Username:   accountLoginUsername,
		}},
		Page:    2,
		Pages:   3,
		Results: 7,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if r.ContentLength != int64(0) {
			t.Errorf("r.ContentLength = %v, want %v", r.ContentLength, int64(0))
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(logins); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListProfileLogins(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if result.Pages != 3 {
		t.Errorf("result.Pages = %v, want %v", result.Pages, 3)
	}

	if result.Results != 7 {
		t.Errorf("result.Results = %v, want %v", result.Results, 7)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != 321 {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, 321)
	}

	if result.Data[0].Username != accountLoginUsername {
		t.Errorf("result.Data[0].Username = %v, want %v", result.Data[0].Username, accountLoginUsername)
	}

	if result.Data[0].IP != "203.0.113.20" {
		t.Errorf("result.Data[0].IP = %v, want %v", result.Data[0].IP, "203.0.113.20")
	}
}

func TestClientListProfileLoginsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListProfileLogins(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientListProfileLoginsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary profile login error"}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileLogins {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileLogins)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.AccountLogin]{
			Data: []linode.AccountLogin{{ID: 321, Username: accountLoginUsername}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListProfileLogins(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
