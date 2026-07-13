package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	passwordlessImage        = "linode/ubuntu24.04"
	passwordlessUserFixture  = "alice"
	passwordlessInstanceType = "g6-nanode-1"
)

type provisioningCallResult struct {
	failed      bool
	circuitOpen bool
}

func TestProvisioningMutationsDoNotReplayTransientFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*linode.Client) provisioningCallResult
	}{
		{
			name: "CreateInstanceProto",
			call: func(client *linode.Client) provisioningCallResult {
				_, err := client.CreateInstanceProto(t.Context(), &linode.CreateInstanceRequest{Region: managedServiceRegion, Type: passwordlessInstanceType})

				return provisioningCallResult{failed: err != nil, circuitOpen: errors.Is(err, linode.ErrCircuitOpen)}
			},
		},
		{
			name: "RebuildInstanceProto",
			call: func(client *linode.Client) provisioningCallResult {
				_, err := client.RebuildInstanceProto(t.Context(), 123, &linode.RebuildInstanceRequest{Image: passwordlessImage, AuthorizedUsers: []string{passwordlessUserFixture}})

				return provisioningCallResult{failed: err != nil, circuitOpen: errors.Is(err, linode.ErrCircuitOpen)}
			},
		},
		{
			name: "CreateInstanceDiskProto",
			call: func(client *linode.Client) provisioningCallResult {
				_, err := client.CreateInstanceDiskProto(t.Context(), 123, &linode.CreateDiskRequest{Label: "root", Size: 2048, AuthorizedUsers: []string{passwordlessUserFixture}})

				return provisioningCallResult{failed: err != nil, circuitOpen: errors.Is(err, linode.ErrCircuitOpen)}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				if _, err := w.Write([]byte(`{"errors":[{"reason":"transient"}]}`)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Resilience: config.ResilienceConfig{
				CircuitBreakerThreshold: 1,
				CircuitBreakerTimeout:   time.Hour,
			}}
			client := linode.NewClient(srv.URL, "test-token", cfg, fastRetryOpts()...)

			first := test.call(client)
			if !first.failed || first.circuitOpen {
				t.Fatalf("first call = %+v, want transient non-circuit error", first)
			}

			if requests.Load() != 1 {
				t.Errorf("requests = %d, want 1", requests.Load())
			}

			second := test.call(client)
			if !second.circuitOpen {
				t.Fatalf("second call = %+v, want open-circuit error", second)
			}

			if requests.Load() != 1 {
				t.Errorf("requests after open circuit = %d, want 1", requests.Load())
			}
		})
	}
}

func TestRebuildInstanceProtoDoesNotRetryRateLimit(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"rate limited"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.RebuildInstanceProto(t.Context(), 123, &linode.RebuildInstanceRequest{
		Image:           passwordlessImage,
		AuthorizedUsers: []string{passwordlessUserFixture},
	})
	if err == nil {
		t.Fatal("expected rate-limit error, got nil")
	}

	if requests.Load() != 1 {
		t.Errorf("requests = %d, want 1", requests.Load())
	}
}
