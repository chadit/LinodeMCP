package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.AddInstanceInterfaceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.Public, "public interface body should be sent") && assert.NotNil(t, got.Public.IPv4, "public IPv4 should be sent") {
			assert.Equal(t, "auto", got.Public.IPv4.Addresses[0].Address, "IPv4 address should match")
		}

		if assert.NotNil(t, got.DefaultRoute, "default route should be sent") && assert.NotNil(t, got.DefaultRoute.IPv4, "IPv4 default route should be sent") {
			assert.True(t, *got.DefaultRoute.IPv4, "IPv4 default route should match")
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created), "encoding response should not fail")
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

	require.NoError(t, err, "AddInstanceInterface should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 1234, got.ID)
	require.NotNil(t, got.FirewallID)
	assert.Equal(t, firewallID, *got.FirewallID)
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
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)

	_, err = client.AddInstanceInterface(t.Context(), 123, nil)
	require.ErrorIs(t, err, linode.ErrAddInstanceInterfaceRequestRequired)

	assert.False(t, called.Load(), "invalid inputs should not reach upstream server")
}

func TestClientAddInstanceInterfaceDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.AddInstanceInterface(t.Context(), 123, &linode.AddInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})

	require.Error(t, err, "server error should be returned")
	assert.Equal(t, int32(1), calls.Load(), "POST create call should not be replayed after transient server error")
}
