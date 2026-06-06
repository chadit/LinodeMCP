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

func TestClientAssignPlacementGroupLinodesRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request query should be empty")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")

		var body map[string][]int

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		checkNoError(t, decodeErr, "request body should decode")
		checkEqual(t, []int{123, 456}, body["linodes"], "request body should include linodes")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
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

	requireNoError(t, err, "AssignPlacementGroupLinodes should succeed")
	requireNotNil(t, group, "response should not be nil")
	checkEqual(t, 528, group.ID, "group ID should match")

	if len(group.Members) != 2 {
		t.Fatalf("length differs: got %d, want %d", len(group.Members), 2)
	}

	checkEqual(t, 123, group.Members[0].LinodeID, "first member should match")
	checkEqual(t, int32(1), requestCount.Load(), "assignment should make one request")
}

func TestClientAssignPlacementGroupLinodesDoesNotRetryTransientPost(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}), "expected no error")
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

	requireError(t, err, "transient POST error should be returned")
	checkNil(t, group, "failed assignment should not return a group")
	checkEqual(t, int32(1), requestCount.Load(), "state-changing POST must not be retried")
}

func TestClientAssignPlacementGroupLinodesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/placement/groups/528/assign", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "expected no error")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	group, err := client.AssignPlacementGroupLinodes(t.Context(), 528, &linode.AssignPlacementGroupLinodesRequest{Linodes: []int{123}})

	requireError(t, err, "API error should be returned")
	checkNil(t, group, "failed assignment should not return a group")

	apiErr := requireAPIError(t, err, "API error should be returned")
	checkEqual(t, errForbidden, apiErr.Message, "API error reason should match")
}
