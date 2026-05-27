package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientListVLANsRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/networking/vlans", r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=50", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyLabel:  "app-vlan",
				"region":  managedServiceRegion,
				"linodes": []int{123, 456},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	vlans, err := client.ListVLANs(t.Context(), 2, 50)

	require.NoError(t, err, "ListVLANs should succeed")
	require.NotNil(t, vlans, "response should not be nil")
	require.Len(t, vlans.Data, 1, "one VLAN should be returned")
	assert.Equal(t, "app-vlan", vlans.Data[0].Label, "label should match")
	assert.Equal(t, managedServiceRegion, vlans.Data[0].Region, "region should match")
	assert.Equal(t, []int{123, 456}, vlans.Data[0].Linodes, "linode IDs should match")
	assert.Equal(t, int32(1), requestCount.Load(), "ListVLANs should make one request")
}

func TestClientListVLANsRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)

		w.Header().Set("Content-Type", "application/json")

		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "temporary"}}}))

			return
		}

		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	require.NoError(t, err, "ListVLANs should retry a transient GET error")
	require.NotNil(t, vlans, "response should not be nil")
	require.Len(t, vlans.Data, 1, "one VLAN should be returned after retry")
	assert.Equal(t, "retry-vlan", vlans.Data[0].Label, "retried response should decode")
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should be retried once")
}
