package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListLKETierVersionsUsesTierPath(t *testing.T) {
	t.Parallel()

	versions := []linode.LKETierVersion{{ID: "1.33", Tier: lkeTierStandard}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want %q", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/lke/tiers/standard/versions" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/lke/tiers/standard/versions")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("request query = %q, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: versions, keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil)

	got, err := client.ListLKETierVersions(t.Context(), lkeTierStandard)
	if err != nil {
		t.Fatalf("ListLKETierVersions returned error: %v", err)
	}

	if len(got) != len(versions) {
		t.Fatalf("tier versions length = %d, want %d", len(got), len(versions))
	}

	if got[0] != versions[0] {
		t.Fatalf("tier version = %#v, want %#v", got[0], versions[0])
	}
}

func TestClientListLKETierVersionsEscapesTierPathSegment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lke/tiers/standard/enterprise/versions" {
			t.Errorf("decoded path = %q, want %q", r.URL.Path, "/lke/tiers/standard/enterprise/versions")
		}

		if got := r.URL.EscapedPath(); got != "/lke/tiers/standard%2Fenterprise/versions" {
			t.Errorf("escaped path = %q, want %q", got, "/lke/tiers/standard%2Fenterprise/versions")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.LKETierVersion{}, keyPage: 1, keyPages: 1, keyResults: 0,
		}); err != nil {
			t.Errorf("encoding response failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, lkeTestToken, nil)

	_, err := client.ListLKETierVersions(t.Context(), "standard/enterprise")
	if err != nil {
		t.Fatalf("ListLKETierVersions returned error: %v", err)
	}
}
