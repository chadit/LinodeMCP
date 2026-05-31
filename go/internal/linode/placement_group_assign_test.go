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

func TestClientAssignPlacementGroupLinodesRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")

		var body map[string][]int

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, decodeErr, "request body should decode")
		assert.Equal(t, []int{123, 456}, body["linodes"], "request body should include linodes")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:                   528,
			keyLabel:                "PG_Miami_failover",
			keyRegion:               regionUSMIA,
			keyPlacementGroupType:   placementGroupTypeLocal,
			keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant:          true,
			keyMembers: []map[string]any{
				{keyLinodeID: 123, keyIsCompliant: true},
				{keyLinodeID: 456, keyIsCompliant: true},
			},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123, 456}})

	require.NoError(t, err, "AssignPlacementGroupLinodes should succeed")
	require.NotNil(t, group, "response should not be nil")
	assert.Equal(t, 528, group.ID, "group ID should match")
	require.Len(t, group.Members, 2, "assigned members should decode")
	assert.Equal(t, 123, group.Members[0].LinodeID, "first member should match")
	assert.Equal(t, int32(1), requestCount.Load(), "assignment should make one request")
}

func TestClientAssignPlacementGroupLinodesDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)
	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123}})

	require.Error(t, err, "transient POST error should be returned")
	assert.Nil(t, group, "failed assignment should not return a group")
	assert.Equal(t, int32(1), requestCount.Load(), "state-changing POST must not be retried")
}

func TestClientAssignPlacementGroupLinodesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123}})

	require.Error(t, err, "API error should be returned")
	assert.Nil(t, group, "failed assignment should not return a group")
	assert.ErrorContains(t, err, errForbidden)
}
