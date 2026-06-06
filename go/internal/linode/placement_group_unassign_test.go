package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const temporaryPlacementGroupUnassignError = "temporary placement group unassign failure"

func TestClientUnassignPlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.PlacementGroupUnassignRequest{Linodes: []int{123, 456}}
	updated := linode.PlacementGroup{ID: 789, Label: placementGroupTestLabel, Region: managedServiceRegion, PlacementGroupType: placementGroupTypeAntiAffinityTest, PlacementGroupPolicy: placementGroupPolicyStrictTest, IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"), "values differ")

		var got linode.PlacementGroupUnassignRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "expected no error")
		checkEqual(t, request, &got, "values differ")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(updated), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, request)

	requireNoError(t, err, "UnassignPlacementGroup should succeed on 200 response")
	requireNotNil(t, got, "result should not be nil")
	checkEqual(t, updated.ID, got.ID, "values differ")
	checkEqual(t, updated.Label, got.Label, "values differ")
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

			requireErrorIs(t, err, linode.ErrPlacementGroupUnassignLinodesRequired, "expected matching error")
			checkNil(t, got, "expected nil")
		})
	}
}

func TestClientUnassignPlacementGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})

	requireError(t, err, "UnassignPlacementGroup should return API errors")
	checkNil(t, got, "expected nil")

	apiErr := requireAPIError(t, err, "UnassignPlacementGroup should return API errors")
	checkEqual(t, errForbidden, apiErr.Message, "API error reason should match")
}

func TestClientUnassignPlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/789/unassign", r.URL.Path, "request path should be /placement/groups/789/unassign")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPlacementGroupUnassignError}}}), "expected no error")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UnassignPlacementGroup(t.Context(), 789, &linode.PlacementGroupUnassignRequest{Linodes: []int{123}})

	requireError(t, err, "UnassignPlacementGroup should return the transient error")
	checkEqual(t, int32(1), requestCount.Load(), "placement group unassignment must not be retried")
}
