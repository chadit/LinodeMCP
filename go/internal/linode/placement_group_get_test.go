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

const (
	keyPlacementGroupType   = "placement_group_type"
	keyPlacementGroupPolicy = "placement_group_policy"
	keyIsCompliant          = "is_compliant"
	keyLinodeID             = "linode_id"
	keyMembers              = "members"
	keyRegion               = "region"
	placementGroupTypeLocal = "anti_affinity:local"
	placementGroupPolicy    = "strict"
	regionUSMIA             = "us-mia"
)

func TestClientGetPlacementGroupRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/placement/groups/528", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"), "authorization header should match")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:                   528,
			keyLabel:                "PG_Miami_failover",
			keyRegion:               regionUSMIA,
			keyPlacementGroupType:   placementGroupTypeLocal,
			keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant:          true,
			keyMembers: []map[string]any{{
				keyLinodeID:    123,
				keyIsCompliant: true,
			}},
			"migrations": map[string]any{
				"inbound":  []map[string]any{{keyLinodeID: 456}},
				"outbound": []map[string]any{{keyLinodeID: 789}},
			},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	group, err := client.GetPlacementGroup(t.Context(), 528)

	require.NoError(t, err, "GetPlacementGroup should succeed")
	require.NotNil(t, group, "response should not be nil")
	assert.Equal(t, 528, group.ID, "group ID should match")
	assert.Equal(t, "PG_Miami_failover", group.Label, "label should match")
	assert.Equal(t, regionUSMIA, group.Region, "region should match")
	assert.Equal(t, placementGroupTypeLocal, group.PlacementGroupType, "type should match")
	assert.Equal(t, placementGroupPolicy, group.PlacementGroupPolicy, "policy should match")
	assert.True(t, group.IsCompliant, "group should be compliant")
	require.Len(t, group.Members, 1, "one member should be returned")
	assert.Equal(t, 123, group.Members[0].LinodeID, "member linode ID should match")
	require.NotNil(t, group.Migrations, "migrations should decode")
	require.Len(t, group.Migrations.Inbound, 1, "one inbound migration should decode")
	assert.Equal(t, 456, group.Migrations.Inbound[0].LinodeID, "inbound migration should match")
	assert.Equal(t, int32(1), requestCount.Load(), "GetPlacementGroup should make one request")
}

func TestClientGetPlacementGroupRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)

		w.Header().Set("Content-Type", "application/json")

		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))

			return
		}

		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 528, keyLabel: "retry-placement", keyRegion: regionUSMIA,
			keyPlacementGroupType: placementGroupTypeLocal, keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant: true, keyMembers: []map[string]any{},
		}), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)
	group, err := client.GetPlacementGroup(t.Context(), 528)

	require.NoError(t, err, "GetPlacementGroup should retry a transient GET error")
	require.NotNil(t, group, "response should not be nil")
	assert.Equal(t, "retry-placement", group.Label, "retried response should decode")
	assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should be retried once")
}
