package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListVLANsRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		stdCheckEqual(t, http.MethodGet, r.Method, "request method should be GET")
		stdCheckEqual(t, "/networking/vlans", r.URL.Path, "request path should match")
		stdCheckEqual(t, "page=2&page_size=50", r.URL.RawQuery, "request query should include pagination")
		stdCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")

		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:  vlanLabelApp,
				"region":  managedServiceRegion,
				"linodes": []int{123, 456},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	vlans, err := client.ListVLANs(t.Context(), 2, 50)

	stdMustNoError(t, err, "ListVLANs should succeed")
	stdMustNotNil(t, vlans, "response should not be nil")
	stdMustLen(t, vlans.Data, 1, "one VLAN should be returned")
	stdCheckEqual(t, vlanLabelApp, vlans.Data[0].Label, "label should match")
	stdCheckEqual(t, managedServiceRegion, vlans.Data[0].Region, "region should match")
	stdCheckEqual(t, []int{123, 456}, vlans.Data[0].Linodes, "linode IDs should match")
	stdCheckEqual(t, int32(1), requestCount.Load(), "ListVLANs should make one request")
}

func TestClientListVLANsRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)

		w.Header().Set("Content-Type", "application/json")

		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyLabel: "retry-vlan", "region": regionUSEast, "linodes": []int{789}}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
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

	stdMustNoError(t, err, "ListVLANs should retry a transient GET error")
	stdMustNotNil(t, vlans, "response should not be nil")
	stdMustLen(t, vlans.Data, 1, "one VLAN should be returned after retry")
	stdCheckEqual(t, "retry-vlan", vlans.Data[0].Label, "retried response should decode")
	stdCheckEqual(t, int32(2), requestCount.Load(), "read-only GET should be retried once")
}

func TestClientDeleteVLANRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		stdCheckEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		stdCheckEqual(t, "/networking/vlans/us-east/app-vlan", r.URL.Path, "request path should match")
		stdCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		stdCheckEqual(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteVLAN(t.Context(), "us-east", vlanLabelApp)

	stdMustNoError(t, err, "DeleteVLAN should succeed")
	stdCheckEqual(t, int32(1), requestCount.Load(), "DeleteVLAN should make one request")
}

func TestClientDeleteVLANURLEncodesPathParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stdCheckEqual(t, "/networking/vlans/us%2Feast/app%2Fvlan%3Fprod", r.URL.EscapedPath(), "path params should be escaped")
		w.Header().Set("Content-Type", "application/json")
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	err := client.DeleteVLAN(t.Context(), "us/east", "app/vlan?prod")

	stdMustNoError(t, err, "DeleteVLAN should URL-encode path params")
}

func TestClientDeleteVLANDoesNotRetryTransientDELETE(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		stdCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
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
	err := client.DeleteVLAN(t.Context(), "us-east", vlanLabelApp)

	stdMustError(t, err, "DeleteVLAN should return the transient error")
	stdCheckEqual(t, int32(1), requestCount.Load(), "destructive DELETE route must not retry transient failures")
}

func TestClientDeleteVLANRejectsEmptyPathParams(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteVLAN(t.Context(), "", vlanLabelApp)
	stdMustErrorIs(t, err, linode.ErrRegionIDRequired, "empty region should be rejected")

	err = client.DeleteVLAN(t.Context(), regionUSEast, "")
	stdMustErrorIs(t, err, linode.ErrLabelRequired, "empty label should be rejected")
}
