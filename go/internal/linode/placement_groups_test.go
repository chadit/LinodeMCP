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
	placementGroupTypeAntiAffinityTest = "anti_affinity:local"
	placementGroupPolicyStrictTest     = "strict"
	placementGroupUpdatedLabel         = "pg-renamed"
)

func TestClientListPlacementGroupsSuccess(t *testing.T) {
	t.Parallel()

	groups := []linode.PlacementGroup{
		{
			ID:                   123,
			Label:                "pg-east",
			Region:               regionUSEast,
			PlacementGroupType:   placementGroupTypeAntiAffinityTest,
			PlacementGroupPolicy: placementGroupPolicyStrictTest,
			IsCompliant:          true,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    groups,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListPlacementGroups(t.Context(), 2, 25)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "pg-east", result.Data[0].Label)
	assert.Equal(t, "us-east", result.Data[0].Region)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 3, result.Pages)
	assert.Equal(t, 51, result.Results)
}

func TestClientListPlacementGroupsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListPlacementGroups(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientListPlacementGroupsRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/placement/groups", r.URL.Path, "request path should be /placement/groups")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.PlacementGroup{{ID: 123, Label: "pg-east"}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListPlacementGroups(t.Context(), 0, 0)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), calls.Load(), "read-only placement group list should retry once")
}

func TestClientUpdatePlacementGroupSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.UpdatePlacementGroupRequest{Label: placementGroupUpdatedLabel}
	updated := linode.PlacementGroup{ID: 123, Label: placementGroupUpdatedLabel, Region: regionUSEast, PlacementGroupType: "anti_affinity:local", PlacementGroupPolicy: "strict", IsCompliant: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/placement/groups/123", r.URL.Path, "request path should include placement group ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, placementGroupUpdatedLabel, body["label"])

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(updated))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdatePlacementGroup(t.Context(), 123, request)

	require.NoError(t, err, "UpdatePlacementGroup should succeed on 200 response")
	require.NotNil(t, got, "result should not be nil")
	assert.Equal(t, updated.ID, got.ID)
	assert.Equal(t, updated.Label, got.Label)
}

func TestClientUpdatePlacementGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/placement/groups/123", r.URL.Path, "request path should include placement group ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdatePlacementGroup(t.Context(), 123, &linode.UpdatePlacementGroupRequest{Label: placementGroupUpdatedLabel})

	require.Error(t, err, "UpdatePlacementGroup should propagate API errors")
	assert.Nil(t, got)
	assert.ErrorContains(t, err, errForbidden)
}

func TestClientUpdatePlacementGroupDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/placement/groups/123", r.URL.Path, "request path should include placement group ID")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.UpdatePlacementGroup(t.Context(), 123, &linode.UpdatePlacementGroupRequest{Label: placementGroupUpdatedLabel})

	require.Error(t, err, "UpdatePlacementGroup should return the transient error")
	assert.Equal(t, int32(1), requestCount.Load(), "mutating placement group update must not be retried")
}
