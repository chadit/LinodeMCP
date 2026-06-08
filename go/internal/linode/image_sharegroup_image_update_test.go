package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const updatedSharedImageLabelFixture = "Updated Shared Debian"

func TestClientUpdateImageShareGroupImageSuccess(t *testing.T) {
	t.Parallel()

	label := updatedSharedImageLabelFixture
	description := "Updated shared image description"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/images/sharegroups/123/images/shared%2F1" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/123/images/shared%2F1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["label"], label) {
			t.Errorf("got %v, want %v", body["label"], label)
		}

		if !reflect.DeepEqual(body["description"], description) {
			t.Errorf("got %v, want %v", body["description"], description)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "shared/1", Label: label, Description: description}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/1", &linode.UpdateImageShareGroupImageRequest{
		Label:       &label,
		Description: &description,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != "shared/1" {
		t.Errorf("result.ID = %v, want %v", result.ID, "shared/1")
	}

	if result.Label != label {
		t.Errorf("result.Label = %v, want %v", result.Label, label)
	}
}

func TestClientUpdateImageShareGroupImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/sharegroups/123/images/shared%2F%2E%2E%3Fquery%23frag" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/123/images/shared%2F%2E%2E%3Fquery%23frag")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "shared/..?query#frag"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	label := updatedSharedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/..?query#frag", &linode.UpdateImageShareGroupImageRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientUpdateImageShareGroupImageDoesNotRetryMutation(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "try later"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	label := updatedSharedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	result, err := client.UpdateImageShareGroupImage(t.Context(), 123, "shared/1", &linode.UpdateImageShareGroupImageRequest{Label: &label})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
