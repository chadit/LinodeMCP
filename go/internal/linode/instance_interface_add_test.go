package linode_test

import (
	"encoding/json"
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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.AddInstanceInterfaceRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		switch {
		case got.Public == nil:
			t.Error("public interface body should be sent")
		case got.Public.IPv4 == nil:
			t.Error("public IPv4 should be sent")
		default:
			checkEqual(t, "auto", got.Public.IPv4.Addresses[0].Address, "IPv4 address should match")
		}

		switch {
		case got.DefaultRoute == nil:
			t.Error("default route should be sent")
		case got.DefaultRoute.IPv4 == nil:
			t.Error("IPv4 default route should be sent")
		case !*got.DefaultRoute.IPv4:
			t.Error("IPv4 default route should match")
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(created), "encoding response should not fail")
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

	requireNoError(t, err, "AddInstanceInterface should succeed on 200 response")
	requireNotNil(t, got)
	checkEqual(t, 1234, got.ID)
	requireNotNil(t, got.FirewallID)
	checkEqual(t, firewallID, *got.FirewallID)
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
	requireErrorIs(t, err, linode.ErrLinodeIDPositive)

	_, err = client.AddInstanceInterface(t.Context(), 123, nil)
	requireErrorIs(t, err, linode.ErrAddInstanceInterfaceRequestRequired)

	if called.Load() {
		t.Error("invalid inputs should not reach upstream server")
	}
}

func TestClientAddInstanceInterfaceDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.AddInstanceInterface(t.Context(), 123, &linode.AddInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})

	requireError(t, err, "server error should be returned")
	checkEqual(t, int32(1), calls.Load(), "POST create call should not be replayed after transient server error")
}
