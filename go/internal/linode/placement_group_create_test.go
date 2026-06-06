package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	placementGroupTestLabel            = "pg-test"
	temporaryPlacementGroupCreateError = "temporary placement group create failure"
)

func TestClientCreatePlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreatePlacementGroupRequest{
		Label:                placementGroupTestLabel,
		Region:               managedServiceRegion,
		PlacementGroupType:   placementGroupTypeAntiAffinityTest,
		PlacementGroupPolicy: placementGroupPolicyStrictTest,
	}
	created := linode.PlacementGroup{ID: 123, Label: request.Label, Region: request.Region, PlacementGroupType: request.PlacementGroupType, PlacementGroupPolicy: request.PlacementGroupPolicy, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")

		var got linode.CreatePlacementGroupRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "expected no error")
		checkEqual(t, request, &got, "values differ")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(created), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreatePlacementGroup(t.Context(), request)

	requireNoError(t, err, "CreatePlacementGroup should succeed on 200 response")
	requireNotNil(t, got, "result should not be nil")
	checkEqual(t, created.ID, got.ID, "values differ")
	checkEqual(t, created.Label, got.Label, "values differ")
}

func TestClientCreatePlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupCreateError}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreatePlacementGroup(t.Context(), &linode.CreatePlacementGroupRequest{Label: "pg-test", Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest})

	requireError(t, err, "CreatePlacementGroup should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "placement group creation must not be retried")
}
