package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const cloneVolumeLabel = "data-copy"

func TestClientCloneVolumeSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/volumes/333/clone", r.URL.Path)

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, cloneVolumeLabel, body["label"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Volume{ID: 444, Label: cloneVolumeLabel, Region: managedServiceRegion, Status: statusPending}))
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	volume, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	require.NoError(t, err)
	require.NotNil(t, volume)
	assert.Equal(t, 444, volume.ID)
	assert.Equal(t, cloneVolumeLabel, volume.Label)
	assert.Equal(t, int32(1), requestCount.Load(), "clone must issue exactly one request")
}

func TestClientCloneVolumeDoesNotRetryPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/volumes/333/clone", r.URL.Path)
		http.Error(w, errTemporaryFailure, http.StatusBadGateway)
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	require.Error(t, err)
	assert.Equal(t, int32(1), requestCount.Load(), "clone POST must not be replayed after a transient failure")
}

func TestClientCloneVolumeAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/volumes/333/clone", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is already taken"}},
		}))
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.CloneVolume(t.Context(), 333, linode.CloneVolumeRequest{Label: cloneVolumeLabel})

	require.Error(t, err)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "label is already taken", apiErr.Message)
}
