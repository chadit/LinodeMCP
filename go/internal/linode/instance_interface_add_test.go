package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAddInstanceInterfaceSuccess(t *testing.T) {
	t.Parallel()

	primary := true
	firewallID := 321
	created := linode.InstanceInterface{
		ID: 1234,
		Public: &linode.InterfacePublicConfig{
			IPv4: &linode.InterfacePublicIPv4{
				Addresses: []linode.InterfaceIPv4Address{{Address: "auto", Primary: true}},
			},
		},
		FirewallID: &firewallID,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		var got linode.AddInstanceInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		switch {
		case got.Public == nil:
			t.Error("public interface body should be sent")
		case got.Public.IPv4 == nil:
			t.Error("public IPv4 should be sent")
		default:
			if got.Public.IPv4.Addresses[0].Address != tcAuto {
				t.Errorf("got.Public.IPv4.Addresses[0].Address = %v, want %v", got.Public.IPv4.Addresses[0].Address, tcAuto)
			}
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

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.AddInstanceInterface(t.Context(), 123, &linode.AddInstanceInterfaceRequest{
		Public: &linode.InterfacePublicConfig{
			IPv4: &linode.InterfacePublicIPv4{
				Addresses: []linode.InterfaceIPv4Address{{Address: "auto", Primary: true}},
			},
		},
		DefaultRoute: &linode.AddInterfaceDefaultRoute{IPv4: &primary},
		FirewallID:   &firewallID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 1234 {
		t.Errorf("got.ID = %v, want %v", got.ID, 1234)
	}

	if got.FirewallID == nil {
		t.Fatal("got.FirewallID is nil")
	}

	if *got.FirewallID != firewallID {
		t.Errorf("*got.FirewallID = %v, want %v", *got.FirewallID, firewallID)
	}
}

func TestClientAddInstanceInterfaceRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.AddInstanceInterface(t.Context(), 0, &linode.AddInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.AddInstanceInterface(t.Context(), 123, nil)
	if !errors.Is(err, linode.ErrAddInstanceInterfaceRequestRequired) {
		t.Fatalf("error = %v, want %v", err, linode.ErrAddInstanceInterfaceRequestRequired)
	}

	if called.Load() {
		t.Error("invalid inputs should not reach upstream server")
	}
}

func TestClientAddInstanceInterfaceDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.AddInstanceInterface(t.Context(), 123, &linode.AddInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
