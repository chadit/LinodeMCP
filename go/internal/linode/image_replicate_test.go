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

const regionUSMiami = "us-mia"

func TestClientReplicateImageSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != "/images/private%2F123/regions" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F123/regions")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body["regions"], []any{regionUSMiami, regionUSEast}) {
			t.Errorf("got %v, want %v", body["regions"], []any{regionUSMiami, regionUSEast})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: privateImage123Fixture, Label: "replicated-image"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	image, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSMiami, regionUSEast}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if image == nil {
		t.Fatal("image is nil")
	}

	if image.ID != privateImage123Fixture {
		t.Errorf("image.ID = %v, want %v", image.ID, privateImage123Fixture)
	}
}

func TestClientReplicateImageEscapesImageID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/images/private%2F123%3Fbad/regions" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F123%3Fbad/regions")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Image{ID: "private/123?bad"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), "private/123?bad", &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientReplicateImageAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errTemporaryFailure {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errTemporaryFailure)
	}
}

func TestClientReplicateImageNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if networkErr.Operation != "ReplicateImage" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "ReplicateImage")
	}
}

func TestClientReplicateImageDoesNotRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.ReplicateImage(t.Context(), privateImage123Fixture, &linode.ReplicateImageRequest{Regions: []string{regionUSEast}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
