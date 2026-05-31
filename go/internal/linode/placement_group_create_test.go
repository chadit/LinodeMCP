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

const temporaryPlacementGroupCreateError = "temporary placement group create failure"

func TestClientCreatePlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreatePlacementGroupRequest{
		Label:                "pg-test",
		Region:               managedServiceRegion,
		PlacementGroupType:   placementGroupTypeAntiAffinityTest,
		PlacementGroupPolicy: placementGroupPolicyStrictTest,
	}
	created := linode.PlacementGroup{ID: 123, Label: request.Label, Region: request.Region, PlacementGroupType: request.PlacementGroupType, PlacementGroupPolicy: request.PlacementGroupPolicy, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreatePlacementGroupRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, request, &got)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(created))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreatePlacementGroup(t.Context(), request)

	require.NoError(t, err, "CreatePlacementGroup should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Label, got.Label)
}

func TestClientCreatePlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupCreateError}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreatePlacementGroup(t.Context(), &linode.CreatePlacementGroupRequest{Label: "pg-test", Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest})

	require.Error(t, err, "CreatePlacementGroup should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "placement group creation must not be retried")
}
