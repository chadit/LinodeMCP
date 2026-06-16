package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestResetInstanceDiskPasswordSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/linode/instances/123/disks/456/password" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/456/password")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if body["password"] != "Str0ngP@ssw0rd!" {
			t.Errorf("got %v, want %v", body["password"], "Str0ngP@ssw0rd!")
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil)

	err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetInstanceDiskPasswordDoesNotRetryTransientServerError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/linode/instances/123/disks/456/password" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/disks/456/password")
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestResetInstanceDiskPasswordOpenCircuitShortCircuitsWithoutUpstreamCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"down"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:              2,
			BaseRetryDelay:          time.Millisecond,
			MaxRetryDelay:           time.Millisecond,
			CircuitBreakerThreshold: 1,
			CircuitBreakerTimeout:   time.Hour,
		},
	}
	client := linode.NewClient(srv.URL, "test-token", cfg)

	err := client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}

	err = client.ResetInstanceDiskPassword(t.Context(), 123, 456, "Str0ngP@ssw0rd!")
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestResetInstanceDiskPasswordValidatesPathIds(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil)
	if err := client.ResetInstanceDiskPassword(t.Context(), 0, 456, "Str0ngP@ssw0rd!"); !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if err := client.ResetInstanceDiskPassword(t.Context(), 123, 0, "Str0ngP@ssw0rd!"); !errors.Is(err, linode.ErrDiskIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrDiskIDPositive)
	}
}
