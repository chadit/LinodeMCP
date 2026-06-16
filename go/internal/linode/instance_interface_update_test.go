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

func TestClientUpdateInstanceInterfaceSuccess(t *testing.T) {
	t.Parallel()

	defaultRoute := true
	updated := linode.InstanceInterface{
		ID:           456,
		VPC:          &linode.InterfaceVPCConfig{SubnetID: 789},
		DefaultRoute: &linode.InterfaceDefaultRoute{IPv4: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		var got linode.UpdateInstanceInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if got.VPC == nil {
			t.Error("vpc interface body should be sent")

			return
		}

		if got.VPC.SubnetID != 789 {
			t.Errorf("got.VPC.SubnetID = %v, want %v", got.VPC.SubnetID, 789)
		}

		switch {
		case got.DefaultRoute == nil:
			t.Error("default route should be sent")
		case got.DefaultRoute.IPv4 == nil:
			t.Error("IPv4 default route should be sent")
		case !*got.DefaultRoute.IPv4:
			t.Error("IPv4 default route should match")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateInstanceInterface(t.Context(), 123, 456, &linode.UpdateInstanceInterfaceRequest{
		VPC:          &linode.UpdateInstanceInterfaceVPCConfig{SubnetID: 789},
		DefaultRoute: &linode.AddInterfaceDefaultRoute{IPv4: &defaultRoute},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.VPC == nil {
		t.Fatal("got.VPC is nil")
	}

	if got.VPC.SubnetID != 789 {
		t.Errorf("got.VPC.SubnetID = %v, want %v", got.VPC.SubnetID, 789)
	}
}

func TestClientUpdateInstanceInterfaceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateInstanceInterface(t.Context(), 0, 456, &linode.UpdateInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.UpdateInstanceInterface(t.Context(), 123, 0, &linode.UpdateInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	_, err = client.UpdateInstanceInterface(t.Context(), 123, 456, nil)
	if !errors.Is(err, linode.ErrUpdateInstanceInterfaceRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrUpdateInstanceInterfaceRequestRequired)
	}

	if called.Load() {
		t.Error("invalid inputs should not reach upstream server")
	}
}

func TestClientUpdateInstanceInterfaceDoesNotReplayTransientPut(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateInstanceInterface(t.Context(), 123, 456, &linode.UpdateInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
