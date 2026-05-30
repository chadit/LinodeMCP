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

const nodeBalancerFirewallsPath = "/nodebalancers/123/firewalls"

func TestClientListNodeBalancerFirewallsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.NoBody, r.Body, "request should not send a body")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: "nb-firewall", keyStatus: statusEnabledFixture}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 456, got[0].ID)
	assert.Equal(t, "nb-firewall", got[0].Label)
	assert.Equal(t, statusEnabledFixture, got[0].Status)
}

func TestClientListNodeBalancerFirewallsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 0)

	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	assert.Nil(t, got)
}

func TestClientListNodeBalancerFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientUpdateNodeBalancerFirewallsSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: "assigned-nodebalancer-firewall", Status: statusEnabledFixture}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body linode.UpdateNodeBalancerFirewallsRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, []int{456, 789}, body.FirewallIDs, "request body should include firewall IDs")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 2, 25, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456, 789}})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 456, got[0].ID)
	assert.Equal(t, "assigned-nodebalancer-firewall", got[0].Label)
}

func TestClientUpdateNodeBalancerFirewallsAllowsEmptyAssignments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body linode.UpdateNodeBalancerFirewallsRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Empty(t, body.FirewallIDs, "empty firewall_ids should be sent to remove assignments")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Firewall{}, keyPage: 1, keyPages: 1, keyResults: 0}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{}})

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestClientUpdateNodeBalancerFirewallsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 0, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	assert.Nil(t, got)
	assert.False(t, called.Load(), "invalid NodeBalancer ID should not reach upstream server")
}

func TestClientUpdateNodeBalancerFirewallsRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, nil)

	require.ErrorIs(t, err, linode.ErrUpdateNodeBalancerFirewallsRequestRequired)
	assert.Nil(t, got)
	assert.False(t, called.Load(), "nil request should not reach upstream server")
}

func TestClientUpdateNodeBalancerFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, nodeBalancerFirewallsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientUpdateNodeBalancerFirewallsDoesNotRetryTransientFailure(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok, "response writer should support hijacking") {
			return
		}

		conn, _, err := hj.Hijack()
		if !assert.NoError(t, err) {
			return
		}

		assert.NoError(t, conn.Close())
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.UpdateNodeBalancerFirewalls(t.Context(), 123, 0, 0, &linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}})

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, int32(1), callCount.Load(), "state-changing PUT must not be replayed after a transient error")
}
