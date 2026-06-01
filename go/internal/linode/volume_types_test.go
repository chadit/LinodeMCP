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

const (
	volumeTypesPath          = "/volumes/types"
	volumeTypeIDBlockStorage = "storage"
	volumeTypeLabelBlock     = "Block Storage"
)

func TestClientListVolumeTypesSuccess(t *testing.T) {
	t.Parallel()

	volumeTypes := []linode.VolumeType{{
		keyID:    volumeTypeIDBlockStorage,
		keyLabel: volumeTypeLabelBlock,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, volumeTypesPath, r.URL.Path, "request path should be /volumes/types")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListVolumeTypes(t.Context())

	require.NoError(t, err, "ListVolumeTypes should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, volumeTypeIDBlockStorage, got[0][keyID])
	assert.Equal(t, volumeTypeLabelBlock, got[0][keyLabel])
}

func TestClientListVolumeTypesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	volumeTypes := []linode.VolumeType{{keyID: volumeTypeIDBlockStorage, keyLabel: volumeTypeLabelBlock}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, volumeTypesPath, r.URL.Path, "request path should be /volumes/types")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListVolumeTypes(t.Context())

	require.NoError(t, err, "read-only ListVolumeTypes should retry transient failures")
	require.Len(t, got, 1)
	assert.Equal(t, volumeTypeIDBlockStorage, got[0][keyID])
	assert.Equal(t, int32(2), attempts.Load(), "transient read should be retried once")
}
