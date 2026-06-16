package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
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
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != volumeTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, volumeTypesPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListVolumeTypes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if !reflect.DeepEqual(got[0][keyID], volumeTypeIDBlockStorage) {
		t.Errorf("got[0][keyID] = %v, want %v", got[0][keyID], volumeTypeIDBlockStorage)
	}

	if !reflect.DeepEqual(got[0][keyLabel], volumeTypeLabelBlock) {
		t.Errorf("got[0][keyLabel] = %v, want %v", got[0][keyLabel], volumeTypeLabelBlock)
	}
}

func TestClientListVolumeTypesRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	volumeTypes := []linode.VolumeType{{keyID: volumeTypeIDBlockStorage, keyLabel: volumeTypeLabelBlock}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != volumeTypesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, volumeTypesPath)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListVolumeTypes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if !reflect.DeepEqual(got[0][keyID], volumeTypeIDBlockStorage) {
		t.Errorf("got[0][keyID] = %v, want %v", got[0][keyID], volumeTypeIDBlockStorage)
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}
