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
	kernelIDLatest = "linode/latest-64bit"
	kernelLabel    = "Latest 64 bit"
)

func TestClientListKernelsSuccess(t *testing.T) {
	t.Parallel()

	kernels := []linode.Kernel{
		{ID: kernelIDLatest, Label: kernelLabel, Version: "6.15.7", KVM: true, Architecture: "x86_64", PVOPS: true, Deprecated: false, Built: "2025-07-21T17:19:17"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/kernels", r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "25", r.URL.Query().Get("page_size"), "page_size query should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    kernels,
			keyPage:    2,
			keyPages:   14,
			keyResults: 338,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListKernels(t.Context(), 2, 25)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, kernelIDLatest, got[0].ID)
	assert.Equal(t, kernelLabel, got[0].Label)
}

func TestClientListKernelsRetriesTransientReadFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/kernels", r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.Kernel{{ID: kernelIDLatest, Label: kernelLabel}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListKernels(t.Context(), 0, 0)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
}
