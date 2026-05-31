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

const temporaryPlacementGroupUnassignError = "temporary placement group unassign failure"

func TestClientUnassignPlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.PlacementGroupUnassignRequest{Linodes: []int{123, 456}}
	updated := linode.PlacementGroup{ID: 789, Label: placementGroupTestLabel, Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.PlacementGroupUnassignRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, request, &got)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updated))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, request)

	require.NoError(t, err, "UnassignPlacementGroup should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, updated.ID, got.ID)
	assert.Equal(t, updated.Label, got.Label)
}

func TestClientUnassignPlacementGroupRequiresLinodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *linode.PlacementGroupUnassignRequest
	}{
		{name: "nil request", req: nil},
		{name: "nil linodes", req: &linode.PlacementGroupUnassignRequest{}},
		{name: "empty linodes", req: &linode.PlacementGroupUnassignRequest{Linodes: []int{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(0))

			got, err := client.UnassignPlacementGroup(t.Context(), 789, tt.req)

			require.ErrorIs(t, err, linode.ErrPlacementGroupUnassignLinodesRequired)
			assert.Nil(t, got)
		})
	}
}

func TestClientUnassignPlacementGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})

	require.Error(t, err, "UnassignPlacementGroup should return API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientUnassignPlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupUnassignError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})

	require.Error(t, err, "UnassignPlacementGroup should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "placement group unassignment must not be retried")
}
