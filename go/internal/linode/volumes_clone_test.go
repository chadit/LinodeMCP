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

const cloneVolumeLabel = "data-copy"

func TestClientCloneVolumeSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcVolumes333Clone {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333Clone)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["label"], cloneVolumeLabel) {
			t.Errorf("got %v, want %v", body["label"], cloneVolumeLabel)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Volume{ID: 444, Label: cloneVolumeLabel, Region: managedServiceRegion, Status: statusPending}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))

	volume, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if volume == nil {
		t.Fatal("volume is nil")
	}

	if volume.ID != 444 {
		t.Errorf("volume.ID = %v, want %v", volume.ID, 444)
	}

	if volume.Label != cloneVolumeLabel {
		t.Errorf("volume.Label = %v, want %v", volume.Label, cloneVolumeLabel)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientCloneVolumeDoesNotRetryPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcVolumes333Clone {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333Clone)
		}

		http.Error(w, errTemporaryFailure, http.StatusBadGateway)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientCloneVolumeAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcVolumes333Clone {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVolumes333Clone)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is already taken"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusBadRequest)
	}

	if apiErr.Message != "label is already taken" {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, "label is already taken")
	}
}
