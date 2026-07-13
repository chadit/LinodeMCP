package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	nodeBalancerCreatePath   = "/nodebalancers"
	nodeBalancerLabelFixture = "web-lb"
	reservedIPv4Fixture      = "192.0.2.141"
)

type nodeBalancerCreateCase struct {
	name     string
	ipv4     *string
	wantBody map[string]any
}

func TestClientCreateNodeBalancerProtoRequestAndResponse(t *testing.T) {
	t.Parallel()

	selectedIPv4 := reservedIPv4Fixture
	tests := []nodeBalancerCreateCase{
		{
			name: "selected reserved IPv4",
			ipv4: &selectedIPv4,
			wantBody: map[string]any{
				keyRegion: regionUSEast,
				keyIPv4:   reservedIPv4Fixture,
			},
		},
		{
			name:     "omitted IPv4",
			wantBody: map[string]any{keyRegion: regionUSEast},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runNodeBalancerCreateParityCase(t, tt)
		})
	}
}

func runNodeBalancerCreateParityCase(t *testing.T, tt nodeBalancerCreateCase) {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(nodeBalancerCreateHandler(t, tt.wantBody, &calls))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	req := linode.CreateNodeBalancerRequest{Region: regionUSEast, IPv4: tt.ipv4}

	protoResponse, err := client.CreateNodeBalancerProto(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if protoResponse.GetId() != 444 {
		t.Errorf("protoResponse.GetId() = %v, want %v", protoResponse.GetId(), 444)
	}

	if protoResponse.GetLabel() != nodeBalancerLabelFixture {
		t.Errorf("protoResponse.GetLabel() = %v, want %v", protoResponse.GetLabel(), nodeBalancerLabelFixture)
	}

	if protoResponse.GetRegion() != regionUSEast {
		t.Errorf("protoResponse.GetRegion() = %v, want %v", protoResponse.GetRegion(), regionUSEast)
	}

	if protoResponse.GetIpv4() != reservedIPv4Fixture {
		t.Errorf("protoResponse.GetIpv4() = %v, want %v", protoResponse.GetIpv4(), reservedIPv4Fixture)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func nodeBalancerCreateHandler(t *testing.T, wantBody map[string]any, calls *atomic.Int32) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != nodeBalancerCreatePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerCreatePath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body, wantBody) {
			t.Errorf("body = %v, want %v", body, wantBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 444, keyLabel: nodeBalancerLabelFixture, keyRegion: regionUSEast, keyIPv4: reservedIPv4Fixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestClientCreateNodeBalancerRejectsInvalidIPv4BeforeRequest(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	invalidIPv4 := "2001:db8::1"
	req := linode.CreateNodeBalancerRequest{Region: regionUSEast, IPv4: &invalidIPv4}

	if _, err := client.CreateNodeBalancerProto(t.Context(), req); !errors.Is(err, linode.ErrIPv4AddressInvalid) {
		t.Errorf("error = %v, want %v", err, linode.ErrIPv4AddressInvalid)
	}

	if calls.Load() != 0 {
		t.Errorf("calls.Load() = %v, want 0", calls.Load())
	}
}

func TestClientCreateNodeBalancerDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	if _, err := client.CreateNodeBalancerProto(t.Context(), linode.CreateNodeBalancerRequest{Region: regionUSEast}); err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
