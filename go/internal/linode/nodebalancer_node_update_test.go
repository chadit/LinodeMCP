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

func TestClientUpdateNodeBalancerNodeSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should match")
		assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		assert.Equal(t, nodeLabelWeb1, body[keyLabel], "request body should include label")
		assert.Equal(t, nodeBalancerNodeAddress, body[keyAddress], "request body should include address")
		assert.Equal(t, nodeBalancerNodeModeAccept, body[keyMode], "request body should include mode")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:             789,
			keyLabel:          nodeLabelWeb1,
			keyAddress:        nodeBalancerNodeAddress,
			keyStatus:         nodeBalancerNodeStatusUP,
			keyMode:           nodeBalancerNodeModeAccept,
			keyNodeBalancerID: 123,
			keyConfigID:       456,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{
		Label:   nodeLabelWeb1,
		Address: nodeBalancerNodeAddress,
		Mode:    nodeBalancerNodeModeAccept,
	})

	require.NoError(t, err, "UpdateNodeBalancerNode should succeed")
	require.NotNil(t, got, "updated node should not be nil")
	assert.Equal(t, 789, got.ID, "node ID should match")
	assert.Equal(t, nodeLabelWeb1, got.Label, "node label should match")
}

func TestClientUpdateNodeBalancerNodeValidation(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateNodeBalancerNode(t.Context(), 0, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 0, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	require.ErrorIs(t, err, linode.ErrConfigIDPositive)

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 456, 0, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})
	require.ErrorIs(t, err, linode.ErrNodeIDPositive)

	_, err = client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, nil)
	require.ErrorIs(t, err, linode.ErrUpdateNodeBalancerNodeRequestRequired)
}

func TestClientUpdateNodeBalancerNodeAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errNotFound}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})

	require.Error(t, err, "UpdateNodeBalancerNode should propagate API errors")
}

func TestClientUpdateNodeBalancerNodeDoesNotRetryTransientFailures(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/nodebalancers/123/configs/456/nodes/789", r.URL.Path, "request path should match")
		callCount.Add(1)
		http.Error(w, errTemporaryFailure, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
	_, err := client.UpdateNodeBalancerNode(t.Context(), 123, 456, 789, &linode.UpdateNodeBalancerNodeRequest{Label: nodeLabelWeb1})

	require.Error(t, err, "transient failure should return an error")
	assert.Equal(t, int32(1), callCount.Load(), "mutating update should not be replayed")
}
