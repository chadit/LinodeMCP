package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListMembersByImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-05T10:09:09"
	members := []linode.ImageShareGroupMember{
		{TokenUUID: imageShareGroupTokenUUID, Status: oauthClientStatus, Label: "Engineering - Backend", Created: "2025-08-04T10:07:59", Updated: &updated},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcImagesSharegroups123Members {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups123Members)
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    members,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].TokenUUID != imageShareGroupTokenUUID {
		t.Errorf("result.Data[0].TokenUUID = %v, want %v", result.Data[0].TokenUUID, imageShareGroupTokenUUID)
	}

	if result.Data[0].Status != oauthClientStatus {
		t.Errorf("result.Data[0].Status = %v, want %v", result.Data[0].Status, oauthClientStatus)
	}

	if result.Data[0].Label != imageShareGroupMemberUpdateLabel {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, imageShareGroupMemberUpdateLabel)
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if result.Pages != 3 {
		t.Errorf("result.Pages = %v, want %v", result.Pages, 3)
	}

	if result.Results != 51 {
		t.Errorf("result.Results = %v, want %v", result.Results, 51)
	}
}

func TestClientListMembersByImageShareGroupError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListMembersByImageShareGroupRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.ImageShareGroupMember{{TokenUUID: imageShareGroupTokenUUID}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListMembersByImageShareGroup(t.Context(), 123, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if calls != int32(2) {
		t.Errorf("calls = %v, want %v", calls, int32(2))
	}
}
