package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The check* helpers used here are package-local, stdlib-backed test helpers
// from image_assertions_test.go. check* reports and continues for HTTP
// handlers; require* stops the current test goroutine after client calls.

func TestClientGetInstanceConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:    float64(456),
			keyLabel: "boot-config",
			"kernel": configKernelLatest,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Label != labelBootConfig {
		t.Errorf("got.Label = %v, want %v", got.Label, labelBootConfig)
	}

	if got.Kernel != configKernelLatest {
		t.Errorf("got.Kernel = %v, want %v", got.Kernel, configKernelLatest)
	}
}

func TestClientGetInstanceConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456)
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceConfig(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientGetInstanceConfigRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		if !false {
			t.Error("false = false, want true")
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	t.Run("invalid linode id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 0, 456)

		if !errors.Is(err, linode.ErrLinodeIDPositive) {
			t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
		}
	})

	t.Run("invalid config id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetInstanceConfig(t.Context(), 123, 0)

		if !errors.Is(err, linode.ErrConfigIDPositive) {
			t.Fatalf("error = %v, want %v", err, linode.ErrConfigIDPositive)
		}
	})
}

func TestClientGetInstanceConfigRetriesTransientRead(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)

		if r.URL.Path != tcLinodeInstances123Configs456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Configs456)
		}

		if attempts.Load() == 1 {
			w.WriteHeader(http.StatusTooManyRequests)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: "rate limited"}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{"label": "boot-config"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	got, err := client.GetInstanceConfig(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Label != labelBootConfig {
		t.Errorf("got.Label = %v, want %v", got.Label, labelBootConfig)
	}

	if attempts.Load() != int32(2) {
		t.Errorf("attempts.Load() = %v, want %v", attempts.Load(), int32(2))
	}
}
