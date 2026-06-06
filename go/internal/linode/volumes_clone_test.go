package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		checkEqual(t, http.MethodPost, r.Method)
		checkEqual(t, "/volumes/333/clone", r.URL.Path)

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body))
		checkEqual(t, cloneVolumeLabel, body["label"])

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.Volume{ID: 444, Label: cloneVolumeLabel, Region: managedServiceRegion, Status: statusPending}))
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	volume, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	requireNoError(t, err)
	requireNotNil(t, volume)
	checkEqual(t, 444, volume.ID)
	checkEqual(t, cloneVolumeLabel, volume.Label)
	checkEqual(t, int32(1), requestCount.Load(), "clone must issue exactly one request")
}

func TestClientCloneVolumeDoesNotRetryPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method)
		checkEqual(t, "/volumes/333/clone", r.URL.Path)
		http.Error(w, errTemporaryFailure, http.StatusBadGateway)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	requireError(t, err)
	checkEqual(t, int32(1), requestCount.Load(), "clone POST must not be replayed after a transient failure")
}

func TestClientCloneVolumeAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method)
		checkEqual(t, "/volumes/333/clone", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is already taken"}},
		}))
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	requireError(t, err)

	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
	checkEqual(t, "label is already taken", apiErr.Message)
}
