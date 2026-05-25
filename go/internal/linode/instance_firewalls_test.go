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

const (
	instanceFirewallLabelFixture  = "web-firewall"
	instanceFirewallStatusEnabled = "enabled"
)

func TestClientListInstanceFirewalls(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: instanceFirewallLabelFixture, Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
		assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
		assert.Equal(t, "50", r.URL.Query().Get("page_size"), "page_size query should match")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}), "encoding response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 123, 2, 50)

	require.NoError(t, err, "list instance firewalls should not fail")
	require.Len(t, got, 1, "one firewall should be returned")
	assert.Equal(t, instanceFirewallLabelFixture, got[0].Label, "firewall label should match")
}

func TestClientListInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 0, 0, 0)

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive, "invalid linode ID should be rejected before request")
	assert.Nil(t, got, "no firewalls should be returned")
}

func TestClientListInstanceFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 123, 0, 0)

	require.Error(t, err, "HTTP error should be returned")
	assert.Nil(t, got, "no firewalls should be returned")
}

func TestClientUpdateInstanceFirewallsSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: "assigned-instance-firewall", Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")

		var body linode.UpdateInstanceFirewallsRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, []int{456, 789}, body.FirewallIDs, "request body should include firewall IDs")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
			Data:    firewalls,
			Page:    2,
			Pages:   4,
			Results: 1,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateInstanceFirewalls(t.Context(), 123, 2, 25, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456, 789}})

	require.NoError(t, err, "UpdateInstanceFirewalls should succeed on 200 response")
	require.Len(t, got, 1)
	assert.Equal(t, 456, got[0].ID)
}

func TestClientUpdateInstanceFirewallsAllowsEmptyAssignments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body linode.UpdateInstanceFirewallsRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Empty(t, body.FirewallIDs, "empty firewall_ids should be sent to remove assignments")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{Data: []linode.Firewall{}}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{}})

	require.NoError(t, err, "UpdateInstanceFirewalls should allow an empty firewall_ids list")
	assert.Empty(t, got)
}

func TestClientUpdateInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateInstanceFirewalls(t.Context(), -1, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456}})

	require.Error(t, err, "UpdateInstanceFirewalls should reject invalid linode IDs before request")
	assert.False(t, called.Load(), "invalid linode ID should not reach upstream server")
	assert.ErrorIs(t, err, linode.ErrLinodeIDPositive, "error should expose invalid linode ID sentinel")
}

func TestClientUpdateInstanceFirewallsRejectsNilRequest(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, nil)

	require.Error(t, err, "UpdateInstanceFirewalls should reject a nil request before request")
	assert.False(t, called.Load(), "nil request should not reach upstream server")
	assert.ErrorIs(t, err, linode.ErrUpdateInstanceFirewallsRequestRequired, "error should expose missing request sentinel")
}

func TestClientUpdateInstanceFirewallsDoesNotRetryTransientFailure(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}), "encoding error should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateInstanceFirewalls(t.Context(), 123, 0, 0, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: []int{456}})

	require.Error(t, err, "UpdateInstanceFirewalls should return the transient error")
	assert.Equal(t, int32(1), callCount.Load(), "state-changing PUT must not be replayed after a transient error")
}
