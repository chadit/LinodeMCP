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
	kernelIDLatest = "linode/latest-64bit"
	kernelLabel    = "Latest 64 bit"
)

func TestClientListKernelsSuccess(t *testing.T) {
	t.Parallel()

	kernels := []linode.Kernel{
		{ID: kernelIDLatest, Label: kernelLabel, Version: "6.15.7", KVM: true, Architecture: "x86_64", PVOPS: true, Deprecated: false, Built: "2025-07-21T17:19:17"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/kernels" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/kernels")
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "25" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "25")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    kernels,
			keyPage:    2,
			keyPages:   14,
			keyResults: 338,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListKernels(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].ID != kernelIDLatest {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, kernelIDLatest)
	}

	if got[0].Label != kernelLabel {
		t.Errorf("got[0].Label = %v, want %v", got[0].Label, kernelLabel)
	}
}

func TestClientListKernelsRetriesTransientReadFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/kernels" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/kernels")
		}

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []linode.Kernel{{ID: kernelIDLatest, Label: kernelLabel}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.ListKernels(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}
