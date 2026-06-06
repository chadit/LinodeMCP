package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListInstanceInterfacesSuccess(t *testing.T) {
	t.Parallel()

	interfaces := []linode.InstanceInterface{
		{
			ID:         1234,
			MACAddress: macAddressFixture,
			Version:    1,
			Public: &linode.InterfacePublicConfig{
				IPv4: &linode.InterfacePublicIPv4{
					Addresses: []linode.InterfaceIPv4Address{{Address: "172.30.0.50", Primary: true}},
				},
			},
			DefaultRoute: &linode.InterfaceDefaultRoute{IPv4: true, IPv6: true},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{"interfaces": interfaces}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceInterfaces(t.Context(), 123)

	requireNoError(t, err, "ListInstanceInterfaces should succeed on 200 response")
	requireLenOne(t, got)
	checkEqual(t, 1234, got[0].ID)
	checkEqual(t, macAddressFixture, got[0].MACAddress)
	requireNotNil(t, got[0].Public)
	requireNotNil(t, got[0].DefaultRoute)
}

func TestClientGetInstanceInterfaceSuccess(t *testing.T) {
	t.Parallel()

	want := linode.InstanceInterface{ID: 456, MACAddress: "22:00:AB:CD:EF:02", Version: 1}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(want), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.GetInstanceInterface(t.Context(), 123, 456)

	requireNoError(t, err, "GetInstanceInterface should succeed on 200 response")
	requireNotNil(t, got)
	checkEqual(t, 456, got.ID)
	checkEqual(t, "22:00:AB:CD:EF:02", got.MACAddress)
}

func TestClientGetInstanceInterfaceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetInstanceInterface(t.Context(), 123, 456)

	requireError(t, err, "GetInstanceInterface should fail on API error")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientGetInstanceInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceInterface(t.Context(), 0, 456)
	requireError(t, err, "GetInstanceInterface should reject invalid Linode IDs before request")
	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid Linode ID sentinel")

	_, err = client.GetInstanceInterface(t.Context(), 123, 0)
	requireError(t, err, "GetInstanceInterface should reject invalid interface IDs before request")
	requireErrorIs(t, err, linode.ErrInterfaceIDPositive, "error should expose invalid interface ID sentinel")

	if called.Load() {
		t.Error("invalid IDs should not reach upstream server")
	}
}

func TestClientListInstanceInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceInterfaces(t.Context(), 123)

	requireError(t, err, "ListInstanceInterfaces should fail on API error")
	apiErr := requireAPIError(t, err)
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientListInstanceInterfacesRejectsInvalidID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListInstanceInterfaces(t.Context(), 0)

	requireError(t, err, "ListInstanceInterfaces should reject invalid IDs before request")

	if called.Load() {
		t.Error("invalid IDs should not reach upstream server")
	}

	requireErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid ID sentinel")
}
