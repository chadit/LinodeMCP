package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListImagesByShareGroupTokenSuccess(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "private/123", Label: "shared-ubuntu", Type: "manual", Status: imageStatusAvailableFixture, Created: "2025-01-01T00:00:00", Size: 2500},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup/images")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].ID != privateImage123Fixture {
		t.Errorf("result.Data[0].ID = %v, want %v", result.Data[0].ID, privateImage123Fixture)
	}

	if result.Data[0].Label != tcSharedUbuntu {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tcSharedUbuntu)
	}
}

func TestClientListImagesByShareGroupTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup/images" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup/images")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImagesByShareGroupToken(t.Context(), "token/..?query#frag", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 0 {
		t.Errorf("result.Data = %v, want empty", result.Data)
	}
}

func TestClientListImagesByShareGroupTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/tokens/%2E%2E/sharegroup/images" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/tokens/%2E%2E/sharegroup/images")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListImagesByShareGroupToken(t.Context(), "..", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 0 {
		t.Errorf("result.Data = %v, want empty", result.Data)
	}
}

func TestClientListImagesByShareGroupTokenError(t *testing.T) {
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

	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListImagesByShareGroupTokenRetriesReadOnlyRoute(t *testing.T) {
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

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Image{{ID: "private/123"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListImagesByShareGroupToken(t.Context(), imageShareGroupTokenUUID, 0, 0)
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
