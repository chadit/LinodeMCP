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

func TestClientUpdateInstanceInterfaceSuccess(t *testing.T) {
	t.Parallel()

	defaultRoute := true
	updated := linode.InstanceInterface{
		ID:           456,
		VPC:          &linode.InterfaceVPCConfig{SubnetID: 789},
		DefaultRoute: &linode.InterfaceDefaultRoute{IPv4: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.UpdateInstanceInterfaceRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.VPC, "vpc interface body should be sent") {
			assert.Equal(t, 789, got.VPC.SubnetID, "subnet id should match")
		}

		if assert.NotNil(t, got.DefaultRoute, "default route should be sent") && assert.NotNil(t, got.DefaultRoute.IPv4, "IPv4 default route should be sent") {
			assert.True(t, *got.DefaultRoute.IPv4, "IPv4 default route should match")
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updated), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateInstanceInterface(t.Context(), 123, 456, &linode.UpdateInstanceInterfaceRequest{
		VPC:          &linode.UpdateInstanceInterfaceVPCConfig{SubnetID: 789},
		DefaultRoute: &linode.AddInterfaceDefaultRoute{IPv4: &defaultRoute},
	})

	require.NoError(t, err, "UpdateInstanceInterface should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, 456, got.ID)
	require.NotNil(t, got.VPC)
	assert.Equal(t, 789, got.VPC.SubnetID)
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
	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)

	_, err = client.UpdateInstanceInterface(t.Context(), 123, 0, &linode.UpdateInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})
	require.ErrorIs(t, err, linode.ErrInterfaceIDPositive)

	_, err = client.UpdateInstanceInterface(t.Context(), 123, 456, nil)
	require.ErrorIs(t, err, linode.ErrUpdateInstanceInterfaceRequestRequired)

	assert.False(t, called.Load(), "invalid inputs should not reach upstream server")
}

func TestClientUpdateInstanceInterfaceDoesNotReplayTransientPut(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.UpdateInstanceInterface(t.Context(), 123, 456, &linode.UpdateInstanceInterfaceRequest{Public: &linode.InterfacePublicConfig{}})

	require.Error(t, err, "server error should be returned")
	assert.Equal(t, int32(1), calls.Load(), "PUT update call should not be replayed after transient server error")
}
