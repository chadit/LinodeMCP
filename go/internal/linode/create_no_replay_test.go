package linode_test

import (
	"context"
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
	noReplayRateLimitBody = `{"errors":[{"reason":"rate limited"}]}`
	noReplayLabel         = "no-replay-fixture"
	noReplayParentID      = 123
	noReplayChildID       = 456
)

// errorFrom keeps the created resource out of the table below, which only ever
// asks whether the call failed and how many requests it took to say so.
func errorFrom[T any](_ T, err error) error {
	return err
}

// createCall names a create entry point and the route its POST lands on. Clone
// and add routes count as creates here because they allocate a new
// server-assigned resource exactly the way the Create routes do.
type createCall struct {
	call  func(ctx context.Context, client *linode.Client) error
	name  string
	route string
}

// noReplayCreates enumerates the creates that must not be replayed. Every one
// of them answers a POST by assigning a new ID, or, for the manual snapshot, by
// overwriting the one the previous attempt took, so a second attempt after a
// transient failure changes the account in a way the caller never sees.
func noReplayCreates() []createCall {
	return append(noReplayInfraCreates(), noReplayInstanceCreates()...)
}

func noReplayInfraCreates() []createCall {
	return []createCall{
		{
			name:  "CreateSSHKeyProto",
			route: "/profile/sshkeys",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateSSHKeyProto(ctx, linode.CreateSSHKeyRequest{Label: noReplayLabel}))
			},
		},
		{
			name:  "CreateFirewallProto",
			route: "/networking/firewalls",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateFirewallProto(ctx, linode.CreateFirewallRequest{Label: noReplayLabel}))
			},
		},
		{
			name:  "CreateDomainRecordProto",
			route: "/domains/123/records",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateDomainRecordProto(ctx, noReplayParentID, &linode.CreateDomainRecordRequest{Type: "A"}))
			},
		},
		{
			name:  "CreateVolumeProto",
			route: "/volumes",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateVolumeProto(ctx, &linode.CreateVolumeRequest{Label: noReplayLabel, Region: regionUSEast}))
			},
		},
		{
			name:  "CreateObjectStorageKeyProto",
			route: "/object-storage/keys",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateObjectStorageKeyProto(ctx, linode.CreateObjectStorageKeyRequest{Label: noReplayLabel}))
			},
		},
		{
			name:  "CreateLKEClusterProto",
			route: "/lke/clusters",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateLKEClusterProto(ctx, &linode.CreateLKEClusterRequest{Label: noReplayLabel, Region: regionUSEast}))
			},
		},
		{
			name:  "CreateLKENodePoolProto",
			route: "/lke/clusters/123/pools",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateLKENodePoolProto(ctx, noReplayParentID, &linode.CreateLKENodePoolRequest{Count: 1}))
			},
		},
		{
			name:  "CreateVPCProto",
			route: "/vpcs",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateVPCProto(ctx, linode.CreateVPCRequest{Label: noReplayLabel, Region: regionUSEast}))
			},
		},
		{
			name:  "CreateVPCSubnetProto",
			route: "/vpcs/123/subnets",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateVPCSubnetProto(ctx, noReplayParentID, linode.CreateSubnetRequest{Label: noReplayLabel, IPv4: cidrV4}))
			},
		},
		{
			name:  "CreateFirewallDeviceProto",
			route: "/networking/firewalls/123/devices",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateFirewallDeviceProto(ctx, noReplayParentID, &linode.CreateFirewallDeviceRequest{ID: noReplayChildID, Type: "linode"}))
			},
		},
		{
			name:  "CreateMonitorServiceToken",
			route: "/monitor/services/linode/token",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateMonitorServiceToken(ctx, "linode", &linode.CreateMonitorServiceTokenRequest{EntityIDs: []int{noReplayParentID}}))
			},
		},
	}
}

func noReplayInstanceCreates() []createCall {
	return []createCall{
		{
			name:  "CreateInstanceProto",
			route: "/linode/instances",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateInstanceProto(ctx, &linode.CreateInstanceRequest{Region: regionUSEast, Type: tcNanode1GB}))
			},
		},
		{
			name:  "CreateInstanceConfigProto",
			route: tcLinodeInstances123Configs,
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateInstanceConfigProto(ctx, noReplayParentID, &linode.CreateConfigRequest{Label: labelBootConfig}))
			},
		},
		{
			name:  "CreateInstanceDiskProto",
			route: "/linode/instances/123/disks",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateInstanceDiskProto(ctx, noReplayParentID, &linode.CreateDiskRequest{Label: noReplayLabel, Size: 1024}))
			},
		},
		{
			name:  "CreateInstanceBackupProto",
			route: "/linode/instances/123/backups",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateInstanceBackupProto(ctx, noReplayParentID, noReplayLabel))
			},
		},
		{
			name:  "CloneInstanceProto",
			route: "/linode/instances/123/clone",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CloneInstanceProto(ctx, noReplayParentID, &linode.CloneInstanceRequest{Region: regionUSEast, Type: tcNanode1GB}))
			},
		},
		{
			name:  "CloneInstanceDiskProto",
			route: "/linode/instances/123/disks/456/clone",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CloneInstanceDiskProto(ctx, noReplayParentID, noReplayChildID))
			},
		},
		{
			name:  "AddInstanceConfigInterfaceProto",
			route: tcLinodeInstances123Configs456Interfaces,
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.AddInstanceConfigInterfaceProto(ctx, noReplayParentID, noReplayChildID, &linode.ConfigInterface{Purpose: purposePublic}))
			},
		},
	}
}

// retrySafeCreates enumerates the two POSTs named "create" that are still
// replayed. Neither can leave a second resource behind: the bucket is
// identified by the region and label the caller chose, and the presigned URL
// is a signature the API computes rather than an object it stores.
func retrySafeCreates() []createCall {
	return []createCall{
		{
			name:  "CreateObjectStorageBucketProto",
			route: "/object-storage/buckets",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreateObjectStorageBucketProto(ctx, linode.CreateObjectStorageBucketRequest{Label: noReplayLabel, Region: regionUSEast}))
			},
		},
		{
			name:  "CreatePresignedURLProto",
			route: "/object-storage/buckets/us-east/no-replay-fixture/object-url",
			call: func(ctx context.Context, client *linode.Client) error {
				return errorFrom(client.CreatePresignedURLProto(ctx, regionUSEast, noReplayLabel, linode.PresignedURLRequest{Method: http.MethodGet, Name: "object.txt"}))
			},
		},
	}
}

// newRateLimitedServer answers every request with 429 and reports how many
// requests reached it, which is what separates a single attempt from a
// replayed one.
//
// 429 rather than 500 because isRetryable already refuses to replay a 5xx on a
// POST, so a 5xx fixture would pass whether or not the call site carries the
// guard. A rate limit is the one transient failure the retry loop does replay
// through a POST, which makes it the case where the guard is the deciding
// factor.
func newRateLimitedServer(t *testing.T, route string, requests *atomic.Int32) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		if r.URL.Path != route {
			t.Errorf("request path = %q, want %q", r.URL.Path, route)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(noReplayRateLimitBody))
	}))
}

// TestClientCreateProtoNoReplayOnTransientFailure pins the no-replay property
// for every non-idempotent create: the rate limit surfaces after one attempt
// instead of being replayed into a duplicate resource. The follow-up call
// proves the single attempt still counted against the circuit breaker, so the
// guard bypasses the retry loop rather than the protection around it.
func TestClientCreateProtoNoReplayOnTransientFailure(t *testing.T) {
	t.Parallel()

	for _, tt := range noReplayCreates() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Int32

			srv := newRateLimitedServer(t, tt.route, &requests)
			defer srv.Close()

			cfg := &config.Config{
				Resilience: config.ResilienceConfig{
					CircuitBreakerThreshold: 1,
					CircuitBreakerTimeout:   time.Hour,
				},
			}
			client := linode.NewClient(srv.URL, domainCreateToken, cfg, fastRetryOpts()...)

			if err := tt.call(t.Context(), client); err == nil {
				t.Fatalf("%s() error = nil, want transient API error", tt.name)
			}

			if got := requests.Load(); got != 1 {
				t.Errorf("request count = %d, want 1", got)
			}

			if err := tt.call(t.Context(), client); !errors.Is(err, linode.ErrCircuitOpen) {
				t.Fatalf("%s() error = %v, want %v", tt.name, err, linode.ErrCircuitOpen)
			}

			if got := requests.Load(); got != 1 {
				t.Errorf("request count after open circuit = %d, want 1", got)
			}
		})
	}
}

// TestClientCreateProtoRetriesWhenReplayIsHarmless keeps the two deliberate
// exceptions from being swept into the guard by a later blanket conversion. If
// either ever starts allocating a server-assigned resource, this test is the
// thing that has to change first.
func TestClientCreateProtoRetriesWhenReplayIsHarmless(t *testing.T) {
	t.Parallel()

	const wantAttempts = 4 // fastRetryOpts allows 3 retries after the first try.

	for _, tt := range retrySafeCreates() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Int32

			srv := newRateLimitedServer(t, tt.route, &requests)
			defer srv.Close()

			client := linode.NewClient(srv.URL, domainCreateToken, nil, fastRetryOpts()...)

			if err := tt.call(t.Context(), client); err == nil {
				t.Fatalf("%s() error = nil, want transient API error", tt.name)
			}

			if got := requests.Load(); got != wantAttempts {
				t.Errorf("request count = %d, want %d", got, wantAttempts)
			}
		})
	}
}
