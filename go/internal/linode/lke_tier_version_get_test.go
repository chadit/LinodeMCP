package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientGetLKETierVersionSuccess(t *testing.T) {
	t.Parallel()

	tierVersion := linode.LKETierVersion{ID: lkeTierVersion131, Tier: lkeTierStandard}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/lke/tiers/standard/versions/1.31" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/tiers/standard/versions/1.31")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("request query = %q, want empty", r.URL.RawQuery)
		}

		if got := r.Header.Get("Authorization"); got != lkeAuthHeader {
			t.Errorf("Authorization header = %q, want %q", got, lkeAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(tierVersion); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(0))

	result, err := client.GetLKETierVersion(t.Context(), lkeTierStandard, lkeTierVersion131)
	if err != nil {
		t.Fatalf("GetLKETierVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKETierVersion result is nil")
	}

	if result.ID != lkeTierVersion131 {
		t.Fatalf("result ID = %q, want %q", result.ID, lkeTierVersion131)
	}

	if result.Tier != lkeTierStandard {
		t.Fatalf("result tier = %q, want %q", result.Tier, lkeTierStandard)
	}
}

func TestClientGetLKETierVersionEscapesPathParameters(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/lke/tiers/standard%2Ftier/versions/1.31%3Fbad=1" {
			t.Errorf("escaped path = %q, want %q", got, "/lke/tiers/standard%2Ftier/versions/1.31%3Fbad=1")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.LKETierVersion{ID: "1.31?bad=1", Tier: "standard/tier"}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(0))

	result, err := client.GetLKETierVersion(t.Context(), "standard/tier", "1.31?bad=1")
	if err != nil {
		t.Fatalf("GetLKETierVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKETierVersion result is nil")
	}
}

func TestClientGetLKETierVersionRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("encoding error response failed: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.LKETierVersion{ID: lkeTierVersion131, Tier: lkeTierStandard}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, lkeTestToken, nil, linode.WithMaxRetries(1))

	result, err := client.GetLKETierVersion(t.Context(), lkeTierStandard, lkeTierVersion131)
	if err != nil {
		t.Fatalf("GetLKETierVersion returned error: %v", err)
	}

	if result == nil {
		t.Fatal("GetLKETierVersion result is nil")
	}

	if calls != 2 {
		t.Fatalf("read-only GET calls = %d, want 2", calls)
	}
}
