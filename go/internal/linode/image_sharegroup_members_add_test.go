package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAddImageShareGroupMembersSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcImagesSharegroups123Members {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcImagesSharegroups123Members)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if body["label"] != memberLabelFixture {
			t.Errorf("got %v, want %v", body["label"], memberLabelFixture)
		}

		if body["token"] != memberTokenFixture {
			t.Errorf("got %v, want %v", body["token"], memberTokenFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shareGroup == nil {
		t.Fatal("shareGroup is nil")
	}

	if shareGroup.ID != 123 {
		t.Errorf("shareGroup.ID = %v, want %v", shareGroup.ID, 123)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAddImageShareGroupMembersEscapesShareGroupID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != tcImagesSharegroups123Members {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), tcImagesSharegroups123Members)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{ID: 123, Label: imageShareGroupLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientAddImageShareGroupMembersError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if shareGroup != nil {
		t.Errorf("shareGroup = %v, want nil", shareGroup)
	}
}

func TestClientAddImageShareGroupMembersDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	shareGroup, err := client.AddImageShareGroupMembers(t.Context(), 123, &linode.AddImageShareGroupMembersRequest{Label: memberLabelFixture, Token: memberTokenFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if shareGroup != nil {
		t.Errorf("shareGroup = %v, want nil", shareGroup)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
