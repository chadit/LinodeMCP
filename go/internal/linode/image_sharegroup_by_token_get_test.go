package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientGetImageShareGroupByTokenSuccess(t *testing.T) {
	t.Parallel()

	description := imageShareGroupDescription
	updated := imageShareGroupUpdated
	shareGroup := linode.ImageShareGroup{
		ID:          1,
		UUID:        imageShareGroupUUID,
		Label:       imageShareGroupLabel,
		Description: &description,
		IsSuspended: false,
		Created:     imageShareGroupCreated,
		Updated:     &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens/"+imageShareGroupTokenUUID+"/sharegroup")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(shareGroup); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupByToken(t.Context(), imageShareGroupTokenUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.UUID != imageShareGroupUUID {
		t.Errorf("result.UUID = %v, want %v", result.UUID, imageShareGroupUUID)
	}

	if result.Label != imageShareGroupLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupLabel)
	}
}

func TestClientGetImageShareGroupByTokenEscapesPathSeparators(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/tokens/token%2F..%3Fquery%23frag/sharegroup")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{UUID: imageShareGroupUUID, Label: imageShareGroupLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupByToken(t.Context(), tcTokenQueryFrag)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Label != imageShareGroupLabel {
		t.Errorf("result.Label = %v, want %v", result.Label, imageShareGroupLabel)
	}
}

func TestClientGetImageShareGroupByTokenEscapesStandaloneTraversalMarker(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/tokens/%2E%2E/sharegroup" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/tokens/%2E%2E/sharegroup")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{UUID: imageShareGroupUUID, Label: imageShareGroupLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupByToken(t.Context(), pathTraversalDotDot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.UUID != imageShareGroupUUID {
		t.Errorf("result.UUID = %v, want %v", result.UUID, imageShareGroupUUID)
	}
}

func TestClientGetImageShareGroupByTokenError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "not found"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetImageShareGroupByToken(t.Context(), imageShareGroupTokenUUID)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}
