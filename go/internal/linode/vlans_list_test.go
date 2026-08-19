package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientListVLANsRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/networking/vlans" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/vlans")
		}

		if r.URL.RawQuery != tcPage2PageSize50 {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, tcPage2PageSize50)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:  vlanLabelApp,
				"region":  managedServiceRegion,
				"linodes": []int{123, 456},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	vlans, err := client.ListVLANs(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vlans == nil {
		t.Fatal("vlans is nil")
	}

	if len(vlans.Data) != 1 {
		t.Fatalf("len(vlans.Data) = %d, want %d", len(vlans.Data), 1)
	}

	if vlans.Data[0].Label != vlanLabelApp {
		t.Errorf("vlans.Data[0].Label = %v, want %v", vlans.Data[0].Label, vlanLabelApp)
	}

	if vlans.Data[0].Region != managedServiceRegion {
		t.Errorf("vlans.Data[0].Region = %v, want %v", vlans.Data[0].Region, managedServiceRegion)
	}

	if !reflect.DeepEqual(vlans.Data[0].Linodes, []int{123, 456}) {
		t.Errorf("vlans.Data[0].Linodes = %v, want %v", vlans.Data[0].Linodes, []int{123, 456})
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientListVLANsRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyLabel: "retry-vlan", "region": regionUSEast, "linodes": []int{789}}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	vlans, err := client.ListVLANs(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vlans == nil {
		t.Fatal("vlans is nil")
	}

	if len(vlans.Data) != 1 {
		t.Fatalf("len(vlans.Data) = %d, want %d", len(vlans.Data), 1)
	}

	if vlans.Data[0].Label != "retry-vlan" {
		t.Errorf("vlans.Data[0].Label = %v, want %v", vlans.Data[0].Label, "retry-vlan")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
