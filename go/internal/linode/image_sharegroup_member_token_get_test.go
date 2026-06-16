package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetImageShareGroupMemberTokenSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-05T10:09:09"
	member := linode.ImageShareGroupMember{
		TokenUUID: shareGroupTokenUUIDFixture,
		Status:    oauthClientStatus,
		Label:     "Engineering - Backend",
		Created:   imageShareGroupTokenCreated,
		Updated:   &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(member); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, shareGroupTokenUUIDFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.TokenUUID != shareGroupTokenUUIDFixture {
		t.Errorf("result.TokenUUID = %v, want %v", result.TokenUUID, shareGroupTokenUUIDFixture)
	}

	if result.Label != imageShareGroupMemberUpdateLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupMemberUpdateLabel)
	}
}

func TestClientGetImageShareGroupMemberTokenEscapesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/123/members/token%2F..%3Fquery%23frag" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/123/members/token%2F..%3Fquery%23frag")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: "token/..?query#frag"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, tcTokenQueryFrag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientGetImageShareGroupMemberTokenRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/members/"+shareGroupTokenUUIDFixture)
		}

		if requestCount.Add(1) == 1 {
			http.Error(w, errTemporaryFailure, http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: shareGroupTokenUUIDFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.GetImageShareGroupMemberToken(t.Context(), 123, shareGroupTokenUUIDFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
