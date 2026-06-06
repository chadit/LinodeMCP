package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, volumeTypesPath, r.URL.Path, "request path should be /volumes/types")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListVolumeTypes(t.Context())

	requireNoError(t, err, "ListVolumeTypes should succeed on 200 response")
	requireLenOne(t, got)
	checkEqual(t, volumeTypeIDBlockStorage, got[0][keyID])
	checkEqual(t, volumeTypeLabelBlock, got[0][keyLabel])
}

func TestClientListVolumeTypesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	volumeTypes := []linode.VolumeType{{keyID: volumeTypeIDBlockStorage, keyLabel: volumeTypeLabelBlock}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, volumeTypesPath, r.URL.Path, "request path should be /volumes/types")

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListVolumeTypes(t.Context())

	requireNoError(t, err, "read-only ListVolumeTypes should retry transient failures")
	requireLenOne(t, got)
	checkEqual(t, volumeTypeIDBlockStorage, got[0][keyID])
	checkEqual(t, int32(2), attempts.Load(), "transient read should be retried once")
}
