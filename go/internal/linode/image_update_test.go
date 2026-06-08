package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const updatedImageLabelFixture = "Updated Debian 12"

func TestClientUpdateImageSuccess(t *testing.T) {
	t.Parallel()

	label := updatedImageLabelFixture
	description := "Updated image description"
	tags := []string{uploadImageTagProd, uploadImageTagWeb}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/images/private%2F12345" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F12345")
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

		if !reflect.DeepEqual(body["tags"], []any{uploadImageTagProd, uploadImageTagWeb}) {
			t.Errorf("got %v, want %v", body["tags"], []any{uploadImageTagProd, uploadImageTagWeb})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "private/12345", Label: label, Description: description, Tags: tags}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImage(t.Context(), "private/12345", &linode.UpdateImageRequest{
		Label:       &label,
		Description: &description,
		Tags:        &tags,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != "private/12345" {
		t.Errorf("result.ID = %v, want %v", result.ID, "private/12345")
	}

	if result.Label != label {
		t.Errorf("result.Label = %v, want %v", result.Label, label)
	}

	if !reflect.DeepEqual(result.Tags, tags) {
		t.Errorf("result.Tags = %v, want %v", result.Tags, tags)
	}
}

func TestClientUpdateImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/private%2F%2E%2E%3Fquery%23frag" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F%2E%2E%3Fquery%23frag")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "private/..?query#frag"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	label := updatedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImage(t.Context(), "private/..?query#frag", &linode.UpdateImageRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestClientUpdateImageRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImage(t.Context(), "private/12345", nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if !errors.Is(err, linode.ErrUpdateImageRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrUpdateImageRequestRequired)
	}

	if requestCount.Load() != int32(0) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
	}
}

func TestClientUpdateImageDoesNotRetryMutation(t *testing.T) {
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

	label := updatedImageLabelFixture

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(3))

	result, err := client.UpdateImage(t.Context(), "private/12345", &linode.UpdateImageRequest{Label: &label})
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
